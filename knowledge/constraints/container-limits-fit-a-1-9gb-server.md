---
type: Constraint
resource: docker-compose.prod.yml
title: Container limits are sized to a 1.9 GB server
description: Production runs on 1.9 GB RAM; limits summed to ~8.7 GB and caused heavy swapping, and were rewritten to ~2.4 GB against measured actuals.
tags: [deployment, memory, capacity]
status: stable
generated: {by: claude-opus-5, at: 2026-08-17}
---

# The budget

The production host has **1.9 GB of RAM and 1.9 GB of swap**. Container limits once summed to
~8.7 GB and the machine ran with 948 MB of swap in use. They were rewritten to ~2.4 GB total on
2026-03-27, each limit set from measured actual usage plus headroom [^1] — backend 128M (16 MB
actual), worker 128M (19 MB), Postgres 192M with `shared_buffers` cut 256→64 MB, Redpanda 384M with
its memory flag at 256M and `smp 1`, Qdrant 128M (9 MB), Tika 192M (73 MB).

Two server-level settings belong to the same decision: `vm.swappiness=10` and
`vm.overcommit_memory=0`.

# The rule this leaves

**A new limit is derived from a measurement on this host, not from a default.** The service
defaults are all one to two orders of magnitude above what these workloads use, and the sum, not
any single limit, is what breaks. Adding a service means re-checking the total against 1.9 GB.

This is also the constraint that eventually removed the observability stack from production — see
[Observability was deprecated in production](../decisions/observability-was-deprecated-in-production.md).
Prometheus, Grafana, Loki, Jaeger, OTEL, Promtail and cAdvisor accounted for roughly 1.1 GB of the
2.4 GB after right-sizing.

[^1]: commit `2b86d70`, `fix: right-size container memory limits for 1.9GB server`.
