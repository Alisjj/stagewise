package planjson

import (
	"encoding/json"
	"fmt"
	"strings"

	"stagewise/internal/model"
)

// Parse extracts the first {...} object (tolerating fences and chatter),
// validates it, renumbers stages sequentially, and defaults blank modes.
func Parse(s string) (*model.Plan, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	if i := strings.LastIndex(s, "}"); i >= 0 {
		s = s[:i+1]
	}
	var plan model.Plan
	if err := json.Unmarshal([]byte(s), &plan); err != nil {
		return nil, fmt.Errorf("plan is not valid JSON: %w", err)
	}
	if len(plan.Stages) == 0 {
		return nil, fmt.Errorf("plan contains no stages")
	}
	for i := range plan.Stages {
		plan.Stages[i].Number = i + 1
		if plan.Stages[i].Mode == "" {
			plan.Stages[i].Mode = "manual"
		}
	}
	return &plan, nil
}
