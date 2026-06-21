package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guarantee-agent/internal/markdown"
)

// TestExtractGotoTarget 覆盖 Bug #1:从第一步文本提取 URL 并拆成 host+path。
func TestExtractGotoTarget(t *testing.T) {
	cases := []struct {
		name     string
		step     string
		wantHost string
		wantPath string
		wantOK   bool
	}{
		{"中文带路径", "打开 http://127.0.0.1:18090/form.html", "http://127.0.0.1:18090", "/form.html", true},
		{"中文无路径", "打开 http://127.0.0.1:18090", "http://127.0.0.1:18090", "", true},
		{"英文带路径", "Navigate to https://example.com/login", "https://example.com", "/login", true},
		{"带query", "Open http://h:8080/p?x=1", "http://h:8080", "/p?x=1", true},
		{"根路径归一为空", "打开 http://127.0.0.1:18090/", "http://127.0.0.1:18090", "", true},
		{"尾部中文标点", "打开 http://127.0.0.1:18090/form.html。", "http://127.0.0.1:18090", "/form.html", true},
		{"非导航步骤", "点击 Submit 按钮", "", "", false},
		{"无URL中文", "打开首页", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			host, path, ok := extractGotoTarget(c.step)
			if ok != c.wantOK || host != c.wantHost || path != c.wantPath {
				t.Errorf("extractGotoTarget(%q) = (%q,%q,%v), want (%q,%q,%v)", c.step, host, path, ok, c.wantHost, c.wantPath, c.wantOK)
			}
		})
	}
}

// TestExportGotoLine 覆盖 Bug #1:生成的 goto 行正确拼接 host+path 或回退。
func TestExportGotoLine(t *testing.T) {
	t.Run("带路径", func(t *testing.T) {
		spec := &markdown.Spec{Steps: []markdown.Step{{Index: 1, Text: "打开 http://127.0.0.1:18090/form.html"}}}
		got := gotoLine(spec)
		want := `    page.goto(os.environ.get("AUTOQA_BASE_URL", "http://127.0.0.1:18090") + "/form.html")`
		if got != want {
			t.Errorf("gotoLine =\n %s\nwant\n %s", got, want)
		}
	})
	t.Run("无路径", func(t *testing.T) {
		spec := &markdown.Spec{Steps: []markdown.Step{{Index: 1, Text: "打开 http://127.0.0.1:18090"}}}
		got := gotoLine(spec)
		want := `    page.goto(os.environ.get("AUTOQA_BASE_URL", "http://127.0.0.1:18090"))`
		if got != want {
			t.Errorf("gotoLine =\n %s\nwant\n %s", got, want)
		}
	})
	t.Run("非导航首步回退", func(t *testing.T) {
		spec := &markdown.Spec{Steps: []markdown.Step{{Index: 1, Text: "点击 Submit 按钮"}}}
		got := gotoLine(spec)
		want := `    page.goto(os.environ.get("AUTOQA_BASE_URL", "/"))`
		if got != want {
			t.Errorf("gotoLine =\n %s\nwant\n %s", got, want)
		}
	})
	t.Run("无步骤回退", func(t *testing.T) {
		got := gotoLine(&markdown.Spec{})
		want := `    page.goto(os.environ.get("AUTOQA_BASE_URL", "/"))`
		if got != want {
			t.Errorf("gotoLine = %q, want %q", got, want)
		}
	})
}

