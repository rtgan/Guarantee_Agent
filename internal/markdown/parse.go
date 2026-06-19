package markdown

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// expectedPrefix 匹配行首/条目开头的 "Expected:" 或中文"预期:"子句(可带列表符号),
// 用于把子句与步骤正文分离。
var expectedPrefix = regexp.MustCompile(`(?is)^[\s\-•]*(?:expected|预期)[:：]\s*`)

// trailingExpected 匹配内联的 "动作\nExpected: 结果" 或 "动作\n预期: 结果" 形式,
// 使相邻两行的步骤与其预期结果能被正确拆分。
var trailingExpected = regexp.MustCompile(`(?is)^(.+?)\n\s*[-•]?\s*(?:expected|预期)[:：]\s*(.+)$`)

// Parse 解析 Markdown 验收用例。支持中英文标题。
//
// 必需结构:
//   - 一个前置条件小节(`## Preconditions` 或 `## 前置条件`),且至少包含一个列表项。
//   - 一个有序(编号)步骤列表,最好位于 `## Steps` 或 `## 步骤` 下。
//     若没有该小节,则取前置条件之后的第一个有序列表。
//
// 步骤编号保留 Markdown 列表的起始值。每个步骤可带 `Expected:` 或 `预期:` 子句(嵌套或内联)。
// 以 Verify/Assert/验证/断言 开头的步骤被分类为 assertion。
func Parse(input string) (*Spec, error) {
	source := []byte(input)
	root := goldmark.DefaultParser().Parse(text.NewReader(source))
	children := nodeChildren(root)
	preIdx := findH2(children, source, []string{"preconditions", "前置条件", "前置条件"}, 0)
	if preIdx < 0 {
		return nil, &ParseError{Code: "MARKDOWN_MISSING_PRECONDITIONS", Message: "Missing required section: ## Preconditions."}
	}
	title := firstTitle(children, source)
	preStart, preEnd := sectionRange(children, preIdx)
	var preList *ast.List
	for i := preStart; i < preEnd; i++ {
		if l, ok := children[i].(*ast.List); ok {
			preList = l
			break
		}
	}
	preconditions := collectListTexts(preList, source)
	if len(preconditions) == 0 {
		return nil, &ParseError{Code: "MARKDOWN_EMPTY_PRECONDITIONS", Message: "Preconditions section is present but contains no list items."}
	}
	var stepsList *ast.List
	stepsIdx := findH2(children, source, []string{"steps", "步骤"}, preIdx+1)
	if stepsIdx >= 0 {
		st, en := sectionRange(children, stepsIdx)
		for i := st; i < en; i++ {
			if l, ok := children[i].(*ast.List); ok && l.IsOrdered() {
				stepsList = l
				break
			}
		}
	}
	if stepsList == nil {
		// 没有 ## Steps 时,取 Preconditions 之后的第一个有序列表。
		start := preEnd
		for i := start; i < len(children); i++ {
			if l, ok := children[i].(*ast.List); ok && l.IsOrdered() {
				stepsList = l
				break
			}
		}
	}
	if stepsList == nil {
		return nil, &ParseError{Code: "MARKDOWN_MISSING_STEPS", Message: "Missing ordered steps list."}
	}
	steps := collectSteps(stepsList, source)
	if len(steps) == 0 {
		return nil, &ParseError{Code: "MARKDOWN_EMPTY_STEPS", Message: "Steps list is present but contains no list items."}
	}
	return &Spec{Title: title, Preconditions: preconditions, Steps: steps}, nil
}

// nodeChildren 返回 n 的直接子节点切片。
func nodeChildren(n ast.Node) []ast.Node {
	var out []ast.Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, c)
	}
	return out
}

// firstTitle 返回第一个 H1 标题的文本,用作用例标题。
func firstTitle(nodes []ast.Node, source []byte) string {
	for _, n := range nodes {
		if h, ok := n.(*ast.Heading); ok && h.Level == 1 {
			return strings.TrimSpace(nodeText(n, source))
		}
	}
	return ""
}

// findH2 返回从 start 起、文本与任一别名相等(不区分大小写)的第一个 H2 的下标,没有则 -1。
// 同时支持英文与中文标题别名,例如 preconditions / 前置条件。
func findH2(nodes []ast.Node, source []byte, names []string, start int) int {
	for i := start; i < len(nodes); i++ {
		if h, ok := nodes[i].(*ast.Heading); ok && h.Level == 2 {
			text := strings.TrimSpace(nodeText(h, source))
			for _, name := range names {
				if strings.EqualFold(text, name) {
					return i
				}
			}
		}
	}
	return -1
}

