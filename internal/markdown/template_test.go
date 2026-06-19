package markdown

import "testing"

func TestRenderTemplateStrict(t *testing.T) {
	out, err := RenderTemplate("Open {{BASE_URL}}", map[string]string{"BASE_URL": "http://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "Open http://example.test" {
		t.Fatalf("out = %q", out)
	}
	if _, err := RenderTemplate("Open {{MISSING}}", map[string]string{}); err == nil {
		t.Fatal("expected missing variable error")
	}
	if _, err := RenderTemplate("Open {{BASE_URL}}", map[string]string{"BASE_URL": ""}); err == nil {
		t.Fatal("expected empty variable error")
	}
}
