package markdown

import (
	"fmt"
	"regexp"
)

// templateVar 匹配 {{VAR}} 占位符,其中 VAR 为大写 A-Z0-9_。
var templateVar = regexp.MustCompile(`\{\{([A-Z0-9_]+)\}\}`)

// RenderTemplate 用 vars 中的值替换 {{VAR}} 占位符。
// 它是严格的:未知变量或空值都会报错,因此用例绝不会带着缺失上下文
// (如空的 USERNAME/PASSWORD)静默运行。
func RenderTemplate(input string, vars map[string]string) (string, error) {
	var firstErr error
	out := templateVar.ReplaceAllStringFunc(input, func(match string) string {
		if firstErr != nil {
			return match
		}
		parts := templateVar.FindStringSubmatch(match)
		name := parts[1]
		val, ok := vars[name]
		if !ok {
			firstErr = fmt.Errorf("unknown template variable %s", name)
			return match
		}
		if val == "" {
			firstErr = fmt.Errorf("template variable %s is empty", name)
			return match
		}
		return val
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

// ExtractTemplateVars 返回 input 中引用的、去重且保序的 {{VAR}} 名列表。
// 便于校验用例只使用了已知变量。
func ExtractTemplateVars(input string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range templateVar.FindAllStringSubmatch(input, -1) {
		if len(m) == 2 && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}
