package provider

import (
	"context"
	"fmt"

	"stagewise/internal/model"
)

// Provider generates a learning plan for a topic.
type Provider interface {
	Name() string
	Plan(ctx context.Context, topic string, opts Options) (*model.Plan, error)
}

// Options tunes plan generation.
type Options struct {
	Model       string
	MaxStages   int
	ExtraPrompt string
}

var registry = map[string]func() Provider{}

// ValidModes lists accepted stage modes.
var ValidModes = map[string]bool{"auto": true, "hybrid": true, "manual": true}

// Register adds a provider constructor.
func Register(name string, fn func() Provider) {
	registry[name] = fn
}

// Get returns a provider by name.
func Get(name string) (Provider, error) {
	fn, ok := registry[name]
	if !ok {
		names := []string{}
		for k := range registry {
			names = append(names, k)
		}
		return nil, fmt.Errorf("unknown provider %q (available: %v)", name, names)
	}
	return fn(), nil
}
