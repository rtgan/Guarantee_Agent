package markdown

import "testing"

func TestParseMarkdownSpecExpectedAndAssertion(t *testing.T) {
	spec, err := Parse(`# Login

## Preconditions
- App is available

## Steps
1. Navigate to {{BASE_URL}}
   - Expected: Welcome
2. Verify Welcome text is visible
`)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Title != "Login" {
		t.Fatalf("title = %q", spec.Title)
	}
	if len(spec.Preconditions) != 1 || spec.Preconditions[0] != "App is available" {
		t.Fatalf("preconditions = %#v", spec.Preconditions)
	}
	if len(spec.Steps) != 2 {
		t.Fatalf("steps = %d", len(spec.Steps))
	}
	if spec.Steps[0].ExpectedResult != "Welcome" {
		t.Fatalf("expected = %q", spec.Steps[0].ExpectedResult)
	}
	if spec.Steps[1].Kind != StepKindAssertion {
		t.Fatalf("kind = %q", spec.Steps[1].Kind)
	}
}

func TestParseRequiresPreconditions(t *testing.T) {
	_, err := Parse("## Steps\n1. Navigate\n")
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*ParseError)
	if !ok || pe.Code != "MARKDOWN_MISSING_PRECONDITIONS" {
		t.Fatalf("err = %#v", err)
	}
}
