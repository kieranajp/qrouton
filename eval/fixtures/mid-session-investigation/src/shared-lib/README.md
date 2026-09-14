# Shared library

Reference-only retry and deadline conventions. Every service in the estate is
expected to take its per-attempt timeout and its backoff schedule from here
rather than rolling its own.
