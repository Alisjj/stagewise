package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"stagewise/internal/model"
	"stagewise/internal/report"
	"stagewise/internal/runner"
	"stagewise/internal/store"
	"stagewise/internal/verify"

	_ "stagewise/internal/provider/anthropic"
	_ "stagewise/internal/provider/openai"
	_ "stagewise/internal/provider/stub"
	"stagewise/internal/provider/external"

	"stagewise/internal/provider"
)

// Version is the CLI version.
const Version = "0.1.0"

// Run dispatches argv (without program name) and returns exit code.
func Run(argv []string) int {
	project := "."
	rest := []string{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--project" && i+1 < len(argv) {
			project, i = argv[i+1], i+1
		} else if v, ok := strings.CutPrefix(a, "--project="); ok {
			project = v
		} else if a == "--version" || a == "-V" {
			fmt.Println(Version)
			return 0
		} else {
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: stagewise [--project DIR] <init|plan|status|show|verify|evidence|approve|report|reset>")
		return 2
	}
	cmd, args := rest[0], rest[1:]
	switch cmd {
	case "init":
		return cmdInit(project, args)
	case "plan":
		return cmdPlan(project, args)
	case "status":
		return cmdStatus(project, args)
	case "show":
		return cmdShow(project, args)
	case "verify":
		return cmdVerify(project, args)
	case "evidence":
		return cmdEvidence(project, args)
	case "approve":
		return cmdApprove(project, args)
	case "report":
		return cmdReport(project, args)
	case "reset":
		return cmdReset(project, args)
	case "--help", "-h", "help":
		fmt.Println("stagewise — AI-planned, stage-verified learning for any project")
		fmt.Println("commands: init plan status show verify evidence approve report reset")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		return 2
	}
}

type loaded struct {
	root     string
	meta     string
	plan     *model.Plan
	st       *store.Store
	planPath string
}

func loadAll(project string) (*loaded, error) {
	root, meta, _, stagesPath, progPath := store.Paths(project)
	plan, err := store.LoadPlan(stagesPath)
	if err != nil {
		return nil, fmt.Errorf("no plan found (run init --topic \"...\"): %w", err)
	}
	st, err := store.New(progPath)
	if err != nil {
		return nil, err
	}
	return &loaded{root, meta, plan, st, stagesPath}, nil
}

func findStage(plan *model.Plan, n int) *model.Stage {
	for i := range plan.Stages {
		if plan.Stages[i].Number == n {
			return &plan.Stages[i]
		}
	}
	return nil
}

