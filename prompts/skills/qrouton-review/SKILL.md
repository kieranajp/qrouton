---
name: qrouton-review
description: Review, audit, or perform a pre-merge diff review of the qrouton repository. Do not use to implement the change under review.
---

# Review qrouton changes

Keep the review read-only. Establish the requirement and relevant invariants, then inspect the diff and its callers for correctness, regressions, missing tests, and unsupported claims.

Check generated-source ownership whenever generated assets, bridge names, or prompt discovery are involved. Check launch/eval parity when either path changes. Treat automated comment findings as shape evidence only; whether a comment is necessary and clear remains review judgment.

Run the cheapest focused verification that exercises the changed boundary, followed by `GOCACHE=/tmp/qrouton-go-cache make check` when the environment supports it. Report findings first and explicitly name any UI, platform, authenticated eval, or other evidence that was not run.
