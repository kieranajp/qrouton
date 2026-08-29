# Retry research

## Summary

The active service owns retry policy in `calculator.go`. Timeout support should use a caller-supplied context, preserve the existing attempt bound, and stop retries immediately when that context is done.

## Where is cancellation enforced?

Nowhere. The current `Retry` function bounds attempts but accepts no context, so cancellation and deadlines cannot stop an in-flight retry sequence.

## Which retry behavior is already tested?

Success and exhaustion. Cancellation and deadline expiry are not covered.
