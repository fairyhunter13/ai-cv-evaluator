# Decision

* [Observability was deprecated in production, and the docs still advertise it](observability-was-deprecated-in-production.md) - In June 2026 the observability stack moved behind a compose profile and trace export was removed, but README and docs/observability.md still publish live Grafana, Prometheus and Jaeger URLs.
* [The bundle gate asserts that it is installed, not only that the bundle passes](the-bundle-gate-asserts-that-it-is-installed.md) - Five arms fail when lint-knowledge stops passing -Werror, when the CI step that reaches it is excused, when the pre-commit hook is non-executable in the index, when the okf install loses its pin, or when the checker accepts everything.
* [The lint gate had never run](the-lint-gate-had-never-run.md) - A v2 version key over v1 directives, plus a pinned binary that was installed and then not invoked, meant golangci-lint never executed on this module until 2026-08-15.