// sectionRange 返回下标为 heading 的标题所属小节的子节点 [start, end) 范围,
// 遇到下一个 H1/H2 即结束。
func sectionRange(nodes []ast.Node, heading int) (int, int) {
	start := heading + 1
	end := len(nodes)
	for i := start; i < len(nodes); i++ {
		if h, ok := nodes[i].(*ast.Heading); ok && h.Level <= 2 {
			end = i
			break
		}
	}
	return start, end
}

// collectListTexts 返回每个列表项的非空文本(用于 preconditions,忽略 Expected 子句)。
func collectListTexts(l *ast.List, source []byte) []string {
	if l == nil {
		return nil
	}
	var out []string
	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		if _, ok := item.(*ast.ListItem); !ok {
			continue
		}
		text := extractListItem(item, source).text
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

// collectSteps 从有序列表构造 Step 记录,保留列表起始编号并对每步分类。空项被跳过。
func collectSteps(l *ast.List, source []byte) []Step {
	var out []Step
	idx := l.Start
	if idx == 0 {
		idx = 1
	}
	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		if _, ok := item.(*ast.ListItem); !ok {
			continue
		}
		ex := extractListItem(item, source)
		if ex.text != "" {
			out = append(out, Step{Index: idx, Text: ex.text, ExpectedResult: ex.expected, Kind: ClassifyStepKind(ex.text)})
		}
		idx++
	}
	return out
}

// extracted 保存一个列表项的正文和可选的 Expected 子句。
type extracted struct{ text, expected string }

// extractListItem 把列表项正文与任意 Expected: 子句分离。
// Expected 可作为嵌套列表项、段落开头,或 "正文\nExpected: 结果" 的尾随形式出现。
func extractListItem(item ast.Node, source []byte) extracted {
	var main []string
	var expected string
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindParagraph, ast.KindTextBlock:
			full := strings.TrimSpace(nodeText(c, source))
			// 当通用遍历对松散列表项返回空文本时,回退到 TextBlock 的原始行。
			if tb, ok := c.(*ast.TextBlock); ok && full == "" {
				full = strings.TrimSpace(textBlockText(tb, source))
			}
			if full == "" {
				continue
			}
			if expectedPrefix.MatchString(full) {
				if expected == "" {
					expected = strings.TrimSpace(expectedPrefix.ReplaceAllString(full, ""))
				}
				continue
			}
			if m := trailingExpected.FindStringSubmatch(full); len(m) == 3 {
				main = append(main, strings.TrimSpace(m[1]))
				if expected == "" {
					expected = strings.TrimSpace(m[2])
				}
				continue
			}
			main = append(main, full)
		case ast.KindList:
			for li := c.FirstChild(); li != nil; li = li.NextSibling() {
				text := strings.TrimSpace(nodeText(li, source))
				if expectedPrefix.MatchString(text) && expected == "" {
					expected = strings.TrimSpace(expectedPrefix.ReplaceAllString(text, ""))
				}
			}
		}
	}
	return extracted{text: strings.Join(main, " "), expected: expected}
}

// ClassifyStepKind 对以 Verify/Assert/验证/断言 开头的步骤返回 StepKindAssertion,
// 否则返回 StepKindAction。
func ClassifyStepKind(text string) StepKind {
	t := strings.TrimSpace(text)
	lower := strings.ToLower(t)
	if strings.HasPrefix(t, "验证") || strings.HasPrefix(t, "断言") || strings.HasPrefix(lower, "verify") || strings.HasPrefix(lower, "assert") {
		return StepKindAssertion
	}
	return StepKindAction
}

// textBlockText 拼接 TextBlock 节点的原始行(当通用遍历对松散列表项返回空文本时的回退)。
func textBlockText(tb *ast.TextBlock, source []byte) string {
	var b strings.Builder
	lines := tb.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(source))
		if i+1 < lines.Len() {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// nodeText 遍历节点并拼接其 Text 片段(保留换行),重建标题/段落/条目的人类可读文本。
func nodeText(n ast.Node, source []byte) string {
	var b bytes.Buffer
	_ = ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := node.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte('\n')
			}
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}
