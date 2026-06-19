package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"guarantee-agent/internal/browser"
	"guarantee-agent/internal/config"
	"guarantee-agent/internal/ir"
	"guarantee-agent/internal/logging"
	"guarantee-agent/internal/markdown"
	"guarantee-agent/internal/tools"
)

// RunOptions 描述执行单个 Markdown 用例所需的全部输入。
type RunOptions struct {
	RunID    string              // 唯一 run 标识,会写入 IR 记录和产物路径
	BaseURL  string              // 目标应用基址 URL
	SpecPath string              // Markdown 用例文件路径(写入 IR)
	Spec     *markdown.Spec      // 已解析的用例
	RunDir   string              // 本次 run 的产物目录(ir.jsonl、截图)
	Debug    bool                // 是否开启详细日志
	Headless bool                // 浏览器是否无头
	Config   config.AutoqaConfig // 合并后的配置(护栏、模型、导出)
	Logger   *logging.Logger     // 结构化日志
}

// RunResult 是执行单个用例的结果。
type RunResult struct {
	Success bool  // 仅当所有步骤都执行完成且必要断言全部成功时为 true
	Actions int   // 执行的工具调用次数
	Error   error // 用例失败或触发护栏时非 nil
}

// Runner 通过真实 Eino ReAct 循环驱动浏览器工具执行 Markdown 用例。
// 模型经 tool_calls 发出动作,runner 执行并把 tool 结果回填,循环到模型不再调用工具。
type Runner struct{ ModelConfig config.ModelConfig }

// stepStats 记录每个步骤的断言要求:Verify/Assert/Expected 步骤必须有过一次成功断言。
type stepStats struct{ assertionNeeded, assertionOK bool }

// maxReactRounds 限制单用例 ReAct 循环轮数,作为护栏之外的硬上限。
const maxReactRounds = 60

// Run 针对配置的基址 URL 执行单个 Markdown 用例。
//
// 流程:
//  1. 启动 Playwright 浏览器 Page,创建 IR writer。
//  2. 构建绑定到该 Page 的 Eino typed 浏览器工具。
//  3. 解析 chat 模型(ark=真实豆包;eino-script=离线)并绑定工具 schema。
//  4. 进入 ReAct 循环:模型生成 → 若有 tool_calls 则逐个执行、回填 tool 消息、
//     记录 IR、更新护栏;无 tool_calls 则结束。
//  5. 校验每个需要断言的步骤是否确实成功执行过断言,否则 STEP_VALIDATION_FAILED。
//
// 会响应 context:每轮开头检查取消。
func (r *Runner) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	page, err := browser.NewPage(opts.BaseURL, opts.RunDir, opts.Headless)
	if err != nil {
		return RunResult{}, err
	}
	defer page.Close()

	writer, err := ir.NewWriter(filepath.Join(opts.RunDir, "ir.jsonl"))
	if err != nil {
		return RunResult{}, err
	}
	defer writer.Close()

	limits := GuardrailLimits{
		opts.Config.Guardrails.MaxToolCallsPerSpec,
		opts.Config.Guardrails.MaxConsecutiveErrors,
		opts.Config.Guardrails.MaxRetriesPerStep,
	}
	counters := NewCounters()

	// 标记哪些步骤需要一次成功断言(Verify/Assert 前缀或带 Expected: 子句)。
	stats := map[int]*stepStats{}
	for _, st := range opts.Spec.Steps {
		stats[st.Index] = &stepStats{
			assertionNeeded: st.Kind == markdown.StepKindAssertion || st.ExpectedResult != "",
		}
	}

	// registry 在每次工具返回时把结果写入 IR,供回放/导出使用。
	registry := &tools.Registry{Page: page, OnResult: func(toolName string, stepIndex int, input any, res tools.Result) {
		_ = writer.Write(ir.ActionRecord{
			RunID:     opts.RunID,
			SpecPath:  opts.SpecPath,
			StepIndex: stepIndex,
			StepText:  stepText(opts.Spec, stepIndex),
			ToolName:  toolName,
			ToolInput: input,
			Outcome:   outcome(res.OK),
			ErrorCode: res.ErrorCode,
			PageURL:   page.URL,
			Timestamp: time.Now().UTC(),
		})
	}}

	einoTools, err := registry.EinoTools()
	if err != nil {
		return RunResult{Error: err}, err
	}
	inv := invokableMap(einoTools)
	infos, err := toolInfos(ctx, einoTools)
	if err != nil {
		return RunResult{Error: err}, err
	}

	baseModel, err := NewChatModel(ctx, opts.Config.Model)
	if err != nil {
		return RunResult{Error: err}, err
	}
	// 绑定工具 schema,得到带工具的模型实例。
	m, err := baseModel.WithTools(infos)
	if err != nil {
		return RunResult{Error: err}, err
	}

	// 构造对话:system 规则 + user 任务提示。
	prompt := BuildPrompt(opts.BaseURL, opts.SpecPath, opts.Spec, os.Getenv("AUTOQA_UI_LANGUAGE"))
	messages := []*schema.Message{
		schema.SystemMessage(systemRules()),
		schema.UserMessage(prompt),
	}

	// ReAct 循环。
	for round := 0; round < maxReactRounds; round++ {
		if err := ctx.Err(); err != nil {
			return RunResult{Error: err}, err
		}
		if err := counters.OnToolCall(limits); err != nil {
			return RunResult{Error: err, Actions: counters.ToolCalls}, err
		}
		assistant, err := m.Generate(ctx, messages)
		if err != nil {
			return RunResult{Error: fmt.Errorf("model generate: %w", err)}, fmt.Errorf("model generate: %w", err)
		}
		messages = append(messages, assistant)

		// 无 tool_calls → 模型认为任务完成,结束循环。
		if len(assistant.ToolCalls) == 0 {
			break
		}

		// 执行每个 tool_call 并回填 tool 结果消息。
		for _, call := range assistant.ToolCalls {
			step := parseStepIndexFromArgs(call.Function.Arguments)
			res := executeTool(ctx, inv, call)
			// 护栏:按工具结果成败更新计数。
			if gErr := counters.OnToolResult(step, res.OK, limits); gErr != nil {
				return RunResult{Error: gErr, Actions: counters.ToolCalls}, gErr
			}
			// 记录断言成功标记。
			if res.OK && (call.Function.Name == tools.ToolAssertTextPresent || call.Function.Name == tools.ToolAssertElementVisible) {
				if s := stats[step]; s != nil {
					s.assertionOK = true
				}
			}
			// 把工具结果作为 tool 消息追加,供模型下一轮参考。
			messages = append(messages, schema.ToolMessage(res.String(), call.ID, schema.WithToolName(call.Function.Name)))
		}
	}

	// 最终硬校验:需要断言的步骤必须确实成功执行过断言。
	for idx, st := range stats {
		if st.assertionNeeded && !st.assertionOK {
			err := fmt.Errorf("STEP_VALIDATION_FAILED: stepIndex=%d expected assertion but none succeeded", idx)
			return RunResult{Actions: counters.ToolCalls, Error: err}, err
		}
	}
	return RunResult{Success: true, Actions: counters.ToolCalls}, nil
}

