---
name: qrspi-plan
description: QRSPI phase P — turn an approved spec into a tactical implementation plan of vertical phases, each shipping something testable with its own verification. Second half of the SP macro-phase. Use when a session has a spec but no plan.
---

# QRSPI — Plan

You are running **P**, the second half of SP. Output: a tactical plan that an implementer (you, later, or a fresh session) executes phase by phase. The spec is binding; this is the "how".

## Vertical phases (the one rule that matters)

Slice the work **vertically**: each phase ships one usable, testable increment end-to-end, and carries **its own verification**. Do **not** produce a horizontal plan (all the models, then all the handlers, then all the tests) — horizontal plans get skim-read and diverge from the code (the plan-illusion failure mode). A phase with no way to verify it isn't a phase.

## Steps

1. **Read the spec fully**, and read the code it will touch — enough to make every phase concrete against real files. Resolve open questions now; a plan with unresolved questions isn't done.
2. **Agree the phasing** with the user before writing detail — a short list of vertical phases and what each delivers. Adjust order/granularity on feedback.
3. **Write the plan** to `thoughts/shared/plans/P<n>-<YYYY-MM-DD>-<slug>.md`, `<n>` = (max existing `P` number) + 1. Match the shape of `thoughts/shared/plans/P002-*.md`:
   - A one-line pointer to the spec (binding), Overview, Current State, Desired End State, What We're NOT Doing, Implementation Approach.
   - Then numbered phases. **Each phase**: the concrete changes (files, `path:line`), and a **Verify** block — automated checks (`make …`, `go test`, etc.) and/or a manual/live check. Prefer runnable commands.
4. **Present** the plan location and ask for review. When approved, propose `qrspi-implement`.

## Guardrails

- Concrete over aspirational: name files and commands, not "wire it up".
- If a phase can't state how it's verified, re-slice until it can.
