# Documentation Index

This `docs/` tree documents the current architecture and workflows of the **ai-cv-evaluator** repo.

## Quick Links

| Resource | URL |
|----------|-----|
| **Production Site** | https://example.com |
| **Admin Dashboard** | https://example.com/app/dashboard |
| **Grafana** | https://example.com/grafana/ |
| **Jaeger Traces** | https://example.com/jaeger/ |

## Development

- [🚀 Developer Quick Reference](./DEVELOPER_QUICK_REFERENCE.md) - Get started quickly
- [🏗️ System Architecture](./architecture/ARCHITECTURE.md) - System design and architecture
- [💻 Frontend Development](./development/FRONTEND_DEVELOPMENT.md) - Vue 3 + Vite admin frontend
- [🔄 Migration Status](./migration/MIGRATION_SUMMARY.md) - Database migration status
- [📁 Directory Structure](./DIRECTORY_STRUCTURE.md) - Project structure overview
- [🌐 Provider Documentation](./providers/README.md) - AI provider APIs, models, and rate limits

## Operations

- [📊 Observability Runbook](./observability.md) - Monitoring, dashboards, alerts, and troubleshooting
- [🗄️ Data Retention Policy](./data-retention.md) - What data is stored, retention periods, backup/recovery
- [⚙️ Operations Runbook](./operations.md) - Deployment, scaling, maintenance procedures

## Security

- [🔐 SSO & Rate Limiting](./security/SSO_RATE_LIMITING.md) - Keycloak SSO configuration and brute force protection

## Testing

- [🧪 Rate-Limit Friendly E2E](./testing/rate-limit-friendly-e2e.md) - E2E testing with AI provider rate limits

## CI/CD Pipeline

The deployment pipeline enforces strict quality gates:

1. **CI Workflow** - Unit tests with 80% coverage gate, security scans, Playwright E2E
2. **Security Gate** - CI and Docker Publish must succeed before deploy
3. **Deploy Workflow** - Requires semantic version tags (v1.2.3), blue/green deployment
4. **Production Validation** - Post-deploy health checks, Playwright E2E, alerting validation

The root `README.md` points here and should always reflect the latest state of the repository.
