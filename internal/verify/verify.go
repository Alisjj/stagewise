package verify

import (
	"fmt"
	"time"

	"stagewise/internal/model"
	"stagewise/internal/runner"
)

// RunStage executes every planned check for one stage.
func RunStage(ctx *runner.Context, st model.Stage) []model.CheckResult {
	var out []model.CheckResult
	for i, c := range st.Checks {
		name := c.Name
		if name == "" {
			name = fmt.Sprintf("check %d", i+1)
		}
		timeout := time.Duration(c.TimeoutSeconds * float64(time.Second))
		start := time.Now()
		var r runner.CommandOutput
		var detail string
		switch {
		case len(c.Command) > 0:
			r = ctx.Run(c.Command, timeout, "")
			detail = fmt.Sprintf("exit=%d", r.ReturnCode)
		case c.Shell != "":
			r = ctx.RunShell(c.Shell, timeout)
			detail = fmt.Sprintf("exit=%d (shell)", r.ReturnCode)
		default:
			out = append(out, ctx.Result(name, false, "check has neither command nor shell", start, nil, nil))
			continue
		}
		out = append(out, ctx.Result(name, r.ReturnCode == 0, detail, start, r.Stdout, r.Stderr))
	}
	return out
}
