package plan

import (
	"testing"

	"guarantee-agent/internal/config"
)

func TestIsURLInScope(t *testing.T) {
	cfg := config.PlanConfig{ExploreScope: "site", ExcludePatterns: []string{"/admin"}}
	if !IsURLInScope("http://app.example.com/home", cfg) {
		t.Fatal("site scope should include /home")
	}
	if IsURLInScope("http://app.example.com/admin", cfg) {
		t.Fatal("exclude pattern should drop /admin")
	}

	cfg = config.PlanConfig{ExploreScope: "site", IncludePatterns: []string{"/products*"}}
	if !IsURLInScope("http://app.example.com/products/1", cfg) {
		t.Fatal("include prefix pattern should match")
	}
	if IsURLInScope("http://app.example.com/cart", cfg) {
		t.Fatal("non-include URL should be out of scope when include patterns set")
	}

	cfg = config.PlanConfig{ExploreScope: "focused"}
	if IsURLInScope("http://app.example.com/anything", cfg) {
		t.Fatal("focused scope without include patterns should exclude")
	}
}

func TestExtractRelativeURL(t *testing.T) {
	if got := ExtractRelativeURL("http://app.example.com/a/b?x=1#frag"); got != "/a/b?x=1#frag" {
		t.Fatalf("got %q", got)
	}
}
