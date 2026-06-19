package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// includeLine 匹配一行 `include: <name>`(不区分大小写,多行模式)。
var includeLine = regexp.MustCompile(`(?im)^\s*include:\s*(\S+)\s*$`)

// includeName 把 include 名限制为安全的相对路径(字母数字、下划线、连字符、斜杠、可选 .md),
// 拒绝路径穿越/绝对路径。
var includeName = regexp.MustCompile(`^[A-Za-z0-9_\-/]+(?:\.md)?$`)

// ExpandIncludes 把 markdown 中每行 `include: <name>` 替换为所引用可复用步骤文件的内容。
// 嵌套 include 会被拒绝。
func ExpandIncludes(projectRoot, markdown string) (string, error) {
	return expandIncludes(projectRoot, markdown, false)
}

// expandIncludes 执行实际工作;nested 标志用于禁止嵌套 include。
func expandIncludes(projectRoot, markdown string, nested bool) (string, error) {
	matches := includeLine.FindAllStringSubmatchIndex(markdown, -1)
	if len(matches) == 0 {
		return markdown, nil
	}
	if nested {
		return "", fmt.Errorf("nested includes are not allowed")
	}
	var out strings.Builder
	last := 0
	for _, m := range matches {
		out.WriteString(markdown[last:m[0]])
		name := markdown[m[2]:m[3]]
		path, err := ResolveIncludePath(projectRoot, name)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		content := string(data)
		// 引用文件自身若又含 include 行,视为嵌套,拒绝。
		if includeLine.MatchString(content) {
			return "", fmt.Errorf("nested include in %s is not allowed", path)
		}
		out.WriteString(strings.TrimRight(content, "\n"))
		last = m[1]
	}
	out.WriteString(markdown[last:])
	return out.String(), nil
}

// ResolveIncludePath 把 include 名解析为 steps/(主)或 specs/steps/(回退)下的文件。
// 拒绝空名、路径穿越(..)、反斜杠、绝对路径以及非法字符。
func ResolveIncludePath(projectRoot, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("include name is empty")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "\\") || filepath.IsAbs(name) || !includeName.MatchString(name) {
		return "", fmt.Errorf("invalid include name %q", name)
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	for _, base := range []string{"steps", filepath.Join("specs", "steps")} {
		path := filepath.Join(projectRoot, base, filepath.FromSlash(name))
		st, err := os.Stat(path)
		if err == nil && !st.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("include %q not found in steps/ or specs/steps/", name)
}
