package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"stagewise/internal/model"
)

// UTCNow returns ISO-8601 UTC, matching the durablemux-verifier layout.
func UTCNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// Paths locates the .stagewise tree for a project.
func Paths(project string) (root, meta, cfg, stages, progress string) {
	r, _ := filepath.Abs(project)
	m := filepath.Join(r, ".stagewise")
	return r, m, filepath.Join(m, "config.json"), filepath.Join(m, "stages.json"), filepath.Join(m, "progress.json")
}

// LoadPlan reads stages.json.
func LoadPlan(path string) (*model.Plan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p model.Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SavePlan writes stages.json (pretty).
func SavePlan(path string, p *model.Plan) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Store persists progress.json.
type Store struct {
	Path string
	Data map[string]any
}

// New loads path when present, else starts empty.
func New(path string) (*Store, error) {
	s := &Store{Path: path, Data: map[string]any{"version": 1, "stages": map[string]any{}}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(raw, &s.Data); err != nil {
		return nil, err
	}
	if _, ok := s.Data["stages"]; !ok {
		s.Data["stages"] = map[string]any{}
	}
	return s, nil
}

// Save writes atomically via a .tmp rename.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

func (s *Store) stages() map[string]any {
	m, ok := s.Data["stages"].(map[string]any)
	if !ok {
		m = map[string]any{}
		s.Data["stages"] = m
	}
	return m
}

// Stage returns (creating) the per-stage record.
func (s *Store) Stage(number int) map[string]any {
	m := s.stages()
	v, ok := m[strconv.Itoa(number)].(map[string]any)
	if !ok {
		v = map[string]any{}
		m[strconv.Itoa(number)] = v
	}
	return v
}

// IsPassed reports status == "passed".
func (s *Store) IsPassed(number int) bool { return s.Stage(number)["status"] == "passed" }

// RecordRun appends history and updates status.
func (s *Store) RecordRun(number int, payload map[string]any) error {
	item := s.Stage(number)
	item["last_run"] = payload
	hist, _ := item["history"].([]any)
	item["history"] = append(hist, payload)
	if payload["passed"] == true {
		item["status"] = "passed"
	} else {
		item["status"] = "failed"
	}
	item["updated_at"] = UTCNow()
	return s.Save()
}

// AddEvidence appends an evidence record.
func (s *Store) AddEvidence(number int, ev map[string]any) error {
	item := s.Stage(number)
	list, _ := item["evidence"].([]any)
	item["evidence"] = append(list, ev)
	item["updated_at"] = UTCNow()
	return s.Save()
}

// Approve marks manual review approved.
func (s *Store) Approve(number int, note string) error {
	item := s.Stage(number)
	item["manual_approval"] = map[string]any{"approved": true, "note": note, "at": UTCNow()}
	item["updated_at"] = UTCNow()
	return s.Save()
}

// EvidenceCount counts evidence entries.
func (s *Store) EvidenceCount(number int) int {
	list, _ := s.Stage(number)["evidence"].([]any)
	return len(list)
}

// Approved reports manual approval.
func (s *Store) Approved(number int) bool {
	m, _ := s.Stage(number)["manual_approval"].(map[string]any)
	return m["approved"] == true
}
