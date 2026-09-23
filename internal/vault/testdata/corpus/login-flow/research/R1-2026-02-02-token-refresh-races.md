---
schema_version: 1
id: login-flow/R1-2026-02-02-token-refresh-races
kind: research
title: "Token refresh races"
date: 2026-01-01
author: Test Author
session: login-flow
repos: []
state: active
---

# Token refresh races

## Symptom

Users are logged out when two concurrent requests both refresh the access token. Refresh token rotation invalidates the first refresh token as soon as the second request uses it.

## Cause

The HTTP client has no single-flight guard around refresh, so parallel requests each start their own rotation.
