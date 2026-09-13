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

Add a provider by implementing `provider.Provider` and calling `provider.Register`.

## Checks

Plans declare checks as argv arrays (`command: ["go", "test", "./..."]`, exit 0 passes, no shell) or explicit `shell:` strings when a pipe is essential. Review any AI-generated plan (`plan`, `show N`, `plan --json`) before verifying.

## Commands

```bash
stagewise init --topic "..." [--provider stub|anthropic] [--model ...] [--force]
stagewise plan [--json]
stagewise status [--json]
stagewise show N
stagewise verify next|N [--force]
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
