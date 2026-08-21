---
type: Decision
resource: .github/workflows/security.yml
title: The security badge could not go red for a finding
description: Every one of security.yml's eleven steps carried continue-on-error, so the Security Scans badge was green by construction; govulncheck is now strict and the remaining ten are fail-open by decision.
tags: [ci, security, gates]
status: stable
generated: {by: claude/opus-5, at: 2026-08-21T00:00:00Z}
sources:
  - id: workflow-security
    resource: .github/workflows/security.yml
    title: "Security Scans workflow"
    last_modified: 2026-08-21T00:00:00Z
---

# What was green by construction

`security.yml` ran eleven steps — govulncheck, Semgrep, git-secrets, OWASP Dependency-Check, Trivy
filesystem, Trivy image, Snyk and four SARIF uploads — and every one of them carried
`continue-on-error: true`. No finding from any scanner could fail the job, so the Security Scans
badge at `README.md:51` reported the workflow's ability to check out a repository and nothing else.
`docker-publish.yml:258` declines to scan on the grounds that "Trivy and other scans run in
dedicated workflows (security.yml)".

Same shape as [The lint gate had never run](the-lint-gate-had-never-run.md): a gate that is wired,
scheduled, and incapable of the outcome it advertises.

# Why govulncheck was the one to make strict

`make vuln` is already hard-failing at `ci.yml:88`, so a red govulncheck costs nothing that push and
PR runs do not already cost. What the CI copy does not give is recurrence: `ci.yml` fires only on
push, pull request and dispatch. `security.yml`'s `0 2 * * *` cron is the only thing here that
re-scans unchanged code against a vulnerability database that moves daily — which is the case a
strict scheduled scan exists for.

The step installs `govulncheck@latest` rather than a pin, deliberately. Pinning the binary does not
make the verdict reproducible, because the vulnerability database is fetched at run time either way;
the pin that mattered was `okfrules`, whose version decides a verdict on its own.

# A red step reddens the badge and blocks nothing

`deploy.yml`'s `security-gate` job blocks a deploy on CI, on Docker Publish and on Codecov coverage
— five `exit 1`s between `:207` and `:332`. Security Scans is not among them: it is read at `:336`
through `print_workflow_status`, under the comment "Soft, non-blocking checks: log latest status for
visibility", and that function returns 0 whether the run is missing, red or green. So making
govulncheck strict changed what the badge can say and not what can ship. Turning it into a
deployment gate is a separate decision with a different blast radius, and this one does not make it.

The step does run under `set -euo pipefail`, so a `curl` or `jq` failure inside the function still
aborts it. It is non-blocking on a red scan, not unconditionally non-failing.

# Why the other ten stay fail-open

`SEMGREP_APP_TOKEN` and `SNYK_TOKEN` are both set, so those steps do run and do produce findings —
the earlier guess that they were unconfigured no-ops was wrong. They are third-party scanners whose
finding sets change without a commit in this repo; making them blocking hands an outside vendor a
veto over merging. They stay advisory, and the badge is now honest about exactly one thing:
this module has no known Go vulnerability.
