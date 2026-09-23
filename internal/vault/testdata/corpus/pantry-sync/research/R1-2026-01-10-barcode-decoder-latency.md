---
schema_version: 1
id: pantry-sync/R1-2026-01-10-barcode-decoder-latency
kind: research
title: "Barcode decoder latency"
date: 2026-01-01
author: Test Author
session: pantry-sync
repos: []
state: active
---

# Barcode decoder latency

## Summary

The ZXing decoder takes 180 ms per frame on low-end Android handsets such as the Moto E. Frame sampling at every third preview frame keeps the camera preview smooth.

## Autofocus

Continuous autofocus hunts on glossy labels. Locking focus after the first sharp frame cuts decode time by a third.
