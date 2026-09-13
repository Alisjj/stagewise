// Package external runs plan providers as child processes.
//
// Protocol: stagewise writes a JSON request to the child's stdin and reads a
// plan (same schema as stages.json, topic optional) from its stdout:
//
//	{"topic": string, "max_stages": int, "extra": string, "model": string}
//
// Any executable in any language can be a provider: call whatever model,
// SDK, or local logic you like and print the plan. Exit 0 with valid JSON
// on stdout; diagnostics go on stderr. This keeps the verifier decoupled
// from every vendor API.
package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"stagewise/internal/model"
	"stagewise/internal/planjson"
	"stagewise/internal/provider"
)

// External is a subprocess-backed provider.
type External struct {
	Argv    []string
	Timeout time.Duration
}

// Command builds an External provider; argv[0] is the executable.
func Command(argv []string, timeout time.Duration) *External {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &External{Argv: argv, Timeout: timeout}
}

func (e *External) Name() string { return "external" }

// Request is what the child receives on stdin.
type Request struct {
	Topic     string `json:"topic"`
	MaxStages int    `json:"max_stages"`
	Extra     string `json:"extra,omitempty"`
	Model     string `json:"model,omitempty"`
}

func (e *External) Plan(ctx context.Context, topic string, opts provider.Options) (*model.Plan, error) {
	if len(e.Argv) == 0 {
		return nil, fmt.Errorf("external provider needs a command")
	}
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	maxStages := opts.MaxStages
	if maxStages <= 0 {
		maxStages = 12
	}
	in, _ := json.Marshal(Request{Topic: topic, MaxStages: maxStages, Extra: opts.ExtraPrompt, Model: opts.Model})
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.Argv[0], e.Argv[1:]...)
	cmd.Stdin = bytes.NewReader(in)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("provider command failed: %v: %s", err, truncate(strings.TrimSpace(se.String()), 300))
	}
	plan, err := planjson.Parse(so.String())
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}
	for _, st := range plan.Stages {
		if st.Title == "" {
			return nil, fmt.Errorf("provider stage %d has no title", st.Number)
		}
		if !provider.ValidModes[st.Mode] {
			return nil, fmt.Errorf("provider stage %d has bad mode %q", st.Number, st.Mode)
		}
	}
	if plan.Topic == "" {
		plan.Topic = topic
	}
	return plan, nil
}

// Split parses a --provider-cmd string into argv (quotes respected).
func Split(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if cur.Len() > 0 || quote != 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
		case quote == 0 && unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
