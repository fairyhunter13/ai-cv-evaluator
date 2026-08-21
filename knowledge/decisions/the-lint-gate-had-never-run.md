---
type: Decision
resource: .golangci.yml
title: The lint gate had never run
description: A v2 version key over v1 directives, plus a pinned binary that was installed and then not invoked, meant golangci-lint never executed on this module until 2026-08-15.
tags: [lint, ci, toolchain]
status: stable
generated: {by: claude/opus-5, at: 2026-08-17T00:00:00Z}
verified:
  - { by: process:okf-verify, at: 2026-08-21T02:26:57Z }
sources:
  - id: commit-5c122fb
    resource: commit 5c122fb
    title: "ci: make the lint gate actually run"
    last_modified: 2026-08-15T00:00:00Z
---

# Two independent faults, each sufficient

`.golangci.yml` declared `version: 2` while every directive in it used the v1 schema. Separately,
the lint target installed a pinned binary into `./bin` and then invoked the bare name from `PATH`,
so whatever global `golangci-lint` happened to exist ran instead. v1 also cannot typecheck a
go1.25 module — it reported `undefined: pgx` in code `go build` compiles cleanly, which is the kind
of output that trains a reader to stop believing the tool [^commit-5c122fb].

Fixed by migrating the config to v2, pinning v2.12.2, and invoking the pinned path.

# What the first real run found

Five gosec findings, all first-time reports. Two are worth keeping:

- The DLQ cooldown requeue uses `context.WithoutCancel`, not `context.Background`: it must outlive
  the call that scheduled it, but should keep the caller's trace.
- G124 is excluded for `_test.go`. It targets `Set-Cookie` attributes, and a request-side
  `AddCookie` cannot carry them.

# The general lesson

A pinned tool that is installed and then invoked by bare name is not pinned. The install and the
invocation have to name the same path, and a green lint on a config the linter cannot parse looks
exactly like a green lint on clean code.
The same green appears one directory over in
[make test-e2e runs zero tests](../defects/make-test-e2e-runs-nothing.md), reached by a selector
that can match nothing.

[^commit-5c122fb]: commit `5c122fb`, `ci: make the lint gate actually run`.
