// Package stub provides an offline deterministic provider for tests
// and for trying the workflow without an API key.
package stub

import (
	"context"
	"fmt"

	"stagewise/internal/model"
	"stagewise/internal/provider"
)

type Stub struct{}

func init() { provider.Register("stub", func() provider.Provider { return &Stub{} }) }

func (s *Stub) Name() string { return "stub" }

func (s *Stub) Plan(_ context.Context, topic string, opts provider.Options) (*model.Plan, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	_ = opts
	return &model.Plan{
		Topic: topic,
		Stages: []model.Stage{
			{
				Number: 1, Title: fmt.Sprintf("Orient: %s", topic),
				Objective: "Confirm the workspace and tooling are ready.",
				Contract:  "Baseline checks run green.",
				Acceptance: []string{"Workspace exists", "Tooling responds"},
				Mode:   "auto",
				Checks: []model.Check{{Name: "workspace check passes", Command: []string{"true"}}},
			},
			{
				Number: 2, Title: "First guided exercise",
				Objective: "Complete the first hands-on step and keep evidence.",
				Contract:  "Check passes and one evidence item is approved.",
				Acceptance: []string{"Do the exercise", "Record what you observed"},
				Mode: "hybrid", MinimumEvidence: 1,
				Checks: []model.Check{{Name: "exercise check passes", Command: []string{"true"}}},
			},
			{
				Number: 3, Title: "Write-up",
				Objective: "Summarise what you learned.",
				Contract:  "One reviewed evidence item is recorded.",
				Acceptance: []string{"Write the summary"},
				Mode: "manual", MinimumEvidence: 1,
			},
		},
	}, nil
}
