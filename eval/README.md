# qrouton prompt evaluations

The evaluation harness runs qrouton's current prompt assets in isolated synthetic
multi-repository sessions. It invokes installed Claude and Codex CLIs using their
existing local authentication, records normalized observable traces, applies
deterministic assertions, and by default runs blinded pairwise judging when both
providers are selected. Claude and Codex each judge the pair with reversed A/B
ordering; reports include consensus wins, ties, and judge disagreement.

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

Pairwise judging requires `--runner all`. Use `--no-judge` for fast structural
runs that apply only deterministic assertions.

Compare two completed runs without invoking a model:

```sh
go run ./cmd/qrouton-eval compare eval/results/OLD eval/results/NEW
```

Authenticated smoke tests are opt-in and never run during a normal test pass:

```sh
QROUTON_EVAL_SMOKE=1 go test ./internal/evalharness -run Smoke
```
