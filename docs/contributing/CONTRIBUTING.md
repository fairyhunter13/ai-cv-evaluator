# Contributing to AI CV Evaluator

Thank you for your interest in contributing to the AI CV Evaluator project!

## Development Setup

### Prerequisites
- Go 1.24+
- Node.js 18+ (for frontend development)
- Docker and Docker Compose
- Make
- SOPS (for secrets management)

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/fairyhunter13/ai-cv-evaluator.git
cd ai-cv-evaluator
```

2. Copy and configure environment variables:
```bash
cp .env.sample .env
# Edit .env with your configuration (or decrypt .env.sops.yaml)
```

3. Start the full stack with frontend (recommended):
```bash
make dev-full  # Starts backend + frontend with HMR
```

Or start backend only:
```bash
docker compose up -d --build
```

4. Migrations run automatically when starting services, but you can run them manually:
```bash
make migrate
```

5. Seed RAG data (optional):
```bash
make seed-rag
```

6. The system will start with:
   - 1 migration container (runs database migrations automatically)
   - 1 server container (API-only HTTP requests)
   - 1 frontend container (Vue 3 + Vite with HMR)
   - 8 worker containers (processes AI tasks)
   - All supporting services (PostgreSQL, Redpanda, Qdrant, Tika)
   - Observability stack (Prometheus, Grafana, Jaeger)

7. For development, you can also run locally:
```bash
make run  # Runs only the server
make frontend-dev  # Runs only the frontend
```

## Code Standards

### Backend Architecture
- Follow Clean Architecture principles
- Keep domain layer pure (no external dependencies)
- Place business logic in usecase layer
- Use adapters for external integrations

### Frontend Architecture
- Use Vue 3 Composition API
- Follow component-based architecture
- Use Pinia for state management
- Implement proper TypeScript types
- Use Tailwind CSS for styling
- Follow Vue 3 best practices

### Testing
- Colocate unit tests with code (`*_test.go` next to implementation)
- E2E tests go under `test/e2e/`
- Aim for >80% coverage on core packages
- Use table-driven tests where appropriate

### Code Style
- Run `make fmt` before committing
- Run `make lint` to check for issues
- Run `make vet` for Go vet checks
- Run `make vuln` for vulnerability scanning
- For frontend: Run `npm run lint` in `admin-frontend/`

## Making Changes

### Branch Naming
- `feature/description` for new features
- `fix/description` for bug fixes
- `docs/description` for documentation
- `refactor/description` for refactoring

### Commit Messages
Follow conventional commits:
- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation
- `test:` for tests
- `refactor:` for refactoring
- `chore:` for maintenance

### Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add/update tests
5. Update documentation if needed
6. Run tests locally:
```bash
make all  # fmt, lint, vet, test
```
7. Submit a pull request with:
   - Clear description of changes
   - Link to related issues
   - Test results
   - Screenshots if UI changes

## Frontend Development

### Development Workflow
1. Start the development environment:
   ```bash
   make dev-full  # Backend + frontend with HMR
   ```

2. Make changes to Vue components in `admin-frontend/src/`
3. Changes are automatically reflected with HMR
4. Test API integration with the backend

### Frontend Structure
- `admin-frontend/src/views/` - Page components
- `admin-frontend/src/stores/` - Pinia state management
- `admin-frontend/src/App.vue` - Root component
- `admin-frontend/src/main.ts` - Application entry point

### Frontend Commands
```bash
# Install dependencies
make frontend-install

# Start development server
make frontend-dev

# Build for production
make frontend-build

# Clean build artifacts
make frontend-clean
```

### Frontend Testing
- Use browser dev tools for debugging
- Test API integration with backend
- Verify responsive design with different screen sizes
- Test authentication flow

## API Changes

When modifying API endpoints:
1. Update `api/openapi.yaml`
2. Validate with `make validate-openapi`
3. Update E2E tests
4. Update frontend API calls if needed
5. Document breaking changes

## Adding Dependencies

1. Use standard library when possible
2. Evaluate license compatibility
3. Consider security implications
4. Update `go.mod` and run `go mod tidy`
5. Document why the dependency is needed

## Security

- Never commit secrets or credentials
- Use SOPS for encrypted secrets
- Follow OWASP guidelines for web security
- Report security issues privately

## Testing Guidelines

### Unit Tests
```go
func TestFunctionName_Scenario(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```


### E2E Tests
```bash
make ci-e2e  # Full E2E with Docker Compose setup
# or
make test-e2e  # Assumes services are already running
```

**Note**: E2E tests require the full stack running with 8 worker replicas for optimal performance.

## Documentation

- Update README.md for user-facing changes
- Update ARCHITECTURE.md for design changes
- Add inline comments for complex logic
- Generate mocks when adding interfaces:
```bash
make generate
```

## Review Checklist

Before submitting PR:
- [ ] Tests pass locally
- [ ] Code is formatted (`make fmt`)
- [ ] No lint issues (`make lint`)
- [ ] Documentation updated
- [ ] OpenAPI spec validated
- [ ] No security vulnerabilities
- [ ] Commits are clean and atomic

## Getting Help

- Check existing issues and PRs
- Read the documentation in `windsurf/rules/`
- Ask questions in discussions
- Join our community chat (if available)

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.
