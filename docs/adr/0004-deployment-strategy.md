# ADR-0004: Deployment Strategy

**Date:** 2025-09-28  
**Status:** Accepted  

## Context

The project requires a deployment strategy that balances cost, simplicity, and reliability for the 5-day development timeline. Requirements:
- Cost-effective infrastructure for demonstration/development
- Simple deployment pipeline via GitHub Actions
- Support for Docker containers and observability stack
- Ability to handle moderate load for evaluation demonstrations
- Infrastructure as code for reproducibility

## Decision

We will deploy to a **VPS (Virtual Private Server)** using Docker Compose with automated deployment via GitHub Actions.

## Consequences

### Positive
- **Cost Effective**: Single VPS much cheaper than managed cloud services
- **Full Control**: Complete control over infrastructure and configuration  
- **Docker Native**: Leverages existing containerization work
- **Simple Pipeline**: SSH-based deployment via GitHub Actions
- **Resource Efficiency**: All services on single machine reduces network overhead
- **Observability Ready**: Can run full monitoring stack (Prometheus/Grafana/Jaeger)

### Negative  
- **Single Point of Failure**: Entire system depends on one machine
- **Limited Scalability**: Cannot auto-scale individual components
- **Manual Scaling**: Requires manual intervention for traffic spikes
- **Security Responsibility**: Full responsibility for OS and security updates
- **Backup Complexity**: Need to implement own backup strategies

### Risks
- Hardware failure causes complete outage
- Resource exhaustion affects entire stack
- Security vulnerabilities require manual patching
- No geographic redundancy for disaster recovery

## Alternatives Considered

### Option A: Managed Kubernetes (GKE/EKS/AKS)
- **Pros**: Auto-scaling, managed infrastructure, high availability
- **Cons**: High complexity, expensive, over-engineered for demo project
- **Rejected**: Cost and complexity exceed project requirements

### Option B: Serverless (Lambda/Cloud Functions)
- **Pros**: Auto-scaling, pay-per-use, managed infrastructure
- **Cons**: Cold starts, complex state management, vendor lock-in
- **Rejected**: Stateful queue/database components difficult to adapt

### Option C: PaaS (Heroku/Railway/Render)
- **Pros**: Simple deployment, managed infrastructure, good developer experience
- **Cons**: Expensive at scale, limited customization, vendor lock-in
- **Rejected**: Cost concerns and limited control over infrastructure

### Option D: Bare Metal Server
- **Pros**: Maximum performance, full control, no virtualization overhead
- **Cons**: Higher costs, complex provisioning, hardware management
- **Rejected**: Unnecessary complexity for demonstration project

## Implementation Details

- **Provider**: VPS with sufficient resources (4+ CPU, 8+ GB RAM)
- **OS**: Ubuntu LTS for stability and support
- **Deployment**: GitHub Actions with SSH key authentication
- **Services**: Docker Compose with persistent volumes
- **Monitoring**: Self-hosted Prometheus/Grafana stack
- **Backup**: Automated database dumps and volume snapshots
- **Security**: UFW firewall, fail2ban, automated security updates