func sortedStages(plan *model.Plan) []model.Stage {
	out := append([]model.Stage{}, plan.Stages...)
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

func stageStatus(plan *model.Plan, n int, st *store.Store) string {
	if st.IsPassed(n) {
		return "passed"
	}
	if n > 1 && findStage(plan, n-1) != nil && !st.IsPassed(n-1) {
		return "locked"
	}
	if s, ok := st.Stage(n)["status"].(string); ok && s != "" {
		return s
	}
	return "ready"
}

func cmdInit(project string, args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	topic := fs.String("topic", "", "")
	providerName := fs.String("provider", "stub", "")
	providerCmd := fs.String("provider-cmd", "", "")
	providerTimeout := fs.Float64("provider-timeout", 120, "")
	modelName := fs.String("model", "", "")
	maxStages := fs.Int("max-stages", 12, "")
	extra := fs.String("extra", "", "")
	force := fs.Bool("force", false, "")
	workdir := fs.String("workdir", ".", "")
	timeout := fs.Float64("timeout", 60, "")
	_ = fs.Parse(args)
	if *topic == "" {
		fmt.Fprintln(os.Stderr, "usage: init --topic \"Learn X\" [--provider anthropic] [--provider-cmd \".../my-provider\"] [--model ...] [--force]")
		return 2
	}
	root, meta, cfgPath, stagesPath, progPath := store.Paths(project)
	if _, err := os.Stat(stagesPath); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "Already initialised: %s (use --force to regenerate)\n", stagesPath)
		return 2
	}
	p, err := provider.Get(*providerName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	providerLabel := p.Name()
	if *providerCmd != "" {
		argv := external.Split(*providerCmd)
		if len(argv) == 0 {
			fmt.Fprintln(os.Stderr, "empty --provider-cmd")
			return 2
		}
		p = external.Command(argv, time.Duration(*providerTimeout*float64(time.Second)))
		providerLabel = "external: " + argv[0]
	}
	fmt.Printf("Planning %q with %s...\n", *topic, providerLabel)
	plan, err := p.Plan(context.Background(), *topic, provider.Options{Model: *modelName, MaxStages: *maxStages, ExtraPrompt: *extra})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cfg := map[string]any{"working_directory": *workdir, "timeout_seconds": *timeout, "provider": *providerName, "provider_cmd": *providerCmd, "model": *modelName}
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(cfgPath, append(raw, '\n'), 0o644)
	if err := store.SavePlan(stagesPath, plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st, err := store.New(progPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_ = os.MkdirAll(filepath.Join(meta, "evidence"), 0o755)
	fmt.Printf("Planned %d stages for %q in %s\n", len(plan.Stages), plan.Topic, meta)
	fmt.Println("Review with: stagewise plan | show N — then: stagewise verify next")
	return 0
}

func cmdPlan(project string, args []string) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "")
	_ = fs.Parse(args)
	l, err := loadAll(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *asJSON {
		raw, _ := json.MarshalIndent(l.plan, "", "  ")
		fmt.Println(string(raw))
		return 0
	}
	fmt.Printf("Topic: %s (%d stages)\n\n", l.plan.Topic, len(l.plan.Stages))
	for _, st := range sortedStages(l.plan) {
		fmt.Printf("%02d  [%-6s]  %s\n", st.Number, st.Mode, st.Title)
	}
	return 0
}

func cmdStatus(project string, args []string) int {
	asJSON := false
	for _, a := range args {
		if a == "--json" {
			asJSON = true
		}
	}
	l, err := loadAll(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	stages := sortedStages(l.plan)
	if asJSON {
		rows := []map[string]any{}
		for _, st := range stages {
			rows = append(rows, map[string]any{"stage": st.Number, "title": st.Title, "mode": st.Mode, "status": stageStatus(l.plan, st.Number, l.st), "evidence": l.st.EvidenceCount(st.Number)})
		}
		raw, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(raw))
		return 0
	}
	passed := 0
	for _, st := range stages {
		if l.st.IsPassed(st.Number) {
			passed++
		}
	}
	fmt.Printf("Learning progress: %d/%d stages passed — %s\n\n", passed, len(stages), l.plan.Topic)
	markers := map[string]string{"passed": "✓", "ready": "→", "failed": "✗", "locked": "·"}
	for _, st := range stages {
		status := stageStatus(l.plan, st.Number, l.st)
		m, ok := markers[status]
		if !ok {
			m = "·"
		}
		fmt.Printf("%s %02d  %-7s  [%-6s]  %s\n", m, st.Number, status, st.Mode, st.Title)
	}
	return 0
}

func cmdShow(project string, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: show <stage>")
		return 2
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid stage")
		return 2
	}
	l, err := loadAll(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st := findStage(l.plan, n)
	if st == nil {
		fmt.Fprintf(os.Stderr, "Unknown stage: %d\n", n)
		return 2
	}
	fmt.Printf("Stage %d: %s\nmode: %s | status: %s\n\nObjective\n%s\n\nContract\n%s\n", st.Number, st.Title, st.Mode, stageStatus(l.plan, n, l.st), st.Objective, st.Contract)
	if len(st.Acceptance) > 0 {
		fmt.Println("\nAcceptance")
		for _, a := range st.Acceptance {
			fmt.Printf("- %s\n", a)
		}
	}
	if len(st.Checks) > 0 {
		fmt.Println("\nChecks")
		for _, c := range st.Checks {
			if len(c.Command) > 0 {
				fmt.Printf("- %s: %s\n", c.Name, strings.Join(c.Command, " "))
			} else {
				fmt.Printf("- %s: (shell) %s\n", c.Name, c.Shell)
			}
		}
	}
	fmt.Printf("\nRequired evidence: %d; recorded: %d\n", st.MinimumEvidence, l.st.EvidenceCount(n))
	return 0
}

func resolveStage(value string, l *loaded) (int, error) {
	if value == "next" {
		for _, st := range sortedStages(l.plan) {
			if !l.st.IsPassed(st.Number) {
				return st.Number, nil
			}
		}
		return 0, fmt.Errorf("all stages are already passed")
	}
	n, err := strconv.Atoi(value)
	if err != nil || findStage(l.plan, n) == nil {
		return 0, fmt.Errorf("unknown stage: %s", value)
	}
	return n, nil
}

func readConfig(project string) (workdir string, timeout float64) {
	_, _, cfgPath, _, _ := store.Paths(project)
	workdir, timeout = ".", 60
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return
	}
	var cfg map[string]any
	if json.Unmarshal(raw, &cfg) != nil {
		return
	}
	if w, ok := cfg["working_directory"].(string); ok && w != "" {
		workdir = w
	}
	if f, ok := cfg["timeout_seconds"].(float64); ok && f > 0 {
		timeout = f
	}
	return
}

