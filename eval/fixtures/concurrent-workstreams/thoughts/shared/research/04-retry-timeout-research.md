# Retry timeout research

## Summary

The current implementation needs a context-aware timer and deterministic cancellation tests.

## How should cancellation bound retry timing?

Through a caller-supplied context: the retry loop should stop as soon as that context is done rather than sleeping out its remaining attempts.
