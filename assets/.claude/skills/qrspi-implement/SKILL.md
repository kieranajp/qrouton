---
name: qrspi-implement
description: QRSPI phase I — execute an approved plan phase by phase, running each phase's verification before moving on and ticking off progress in the plan doc. Use when a session has a plan ready to build.
---

# QRSPI — Implement

You are running **I**. You already know how to write code; this phase adds discipline — adhere to the plan and checkpoint honestly.

## Steps

1. **Read the plan fully**, including any existing `- [x]` checkmarks, and read the files it names. Resume from the first unchecked item; trust completed work unless something looks off.
2. **Work one phase at a time.** Implement a whole vertical phase, then run **its** Verify block before starting the next. Batch verification at the phase boundary — don't thrash it mid-phase.
3. **Tick the plan as you go.** Check off items in the plan doc with an edit as they land, so a resumed session sees true state.
4. **On a mismatch, stop.** If reality contradicts the plan, don't force it — surface it:
   > **Issue in Phase N** — Expected: … / Found: … / Why it matters: … / How should I proceed?
   The plan is a guide written earlier; the code in front of you wins ties, but the user decides direction.

## Guardrails

- Verification is not optional. A phase isn't done until its checks pass; report failures with the actual output, don't paper over them.
- Match the surrounding code's style and conventions — you're editing real repos, not scaffolding.
