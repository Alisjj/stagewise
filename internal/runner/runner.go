package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"stagewise/internal/model"
)

// CommandOutput is the captured result of one command.
type CommandOutput struct {
	Argv       []string
	ReturnCode int
	Stdout     []byte
	Stderr     []byte
	Duration   time.Duration
}

// Context runs planned checks inside the project.
type Context struct {
	Project string
	Workdir string
	Timeout time.Duration
	TempDir string
	Env     []string
}

// New resolves paths and creates a scratch dir.
func New(project, workdir string, timeout time.Duration) (*Context, error) {
	proj, err := filepath.Abs(project)
	if err != nil {
		return nil, err
	}
	wd := workdir
	if wd == "" {
		wd = "."
	}
	if !filepath.IsAbs(wd) {
		wd = filepath.Join(proj, wd)
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	tmp, err := os.MkdirTemp("", "stagewise-")
	if err != nil {
		return nil, err
	}
	return &Context{Project: proj, Workdir: wd, Timeout: timeout, TempDir: tmp, Env: os.Environ()}, nil
}

// Close removes the scratch dir.
func (c *Context) Close() { os.RemoveAll(c.TempDir) }

// Run executes argv; timeout expiry yields exit 124.
func (c *Context) Run(argv []string, timeout time.Duration, cwd string) CommandOutput {
	if timeout <= 0 {
		timeout = c.Timeout
	}
	if cwd == "" {
		cwd = c.Workdir
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = c.Env
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	rc := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			rc = 124
		} else {
			rc = 127
		}
	}
	return CommandOutput{argv, rc, so.Bytes(), se.Bytes(), time.Since(start)}
}

// RunShell executes an explicit shell string (opt-in per check).
func (c *Context) RunShell(script string, timeout time.Duration) CommandOutput {
	return c.Run([]string{"sh", "-c", script}, timeout, "")
}

// Result builds a timed CheckResult.
func (c *Context) Result(name string, passed bool, detail string, start time.Time, stdout, stderr []byte) model.CheckResult {
	return model.CheckResult{
		Name: name, Passed: passed, Detail: detail,
		DurationSeconds: time.Since(start).Seconds(),
		Stdout:          string(stdout), Stderr: string(stderr),
	}
}
