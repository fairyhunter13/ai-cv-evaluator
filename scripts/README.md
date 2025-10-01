# Scripts Directory

This directory contains all generated scripts and utilities for the AI CV Evaluator project.

## Directory Structure

```
scripts/
├── README.md           # This file
└── entrypoint.sh       # Docker entrypoint script (legacy)
```

## Scripts

### entrypoint.sh
- **Purpose**: Docker entrypoint script for selecting between server and worker modes
- **Status**: Legacy - Currently not used in production Dockerfiles
- **Usage**: `./entrypoint.sh {server|worker}`
- **Note**: The current Dockerfiles use direct binary execution instead of this script

## Related Directories

- `build/` - Build artifacts and compiled binaries
- `logs/` - Application and test logs
- `coverage/` - Test coverage reports

## Notes

- All scripts should be executable (`chmod +x`)
- Scripts are organized by functionality and usage
- Legacy scripts are kept for reference but may not be actively used
