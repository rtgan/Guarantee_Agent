package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"guarantee-agent/internal/agent"
	"guarantee-agent/internal/browser"
	"guarantee-agent/internal/config"
	"guarantee-agent/internal/logging"
	"guarantee-agent/internal/markdown"
	"guarantee-agent/internal/tools"
)

// ExploreResult 是探索一个页面后得到的结构化摘要。
type ExploreResult struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Elements    []string `json:"elements"`
	Links       []string `json:"links"`
}

// Explore 用真实浏览器 + 真实 LLM 探索 baseURL 指向的页面,返回结构化摘要。
// 它复用 agent 的模型工厂与浏览器工具,让模型用 snapshot/navigate 等工具观察页面,
// 最终在文本中输出 JSON。
func Explore(ctx context.Context, cfg config.AutoqaConfig, baseURL, runDir string, headless bool, logger *logging.Logger) (ExploreResult, error) {
	page, err := browser.NewPage(baseURL, runDir, headless)
	if err != nil {
		return ExploreResult{}, err
	}
	defer page.Close()

	registry := &tools.Registry{Page: page}
	einoTools, err := registry.EinoTools()
	if err != nil {
		return ExploreResult{}, err
	}
	inv := invokableMap(einoTools)
	infos, err := toolInfos(ctx, einoTools)
	if err != nil {
		return ExploreResult{}, err
	}
	baseModel, err := agent.NewChatModel(ctx, cfg.Model)
	if err != nil {
		return ExploreResult{}, err
	}
	m, err := baseModel.WithTools(infos)
	if err != nil {
		return ExploreResult{}, err
	}

	messages := []*schema.Message{
		schema.SystemMessage(strings.TrimSpace(`You explore a web page to help generate test cases.
Use the provided browser tools: call navigate to open the URL, then snapshot to read the page.
After observing, reply with ONLY a JSON object (no prose, no code fence) describing the page:
{"url": string, "title": string, "description": string, "elements": [string], "links": [string]}.
elements: notable interactive elements (buttons, inputs, links) as visible text or selectors.
links: visible link texts on the page.`)),
		schema.UserMessage(fmt.Sprintf("Explore this page and summarize it: %s", baseURL)),
	}

	// ReAct 循环(与 agent.Runner 同构,但更宽松,用于探索)。
	var lastText string
	for round := 0; round < 30; round++ {
		if err := ctx.Err(); err != nil {
			return ExploreResult{}, err
		}
		assistant, err := m.Generate(ctx, messages)
		if err != nil {
			return ExploreResult{}, fmt.Errorf("explore model generate: %w", err)
		}
		messages = append(messages, assistant)
		if strings.TrimSpace(assistant.Content) != "" {
			lastText = assistant.Content
		}
		if len(assistant.ToolCalls) == 0 {
			break
		}
		for _, call := range assistant.ToolCalls {
			out, runErr := inv[call.Function.Name].InvokableRun(ctx, call.Function.Arguments)
			if runErr != nil {
				out = `{"ok":false,"errorCode":"TOOL_ERROR","message":"` + runErr.Error() + `"}`
			}
			messages = append(messages, schema.ToolMessage(out, call.ID, schema.WithToolName(call.Function.Name)))
		}
	}

	res := ExploreResult{URL: baseURL}
	if obj := extractJSON(lastText); obj != nil {
		_ = json.Unmarshal(obj, &res)
	}
	// 兜底:若模型没给出,用页面实际信息补。
	if res.URL == "" {
		res.URL = page.URL
	}
	if len(res.Elements) == 0 {
		res.Elements = splitElements(page.Snapshot())
	}
	if logger != nil {
		logger.Info("explore done", "url", res.URL, "elements", len(res.Elements))
	}
	return res, nil
}

