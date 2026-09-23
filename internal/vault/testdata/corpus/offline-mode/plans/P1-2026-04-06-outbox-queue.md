---
schema_version: 1
id: offline-mode/P1-2026-04-06-outbox-queue
kind: plan
title: "Outbox queue"
date: 2026-01-01
author: Test Author
session: offline-mode
repos: []
state: active
---

# Outbox queue

## Phase 1

Store pending writes in an outbox table and replay them in order when the device reconnects.
