---
schema_version: 1
id: offline-mode/R1-2026-04-01-list-merge-conflicts
kind: research
title: "List merge conflicts"
date: 2026-01-01
author: Test Author
session: offline-mode
repos: []
state: active
---

# List merge conflicts

## Behaviour

Concurrent offline mutations to the same list are merged on reconnect. Last-writer-wins drops one device's changes when both devices edit one item.

## Options

A CRDT with per-item vector clocks keeps both mutations and resolves the conflict without losing data.
