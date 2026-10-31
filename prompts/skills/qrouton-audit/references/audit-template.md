---
kind: audit
title: <title>
target:
tells: [duplicated, dead, homegrown, volkswagen]
---

# <title>

## Summary

> <what was audited, how many findings per tell, then the findings by impact: one sentence per high and medium finding, high first>

## Duplicated behaviour or facts

> <findings where one behaviour or fact is expressed more than one way, kept in step only by convention>

## Dead or needlessly defensive code

> <findings where code is unreachable, or guards a case that cannot occur>

## Homegrown library or platform behaviour

> <findings where code redoes what a well-known library or the platform provides, installed or not>

## Tests that pass or fail for the wrong reason

> <findings where a test never runs, cannot fail, dodges the code it names, was weakened to pass, pins a detail a harmless change breaks or repeats another test, or where shipped code behaves differently under test>

## Checked and deliberate

> <candidates judged deliberate, one line per candidate with its location and the tie, written reason or weighed install cost that keeps it>

## Tools

> <each tool or reading pass: name, version, command, mode, candidate count, traps checked, any error; then the full inventory, one row per import, built-in, global, internal module or kind of shared machinery over the threshold, with its file count and outcome; then each library category and its outcome>
