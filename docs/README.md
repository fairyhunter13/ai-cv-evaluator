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
│   └── DOMAIN_MODELS.md
├── contributing/                 # Contribution Guidelines
│   └── CONTRIBUTING.md
├── development/                 # Development Guides
│   ├── GO_DEVELOPMENT_STANDARDS.md
│   ├── TESTING.md
│   ├── DOCKER_AND_LOCAL_DEVELOPMENT.md
│   ├── FRONTEND_DEVELOPMENT.md
│   ├── FRONTEND_COMPONENTS.md
│   ├── TEST_DATA_STRUCTURE.md
│   ├── E2E_DEBUGGING.md
│   ├── E2E_PERFORMANCE_ANALYSIS.md
│   ├── QUEUE_OPTIMIZATION_IMPLEMENTATION.md
│   ├── CONTEXT_DEADLINE_ROOT_CAUSE_ANALYSIS.md
│   └── ADMIN_API.md
├── implementation/               # Implementation Details
│   ├── AI_LLM_PIPELINE.md
│   ├── STORAGE_AND_QUEUEING.md
│   ├── API_DOCUMENTATION.md
│   ├── AI_EVALUATION_SYSTEM.md
│   ├── PROGRAMMATIC_TOPIC_CREATION.md
│   ├── EXACTLY_ONCE_ANALYSIS.md
│   ├── EXACTLY_ONCE_IMPLEMENTATION.md
│   ├── MIGRATION_SYSTEM.md
│   ├── FREE_MODELS_IMPLEMENTATION.md
│   ├── RAG_QDRANT_IMPLEMENTATION.md
│   ├── AI_ENHANCED_FEATURES.md
│   ├── RETRY_DLQ_SYSTEM.md
│   ├── FREE_MODELS_SYSTEM.md
│   └── DATA_RETENTION_SYSTEM.md
├── migration/                   # Migration Documentation
│   └── REDPANDA_MIGRATION.md
├── ops/                         # Operations
│   ├── CI_CD_GITHUB_ACTIONS.md
│   ├── OBSERVABILITY.md
│   ├── GITHUB_SECRETS.md
│   ├── TROUBLESHOOTING.md
│   ├── PERFORMANCE.md
│   ├── SECURITY_AUDIT.md
│   ├── INCIDENT_RESPONSE.md
│   ├── MAINTENANCE_PROCEDURES.md
│   ├── BACKUP_RECOVERY.md
│   └── SCALING_GUIDE.md
├── planning/                    # Project Planning
│   └── PROJECT_STATUS.md
├── rfc/                         # Request for Comments
│   ├── rfc-email.md
│   └── rfc-submission.md
├── security/                    # Security Documentation
│   └── SECURITY.md
├── configuration/               # Configuration Documentation
│   └── ENVIRONMENT_VARIABLES.md
├── README.md                    # This file
├── MAINTENANCE.md               # Documentation maintenance protocols
├── DEVELOPER_QUICK_REFERENCE.md # Quick reference guide
├── directory-structure.md       # Project structure
├── project.md                   # Project overview
└── STUDY_CASE.md               # Study case documentation
```

## 🚀 Quick Start

### For Developers
- **[Developer Quick Reference](DEVELOPER_QUICK_REFERENCE.md)** - Get started quickly
- **[Frontend Development](development/FRONTEND_DEVELOPMENT.md)** - Comprehensive frontend development guide

### For Architects
- **[Architecture Overview](architecture/ARCHITECTURE.md)** - System architecture
- **[Domain Models](architecture/DOMAIN_MODELS.md)** - Domain entities and business logic
- **[ADRs](adr/)** - Architecture Decision Records

### For Operations
- **[Redpanda Migration](migration/REDPANDA_MIGRATION.md)** - Complete migration guide
- **[Implementation Details](implementation/)** - Technical implementation
- **[Operations](ops/)** - Operational procedures

## 📚 Documentation Categories

### 🏗️ Architecture & Design
- **[System Architecture](architecture/ARCHITECTURE.md)** - Comprehensive system design
- **[Domain Models](architecture/DOMAIN_MODELS.md)** - Domain entities and business logic
- **[ADRs](adr/)** - Architecture Decision Records for key decisions
  - **[ADR-0006: Redpanda Migration](adr/0006-redpanda-migration.md)** - Migration from Asynq/Redis to Redpanda

### 💻 Development
- **[Developer Quick Reference](DEVELOPER_QUICK_REFERENCE.md)** - Quick start guide
- **[Go Development Standards](development/GO_DEVELOPMENT_STANDARDS.md)** - Comprehensive Go development standards and best practices
- **[Testing Strategy](development/TESTING.md)** - Comprehensive testing strategy and standards
- **[Docker and Local Development](development/DOCKER_AND_LOCAL_DEVELOPMENT.md)** - Containerization and local development setup
- **[Frontend Development](development/FRONTEND_DEVELOPMENT.md)** - Comprehensive frontend development with HMR
- **[Frontend Components](development/FRONTEND_COMPONENTS.md)** - Detailed Vue.js component documentation
- **[Test Data Structure](development/TEST_DATA_STRUCTURE.md)** - Test data organization and helper functions
- **[E2E Debugging](development/E2E_DEBUGGING.md)** - End-to-end testing debugging guide
- **[E2E Performance Analysis](development/E2E_PERFORMANCE_ANALYSIS.md)** - Performance optimization and analysis
- **[Queue Optimization](development/QUEUE_OPTIMIZATION_IMPLEMENTATION.md)** - Queue optimization and retry implementation
- **[Context Deadline Analysis](development/CONTEXT_DEADLINE_ROOT_CAUSE_ANALYSIS.md)** - Root cause analysis for timeout issues
- **[Admin API](development/ADMIN_API.md)** - Administrative API documentation
- **[Directory Structure](directory-structure.md)** - Project structure overview

### 🔧 Implementation
- **[AI and LLM Pipeline](implementation/AI_LLM_PIPELINE.md)** - AI pipeline design, prompt engineering, and RAG implementation
- **[Storage and Queueing](implementation/STORAGE_AND_QUEUEING.md)** - Database schema, queue system, and data management
- **[API Documentation](implementation/API_DOCUMENTATION.md)** - Comprehensive API documentation, contracts, and standards
- **[AI Evaluation System](implementation/AI_EVALUATION_SYSTEM.md)** - AI evaluation system implementation
- **[Programmatic Topic Creation](implementation/PROGRAMMATIC_TOPIC_CREATION.md)** - Redpanda topic creation implementation
- **[Exactly-Once Analysis](implementation/EXACTLY_ONCE_ANALYSIS.md)** - Critical analysis of exactly-once semantics
- **[Exactly-Once Implementation](implementation/EXACTLY_ONCE_IMPLEMENTATION.md)** - Comprehensive implementation guide
- **[Migration System](implementation/MIGRATION_SYSTEM.md)** - Containerized database migration system
- **[Free Models Implementation](implementation/FREE_MODELS_IMPLEMENTATION.md)** - Free AI models implementation
- **[RAG & Qdrant Implementation](implementation/RAG_QDRANT_IMPLEMENTATION.md)** - RAG and vector database implementation
- **[AI Enhanced Features](implementation/AI_ENHANCED_FEATURES.md)** - Advanced AI features: refusal detection, response validation, model switching
- **[Retry and DLQ System](implementation/RETRY_DLQ_SYSTEM.md)** - Comprehensive retry and Dead Letter Queue implementation
- **[Free Models System](implementation/FREE_MODELS_SYSTEM.md)** - Cost-effective AI processing with free models
- **[Data Retention System](implementation/DATA_RETENTION_SYSTEM.md)** - Automatic data lifecycle management and cleanup

### 🔄 Migration & Operations
- **[Redpanda Migration](migration/REDPANDA_MIGRATION.md)** - Complete migration guide and status
- **[CI/CD and GitHub Actions](ops/CI_CD_GITHUB_ACTIONS.md)** - Continuous integration and deployment pipeline
- **[Observability](ops/OBSERVABILITY.md)** - Comprehensive observability and monitoring guide
- **[GitHub Secrets](ops/GITHUB_SECRETS.md)** - Comprehensive GitHub secrets management
- **[Troubleshooting Guide](ops/TROUBLESHOOTING.md)** - Comprehensive troubleshooting and debugging guide
- **[Performance](ops/PERFORMANCE.md)** - Performance monitoring and optimization guide
- **[Security Audit](ops/SECURITY_AUDIT.md)** - Security audit procedures and compliance
- **[Incident Response](ops/INCIDENT_RESPONSE.md)** - Incident response procedures and protocols
- **[Maintenance Procedures](ops/MAINTENANCE_PROCEDURES.md)** - System maintenance and optimization
- **[Backup & Recovery](ops/BACKUP_RECOVERY.md)** - Backup and disaster recovery procedures
- **[Scaling Guide](ops/SCALING_GUIDE.md)** - System scaling and capacity planning

### 📋 Planning & RFCs
- **[Project Status](planning/PROJECT_STATUS.md)** - Current project status and recent changes
- **[RFC Email](rfc/rfc-email.md)** - Email RFC
- **[RFC Submission](rfc/rfc-submission.md)** - Submission RFC

### 🔒 Security & Compliance
- **[Security and Compliance](security/SECURITY.md)** - Comprehensive security measures, compliance requirements, and best practices
- **[Contributing Guidelines](contributing/CONTRIBUTING.md)** - How to contribute

### ⚙️ Configuration
- **[Environment Variables](configuration/ENVIRONMENT_VARIABLES.md)** - Complete environment variables reference

### 📖 Study & Research
- **[Study Case](STUDY_CASE.md)** - Study case documentation
- **[Project Overview](project.md)** - Project overview and goals

### 🔧 Documentation Maintenance
- **[Documentation Maintenance](MAINTENANCE.md)** - Comprehensive maintenance protocols and cleanup procedures

## 🎯 Key Features

### Comprehensive Coverage
- **100% Go Package Documentation** - All packages documented
- **Complete API Documentation** - OpenAPI specifications
- **Full Configuration Guide** - Environment variables and settings
- **Comprehensive Testing** - Unit, integration, and E2E testing

### Quality Standards
- **Consistent Formatting** - All documents follow markdown standards
- **Clear Navigation** - Logical organization and cross-references
- **Up-to-Date Content** - Regular updates with code changes
- **User-Friendly** - Accessible to developers and users

### Maintenance
- **Single Source of Truth** - Each topic has one authoritative document
- **Regular Updates** - Documentation updated with code changes
- **Quality Assurance** - Link validation and content review
- **Continuous Improvement** - Feedback incorporation and optimization

## 📈 Documentation Metrics

### Coverage
- **Go Packages**: 100% documented
- **API Endpoints**: 100% documented
- **Configuration**: 100% documented
- **Deployment**: 100% documented

### Quality
- **Consistency**: All documents follow standard format
- **Accuracy**: Content matches current implementation
- **Completeness**: All essential information included
- **Maintainability**: Easy to update and maintain

## 🚀 Getting Started

1. **Start Here**: [Developer Quick Reference](DEVELOPER_QUICK_REFERENCE.md)
2. **Architecture**: [System Architecture](architecture/ARCHITECTURE.md)
3. **Development**: [Frontend Development](development/FRONTEND_DEVELOPMENT.md)
4. **Implementation**: [AI Evaluation System](implementation/AI_EVALUATION_SYSTEM.md)
5. **Operations**: [Redpanda Migration](migration/REDPANDA_MIGRATION.md)

## 📞 Support

For questions or issues with documentation:
- Check the [Troubleshooting Guide](ops/TROUBLESHOOTING.md)
- Review [Documentation Status](DOCUMENTATION_STATUS.md)
- Follow [Contributing Guidelines](contributing/CONTRIBUTING.md)