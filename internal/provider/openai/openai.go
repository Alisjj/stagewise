// Package openai implements provider.Provider over the OpenAI
// Chat Completions API using only the standard library.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"stagewise/internal/model"
	"stagewise/internal/planner"
	"stagewise/internal/planjson"
	"stagewise/internal/provider"
)

// OpenAI plans via POST {baseURL}/chat/completions.
type OpenAI struct {
	HTTP    *http.Client
	BaseURL string
}

func init() {
	provider.Register("openai", func() provider.Provider {
		return &OpenAI{HTTP: &http.Client{Timeout: 120 * time.Second}}
	})
}

func (o *OpenAI) Name() string { return "openai" }

func baseURL(o *OpenAI) string {
	if o.BaseURL != "" {
		return o.BaseURL
	}
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		return v
	}
	return "https://api.openai.com/v1"
}

func (o *OpenAI) Plan(ctx context.Context, topic string, opts provider.Options) (*model.Plan, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
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
		"model":       modelName,
		"max_tokens":  4000,
		"temperature": 0.2,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": planner.SystemPrompt(maxStages)},
			{"role": "user", "content": planner.UserPrompt(topic, opts.ExtraPrompt)},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL(o)+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openai API %d: %s", resp.StatusCode, truncate(string(raw), 500))
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}
	plan, err := planjson.Parse(envelope.Choices[0].Message.Content)
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
