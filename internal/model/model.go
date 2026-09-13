package model

// Check is one AI-planned verification step.
type Check struct {
	Name           string   `json:"name"`
	Command        []string `json:"command,omitempty"`
	Shell          string   `json:"shell,omitempty"`
	TimeoutSeconds float64  `json:"timeout_seconds,omitempty"`
}

// Stage is one learning stage from the AI plan.
type Stage struct {
	Number          int      `json:"number"`
	Title           string   `json:"title"`
	Objective       string   `json:"objective,omitempty"`
	Contract        string   `json:"contract,omitempty"`
	Acceptance      []string `json:"acceptance,omitempty"`
	Mode            string   `json:"mode"` // auto | hybrid | manual
	MinimumEvidence int      `json:"minimum_evidence,omitempty"`
	Checks          []Check  `json:"checks,omitempty"`
}

// Plan is the full AI-generated breakdown for a topic.
type Plan struct {
	Topic  string  `json:"topic"`
	Stages []Stage `json:"stages"`
}

// CheckResult is the outcome of one executed check.
type CheckResult struct {
	Name            string  `json:"name"`
	Passed          bool    `json:"passed"`
	Detail          string  `json:"detail"`
	DurationSeconds float64 `json:"duration_seconds"`
	Stdout          string  `json:"stdout"`
	Stderr          string  `json:"stderr"`
}
