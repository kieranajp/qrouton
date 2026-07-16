# qrouton prompt evaluations

The evaluation harness runs qrouton's current prompt assets in isolated synthetic
multi-repository sessions. It invokes installed Claude and Codex CLIs using their
existing local authentication, records normalized observable traces, applies
deterministic assertions, and optionally asks the other provider to judge each
run.

Run the complete suite:

```sh
go run ./cmd/qrouton-eval
```

Run one structural case without a cross-judge:

```sh
go run ./cmd/qrouton-eval \
  --runner codex \
  --scenario skip-implementation \
  --no-judge
```

Useful flags include `--samples`, `--assets-dir`, `--claude-model`,
`--codex-model`, `--timeout`, and `--output`. Results default to a timestamped
directory under `eval/results/`, which is ignored by Git.

Compare two completed runs without invoking a model:

```sh
go run ./cmd/qrouton-eval compare eval/results/OLD eval/results/NEW
```

Authenticated smoke tests are opt-in and never run during a normal test pass:

```sh
QROUTON_EVAL_SMOKE=1 go test ./internal/evalharness -run Smoke
```
