---
type: Defect
resource: .github/workflows/deploy.yml
title: The disk-full outage, and the five safeguards it bought
description: Image cleanup used double-quoted awk "{print $2}", so the shell expanded $2 to empty and nothing was ever pruned; 216 images filled the 40 GB disk.
tags: [deployment, outage, docker]
status: stable
generated: {by: claude/opus-5, at: 2026-08-17T00:00:00Z}
sources:
  - id: commit-da0eb09
    resource: commit da0eb09
    title: "fix: prevent disk-full outage with 5 permanent safeguards"
    last_modified: 2026-03-27
---

# Root cause

The deploy workflow's image cleanup ran `awk "{print $2}"` in double quotes. The **shell** expanded
`$2` before `awk` saw it, to the empty string, so the pipeline named no image and pruned nothing.
216 images accumulated — 19.5 GB — and filled the 40 GB disk to 100% [^commit-da0eb09].

The failure mode is the one to remember: the broken command succeeded every time. Nothing in the
deploy log distinguished "cleaned up 0 images because there were none" from "cleaned up 0 images
because the filter was empty".

# The five safeguards

1. `docker image prune -af` replaces the hand-built cleanup — no field extraction to get wrong.
2. Docker log rotation: 10 MB max, 3 files.
3. Weekly Docker cleanup cron, Sunday 03:00.
4. `health-monitor.sh` checks disk every 30 s, auto-cleans at 80% and treats 90% as an emergency.
5. `proxy_max_temp_file_size 0` in nginx globally, so the proxy never writes to disk and stays
   serving through a full one. This also removed the per-service buffer overrides in
   `authelia.conf`.

# The rule

A cleanup step that can do nothing must say so. Prefer a command with no interpolated field over a
pipeline that computes what to delete, and where a computation is unavoidable, assert on the count
it produced.

[^commit-da0eb09]: commit `da0eb09`, `fix: prevent disk-full outage with 5 permanent safeguards`.
