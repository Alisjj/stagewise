// Package anthropic implements provider.Provider over the Anthropic
// Messages API using only the standard library.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"stagewise/internal/model"
	"stagewise/internal/planjson"
	"stagewise/internal/planner"
	"stagewise/internal/provider"
)

type Anthropic struct {
	HTTP *http.Client
}

func init() { provider.Register("anthropic", func() provider.Provider { return &Anthropic{HTTP: &http.Client{Timeout: 120 * time.Second}} }) }

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Plan(ctx context.Context, topic string, opts provider.Options) (*model.Plan, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}
	modelName := opts.Model
	if modelName == "" {
		modelName = os.Getenv("STAGEWISE_MODEL")
	}
	if modelName == "" {
		return nil, fmt.Errorf("model is required (--model or STAGEWISE_MODEL)")
	}
	maxStages := opts.MaxStages
	if maxStages <= 0 {
		maxStages = 12
	}
	body, _ := json.Marshal(map[string]any{
		"model": modelName, "max_tokens": 4000,
		"system":   planner.SystemPrompt(maxStages),
		"messages": []map[string]string{{"role": "user", "content": planner.UserPrompt(topic, opts.ExtraPrompt)}},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("anthropic API %d: %s", resp.StatusCode, truncate(string(raw), 500))
	}
	var envelope struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	var text strings.Builder
	for _, c := range envelope.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	plan, err := planjson.Parse(text.String())
	if err != nil {
		return nil, err
	}
	plan.Topic = topic
	return plan, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
