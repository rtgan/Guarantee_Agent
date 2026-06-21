package specs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover 发现待执行的 Markdown 用例。入参可以是单个 .md 文件,也可以是目录。
// 目录会递归遍历(跳过以点开头的目录),返回按路径排序的 .md/.markdown 文件列表。
// 单个文件必须本身是 Markdown;空目录返回错误。
func Discover(path string) ([]string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	var out []string
	if !st.IsDir() {
		if !isMarkdown(path) {
			return nil, fmt.Errorf("%s is not a markdown file", path)
		}
		return []string{path}, nil
	}
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// 跳过隐藏目录(根目录本身除外)。
			if strings.HasPrefix(d.Name(), ".") && p != path {
				return filepath.SkipDir
			}
			return nil
		}
		if isMarkdown(p) {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no markdown specs found in %s", path)
	}
	return out, nil
}

// isMarkdown 判断文件扩展名是否为 .md 或 .markdown(不区分大小写)。
func isMarkdown(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}
