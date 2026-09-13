# Stagewise

AI-planned, stage-verified learning for any project. You describe what you want to learn at `init`; a provider breaks it into stages; the verifier runs each stage's checks, locks later stages until prerequisites pass, collects evidence for hybrid/manual stages, and writes a Markdown report.

```bash
stagewise --project ./myproject init --topic "Learn Go slices" --provider stub
stagewise --project ./myproject plan
stagewise --project ./myproject verify next
```

## Providers (pluggable)

- `stub` (default): offline 3-stage plan for trying the workflow.
- `anthropic`: generates a real plan via the Messages API. Requires `ANTHROPIC_API_KEY` and `--model` (or `STAGEWISE_MODEL`), e.g.:

```bash
export ANTHROPIC_API_KEY=...
stagewise --project ./myproject init --topic "Learn TCP in Go" \
  --provider anthropic --model <model-id> --max-stages 8 --force
```

- `openai`: Chat Completions API (`OPENAI_API_KEY` + `--model`/`STAGEWISE_MODEL`, optional `OPENAI_BASE_URL` for compatibles).
- **Anything else, no recompile**: `--provider-cmd` runs your own executable as the provider. It receives `{"topic", "max_stages", "extra", "model"}` on stdin and prints plan JSON (same schema as `.stagewise/stages.json`) on stdout, exit 0:

```bash
stagewise --project ./myproject init --topic "Learn TCP in Go" \
  --provider-cmd "./my-provider --model foo"
```

Add a compiled-in provider by implementing `provider.Provider` and calling `provider.Register`.

## Checks

Plans declare checks as argv arrays (`command: ["go", "test", "./..."]`, exit 0 passes, no shell) or explicit `shell:` strings when a pipe is essential. Review any AI-generated plan (`plan`, `show N`, `plan --json`) before verifying.

Lock shell checks out entirely — per run or permanently in `.stagewise/config.json` (`allow_shell`):

```bash
stagewise verify 3 --allow-shell=false
```

Blocked shell checks fail closed with a pointer to regenerate the plan.

## Plan surgery

Redo one bad stage without losing progress (history is keyed by stage number; the old plan is kept as `stages.json.bak`):

```bash
stagewise plan --regenerate 3 --extra "use table tests, no network"
```

Provider/model flags default to the ones stored at `init` and can be overridden per call.

## Commands

```bash
stagewise init --topic "..." [--provider stub|anthropic|openai] [--provider-cmd "..."] [--model ...] [--force]
stagewise plan [--json] [--regenerate N --extra "..."]
stagewise status [--json]
stagewise show N
stagewise verify next|N [--force] [--allow-shell=false]
stagewise evidence N --command "..." | --file ... [--note ...]
stagewise approve N --note "..."
stagewise report [--output ...]
stagewise reset --stage N
```

## Build / test

```bash
make build   # ./stagewise
go test ./...
```
