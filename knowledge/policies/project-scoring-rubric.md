---
type: Policy
resource: internal/adapter/queue/redpanda/integrated_evaluation_handler.go
title: The project scoring rubric
description: Five weighted parameters scored 1-5 — correctness 30, code quality 25, resilience 20, documentation 15, creativity 10 — stated only inside a Go prompt literal.
tags: [scoring, rubric, product]
status: stable
generated: {by: claude/opus-5, at: 2026-08-17T00:00:00Z}
---

# The rubric

| Parameter | Weight | Scored |
|---|---|---|
| Correctness — prompt design and LLM chaining, RAG, requirements met, endpoints work, async job processing | 30% | 1–5 |
| Code Quality & Structure — modular, reusable, testable, tested, error handling | 25% | 1–5 |
| Resilience — failure handling and recovery | 20% | 1–5 |
| Documentation — setup instructions and explanations | 15% | 1–5 |
| Creativity — bonus features and innovation | 10% | 1–5 |

# Why this is filed as knowledge and not left in the code

This is the product's scoring policy — what the service is *for* — and it exists in exactly one
place: a concatenated string literal in `generateProjectEvaluationPrompt`, repeated a second time
inside the same literal as a JSON output template. Changing a weight means editing prose in two
halves of one Go string, and nothing compares them.

A second rubric arrives at runtime as the `scoringRubric` argument (RAG top-2 from the Qdrant
`scoring_rubric` collection) and is pasted into the prompt above this table under "Additional
Scoring Rubric". When the two disagree, the model decides, and nothing records that it did.

# What the weights do not do

They are stated to the model; nothing computes with them. The final `project_score ∈ [1,10]` comes
from a later refinement call, not from a weighted sum of the five 1–5 scores. No code asserts that
the returned score is consistent with the returned parameter scores.
