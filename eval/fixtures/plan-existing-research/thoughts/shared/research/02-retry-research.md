# Retry research

The active service owns retry policy in `calculator.go`. Context cancellation must stop retries immediately. Tests should cover success and cancellation.
