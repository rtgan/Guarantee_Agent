package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"guarantee-agent/internal/agent"
	"guarantee-agent/internal/config"
	"guarantee-agent/internal/env"
	"guarantee-agent/internal/logging"
	"guarantee-agent/internal/markdown"
	"guarantee-agent/internal/specs"
)

// RunOptions 描述 `autoqa run` 命令的输入。
type RunOptions struct {
	Path     string // 要运行的 Markdown 文件或目录
	BaseURL  string // 目标基址 URL(命令行;回退到 env/config)
	EnvName  string // 选择 .env.<name>
	Headless bool   // 无头模式(为真实浏览器实现预留)
	Debug    bool   // 调试/详细模式;与 Headless 冲突
	CWD      string // 用于解析配置/用例/产物的工作目录
}

// Summary 是一次 run 跨全部已发现用例的聚合结果。
type Summary struct {
	RunID                 string
	Total, Passed, Failed int
	RunDir                string
}

// Run 执行完整的 `autoqa run` 流程:
//  1. 加载 .env 文件与配置,解析基址 URL。
//  2. 在 Path 下发现 Markdown 用例。
//  3. 创建 run 目录与日志。
//  4. 对每个用例:读取、展开 include、渲染模板、解析,然后运行 agent。
//     单个用例失败会被记录并计数,但不会中断整批。
//  5. 成功的用例导出为 Playwright 风格测试文件。
//
// 配置/env 错误以 error 返回;单个用例的失败反映在 Summary 中。
func Run(ctx context.Context, opts RunOptions) (Summary, error) {
	if opts.CWD == "" {
		opts.CWD, _ = os.Getwd()
	}
	if opts.Debug && opts.Headless {
		return Summary{}, fmt.Errorf("--debug and --headless cannot be used together")
	}
	if _, err := env.Load(opts.CWD, opts.EnvName); err != nil {
		return Summary{}, err
	}
	cfg, err := config.Load(opts.CWD)
	if err != nil {
		return Summary{}, err
	}

	// 基址 URL 优先级:命令行 > 环境变量 > 配置 plan.baseUrl。
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("AUTOQA_BASE_URL")
	}
	if baseURL == "" {
		baseURL = cfg.Plan.BaseURL
	}
	if baseURL == "" {
		return Summary{}, fmt.Errorf("base URL is required via --url, AUTOQA_BASE_URL, or config plan.baseUrl")
	}

	paths, err := specs.Discover(opts.Path)
	if err != nil {
		return Summary{}, err
	}

	runID := logging.RunID()
	runDir := filepath.Join(opts.CWD, ".autoqa", "runs", runID)
	logger, err := logging.New(runDir, opts.Debug)
	if err != nil {
		return Summary{}, err
	}
	defer logger.Close()

	summary := Summary{RunID: runID, Total: len(paths), RunDir: runDir}
	r := &agent.Runner{ModelConfig: cfg.Model}

	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			summary.Failed++
			logger.Error("read spec", "path", p, "error", err)
			continue
		}
		expanded, err := markdown.ExpandIncludes(opts.CWD, string(data))
		if err != nil {
			summary.Failed++
			logger.Error("expand includes", "path", p, "error", err)
			continue
		}
		rendered, err := markdown.RenderTemplate(expanded, templateVars(baseURL, opts.EnvName))
		if err != nil {
			summary.Failed++
			logger.Error("render template", "path", p, "error", err)
			continue
		}
		spec, err := markdown.Parse(rendered)
		if err != nil {
			summary.Failed++
			logger.Error("parse spec", "path", p, "error", err)
			continue
		}
		res, err := r.Run(ctx, agent.RunOptions{
			RunID:    runID,
			BaseURL:  baseURL,
			SpecPath: p,
			Spec:     spec,
			RunDir:   runDir,
			Debug:    opts.Debug,
			Headless: opts.Headless,
			Config:   cfg,
			Logger:   logger,
		})
		if err != nil || !res.Success {
			summary.Failed++
			logger.Error("spec failed", "path", p, "error", err)
			continue
		}
		summary.Passed++
		// 仅在成功时导出,绝不生成不稳定测试。
		if err := ExportPlaceholder(opts.CWD, cfg.ExportDir, p, spec); err != nil {
			logger.Error("export failed", "path", p, "error", err)
		}
	}
	return summary, nil
}

// templateVars 构造一次 run 的严格模板变量 map。原项目支持的全部 key 都被填充;
// RenderTemplate 对未知或空值会报错,因此使用 USERNAME/PASSWORD 但未提供时用例会失败。
func templateVars(baseURL, envName string) map[string]string {
	return map[string]string{
		"BASE_URL":       baseURL,
		"LOGIN_BASE_URL": first(os.Getenv("AUTOQA_LOGIN_BASE_URL"), baseURL),
		"ENV":            envName,
		"USERNAME":       os.Getenv("AUTOQA_USERNAME"),
		"PASSWORD":       os.Getenv("AUTOQA_PASSWORD"),
	}
}

// first 返回第一个非空字符串参数,没有则返回 ""。
func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
