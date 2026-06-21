package agent

import (
	"fmt"
	"strings"

	"guarantee-agent/internal/markdown"
)

// BuildPrompt 构造交给 chat 模型的任务提示词。
//
// 保留原项目的关键规则:只使用提供的浏览器工具、按顺序执行步骤、每次调用必须带
// stepIndex、优先使用 snapshot 中的 ref 而非臆造、Verify/Assert 步骤和带 Expected:
// 子句的步骤必须调用断言工具。提示词用于对齐和日志;当前确定性脚本 runner 直接
// 在 planStepCalls 中规划调用,不依赖模型输出。
func BuildPrompt(baseURL, specPath string, spec *markdown.Spec, uiLanguage string) string {
	pre := make([]string, len(spec.Preconditions))
	for i, p := range spec.Preconditions {
		pre[i] = "- " + p
	}
	steps := make([]string, len(spec.Steps))
	for i, s := range spec.Steps {
		line := fmt.Sprintf("%d. %s", s.Index, s.Text)
		if s.ExpectedResult != "" {
			line += "\n   - Expected: " + s.ExpectedResult
		}
		steps[i] = line
	}
	lang := "UI language: English (en). Use the same wording that appears in the UI."
	if strings.HasPrefix(strings.ToLower(uiLanguage), "zh") {
		lang = "UI language: Chinese (zh-CN). Copy Chinese UI labels exactly; do not translate them."
	}
	return fmt.Sprintf(`You are an AutoQA agent.

Base URL: %s
Spec Path: %s

%s

Preconditions:
%s

Steps:
%s

Rules:
- Use ONLY the provided browser tools: snapshot, navigate, click, fill, select_option, scroll, wait, assertTextPresent, assertElementVisible.
- Execute steps in order.
- The browser page starts at about:blank. At Step 1 you MUST call navigate with stepIndex=1.
- For EVERY tool call, include stepIndex matching the current Markdown step number.
- Before each interaction/assertion, call snapshot and prefer stable refs/descriptions from the snapshot.
- NEVER guess or invent a ref.
- Steps starting with Verify/Assert/验证/断言 MUST call an assertion tool.
- Any step with Expected MUST perform the action then call an assertion tool before moving on.
- Do not report success unless all mandatory assertions succeeded.`, baseURL, specPath, lang, strings.Join(pre, "\n"), strings.Join(steps, "\n"))
}
