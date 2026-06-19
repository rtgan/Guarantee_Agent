package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"guarantee-agent/internal/browser"
)

// 工具名常量。与原项目 browser MCP 工具名一致(去掉 mcp__browser__ 前缀),
// 使提示词和 IR 保持兼容。
const (
	ToolSnapshot             = "snapshot"
	ToolNavigate             = "navigate"
	ToolClick                = "click"
	ToolFill                 = "fill"
	ToolSelectOption         = "select_option"
	ToolScroll               = "scroll"
	ToolWait                 = "wait"
	ToolAssertTextPresent    = "assertTextPresent"
	ToolAssertElementVisible = "assertElementVisible"
)

// AllowedNames 是 run/plan agent 允许使用的工具白名单。
var AllowedNames = []string{ToolSnapshot, ToolNavigate, ToolClick, ToolFill, ToolSelectOption, ToolScroll, ToolWait, ToolAssertTextPresent, ToolAssertElementVisible}

// BaseInput 被每个工具的输入结构体内嵌,强制模型为每次调用标注
// 1-based 的 Markdown 步骤序号(用于 IR 和护栏)。
type BaseInput struct {
	StepIndex int `json:"stepIndex" jsonschema_description:"1-based Markdown step index"`
}

// 各工具的输入结构体。Eino 的 utils.InferTool 会根据这些结构体的 tag 自动推导
// JSON schema,因此模型看到的参数名/类型与这里完全一致。
type SnapshotInput struct{ BaseInput }

type NavigateInput struct {
	BaseInput
	URL string `json:"url" jsonschema:"required" jsonschema_description:"Absolute URL or slash-relative path"`
}

type ClickInput struct {
	BaseInput
	Ref               string `json:"ref,omitempty"`
	TargetDescription string `json:"targetDescription,omitempty"`
}

type FillInput struct {
	BaseInput
	Ref               string `json:"ref,omitempty"`
	TargetDescription string `json:"targetDescription,omitempty"`
	Text              string `json:"text" jsonschema:"required"`
}

type SelectInput struct {
	BaseInput
	Ref               string `json:"ref,omitempty"`
	TargetDescription string `json:"targetDescription,omitempty"`
	Value             string `json:"value,omitempty"`
	Label             string `json:"label,omitempty"`
}

type ScrollInput struct {
	BaseInput
	Direction string `json:"direction,omitempty"`
}

type WaitInput struct {
	BaseInput
	Milliseconds int `json:"milliseconds,omitempty"`
}

type AssertTextInput struct {
	BaseInput
	Text string `json:"text" jsonschema:"required"`
}

type AssertVisibleInput struct {
	BaseInput
	Ref               string `json:"ref,omitempty"`
	TargetDescription string `json:"targetDescription,omitempty"`
	Text              string `json:"text,omitempty"`
}

// Registry 把一个浏览器 Page 绑定到一组 Eino 工具,并把每次工具结果经
// OnResult 回调转发(runner 用它写 IR)。
type Registry struct {
	Page     *browser.Page
	OnResult func(toolName string, stepIndex int, input any, result Result)
}

