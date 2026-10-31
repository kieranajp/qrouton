# Testing first run

First run shows when config.json has no `welcomed` marker and no session is open. You can reach it with a throwaway build without touching your own config or the workbench you are working in.

## Build a debug copy

Only one workbench may run per user, found through `/tmp/qrouton-sock/<uid>`. Give the debug copy its own socket directory by building from a detached worktree with `socketRoot` changed:

```sh
git worktree add --detach /tmp/qr-debug HEAD
cd /tmp/qr-debug
sed -i '' 's#"/tmp/qrouton-sock"#"/tmp/qrouton-sock-debug"#' internal/workbench/strings.go
make front
go build -o /tmp/qr-firstrun/qrouton-debug .
```

A plain `go build` has no production tag, so the web inspector is available.

## Launch it

Run it from a directory outside any session, with its own config and cache and no qrouton environment:

```sh
mkdir -p /tmp/qr-firstrun/{config,cache,cwd}
cd /tmp/qr-firstrun/cwd
env -u QROUTON_ROOT -u QROUTON_ORGS \
  XDG_CONFIG_HOME=/tmp/qr-firstrun/config \
  XDG_CACHE_HOME=/tmp/qr-firstrun/cache \
  /tmp/qr-firstrun/qrouton-debug
```

Launched from inside a session folder, or from a terminal a workbench opened, it opens that session instead of first run.

Pick a dummy sessions root such as `/tmp/qr-firstrun/root`. The copy shares your home directory, so Install Linear writes your real `~/.linear/coding-tools.json` and the GitHub check uses your real `gh` login.

After Save, the answers are in `/tmp/qr-firstrun/config/qrouton/config.json`. To test under an override, add `QROUTON_ROOT=…` to the `env` line.

## Reset and clean up

Quit the copy, then `rm -rf /tmp/qr-firstrun/config/*` and launch again for a fresh first run.

When done:

```sh
rm -rf /tmp/qr-firstrun /tmp/qrouton-sock-debug
git worktree remove --force /tmp/qr-debug
```
