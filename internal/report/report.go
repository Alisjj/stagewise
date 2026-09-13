package report

import (
	"fmt"
	"sort"
	"strings"

	"stagewise/internal/model"
	"stagewise/internal/store"
)

// Markdown builds the progress report.
func Markdown(plan *model.Plan, s *store.Store) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Stagewise Report: %s\n\n", plan.Topic)
	passed := 0
	for _, st := range plan.Stages {
		if s.IsPassed(st.Number) {
			passed++
		}
	}
	fmt.Fprintf(&b, "**Progress:** %d/%d stages passed\n\n", passed, len(plan.Stages))
	b.WriteString("| Stage | Title | Mode | Status | Evidence |\n|---:|---|---|---|---:|\n")
	keys := append([]model.Stage{}, plan.Stages...)
	sort.Slice(keys, func(i, j int) bool { return keys[i].Number < keys[j].Number })
	for _, st := range keys {
		status, _ := s.Stage(st.Number)["status"].(string)
		if status == "" {
			status = "ready"
		}
		ev, _ := s.Stage(st.Number)["evidence"].([]any)
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %d |\n", st.Number, st.Title, st.Mode, status, len(ev))
	}
	return b.String()
}
