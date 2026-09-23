---
schema_version: 1
id: offline-mode/R2-2026-04-03-sqlite-migrations
kind: research
title: "SQLite migrations"
date: 2026-01-01
author: Test Author
session: offline-mode
repos: []
state: active
---

# SQLite migrations

## Room

Room runs schema migrations on device when the database version increases. A missing migration path falls back to destructive recreation unless disabled.
