package plan

import (
	"net/url"
	"strings"

	"guarantee-agent/internal/config"
)

// ExtractRelativeURL 从完整 URL 中提取相对部分(path + query + fragment),
// 用于和 plan 配置中的 include/exclude 模式做匹配。解析失败时原样返回。
func ExtractRelativeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	out := u.EscapedPath()
	if out == "" {
		out = "/"
	}
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}

// IsURLInScope 判断一个 URL 是否落在 plan 配置的探索范围内。
// 规则:exclude 模式优先(命中即排除);若有 include 模式则必须命中其一;
// 否则在 site 范围下默认放行,focused/single_page 默认排除。
func IsURLInScope(raw string, cfg config.PlanConfig) bool {
	rel := ExtractRelativeURL(raw)
	for _, p := range cfg.ExcludePatterns {
		if matchPattern(rel, p) {
			return false
		}
	}
	if len(cfg.IncludePatterns) > 0 {
		for _, p := range cfg.IncludePatterns {
			if matchPattern(rel, p) {
				return true
			}
		}
		return false
	}
	return cfg.ExploreScope == "site"
}

// matchPattern 匹配单个模式:以 * 结尾表示前缀匹配,否则要求完全相等。
func matchPattern(s, p string) bool {
	if p == "" {
		return false
	}
	if strings.HasSuffix(p, "*") {
		return strings.HasPrefix(s, strings.TrimSuffix(p, "*"))
	}
	return s == p
}
