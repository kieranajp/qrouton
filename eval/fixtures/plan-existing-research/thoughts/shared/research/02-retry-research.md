# Retry research

The active service owns retry policy in `calculator.go`. Its current `Retry` function bounds attempts but accepts no context, so cancellation and deadlines cannot stop an in-flight retry sequence. Timeout support should use a caller-supplied context, preserve the existing attempt bound, and stop retries immediately when that context is done. Tests should cover success, exhaustion, and cancellation or deadline expiry.
