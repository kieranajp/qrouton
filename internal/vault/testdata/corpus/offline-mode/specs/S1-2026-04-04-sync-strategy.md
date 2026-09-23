---
schema_version: 1
id: offline-mode/S1-2026-04-04-sync-strategy
kind: spec
title: "Sync strategy"
date: 2026-01-01
author: Test Author
session: offline-mode
repos: []
state: active
---

# Sync strategy

## Decision

Sync pending writes through an outbox and merge list items with a CRDT.
