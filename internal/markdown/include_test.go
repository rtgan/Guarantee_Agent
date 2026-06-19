package markdown

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandIncludes(t *testing.T) {
	root := t.TempDir()
	stepsDir := filepath.Join(root, "steps")
	if err := os.MkdirAll(stepsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stepsDir, "login.md"), []byte("1. Fill username\n2. Fill password\n3. Click login\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := ExpandIncludes(root, "## Preconditions\n- ready\n\n## Steps\ninclude: login\n4. Verify dashboard\n")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Steps) != 4 {
		t.Fatalf("steps = %d", len(spec.Steps))
	}
}

func TestResolveIncludePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveIncludePath(root, "../escape"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	if _, err := ResolveIncludePath(root, ""); err == nil {
		t.Fatal("expected empty name rejection")
	}
}
