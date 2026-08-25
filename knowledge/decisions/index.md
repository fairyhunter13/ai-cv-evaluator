# Decision

* [Observability was deprecated in production, and the docs still advertise
  it](observability-was-deprecated-in-production.md) - In June 2026 the observability stack moved
  behind a compose profile and trace export was removed. But README and docs/observability.md
  still publish live Grafana, Prometheus and Jaeger URLs.
* [The bundle gate asserts that it is installed, not only that the bundle
  passes](the-bundle-gate-asserts-that-it-is-installed.md) - Five arms fail. Two are
  lint-knowledge dropping -Werror, and the CI step reaching it being excused. Two more are the
  pre-commit hook non-executable in the index, and the okf install losing its pin. The fifth is a
  checker that accepts everything.
* [The lint gate had never run](the-lint-gate-had-never-run.md) - A v2 version key sat over v1
  directives, and a pinned binary was installed and then not invoked. So golangci-lint never
  executed on this module until 2026-08-15.
* [The push gate has no escape hatch the commit gate has](the-push-gate-has-no-escape-hatch.md) -
  pre-commit reaches the checker only through make lint-all, exits 0 when make is absent and is
  skipped outright by SKIP_PRE_COMMIT_LINT=1. The right shape for a fast local loop and the wrong
  shape for a gate, so pre-push calls the checker directly.
* [The security badge could not go red for a finding](the-security-badge-could-not-go-red.md) -
  Every one of security.yml's eleven steps carried continue-on-error, so the Security Scans badge
  was green by construction. Govulncheck is now strict and the remaining ten are fail-open by
  decision.
