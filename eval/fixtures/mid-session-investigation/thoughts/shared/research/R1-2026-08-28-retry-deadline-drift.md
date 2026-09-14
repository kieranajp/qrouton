---
kind: research
title: The retry deadline drifts between configuration and the wire
---

# The retry deadline drifts between configuration and the wire

## Summary

Operators set `ACCOUNT_RETRY_DEADLINE_MS` and get something else. Three of last
month's gateway-timeout incidents were traced to a request whose observed
budget was several times the configured one, and one to a request whose budget
was half of it. The configured value is a starting point that several layers
then adjust, and nobody has an inventory of the layers.

What is settled: the value is read once at boot, the estate ceiling in the
shared library is real and does clamp, and no incident has been caused by the
ledger path.

What is not settled is the inventory itself. This document records the symptom
and the two known adjusters; it does not claim to be complete.

## The configured value is read once

`LoadConfig` parses the environment at boot and never re-reads it, so a running
process cannot be retuned without a restart. Nothing in the incident timelines
contradicts that, and the operators' reported values match the boot logs.

## Two adjusters are known

Caller tier scales the budget by a fixed multiplier, and a bulk caller is
allowed a multiple of the operator's figure rather than a fraction of it. That
alone accounts for the "several times" shape of three incidents.

The shared library clamps to an estate-wide ceiling and floor. That explains
the halved case only if the tier multiplier had already pushed the value past
the ceiling, which the timelines neither confirm nor rule out.

## Open Questions

> How many places actually read, clamp, or override the deadline, across both
> repositories? Two are named above. Whether that is the whole set is unknown,
> and the inventory is what any fix depends on.

> Does the inbound request header path interact with the tier multiplier, or
> are they applied to different contexts?

> Is the per-attempt timeout derived from the same budget as the overall retry
> window, or from a separate figure?