// GenerateSpecs 根据探索结果,让 LLM 生成一个可被 markdown.Parse 解析的 Markdown 用例,
// 并用 parser 自检;通过则写入 outDir,返回生成的文件路径列表。
func GenerateSpecs(ctx context.Context, cfg config.AutoqaConfig, exploreResult ExploreResult, outDir string, logger *logging.Logger) ([]string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, err
	}
	baseModel, err := agent.NewChatModel(ctx, cfg.Model)
	if err != nil {
		return nil, err
	}
	// 生成阶段不需要工具,直接用 baseModel.Generate。
	m := baseModel

	summary, _ := json.Marshal(exploreResult)
	uiLanguage := os.Getenv("AUTOQA_UI_LANGUAGE")
	messages := []*schema.Message{
		schema.SystemMessage(generateSpecSystemPrompt(uiLanguage)),
		schema.UserMessage(fmt.Sprintf("Explored page summary:\n%s\n\nGenerate one Markdown test spec.", string(summary))),
	}

	assistant, err := m.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("generate model: %w", err)
	}
	md := strings.TrimSpace(assistant.Content)
	md = stripCodeFence(md)

	// 自检:必须能被 markdown.Parse 解析。
	parsed, perr := markdown.Parse(md)
	if perr != nil {
		return nil, fmt.Errorf("generated spec failed self-check: %w", perr)
	}
	name := sanitize(parsed.Title) + ".md"
	if name == ".md" {
		name = "generated-" + time.Now().UTC().Format("150405") + ".md"
	}
	path := filepath.Join(outDir, name)
	if err := os.WriteFile(path, []byte(md), 0644); err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Info("spec generated", "path", path, "steps", len(parsed.Steps))
	}
	return []string{path}, nil
}

// generateSpecSystemPrompt 根据 AUTOQA_UI_LANGUAGE 决定生成中文还是英文用例。
// 中文时用中文模板与关键字(验证/断言、预期:),英文时用英文模板(Verify/Assert、Expected:)。
// 两种语言下断言关键字约束对等,确保生成的用例都能被 markdown.Parse 正确分类为断言步骤。
func generateSpecSystemPrompt(uiLanguage string) string {
	if strings.HasPrefix(strings.ToLower(uiLanguage), "zh") {
		return strings.TrimSpace(`你负责生成 Markdown 验收测试用例。
只输出原始 Markdown(不要任何说明文字,不要代码围栏),严格使用如下结构:

# <标题>

## 前置条件
- <前置条件>

## 步骤
1. 打开 {{BASE_URL}}
2. 验证 <页面上实际可见的内容>

规则:
- Markdown 必须包含 "## 前置条件" 小节,且至少一个列表项。
- 步骤必须是有序(编号)列表。
- 至少一个步骤必须以 "验证" 或 "断言" 开头,并断言页面上实际可见的文本。
- 也可使用 "预期:" 子句描述某步骤的预期结果。
- URL 一律用 {{BASE_URL}},绝不使用字面量 URL。`)
	}
	return strings.TrimSpace(`You generate Markdown acceptance test specs.
Output ONLY raw Markdown (no prose, no code fence) with this exact structure:

# <title>

## Preconditions
- <precondition>

## Steps
1. Navigate to {{BASE_URL}}
2. Verify <something visible on the page>

Rules:
- The Markdown MUST contain a "## Preconditions" section with at least one list item.
- Steps MUST be an ordered (numbered) list.
- At least one step must start with "Verify" or "Assert" and assert text that is actually visible on the explored page.
- You may use an "Expected:" clause to describe a step's expected result.
- Use {{BASE_URL}} for the URL, never a literal URL.`)
}

var jsonRe = regexp.MustCompile(`(?s)\{.*\}`)

// extractJSON 从文本中抽取第一个 JSON 对象字节。
func extractJSON(s string) []byte {
	if m := jsonRe.FindString(s); m != "" {
		return []byte(m)
	}
	return nil
}

// stripCodeFence 去掉首尾的 ``` 围栏。
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	for _, fence := range []string{"```markdown", "```md", "```"} {
		if strings.HasPrefix(s, fence) {
			s = strings.TrimPrefix(s, fence)
			break
		}
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

// splitElements 把快照文本按空格切分,取较长的片段作为元素候选(粗略兜底)。
func splitElements(snapshot string) []string {
	var out []string
	for _, f := range strings.Fields(snapshot) {
		if len(f) > 2 {
			out = append(out, f)
		}
	}
	return out
}

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

// sanitize 把字符串转小写,非字母数字连续字符替换为单个连字符,生成文件名安全的形式。
func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
