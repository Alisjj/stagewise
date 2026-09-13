package planner

import (
	"fmt"
	"strings"
)

// SystemPrompt instructs the model to return strict plan JSON only.
func SystemPrompt(maxStages int) string {
	return fmt.Sprintf(`You break any learning topic into verifiable stages for a CLI tool called stagewise.

Return STRICT JSON only, no fences, no commentary, matching this schema:
{"topic": string, "stages": [{"number": int (1-based, sequential), "title": string,
"objective": string, "contract": string, "acceptance": [string],
"mode": "auto"|"hybrid"|"manual", "minimum_evidence": int,
"checks": [{"name": string, "command": [string], "timeout_seconds": number}]}]}

Rules:
- At most %d stages, ordered so each unlocks the next.
- "auto": every claim is checkable by running "command" (argv array, no shell) in the project dir; exit 0 passes.
- "hybrid": runnable checks plus minimum_evidence >= 1 (human confirms understanding).
- "manual": conceptual work; checks may be empty; minimum_evidence >= 1.
- Keep commands portable (sh, go, python3, ls) with timeout_seconds 10-120.
- Only use the "shell" key instead of "command" when a pipe/redirect is essential; prefer argv.`, maxStages)
}

// UserPrompt wraps the topic and any extra instructions.
func UserPrompt(topic, extra string) string {
	var b strings.Builder
	b.WriteString("Topic to break down:\n" + topic)
	if strings.TrimSpace(extra) != "" {
		b.WriteString("\n\nAdditional instructions:\n" + extra)
	}
	return b.String()
}
