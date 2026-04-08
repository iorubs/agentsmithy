---
sidebar_position: 4
---

# Iterative Refinement

A `loop` agent that keeps improving its output against a quality bar,
gated by guards (e.g. minimum length, required tool call, required
keywords) that re-prompt with corrective feedback when the check fails.

> _Available once the `loop` kind and the guard system land._
