---
name: qrspi-spec
description: Internally capture design decisions during the Plan part of qrouton's RPI workflow. Use when research exists and meaningful product or technical choices must be aligned before drafting the tactical plan.
---

# Align the design

The user experiences this as Planning, not a separate workflow phase.

1. Read the relevant research summary and follow its workstream links.
2. Discuss only decisions that change behavior, scope, risk, or architecture. State the trade-offs and make implicit choices explicit. Do not manufacture a checkpoint for routine implementation detail.
3. Delegate code inspection or prior-art searches to a planning lead or specialist; request concise conclusions rather than raw exploration.
4. Have a planning lead draft `thoughts/shared/specs/S<n>-<date>-<slug>.md` when the work benefits from a durable design record. Keep it at or below 200 lines and include desired end state, resolved decisions with rejected alternatives, scope in/out, and open questions. Summarize linked research instead of reproducing it, and run a final compression pass before presenting the artifact.
5. Ask the user to review the decisions when changing them later would cause substantial rework. For small or already-explicit work, proceed without ceremony.

The spec records what and why. File-by-file execution belongs in the tactical plan.
