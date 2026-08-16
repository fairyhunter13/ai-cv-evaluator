---
type: Defect
resource: internal/adapter/queue/redpanda/integrated_evaluation_handler.go
title: Scores are fabricated from unrelated fields when the model omits them
description: calculateCVMatchRateFromAnalysis and calculateProjectScoreFromAnalysis derive a score from skill counts and complexity words, and it is returned as if the model produced it.
tags: [scoring, llm, correctness]
status: stable
generated: {by: claude-opus-5, at: 2026-08-17}
---

# What is wrong

When the LLM response carries no `cv_match_rate` or `project_score`, the handler computes one:

- `calculateCVMatchRateFromAnalysis` — `len(technical_skills)/10`, else `experience_years/5`, else a
  `project_complexity` word mapped to `0.9`/`0.7`/`0.5`/`0.6`.
- `calculateProjectScoreFromAnalysis` — `len(technologies)/5*10`, else the same complexity words
  mapped to `9`/`7`/`5`/`6`.

None of these are the rubric. Listing ten technologies scores 10/10 on a rubric whose five
parameters are correctness, code quality, resilience, documentation and creativity.

# Why it matters more than a wrong number

The fabricated value is written to `results` through the same path as a model-produced one, and the
row has no column that distinguishes them — see
[Candidate evaluation](../computations/candidate-evaluation.md). A caller reading
`project_score: 8` cannot tell whether a reviewer model assessed the project or whether the
candidate listed four technologies.

# The two honest shapes

Either the missing field is a failure (the job fails and says why), or the derived value is stored
with a flag naming it as derived. Silently substituting is the one shape that cannot be audited.
