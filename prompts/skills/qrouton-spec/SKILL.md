---
name: qrouton-spec
description: Internally capture design decisions during the Plan part of qrouton's workflow. Use when research exists and meaningful product or technical choices must be aligned before drafting the tactical plan.
---

# Align the design

The user experiences this as Planning, not a separate workflow phase.

1. Read the relevant research summary and follow its workstream links.
2. Discuss only decisions that change behavior, scope, risk, or architecture. State the trade-offs and make implicit choices explicit. Do not manufacture a checkpoint for routine implementation detail.
3. Delegate code inspection or prior-art searches to a planning lead or specialist; request concise conclusions rather than raw exploration.
4. Have a planning lead draft `thoughts/shared/specs/S<n>-<date>-<slug>.md` when the work benefits from a durable design record. It states the desired end state, the resolved decisions with their rejected alternatives, scope in and out, and the open questions.
5. Ask the user to review the decisions when changing them later would cause substantial rework. For small or already-explicit work, proceed without ceremony.

The spec records what and why. File-by-file execution belongs in the tactical plan.

Pass this to the lead, and hold the returned spec to it:

{{artifact-discipline}}
