---
type: Decision
resource: docker-compose.prod.yml
title: Observability was deprecated in production, and the docs still advertise it
description: In June 2026 the observability stack moved behind a compose profile and trace export was removed, but README and docs/observability.md still publish live Grafana, Prometheus and Jaeger URLs.
tags: [observability, deployment, stale-docs]
status: stable
generated: {by: claude/opus-5, at: 2026-08-17T00:00:00Z}
verified:
  - { by: process:okf-verify, at: 2026-08-21T02:26:57Z }
sources:
  - id: observability-teardown
    resource: commits 0f826d2, 87520ff, 88bfaa5, f6b0502
    title: "The four commits that put observability behind a compose profile"
    last_modified: 2026-06-12T00:00:00Z
---

# What changed

Over 2026-06-12 the observability services were put behind a compose `profiles: ["observability"]`
gate, dropped from the deploy path, and trace export was switched off because the collector was
gone [^observability-teardown]. The driver was the same one behind
[Container limits are sized to a 1.9 GB server](../constraints/container-limits-fit-a-1-9gb-server.md):
Prometheus, Grafana, Loki, Jaeger, OTEL, Promtail and cAdvisor together are most of a budget the
host does not have.

# The consequence that outlived the change

`README.md` and `docs/observability.md` still document live production Grafana, Prometheus and
Jaeger endpoints. Someone debugging a production incident will follow them, find nothing, and not
know whether the stack is down or was never there.

This is also why `slog` output is not a substitute for a stored receipt — see
[Candidate evaluation](../computations/candidate-evaluation.md). The reasoning that made logs
sufficient assumed a collector that is no longer deployed.

[^observability-teardown]: commits `0f826d2`, `87520ff`, `88bfaa5`, `f6b0502`, all 2026-06-12.
