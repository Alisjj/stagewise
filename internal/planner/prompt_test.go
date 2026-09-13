package planner_test

import (
	"strings"
	"testing"

	"stagewise/internal/planner"
)

func TestSystemPromptGuardrails(t *testing.T) {
	p := planner.SystemPrompt(8)
	for _, want := range []string{
		"STRICT JSON only",
		"MODE DECISION",
		"CHECK AUTHORING",
		"NEVER emit commands",
		"no curl|sh",
		"At most 8 stages",
		"UserPrompt",
	} {
		if want == "UserPrompt" {
			continue // covered below
		}
		if !strings.Contains(p, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
	u := planner.UserPrompt("Learn Go slices", "focus on append")
	if !strings.Contains(u, "Learn Go slices") || !strings.Contains(u, "focus on append") {
		t.Errorf("user prompt drops input: %q", u)
	}
}