// EinoTools 把全部浏览器工具构建为 Eino InvokableTool。每个工具:
//   - 从输入读取 stepIndex;
//   - 执行对应的页面操作;
//   - 对预期内的失败返回 Result(OK 或结构化 Fail),而非 Go error;
//   - 调用 OnResult,让 runner 把动作写入 IR。
func (r *Registry) EinoTools() ([]tool.BaseTool, error) {
	// report 把结果转发给可选回调并原样返回。
	report := func(name string, step int, in any, res Result) Result {
		if r.OnResult != nil {
			r.OnResult(name, step, in, res)
		}
		return res
	}

	snapshot, err := utils.InferTool[SnapshotInput, Result](ToolSnapshot, "Capture page text/accessibility snapshot", func(ctx context.Context, in SnapshotInput) (Result, error) {
		return report(ToolSnapshot, in.StepIndex, in, OK("snapshot captured", map[string]any{"snapshot": r.Page.Snapshot(), "url": r.Page.URL})), nil
	})
	navigate, err := utils.InferTool[NavigateInput, Result](ToolNavigate, "Navigate browser page", func(ctx context.Context, in NavigateInput) (Result, error) {
		if err := r.Page.Navigate(ctx, in.URL); err != nil {
			return report(ToolNavigate, in.StepIndex, in, Fail("NAVIGATION_FAILED", err.Error(), true)), nil
		}
		return report(ToolNavigate, in.StepIndex, in, OK("navigated", map[string]any{"url": r.Page.URL})), nil
	})
	click, err := utils.InferTool[ClickInput, Result](ToolClick, "Click an element by ref or description", func(ctx context.Context, in ClickInput) (Result, error) {
		target := firstNonEmpty(in.Ref, in.TargetDescription)
		if err := r.Page.Click(ctx, target); err != nil {
			return report(ToolClick, in.StepIndex, in, Fail("ELEMENT_NOT_FOUND", err.Error(), true)), nil
		}
		return report(ToolClick, in.StepIndex, in, OK("clicked", map[string]any{"target": target, "url": r.Page.URL})), nil
	})
	fill, err := utils.InferTool[FillInput, Result](ToolFill, "Fill a field by ref or description", func(ctx context.Context, in FillInput) (Result, error) {
		target := firstNonEmpty(in.Ref, in.TargetDescription)
		if err := r.Page.Fill(target, in.Text); err != nil {
			return report(ToolFill, in.StepIndex, in, Fail("ELEMENT_NOT_FOUND", err.Error(), true)), nil
		}
		// 只记录文本长度,绝不记录原始值,避免泄露密钥。
		return report(ToolFill, in.StepIndex, in, OK("filled", map[string]any{"target": target, "textLength": len(in.Text)})), nil
	})
	selectTool, err := utils.InferTool[SelectInput, Result](ToolSelectOption, "Select an option", func(ctx context.Context, in SelectInput) (Result, error) {
		target := firstNonEmpty(in.Ref, in.TargetDescription)
		val := firstNonEmpty(in.Label, in.Value)
		if err := r.Page.Fill(target, val); err != nil {
			return report(ToolSelectOption, in.StepIndex, in, Fail("ELEMENT_NOT_FOUND", err.Error(), true)), nil
		}
		return report(ToolSelectOption, in.StepIndex, in, OK("selected", map[string]any{"target": target})), nil
	})
	scroll, err := utils.InferTool[ScrollInput, Result](ToolScroll, "Scroll page", func(ctx context.Context, in ScrollInput) (Result, error) {
		return report(ToolScroll, in.StepIndex, in, OK("scrolled", map[string]any{"direction": firstNonEmpty(in.Direction, "down")})), nil
	})
	waitTool, err := utils.InferTool[WaitInput, Result](ToolWait, "Wait for milliseconds", func(ctx context.Context, in WaitInput) (Result, error) {
		return report(ToolWait, in.StepIndex, in, OK("waited", map[string]any{"milliseconds": in.Milliseconds})), nil
	})
	assertText, err := utils.InferTool[AssertTextInput, Result](ToolAssertTextPresent, "Assert text is present", func(ctx context.Context, in AssertTextInput) (Result, error) {
		if err := r.Page.AssertText(in.Text); err != nil {
			return report(ToolAssertTextPresent, in.StepIndex, in, Fail("ASSERTION_FAILED", err.Error(), true)), nil
		}
		return report(ToolAssertTextPresent, in.StepIndex, in, OK("asserted text", map[string]any{"text": in.Text})), nil
	})
	assertVisible, err := utils.InferTool[AssertVisibleInput, Result](ToolAssertElementVisible, "Assert element/text is visible", func(ctx context.Context, in AssertVisibleInput) (Result, error) {
		target := firstNonEmpty(in.Text, in.TargetDescription, in.Ref)
		if err := r.Page.AssertText(target); err != nil {
			return report(ToolAssertElementVisible, in.StepIndex, in, Fail("ASSERTION_FAILED", err.Error(), true)), nil
		}
		return report(ToolAssertElementVisible, in.StepIndex, in, OK("asserted visible", map[string]any{"target": target})), nil
	})

	// 上面的 InferTool 错误是顺序赋值的;这里把首个非空错误连同工具名一起暴露。
	toolsByName := []struct {
		name string
		t    tool.BaseTool
		err  error
	}{
		{ToolSnapshot, snapshot, err},
		{ToolNavigate, navigate, err},
		{ToolClick, click, err},
		{ToolFill, fill, err},
		{ToolSelectOption, selectTool, err},
		{ToolScroll, scroll, err},
		{ToolWait, waitTool, err},
		{ToolAssertTextPresent, assertText, err},
		{ToolAssertElementVisible, assertVisible, err},
	}
	out := make([]tool.BaseTool, 0, len(toolsByName))
	for _, d := range toolsByName {
		if d.err != nil {
			return nil, fmt.Errorf("create tool %s: %w", d.name, d.err)
		}
		out = append(out, d.t)
	}
	return out, nil
}

// firstNonEmpty 返回第一个非空字符串参数,没有则返回 ""。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