// TestExportExpectedTarget 覆盖 Bug #2:ExpectedResult 剥可见性后缀但不误伤前缀。
func TestExportExpectedTarget(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"剥可见后缀", "仪表盘可见", "仪表盘"},
		{"正常英文不变", "Form submitted successfully", "Form submitted successfully"},
		{"防前缀误伤-验证码", "验证码已发送", "验证码已发送"},
		{"防前缀误伤-assert", "Assert page shows error", "Assert page shows error"},
		{"带引号取引号内", `验证 "仪表盘" 可见`, "仪表盘"},
		{"剥存在后缀", "元素存在", "元素"},
		{"剥英文exists", "dashboard exists", "dashboard"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := exportExpectedTarget(c.in); got != c.want {
				t.Errorf("exportExpectedTarget(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestExportAssertionTargetRegression 确保 exportAssertionTarget 仍正确剥前缀+后缀。
func TestExportAssertionTargetRegression(t *testing.T) {
	cases := map[string]string{
		`Verify "AutoQA Form Fixture" text is visible`: "AutoQA Form Fixture",
		`验证 "Example Domain" 文字可见`:                   "Example Domain",
		`验证仪表盘可见`:                                  "仪表盘",
		`Verify dashboard is visible`:                  "dashboard",
	}
	for in, want := range cases {
		if got := exportAssertionTarget(in); got != want {
			t.Errorf("exportAssertionTarget(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPyStrEscapes 覆盖 Bug #3:转义换行/制表/回车/引号。
func TestPyStrEscapes(t *testing.T) {
	got := pyStr("a\nb\tc\rd\"e\\f")
	want := `"a\nb\tc\rd\"e\\f"`
	if got != want {
		t.Errorf("pyStr = %q, want %q", got, want)
	}
	// 生成的应是合法 Python 字符串字面量(无双引号语法错误):首尾各一个双引号,中间无裸换行。
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Errorf("pyStr not quoted: %q", got)
	}
	if strings.ContainsAny(got[1:len(got)-1], "\n\t\r") {
		t.Errorf("pyStr contains raw control char: %q", got)
	}
}

// TestPyFuncNameFallback 覆盖 Bug #4:中文标题回退 fallback,英文标题优先。
func TestPyFuncNameFallback(t *testing.T) {
	cases := []struct {
		name     string
		title    string
		fallback string
		want     string
	}{
		{"中文标题回退", "表单全流程覆盖", "form", "test_form"},
		{"英文标题优先", "Login Flow", "form", "test_login_flow"},
		{"空标题回退", "", "simple", "test_simple"},
		{"标题和fallback都空", "!!!", "", "test_generated"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pyFuncName(c.title, c.fallback); got != c.want {
				t.Errorf("pyFuncName(%q,%q) = %q, want %q", c.title, c.fallback, got, c.want)
			}
		})
	}
}

// TestSpecSlugCollision 覆盖 Bug #5:同 basename 不同子目录区分,直接子文件不变。
func TestSpecSlugCollision(t *testing.T) {
	cases := []struct {
		name      string
		specPath  string
		specsRoot string
		want      string
	}{
		{"直接子文件", "specs/form.md", "specs", "form"},
		{"子目录a", "specs/a/login.md", "specs", "a_login"},
		{"子目录b", "specs/b/login.md", "specs", "b_login"},
		{"空root退化basename", "specs/form.md", "", "form"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := specSlug(c.specPath, c.specsRoot); got != c.want {
				t.Errorf("specSlug(%q,%q) = %q, want %q", c.specPath, c.specsRoot, got, c.want)
			}
		})
	}
}

// TestExportEndToEnd 综合 Bug #1-#5:构造 form.md 等价 spec 导出,断言生成内容。
func TestExportEndToEnd(t *testing.T) {
	cwd := t.TempDir()
	specPath := filepath.Join(cwd, "specs", "form.md")
	spec := &markdown.Spec{
		Title: "表单全流程覆盖",
		Steps: []markdown.Step{
			{Index: 1, Text: "打开 http://127.0.0.1:18090/form.html", Kind: markdown.StepKindAction},
			{Index: 2, Text: `验证 "AutoQA Form Fixture" 文字可见`, Kind: markdown.StepKindAssertion},
			{Index: 3, Text: "点击 Submit 按钮", ExpectedResult: "Form submitted successfully", Kind: markdown.StepKindAction},
		},
	}
	if err := ExportPlaceholder(cwd, "out", specPath, spec, filepath.Join(cwd, "specs")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(cwd, "out", "test_form.py"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	checks := map[string]string{
		"函数名":   "def test_form(page: Page) -> None:",
		"goto拼接": `page.goto(os.environ.get("AUTOQA_BASE_URL", "http://127.0.0.1:18090") + "/form.html")`,
		"断言标题": `expect(page.get_by_text("AutoQA Form Fixture").first).to_be_visible()`,
		"Expected原文": `expect(page.get_by_text("Form submitted successfully").first).to_be_visible()`,
	}
	for name, sub := range checks {
		if !strings.Contains(s, sub) {
			t.Errorf("%s: 生成文件不含 %q\n--- 文件内容 ---\n%s", name, sub, s)
		}
	}
}
