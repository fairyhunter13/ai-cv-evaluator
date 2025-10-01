# Admin API Documentation

This document provides documentation for the Admin API endpoints in the AI CV Evaluator project.

## Overview

The Admin API provides administrative functionality for managing the AI CV Evaluator system, including system monitoring and job management.

## Base URL

```
Production: https://api.example.com/admin/api
Development: http://localhost:8080/admin/api
```

## Authentication

### Session-Based Authentication
The Admin API uses session-based authentication with CSRF protection.

#### Login
```http
POST /admin/api/login
Content-Type: application/x-www-form-urlencoded

username=admin&password=password
```

**Response**:
```json
{
  "success": true,
  "message": "Login successful"
}
```

#### Logout
```http
POST /admin/api/logout
```

## Endpoints

### System Status

#### GET /admin/api/status
Get system status information.

**Response**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "2h30m15s",
  "services": {
    "database": "healthy",
    "qdrant": "healthy",
    "tika": "healthy"
  }
}
```

#### GET /admin/api/stats
Get system statistics.

**Response**:
```json
{
  "uploads": 1250,
  "evaluations": 1200,
  "completed": 1150,
  "avg_time": 45.5,
  "failed": 50
}
```

### Job Management

#### GET /admin/api/jobs
List all jobs with pagination.

**Parameters**:
- `page` (integer): Page number (default: 1)
- `limit` (integer): Items per page (default: 20, max: 100)
- `search` (string): Search term
- `status` (string): Filter by status (queued, processing, completed, failed)

**Response**:
```json
{
  "jobs": [
    {
      "id": "job-uuid",
      "status": "completed",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:31:30Z",
      "cv_id": "cv-uuid",
      "project_id": "project-uuid"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 1200
  }
}
```

#### GET /admin/api/jobs/{id}
Get detailed job information.

**Response**:
```json
{
  "id": "job-uuid",
  "status": "completed",
  "created_at": "2024-01-15T10:30:00Z",
  "completed_at": "2024-01-15T10:31:30Z",
  "cv_id": "cv-uuid",
  "project_id": "project-uuid",
  "result": {
    "cv_match_rate": 0.85,
    "cv_feedback": "Strong technical background...",
    "project_score": 8.5,
    "project_feedback": "Well-structured project...",
    "overall_summary": "Excellent candidate..."
  }
}
```

## Error Handling

### Error Response Format
```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message"
  }
}
```

### Common Error Codes
- `UNAUTHORIZED` - Authentication required
- `FORBIDDEN` - Access denied
- `NOT_FOUND` - Resource not found
- `VALIDATION_ERROR` - Invalid request parameters

## Rate Limiting

- **Admin endpoints**: No rate limiting
- **Authentication**: 5 attempts per minute per IP

## Security

### CSRF Protection
All state-changing operations require CSRF tokens.

### Session Management
- Sessions expire after 24 hours
- Secure session cookies
- Session invalidation on logout