// executeTool 调用对应 InvokableTool 并解析其 JSON 字符串结果为 tools.Result。
// 未知工具或调用异常时返回带稳定错误码的失败结果,而非 Go error,保持 ReAct 可恢复。
func executeTool(ctx context.Context, inv map[string]tool.InvokableTool, call schema.ToolCall) tools.Result {
	t, ok := inv[call.Function.Name]
	if !ok {
		return tools.Fail("UNKNOWN_TOOL", "unknown tool "+call.Function.Name, false)
	}
	out, err := t.InvokableRun(ctx, call.Function.Arguments)
	if err != nil {
		return tools.Fail("TOOL_ERROR", err.Error(), true)
	}
	var res tools.Result
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		// 工具返回非 JSON 文本时,包装成成功结果。
		return tools.OK(out, nil)
	}
	return res
}

// parseStepIndexFromArgs 从工具调用参数 JSON 中取 stepIndex(0 表示缺失)。
func parseStepIndexFromArgs(args string) int {
	if args == "" {
		return 0
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return 0
	}
	switch v := m["stepIndex"].(type) {
	case float64:
		return int(v)
	case string:
		var i int
		_, _ = fmt.Sscanf(v, "%d", &i)
		return i
	default:
		return 0
	}
}

// systemRules 是给模型的系统级硬规则,补充 BuildPrompt 中的任务细节。
// 用例可能是中文或英文,规则里同时列出两种断言关键字,确保中文用例的"验证/断言"步骤也被强制断言。
func systemRules() string {
	return strings.TrimSpace(`You are an AutoQA browser testing agent.
You operate a real browser ONLY through the provided tools.
Always include stepIndex in every tool call matching the current Markdown step you are working on.
Execute steps strictly in order: finish step N (including its assertion) before step N+1.
Before clicking/filling/asserting, call snapshot to see the page.
The spec may be written in Chinese or English. Treat these as assertion steps that MUST end with a successful assertion tool call (assertTextPresent or assertElementVisible) using THAT step's stepIndex:
  - steps starting with Verify / Assert (English), or 验证 / 断言 (Chinese);
  - any step with an "Expected:" or "预期:" clause.
Do not defer a step's assertion to a later step. Never claim success without a successful assertion for every required step. When all steps are done, reply with a short summary and no tool calls.`)
}

// stepText 按步骤序号查找其可读文本(用于 IR 记录)。
func stepText(spec *markdown.Spec, idx int) string {
	for _, s := range spec.Steps {
		if s.Index == idx {
			return s.Text
		}
	}
	return ""
}

// outcome 把工具是否成功映射为 IR 中的 outcome 字符串。
func outcome(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}

// toolInfos 收集每个工具的 ToolInfo 元数据,供 WithTools 绑定到 chat 模型。
func toolInfos(ctx context.Context, ts []tool.BaseTool) ([]*schema.ToolInfo, error) {
	var out []*schema.ToolInfo
	for _, t := range ts {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

// invokableMap 按工具名索引 InvokableTool,便于 runner 直接调用。
func invokableMap(ts []tool.BaseTool) map[string]tool.InvokableTool {
	out := map[string]tool.InvokableTool{}
	for _, t := range ts {
		if it, ok := t.(tool.InvokableTool); ok {
			info, _ := t.Info(context.Background())
			out[info.Name] = it
		}
	}
	return out
}
