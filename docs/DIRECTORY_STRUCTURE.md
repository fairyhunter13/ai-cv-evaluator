# Directory Structure

This document describes the organized directory structure for the AI CV Evaluator project.

## Project Root Structure

```
ai-cv-evaluator/
├── admin-frontend/         # Vue 3 + Vite frontend application
├── api/                    # OpenAPI specifications
├── build/                  # Build artifacts and compiled binaries
├── cmd/                    # Application entry points (server, worker)
├── configs/               # Configuration files
├── coverage/              # Test coverage reports
├── deploy/                # Deployment configurations
├── docs/                  # Documentation
├── internal/              # Internal application code
├── logs/                  # Application and test logs
├── pkg/                   # Public packages
├── scripts/               # Generated scripts and utilities
├── secrets/               # Encrypted secrets
├── test/                  # Test files and test data
└── windsurf/              # Windsurf rules
```

## Frontend Directory Organization

### Admin Frontend Directory (`admin-frontend/`)
Contains the Vue 3 + Vite frontend application:

```
admin-frontend/
├── src/
│   ├── views/           # Page components (Login, Dashboard, Upload, etc.)
│   ├── stores/          # Pinia state management (auth store)
│   ├── App.vue          # Root Vue component
│   ├── main.ts          # Application entry point
│   └── style.css        # Global styles
├── public/              # Static assets (favicon, etc.)
├── package.json         # Frontend dependencies and scripts
├── vite.config.ts      # Vite configuration with HMR
├── tailwind.config.js   # Tailwind CSS configuration
├── postcss.config.js    # PostCSS configuration
├── tsconfig.json        # TypeScript configuration
├── Dockerfile           # Development Docker image
├── Dockerfile.prod      # Production Docker image
└── nginx.conf           # Production Nginx configuration
```

**Purpose**: Modern frontend application with Hot Module Replacement
**Technology**: Vue 3, Vite, Tailwind CSS, Pinia, TypeScript
**Git Status**: Tracked (frontend is part of the codebase)

## Generated Files Organization

### Scripts Directory (`scripts/`)
Contains all generated scripts and utilities:

```
scripts/
├── README.md           # Documentation
└── entrypoint.sh       # Docker entrypoint script (legacy)
```

**Purpose**: Organize all generated scripts in one location
**Git Status**: Tracked (scripts are part of the codebase)

### Build Directory (`build/`)
Contains build artifacts and compiled binaries:

```
build/
├── README.md           # Documentation
└── (build artifacts)   # Generated during build process
```

**Purpose**: Store build artifacts and compiled binaries
**Git Status**: Ignored (see .gitignore)

### Logs Directory (`logs/`)
Contains application and test logs:

```
logs/
├── README.md           # Documentation
└── (log files)         # Application and test logs
```

**Purpose**: Store all log files in one location
**Git Status**: Ignored (see .gitignore)

### Coverage Directory (`coverage/`)
Contains test coverage reports:

```
coverage/
├── README.md           # Documentation
├── coverage.unit.out   # Unit test coverage
├── coverage.core.out   # Core test coverage
├── coverage.func.txt   # Coverage function report
└── coverage.html       # HTML coverage report
```

**Purpose**: Store test coverage reports and metrics
**Git Status**: Ignored (see .gitignore)

## Configuration Updates

### .gitignore Updates
```gitignore
# Generated files and directories
coverage/
logs/
build/
dist/
bin/
*.log
coverage*.out
coverage.html
coverage.func.txt
coverage.core.func.txt
```

### Makefile Updates
- Coverage files now generated in `coverage/` directory
- All coverage-related targets updated to use new paths
- CI/CD workflows updated to use new paths

### GitHub Workflows Updates
- CI workflow updated to use `coverage/coverage.unit.out`
- Artifact uploads use new directory structure

## Benefits of This Organization

### 1. **Clean Root Directory**
- No clutter in project root
- Easy to find specific types of files
- Better project navigation

### 2. **Logical Grouping**
- Scripts in `scripts/`
- Build artifacts in `build/`
- Logs in `logs/`
- Coverage in `coverage/`

### 3. **Git Management**
- Generated files properly ignored
- Source files remain tracked
- Clean git status

### 4. **CI/CD Integration**
- Workflows use organized paths
- Artifacts properly organized
- Build processes use structured directories

### 5. **Documentation**
- Each directory has README.md
- Clear purpose and usage
- Easy to understand structure

## Usage Examples

### Running Tests with Coverage
```bash
# Coverage files will be generated in coverage/ directory
make ci-test
```

### Viewing Coverage Report
```bash
# HTML report generated in coverage/ directory
make cover
# Open coverage/coverage.html in browser
```

### Accessing Logs
```bash
# Logs are stored in logs/ directory
ls logs/
```

### Build Artifacts
```bash
# Build artifacts stored in build/ directory
ls build/
```

## Migration Notes

### Moved Files
- `entrypoint.sh` → `scripts/entrypoint.sh`
- `coverage.*.out` → `coverage/coverage.*.out`
- `*.log` → `logs/`

### Updated References
- Makefile targets updated
- GitHub workflows updated
- .gitignore updated

### Backward Compatibility
- All existing functionality preserved
- Paths updated in configuration files
- No breaking changes to build process

## Maintenance

### Adding New Generated Files
1. Determine appropriate directory (`scripts/`, `build/`, `logs/`, `coverage/`)
2. Update .gitignore if needed
3. Update Makefile targets if needed
4. Update CI/CD workflows if needed

### Directory Cleanup
```bash
# Clean all generated directories
rm -rf build/ logs/ coverage/
```

### Directory Recreation
```bash
# Recreate directories
mkdir -p build logs coverage scripts
```

This organization ensures a clean, maintainable project structure with proper separation of concerns.
