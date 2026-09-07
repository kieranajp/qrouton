---
name: qrouton-development
description: Implement or debug qrouton's Go/Wails/Svelte application, prompt assets, build or release tooling, and repository checks. Do not use for operating qrouton on an unrelated project.
---

# Develop qrouton

Read the nearest `AGENTS.md` before changing code. Keep dependency direction and generated-source ownership intact: change prompt sources under `prompts/`, frontend sources rather than generated assets, and the bridge generator rather than `src/lib/bridge/generated.js`.

Start verification at the touched boundary: focused Go package tests, `npm run test:unit` for frontend logic, or `npm run test:browser` for visible behavior. Use `make comment-check` for the mechanical comment gate; it checks shape, not whether prose earns its place.

Before handoff, run the repository gate:

```sh
GOCACHE=/tmp/qrouton-go-cache make check
```

Build and release commands remain those in the Makefile and packaging scripts; do not infer that a release path runs the full repository gate.
