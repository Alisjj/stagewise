package planjson_test

import (
	"testing"

	"stagewise/internal/planjson"
)

func TestParseStrict(t *testing.T) {
	p, err := planjson.Parse(`{"topic":"x","stages":[{"number":5,"title":"A","mode":"auto","checks":[{"name":"c","command":["true"]}]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Stages[0].Number != 1 {
		t.Errorf("expected renumber to 1, got %d", p.Stages[0].Number)
	}
}

func TestParseFencedAndDefaults(t *testing.T) {
	p, err := planjson.Parse("here you go:\n```json\n{\"topic\":\"x\",\"stages\":[{\"title\":\"A\"}]}\n```\nbye")
	if err != nil {
		t.Fatal(err)
	}
	if p.Stages[0].Mode != "manual" {
		t.Errorf("expected default manual, got %q", p.Stages[0].Mode)
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := planjson.Parse(`{"topic":"x","stages":[]}`); err == nil {
		t.Error("expected error for empty stages")
	}
}
