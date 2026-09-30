---
schema_version: 1
id: login-flow/P1-2026-02-08-refresh-lock
kind: plan
title: "Refresh lock"
date: 2026-01-01
author: Test Author
session: login-flow
repos: []
state: active
---

# Refresh lock

## Change

Wrap token refresh in a single-flight lock so parallel requests wait for one rotation.
