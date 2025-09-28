# Contributing to AI CV Evaluator

Thank you for your interest in contributing to the AI CV Evaluator project!

## Development Setup

### Prerequisites
- Go 1.24+
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

3. Start dependencies:
```bash
docker compose up -d db redis qdrant tika
```

4. Run migrations:
```bash
make migrate
```

5. Seed RAG data (optional):
```bash
make seed-rag
```

6. Run the application:
```bash
make run
```

## Code Standards

### Architecture
- Follow Clean Architecture principles
- Keep domain layer pure (no external dependencies)
- Place business logic in usecase layer
- Use adapters for external integrations

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

## API Changes

When modifying API endpoints:
1. Update `api/openapi.yaml`
2. Validate with `make validate-openapi`
3. Update integration tests
4. Document breaking changes

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

### Integration Tests
```bash
go test -tags=integration ./...
```

### E2E Tests
```bash
make test-e2e
```

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
