package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
	bin := filepath.Join(t.TempDir(), "stagewise")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/stagewise")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func runCLI(t *testing.T, bin, project string, args ...string) (int, string, string) {
	t.Helper()
	full := append([]string{"--project", project}, args...)
	c := exec.Command(bin, full...)
	var so, se strings.Builder
	c.Stdout, c.Stderr = &so, &se
	rc := 0
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("run failed: %v", err)
		}
	}
	return rc, so.String(), se.String()
}

func TestStubInitAndVerify(t *testing.T) {
	bin := buildBinary(t)
	project := t.TempDir()
	if rc, _, se := runCLI(t, bin, project, "init", "--topic", "Learn Go slices", "--provider", "stub"); rc != 0 {
		t.Fatalf("init failed: %s", se)
	}
	if rc, so, _ := runCLI(t, bin, project, "status", "--json"); rc != 0 {
		t.Fatal("status failed")
	} else {
		var rows []map[string]any
		if err := json.Unmarshal([]byte(so), &rows); err != nil || len(rows) != 3 {
			t.Fatalf("expected 3 stages, got %q err=%v", so, err)
		}
	}
	// Stage 2 is locked before stage 1 passes.
	if rc, _, se := runCLI(t, bin, project, "verify", "2"); rc != 2 || !strings.Contains(strings.ToLower(se), "locked") {
		t.Fatalf("expected lock, rc=%d se=%s", rc, se)
	}
	if rc, _, se := runCLI(t, bin, project, "verify", "1"); rc != 0 {
		t.Fatalf("verify 1 failed: %s", se)
	}
	// Stage 2 is hybrid: needs evidence + approval.
	if rc, _, _ := runCLI(t, bin, project, "verify", "2"); rc == 0 {
		t.Fatal("stage 2 should not pass without evidence")
	}
	if rc, _, se := runCLI(t, bin, project, "evidence", "2", "--command", "echo learned", "--note", "test"); rc != 0 {
		t.Fatalf("evidence failed: %s", se)
	}
	if rc, _, se := runCLI(t, bin, project, "approve", "2", "--note", "good"); rc != 0 {
		t.Fatalf("approve failed: %s", se)
	}
	if rc, _, se := runCLI(t, bin, project, "verify", "2"); rc != 0 {
		t.Fatalf("verify 2 failed: %s", se)
	}
	if rc, _, se := runCLI(t, bin, project, "report"); rc != 0 {
		t.Fatalf("report failed: %s", se)
	}
}

func TestExternalProviderInit(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	// Any executable speaking plan-JSON on stdout is a provider.
	script := `#!/bin/sh
cat >/dev/null
cat <<'JSON'
{"stages": [{"title": "External stage", "objective": "o", "contract": "c",
"acceptance": ["a"], "mode": "auto", "checks": [{"name": "ok", "command": ["true"]}]}]}
JSON`
	prov := filepath.Join(dir, "my-provider.sh")
	if err := os.WriteFile(prov, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	if rc, _, se := runCLI(t, bin, project, "init", "--topic", "Whatever", "--provider-cmd", prov); rc != 0 {
		t.Fatalf("init via external provider failed: %s", se)
	}
	if rc, so, se := runCLI(t, bin, project, "verify", "1"); rc != 0 {
		t.Fatalf("verify 1 failed: %s %s", so, se)
	}
}

func writeProvider(t *testing.T, dir, name, title, checkJSON string) string {
	t.Helper()
	script := `#!/bin/sh
cat >/dev/null
cat <<'JSON'
{"stages": [{"title": "` + title + `", "objective": "o", "contract": "c",
"acceptance": ["a"], "mode": "auto", "checks": [` + checkJSON + `]}]}
JSON`
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAllowShellFalseBlocks(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	prov := writeProvider(t, dir, "sh-provider.sh", "Shell stage", `{"name": "piped", "shell": "echo hi | grep hi"}`)
	project := t.TempDir()
	if rc, _, se := runCLI(t, bin, project, "init", "--topic", "T", "--provider-cmd", prov); rc != 0 {
		t.Fatalf("init failed: %s", se)
	}
	if rc, so, _ := runCLI(t, bin, project, "verify", "1"); rc != 0 {
		t.Fatalf("shell check should pass when allowed: %s", so)
	}
	rc, so, _ := runCLI(t, bin, project, "verify", "1", "--allow-shell=false")
	if rc == 0 || !strings.Contains(so, "blocked") {
		t.Fatalf("expected blocked failure, rc=%d out=%s", rc, so)
	}
}

func TestPlanRegenerate(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	v1 := writeProvider(t, dir, "v1.sh", "Original title", `{"name": "ok", "command": ["true"]}`)
	v2 := writeProvider(t, dir, "v2.sh", "Regenerated title", `{"name": "ok", "command": ["true"]}`)
	project := t.TempDir()
	if rc, _, se := runCLI(t, bin, project, "init", "--topic", "T", "--provider-cmd", v1); rc != 0 {
		t.Fatalf("init failed: %s", se)
	}
	if rc, _, se := runCLI(t, bin, project, "plan", "--regenerate", "1", "--extra", "better", "--provider-cmd", v2); rc != 0 {
		t.Fatalf("regenerate failed: %s", se)
	}
	_, so, _ := runCLI(t, bin, project, "show", "1")
	if !strings.Contains(so, "Regenerated title") {
		t.Fatalf("expected new title, got: %s", so)
	}
	if _, err := os.Stat(filepath.Join(project, ".stagewise", "stages.json.bak")); err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
}
