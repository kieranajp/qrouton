---
schema_version: 1
id: billing/R1-2026-05-01-chargeback-webhooks
kind: research
title: "Chargeback webhooks"
date: 2026-01-01
author: Test Author
session: billing
repos: []
state: active
---

# Chargeback webhooks

## Events

Stripe sends charge.dispute.created when a chargeback opens and charge.dispute.closed when it resolves. Each event carries the disputed amount, reason and evidence due date.

## Refunds

A refund issued after a chargeback opens does not close the dispute.
