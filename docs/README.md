# Documentation Index

This directory contains all project documentation organized by category.

## 📁 Directory Structure

```
docs/
├── adr/                          # Architecture Decision Records
│   ├── 0001-queue-system-choice.md
│   ├── 0002-vector-database-choice.md
│   ├── 0003-ai-provider-choice.md
│   ├── 0004-deployment-strategy.md
│   ├── 0005-frontend-separation.md
│   └── 0006-redpanda-migration.md
├── architecture/                 # System Architecture
│   ├── ARCHITECTURE.md
│   └── QUEUE_MIGRATION.md
├── contributing/                 # Contribution Guidelines
│   └── CONTRIBUTING.md
├── development/                 # Development Guides
│   ├── FRONTEND_DEVELOPMENT.md
│   ├── FRONTEND_COMPONENTS.md
│   └── TEST_DATA_STRUCTURE.md
├── implementation/               # Implementation Details
│   ├── PROGRAMMATIC_TOPIC_CREATION.md
│   ├── EXACTLY_ONCE_ANALYSIS.md
│   ├── EXACTLY_ONCE_IMPLEMENTATION.md
│   └── MIGRATION_SYSTEM.md
├── migration/                   # Migration Documentation
│   └── REDPANDA_MIGRATION_STATUS.md
├── ops/                         # Operations
│   ├── github-optional-secrets.md
│   ├── github-secrets.md
│   ├── TROUBLESHOOTING.md
│   └── PERFORMANCE_TUNING.md
├── planning/                    # Project Planning
│   └── TODOS.md
├── rfc/                         # Request for Comments
│   ├── rfc-email.md
│   └── rfc-submission.md
├── security/                    # Security Documentation
│   └── SECURITY.md
├── configuration/               # Configuration Documentation
│   └── ENVIRONMENT_VARIABLES.md
├── README.md                    # This file
├── DEVELOPER_QUICK_REFERENCE.md # Quick reference guide
├── directory-structure.md       # Project structure
├── DOCUMENTATION_MAINTENANCE.md # Documentation maintenance protocols
├── project.md                   # Project overview
└── STUDY_CASE.md               # Study case documentation
```

## 🚀 Quick Start

### For Developers
- **[Developer Quick Reference](DEVELOPER_QUICK_REFERENCE.md)** - Get started quickly
- **[Frontend Development](development/FRONTEND_DEVELOPMENT.md)** - Comprehensive frontend development guide

### For Architects
- **[Architecture Overview](architecture/ARCHITECTURE.md)** - System architecture
- **[Queue Migration](architecture/QUEUE_MIGRATION.md)** - Queue system migration
- **[ADRs](adr/)** - Architecture Decision Records

### For Operations
- **[Migration Status](migration/REDPANDA_MIGRATION_STATUS.md)** - Migration progress
- **[Implementation Details](implementation/)** - Technical implementation
- **[Operations](ops/)** - Operational procedures

## 📚 Documentation Categories

### 🏗️ Architecture & Design
- **[System Architecture](architecture/ARCHITECTURE.md)** - Comprehensive system design including production architecture
- **[Queue Migration](architecture/QUEUE_MIGRATION.md)** - Queue system migration details
- **[ADRs](adr/)** - Architecture Decision Records for key decisions
  - **[ADR-0006: Redpanda Migration](adr/0006-redpanda-migration.md)** - Migration from Asynq/Redis to Redpanda

### 💻 Development
- **[Developer Quick Reference](DEVELOPER_QUICK_REFERENCE.md)** - Quick start guide
- **[Frontend Development](development/FRONTEND_DEVELOPMENT.md)** - Comprehensive frontend development with HMR
- **[Frontend Components](development/FRONTEND_COMPONENTS.md)** - Detailed Vue.js component documentation
- **[Test Data Structure](development/TEST_DATA_STRUCTURE.md)** - Test data organization and helper functions
- **[Directory Structure](directory-structure.md)** - Project structure overview

### 🔧 Implementation
- **[Programmatic Topic Creation](implementation/PROGRAMMATIC_TOPIC_CREATION.md)** - Redpanda topic creation implementation
- **[Exactly-Once Analysis](implementation/EXACTLY_ONCE_ANALYSIS.md)** - Critical analysis of exactly-once semantics
- **[Exactly-Once Implementation](implementation/EXACTLY_ONCE_IMPLEMENTATION.md)** - Comprehensive implementation guide
- **[Migration System](implementation/MIGRATION_SYSTEM.md)** - Containerized database migration system
- **[Free Models Implementation](FREE_MODELS_IMPLEMENTATION.md)** - Free AI models implementation

