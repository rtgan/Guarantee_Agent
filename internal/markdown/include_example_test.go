package markdown

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIncludeExampleFixture 用 testdata/steps + testdata/specs/login_flow.md
// 验证 README 宣称的 include: 用法在真实文件上可展开并解析。
func TestIncludeExampleFixture(t *testing.T) {
	root := "../../testdata"
	out, err := ExpandIncludes(root, mustRead(t, filepath.Join(root, "specs", "login_flow.md")))
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderTemplate(out, map[string]string{
		"LOGIN_BASE_URL": "http://127.0.0.1:18090",
		"USERNAME":       "alice",
		"PASSWORD":       "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := Parse(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Steps) != 5 {
		t.Fatalf("steps = %d, want 5", len(spec.Steps))
	}
	if spec.Steps[2].ExpectedResult != "仪表盘可见" {
		t.Fatalf("login fragment expected = %q", spec.Steps[2].ExpectedResult)
	}
	if spec.Steps[3].Kind != StepKindAssertion {
		t.Fatalf("step 4 kind = %s, want assertion", spec.Steps[3].Kind)
	}
	if spec.Steps[4].ExpectedResult != "再次显示登录页" {
		t.Fatalf("step 5 expected = %q", spec.Steps[4].ExpectedResult)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
