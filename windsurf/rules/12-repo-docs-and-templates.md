---
trigger: always_on
---

Standardize repository documentation, governance, and collaboration workflows to accelerate onboarding and ensure high-quality changes.

# Required Documentation
- `README.md`
  - Quick Start (local dev with docker compose and with `make run`).
  - Architecture overview (Clean Architecture layers, key packages).
  - Configuration (env vars table) and `.env.example` reference.
  - API overview with link to `api/openapi.yaml`.
  - Testing strategy and how to run unit/e2e/E2E suites.
  - Deployment overview (VPS steps and GitHub Actions).
- `docs/ARCHITECTURE.md`
  - Layered diagram, dependency rules, main flows (`/upload`, `/evaluate`, `/result/{id}`).
  - Data model (ERD), queues, vector DB collections.
  - Observability topology (OTEL → Collector → Jaeger; Prometheus scraping).
- `docs/CONTRIBUTING.md`
  - Dev setup, coding standards, linters, commit conventions, branching model.
  - How to add/modify migrations; how to create/use mocks.
  - How to update OpenAPI and re-generate clients (if any).
- `docs/SECURITY.md`
  - Vulnerability disclosure policy, secret handling, supported versions.
- `docs/STUDY_CASE.md`
  - Follow the study case submission template with the following sections:
    - Initial Plan
    - System & Database Design (API, schema/ERD, job queue/long-running handling)
    - LLM Integration (provider rationale, prompt design, chaining, RAG strategy)
    - Resilience & Error Handling (timeouts, retries, randomness control)
    - Edge Cases Considered (inputs/scenarios and how they were tested)
    - Results & Reflection (what worked, what didn’t, stability rationale)
    - Future Improvements (trade-offs and constraints)
  - Link this file from `README.md` for easy discovery.
- `docs/` (optional)
  - ADRs (Architecture Decision Records) for key choices (queue, vector DB, AI provider).
  - Runbooks for common ops tasks (restart, roll back, run migrations, rotate secrets).

# GitHub Hygiene
- PR Template: `.github/pull_request_template.md`
  - Sections: Summary, Screenshots/Logs (if applicable), Test Plan, Breaking Changes, Checklist (lint/tests/coverage), Linked Issues.
- Issue Templates: `.github/ISSUE_TEMPLATE/`
  - `bug_report.md`, `feature_request.md`.
- `CODEOWNERS` (optional) to enforce reviews.

# Conventions
- Commit messages: Conventional Commits (feat, fix, docs, refactor, test, chore, ci, build).
- Branch naming: `type/short-description` (e.g., `feat/evaluate-job-retries`).
- Changelog: maintain GitHub Releases; optionally `CHANGELOG.md` for highlights.

# README Structure (guidance)
- Badges (CI, Go version, license, coverage optional).
- Overview and architecture diagram.
- Quick Start (docker compose and local run with `make`).
- Configuration table (env var, description, default, required).
- API overview with link to `api/openapi.yaml` and curl examples.
- Testing (`make test`, `make test-int`, E2E); coverage instructions.
- Deployment (VPS via GitHub Actions); environment/secrets notes.
- Troubleshooting and FAQ.
- Name hygiene: Avoid sensitive or external brand terms in repository name, commits, or documentation.

# ARCHITECTURE.md Outline
- Context and goals of the system.
- Clean Architecture layers and dependencies.
- Domain model and key entities (uploads, jobs, results) with ERD.
- Component diagram (API, worker, DB, Redis, Qdrant, AI provider).
- Main flows:
  - `/upload` ingestion and text extraction.
  - `/evaluate` enqueue and worker pipeline.
  - `/result/{id}` read patterns and caching.
- Observability design (OTEL, Prometheus) and SLOs.
- Security boundaries and assumptions.

# ADR Template (docs/adr/NNNN-title.md)
```
# Title
Date: YYYY-MM-DD
Status: Proposed | Accepted | Superseded by NNNN

Context
We need to decide ...

Decision
We will ...

Consequences
Positive, negative, and risks.

Alternatives Considered
Option A, Option B, trade-offs.
```

# PR Template (\.github/pull_request_template.md)
```
## Summary

## Test Plan
- [ ] Unit tests
- [ ] E2E tests
- [ ] E2E tests

## Checklist
- [ ] Lint and vet pass
- [ ] Vulnerability scan clean
- [ ] Docs updated (README/ARCHITECTURE/OpenAPI)

## Breaking Changes

## Linked Issues
```

# Issue Templates (\.github/ISSUE_TEMPLATE/)
- bug_report.md
```
## Bug Report
**What happened**

**Expected behavior**

**Repro steps**

**Environment**
```
- feature_request.md
```
## Feature Request
**Problem to solve**

**Proposal**

**Alternatives**
```

# CODEOWNERS (example)
```
* @repo-owner
/internal/ @backend-leads
/deploy/ @devops-team
```

# Labels & Project Boards
- Use labels: `area/api`, `area/ai`, `bug`, `enhancement`, `security`, `ci`, `docs`.
- Optional GitHub Projects board to track roadmap and PR status.

# Versioning & Releases
- Semantic Versioning (MAJOR.MINOR.PATCH).
- Tag releases `vX.Y.Z`; generate release notes with highlights and breaking changes.
- Attach SBOM/signatures if publishing images.

# Contribution Workflow
- Fork-and-branch or branch-in-repo based on permissions.
- Create feature branches, open PR early as draft.
- Require at least one approving review; CI must be green.
- Squash merge with meaningful commit message.

# OpenAPI Maintenance
- Keep `api/openapi.yaml` current; add schemas for request/response and errors.
- Validate API handlers against OpenAPI in e2e tests.

# Quality & Gates
- Lint, tests, and coverage thresholds enforced in CI.
- Block merges on failing checks; require at least one approving review.

# Definition of Done (Docs)
- Core docs present and discoverable from `README.md`.
- PR and issue templates in place; contributors guided by `CONTRIBUTING.md`.
- OpenAPI maintained and referenced.