### 🔄 Migration & Operations
- **[Redpanda Migration Status](migration/REDPANDA_MIGRATION_STATUS.md)** - Migration progress and status
- **[GitHub Secrets](ops/github-secrets.md)** - GitHub secrets management
- **[GitHub Optional Secrets](ops/github-optional-secrets.md)** - Optional secrets configuration
- **[Troubleshooting Guide](ops/TROUBLESHOOTING.md)** - Comprehensive troubleshooting and debugging guide
- **[Performance Tuning](ops/PERFORMANCE_TUNING.md)** - Performance optimization strategies

### 📋 Planning & RFCs
- **[Project TODOs](planning/TODOS.md)** - Project planning and tasks
- **[RFC Email](rfc/rfc-email.md)** - Email RFC
- **[RFC Submission](rfc/rfc-submission.md)** - Submission RFC

### 🔒 Security & Compliance
- **[Security Policy](security/SECURITY.md)** - Security guidelines and procedures
- **[Contributing Guidelines](contributing/CONTRIBUTING.md)** - How to contribute

### ⚙️ Configuration
- **[Environment Variables](configuration/ENVIRONMENT_VARIABLES.md)** - Complete environment variables reference

### 📖 Study & Research
- **[Study Case](STUDY_CASE.md)** - Study case documentation
- **[Project Overview](project.md)** - Project overview and goals

### 🔧 Documentation Maintenance
- **[Documentation Maintenance](DOCUMENTATION_MAINTENANCE.md)** - Maintenance protocols and cleanup procedures

## 🎯 Getting Started

### New to the Project?
1. Start with **[Developer Quick Reference](DEVELOPER_QUICK_REFERENCE.md)**
2. Read **[System Architecture](architecture/ARCHITECTURE.md)**
3. Check **[Migration Status](migration/REDPANDA_MIGRATION_STATUS.md)**

### Frontend Development?
1. **[Frontend Development Guide](development/FRONTEND_DEVELOPMENT.md)** - Comprehensive guide including separation and HMR

### Backend Development?
1. **[Developer Quick Reference](DEVELOPER_QUICK_REFERENCE.md)**
2. **[Implementation Details](implementation/)**
3. **[Architecture Overview](architecture/ARCHITECTURE.md)**

### Operations & Deployment?
1. **[Migration Status](migration/REDPANDA_MIGRATION_STATUS.md)**
2. **[Operations Documentation](ops/)**
3. **[System Architecture](architecture/ARCHITECTURE.md)** - Includes production architecture details

## 📝 Contributing to Documentation

When adding new documentation:

1. **Choose the right directory** based on the content type
2. **Follow naming conventions** (UPPERCASE_WITH_UNDERSCORES.md)
3. **Update this README** to include the new document
4. **Add appropriate cross-references** to related documents

### Documentation Standards

- Use clear, descriptive titles
- Include a table of contents for long documents
- Add cross-references to related documents
- Keep documentation up-to-date with code changes
- Use consistent formatting and structure

## 🔍 Finding Information

### By Topic
- **Architecture**: `architecture/`, `adr/`
- **Development**: `development/`, `DEVELOPER_QUICK_REFERENCE.md`
- **Implementation**: `implementation/`
- **Migration**: `migration/`
- **Operations**: `ops/`
- **Planning**: `planning/`, `rfc/`
- **Security**: `security/`, `contributing/`

### By Audience
- **Developers**: `development/`, `DEVELOPER_QUICK_REFERENCE.md`
- **Architects**: `architecture/`, `adr/`
- **Operations**: `migration/`, `ops/`
- **Contributors**: `contributing/`, `security/`

## 📞 Need Help?

- Check the **[Developer Quick Reference](DEVELOPER_QUICK_REFERENCE.md)** for common tasks
- Review **[Architecture Documentation](architecture/)** for system understanding
- Look at **[Migration Status](migration/REDPANDA_MIGRATION_STATUS.md)** for current state
- Consult **[Implementation Details](implementation/)** for technical specifics

---

*This documentation is maintained alongside the codebase. Please keep it updated when making changes.*