func cmdVerify(project string, args []string) int {
	force := false
	stageArg := ""
	for _, a := range args {
		if a == "--force" {
			force = true
		} else if stageArg == "" {
			stageArg = a
		}
	}
	if stageArg == "" {
		fmt.Fprintln(os.Stderr, "usage: verify <stage|next> [--force]")
		return 2
	}
	l, err := loadAll(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	number, err := resolveStage(stageArg, l)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	st := findStage(l.plan, number)
	if number > 1 && findStage(l.plan, number-1) != nil && !l.st.IsPassed(number-1) && !force {
		fmt.Fprintf(os.Stderr, "Stage %d is locked. Pass stage %d first, or use --force.\n", number, number-1)
		return 2
	}
	fmt.Printf("Verifying stage %d: %s [%s]\n", number, st.Title, st.Mode)
	workdir, timeout := readConfig(project)
	ctx, err := runner.New(l.root, workdir, time.Duration(timeout*float64(time.Second)))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer ctx.Close()
	results := verify.RunStage(ctx, *st)
	for _, r := range results {
		if r.Passed {
			fmt.Printf("PASS  %s\n", r.Name)
		} else {
			fmt.Printf("FAIL  %s\n", r.Name)
		}
		if r.Detail != "" {
			fmt.Printf("      %s\n", r.Detail)
		}
		if !r.Passed && strings.TrimSpace(r.Stderr) != "" {
			fmt.Printf("      stderr: %s\n", truncate(strings.ReplaceAll(strings.TrimSpace(r.Stderr), "\n", " | "), 500))
		}
	}
	checksPassed := true
	for _, r := range results {
		checksPassed = checksPassed && r.Passed
	}
	evidenceOK := l.st.EvidenceCount(number) >= st.MinimumEvidence
	approvalOK := st.Mode == "auto" || l.st.Approved(number)
	if st.Mode != "auto" {
		fmt.Printf("%s  evidence %d/%d\n", passNeed(evidenceOK), l.st.EvidenceCount(number), st.MinimumEvidence)
		fmt.Printf("%s  review approval\n", passNeed(approvalOK))
	}
	passed := checksPassed && evidenceOK && approvalOK
	dicts := []any{}
	for _, r := range results {
		dicts = append(dicts, map[string]any{"name": r.Name, "passed": r.Passed, "detail": r.Detail,
			"duration_seconds": math.Round(r.DurationSeconds*10000) / 10000, "stdout": r.Stdout, "stderr": r.Stderr})
	}
	if err := l.st.RecordRun(number, map[string]any{"at": store.UTCNow(), "passed": passed, "stage": number,
		"mode": st.Mode, "checks": dicts, "evidence_count": l.st.EvidenceCount(number), "manual_approved": l.st.Approved(number)}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if passed {
		fmt.Printf("\nStage %d: PASSED\n", number)
		return 0
	}
	fmt.Printf("\nStage %d: NOT PASSED\n", number)
	if st.Mode != "auto" && !approvalOK {
		fmt.Printf("After reviewing the evidence, run: stagewise approve %d --note \"...\"\n", number)
	}
	return 1
}

func cmdEvidence(project string, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: evidence <stage> --file X | --command Y [--note Z]")
		return 2
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid stage")
		return 2
	}
	fs := flag.NewFlagSet("evidence", flag.ContinueOnError)
	fileFlag := fs.String("file", "", "")
	cmdFlag := fs.String("command", "", "")
	note := fs.String("note", "", "")
	_ = fs.Parse(args[1:])
	l, err := loadAll(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if findStage(l.plan, n) == nil {
		fmt.Fprintf(os.Stderr, "Unknown stage: %d\n", n)
		return 2
	}
	evDir := filepath.Join(l.meta, "evidence", fmt.Sprintf("stage-%02d", n))
	if err := os.MkdirAll(evDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	record := map[string]any{"at": store.UTCNow(), "note": *note}
	if *fileFlag != "" {
		src := *fileFlag
		if !filepath.IsAbs(src) {
			src, _ = filepath.Abs(src)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Evidence file not found: %s\n", src)
			return 2
		}
		target := filepath.Join(evDir, stamp+"-"+filepath.Base(src))
		if err := os.WriteFile(target, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		rel, _ := filepath.Rel(l.root, target)
		record["type"], record["path"] = "file", rel
	} else if *cmdFlag != "" {
		c := exec.Command("sh", "-lc", *cmdFlag)
		c.Dir = l.root
		var so, se strings.Builder
		c.Stdout, c.Stderr = &so, &se
		rc := 0
		if err := c.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			} else {
				rc = 1
			}
		}
		target := filepath.Join(evDir, stamp+"-command.txt")
		content := fmt.Sprintf("$ %s\n\n[exit] %d\n\n[stdout]\n%s\n[stderr]\n%s", *cmdFlag, rc, so.String(), se.String())
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		rel, _ := filepath.Rel(l.root, target)
		record["type"], record["command"], record["exit_code"], record["path"] = "command", *cmdFlag, rc, rel
	} else {
		fmt.Fprintln(os.Stderr, "Provide --file or --command")
		return 2
	}
	if err := l.st.AddEvidence(n, record); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Recorded evidence for stage %d: %v\n", n, record["path"])
	return 0
}

func cmdApprove(project string, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: approve <stage> --note \"...\"")
		return 2
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid stage")
		return 2
	}
	fs := flag.NewFlagSet("approve", flag.ContinueOnError)
	note := fs.String("note", "", "")
	_ = fs.Parse(args[1:])
	if *note == "" {
		fmt.Fprintln(os.Stderr, "usage: approve <stage> --note \"...\"")
		return 2
	}
	l, err := loadAll(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	st := findStage(l.plan, n)
	if st == nil {
		fmt.Fprintf(os.Stderr, "Unknown stage: %d\n", n)
		return 2
	}
	if l.st.EvidenceCount(n) < st.MinimumEvidence {
		fmt.Fprintf(os.Stderr, "Stage %d needs at least %d evidence item(s) before approval.\n", n, st.MinimumEvidence)
		return 2
	}
	if err := l.st.Approve(n, *note); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Approved stage %d. Run verify %d to finalise it.\n", n, n)
	return 0
}

func cmdReport(project string, args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	output := fs.String("output", "", "")
	_ = fs.Parse(args)
	l, err := loadAll(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	out := *output
	if out == "" {
		out = filepath.Join(l.meta, "report.md")
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(l.root, out)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(out, []byte(report.Markdown(l.plan, l.st)+"\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Wrote report to %s\n", out)
	return 0
}

func cmdReset(project string, args []string) int {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	stageFlag := fs.Int("stage", 0, "")
	_ = fs.Parse(args)
	if *stageFlag == 0 {
		fmt.Fprintln(os.Stderr, "Specify --stage N")
		return 2
	}
	l, err := loadAll(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if m, ok := l.st.Data["stages"].(map[string]any); ok {
		delete(m, strconv.Itoa(*stageFlag))
	}
	if err := l.st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Reset stage %d\n", *stageFlag)
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func passNeed(ok bool) string {
	if ok {
		return "PASS"
	}
	return "NEED"
}
