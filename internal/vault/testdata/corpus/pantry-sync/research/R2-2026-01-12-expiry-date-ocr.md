---
schema_version: 1
id: pantry-sync/R2-2026-01-12-expiry-date-ocr
kind: research
title: "Expiry date OCR"
date: 2026-01-01
author: Test Author
session: pantry-sync
repos: []
state: active
---

# Expiry date OCR

## Findings

ML Kit text recognition reads best-before dates from the label in about 90 ms. Dates printed with dot-matrix ink fail more often than embossed dates.

## Formats

Best-before dates appear as DD/MM/YY, as month abbreviations, or as a julian day code on tins.
