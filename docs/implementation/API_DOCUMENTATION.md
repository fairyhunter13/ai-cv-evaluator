# API Documentation and Standards

This document provides comprehensive API documentation, contracts, and standards for the AI CV Evaluator system.

## 🎯 API Overview

The API follows RESTful principles and provides endpoints for:
- **File Upload** - Upload CV and project files
- **Evaluation** - Trigger AI evaluation process
- **Results** - Retrieve evaluation results
- **Health Checks** - System health monitoring
- **Admin** - Administrative operations

## 📋 API Standards

### Core Endpoints
The service provides three main endpoints for CV evaluation:
- **POST `/v1/upload`** - Upload CV and project files
- **POST `/v1/evaluate`** - Start evaluation job
- **GET `/v1/result/{id}`** - Get evaluation results

## Base URL

```
Production: https://api.ai-cv-evaluator.com/v1
Development: http://localhost:8080/v1
```

## 🔒 Input Security and Validation

### File Upload Security
- **Allowlist approach**: Only allow `.txt`, `.pdf`, `.docx`
- **Content sniffing**: Detect MIME type by content, not extension
- **Size limits**: 10MB per file (configurable)
- **Content sanitization**: Strip control characters and malicious content

### Input Validation
```go
// File type validation
func validateFileType(filename string, content []byte) error {
    // Check extension
    ext := strings.ToLower(filepath.Ext(filename))
    if !contains(allowedExtensions, ext) {
        return ErrUnsupportedFileType
    }
    
    // Check MIME type by content
    mimeType := http.DetectContentType(content)
    if !contains(allowedMimeTypes, mimeType) {
        return ErrUnsupportedMimeType
    }
    
    return nil
}
```

## 🚨 Error Model

### Unified Error Response
```json
{
  "error": {
    "code": "string",
    "message": "string", 
    "details": {}
  }
}
```

### HTTP Status Code Mapping
- **400 Bad Request**: `INVALID_ARGUMENT` - Bad or missing inputs
- **404 Not Found**: `NOT_FOUND` - Missing uploads/jobs/results
- **409 Conflict**: `CONFLICT` - Idempotency conflict or invalid state
- **413 Payload Too Large**: `INVALID_ARGUMENT` - File too large
- **415 Unsupported Media Type**: `INVALID_ARGUMENT` - Unsupported file type
- **429 Too Many Requests**: `RATE_LIMITED` - Rate limit exceeded
- **503 Service Unavailable**: `UPSTREAM_TIMEOUT`, `UPSTREAM_RATE_LIMIT`, `SCHEMA_INVALID`
- **500 Internal Server Error**: `INTERNAL` - Unexpected condition

### Error Codes
- `INVALID_ARGUMENT` - Bad or missing inputs, invariant violations
- `NOT_FOUND` - Missing uploads/jobs/results
- `CONFLICT` - Idempotency conflict or invalid state transition
- `RATE_LIMITED` - Local or upstream rate limiting
- `UPSTREAM_TIMEOUT` - LLM/embeddings/Vector DB timeout
- `UPSTREAM_RATE_LIMIT` - Upstream 429 response
- `SCHEMA_INVALID` - LLM JSON invalid against schema
- `INTERNAL` - Unexpected condition

## Authentication

### Admin Authentication
Admin endpoints require authentication via session or Basic Auth:

```bash
# Session-based authentication
curl -X POST http://localhost:8080/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "password"}'

# Basic authentication
curl -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
  http://localhost:8080/admin/api/status
```

### Rate Limiting
- **Upload/Evaluate**: 10 requests per minute per IP
- **Read-only**: No rate limiting
- **Admin**: No rate limiting

## Endpoints

### 1. File Upload

#### POST /v1/upload
Upload CV and project files for evaluation.

**Request:**
```http
POST /v1/upload
Content-Type: multipart/form-data

cv_file: [binary file]
project_file: [binary file]
```

**Response:**
```json
{
  "cv_id": "uuid-cv-id",
  "project_id": "uuid-project-id"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/v1/upload \
  -F "cv_file=@cv.pdf" \
  -F "project_file=@project.pdf"
```

**Error Responses:**
```json
// 400 Bad Request
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "validation failed",
    "details": {
      "cv_file": "required"
    }
  }
}

// 413 Payload Too Large
{
  "error": {
    "code": "INVALID_ARGUMENT", 
    "message": "file too large",
    "details": {
      "max_size": "10MB"
    }
  }
}
```

### 2. Evaluation

#### POST /v1/evaluate
Trigger AI evaluation of uploaded files.

**Request:**
```json
{
  "cv_id": "uuid-cv-id",
  "project_id": "uuid-project-id",
  "job_description": "Software Engineer position...",
  "study_case_brief": "Develop a web application...",
  "scoring_rubric": "Technical skills: 40%, Experience: 30%..."
}
```

**Response:**
```json
{
  "id": "job-uuid",
  "status": "queued"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/v1/evaluate \
  -H "Content-Type: application/json" \
  -d '{
    "cv_id": "uuid-cv-id",
    "project_id": "uuid-project-id"
  }'
```

**Error Responses:**
```json
// 400 Bad Request
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "validation failed",
    "details": {
      "cv_id": "required"
    }
  }
}

// 404 Not Found
{
  "error": {
    "code": "NOT_FOUND",
    "message": "CV or project not found"
  }
}
```

### 3. Results

#### GET /v1/result/{id}
Retrieve evaluation results by job ID.

**Request:**
```http
GET /v1/result/job-uuid
If-None-Match: "etag-value"  # Optional
```

**Response (Completed):**
```json
{
  "id": "job-uuid",
  "status": "completed",
  "result": {
    "cv_match_rate": 0.85,
    "cv_feedback": "Strong technical background...",
    "project_score": 8.5,
    "project_feedback": "Well-structured project...",
    "overall_summary": "Excellent candidate..."
  }
}
```

**Response (Processing):**
```json
{
  "id": "job-uuid",
  "status": "processing"
}
```

**Response (Failed):**
```json
{
  "id": "job-uuid",
  "status": "failed",
  "error": "AI service unavailable"
}
```

**Example:**
```bash
curl http://localhost:8080/v1/result/job-uuid
```

**Error Responses:**
```json
// 404 Not Found
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Job not found"
  }
}

// 304 Not Modified (with If-None-Match)
HTTP/1.1 304 Not Modified
ETag: "etag-value"
```

### 4. Health Checks

#### GET /healthz
Basic health check endpoint.

**Response:**
```http
HTTP/1.1 200 OK
```

#### GET /readyz
Detailed readiness check for all dependencies.

**Response:**
```json
{
  "checks": [
    {
      "name": "db",
      "ok": true
    },
    {
      "name": "qdrant",
      "ok": true
    },
    {
      "name": "tika",
      "ok": true
    }
  ]
}
```

**Error Response:**
```json
{
  "checks": [
    {
      "name": "db",
      "ok": false,
      "details": "connection failed"
    }
  ]
}
```

### 5. Metrics

#### GET /metrics
Prometheus metrics endpoint.

**Response:**
```
# HELP http_requests_total Total HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="POST",path="/v1/upload",status="200"} 42

# HELP http_request_duration_seconds HTTP request duration
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="POST",path="/v1/upload",le="0.1"} 10
```

### 6. OpenAPI Specification

#### GET /openapi.yaml
OpenAPI specification file.

**Response:**
```yaml
openapi: 3.0.0
info:
  title: AI CV Evaluator API
  version: 1.0.0
paths:
  /v1/upload:
    post:
      summary: Upload files
      # ... rest of specification
```

## Admin Endpoints

### 1. Authentication

#### POST /admin/login
Authenticate admin user.

**Request:**
```json
{
  "username": "admin",
  "password": "password"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Login successful"
}
```

#### POST /admin/logout
Logout admin user.

**Response:**
```json
{
  "success": true,
  "message": "Logout successful"
}
```

### 2. System Status

#### GET /admin/api/status
Get system status information.

**Response:**
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

**Response:**
```json
{
  "total_jobs": 1250,
  "completed_jobs": 1200,
  "failed_jobs": 50,
  "average_processing_time": "45s",
  "active_jobs": 5
}
```

### 3. Job Management

#### GET /admin/api/jobs
List all jobs with pagination.

**Request:**
```http
GET /admin/api/jobs?page=1&limit=20&status=completed
```

**Response:**
```json
{
  "jobs": [
    {
      "id": "job-uuid",
      "status": "completed",
      "created_at": "2024-01-15T10:30:00Z",
      "completed_at": "2024-01-15T10:31:30Z",
      "processing_time": "1m30s"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 1200,
    "pages": 60
  }
}
```

#### GET /admin/api/jobs/{id}
Get detailed job information.

**Response:**
```json
{
  "id": "job-uuid",
  "status": "completed",
  "created_at": "2024-01-15T10:30:00Z",
  "completed_at": "2024-01-15T10:31:30Z",
  "processing_time": "1m30s",
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
All errors follow a consistent format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {
      "field": "specific error details"
    }
  }
}
```

### Error Codes

#### Client Errors (4xx)
- `INVALID_ARGUMENT` - Invalid request parameters
- `NOT_FOUND` - Resource not found
- `CONFLICT` - Resource conflict
- `RATE_LIMITED` - Rate limit exceeded

#### Server Errors (5xx)
- `INTERNAL` - Internal server error
- `UPSTREAM_TIMEOUT` - External service timeout
- `UPSTREAM_RATE_LIMIT` - External service rate limit
- `SCHEMA_INVALID` - Response schema validation failed

### HTTP Status Codes
- `200 OK` - Successful request
- `201 Created` - Resource created
- `304 Not Modified` - Resource not modified
- `400 Bad Request` - Invalid request
- `401 Unauthorized` - Authentication required
- `403 Forbidden` - Access denied
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource conflict
- `413 Payload Too Large` - File too large
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service unavailable

## Request/Response Examples

### Complete Workflow Example

#### 1. Upload Files
```bash
curl -X POST http://localhost:8080/v1/upload \
  -F "cv_file=@cv.pdf" \
  -F "project_file=@project.pdf"
```

**Response:**
```json
{
  "cv_id": "550e8400-e29b-41d4-a716-446655440000",
  "project_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

#### 2. Start Evaluation
```bash
curl -X POST http://localhost:8080/v1/evaluate \
  -H "Content-Type: application/json" \
  -d '{
    "cv_id": "550e8400-e29b-41d4-a716-446655440000",
    "project_id": "550e8400-e29b-41d4-a716-446655440001",
    "job_description": "Senior Software Engineer position requiring 5+ years experience in Go, PostgreSQL, and microservices architecture.",
    "study_case_brief": "Develop a scalable web application with REST API, authentication, and database integration.",
    "scoring_rubric": "Technical skills: 40%, Experience: 30%, Project quality: 20%, Communication: 10%"
  }'
```

**Response:**
```json
{
  "id": "job-550e8400-e29b-41d4-a716-446655440002",
  "status": "queued"
}
```

#### 3. Check Status
```bash
curl http://localhost:8080/v1/result/job-550e8400-e29b-41d4-a716-446655440002
```

**Response (Processing):**
```json
{
  "id": "job-550e8400-e29b-41d4-a716-446655440002",
  "status": "processing"
}
```

**Response (Completed):**
```json
{
  "id": "job-550e8400-e29b-41d4-a716-446655440002",
  "status": "completed",
  "result": {
    "cv_match_rate": 0.87,
    "cv_feedback": "Strong technical background with 6+ years of Go experience. Excellent knowledge of PostgreSQL and microservices architecture. Previous experience with similar projects.",
    "project_score": 8.5,
    "project_feedback": "Well-structured project with clear architecture. Good use of modern technologies and best practices. Comprehensive documentation and testing approach.",
    "overall_summary": "Excellent candidate with strong technical skills and relevant experience. Highly recommended for the position."
  }
}
```

## Rate Limiting

### Limits
- **Upload/Evaluate**: 10 requests per minute per IP
- **Read-only**: No rate limiting
- **Admin**: No rate limiting

### Rate Limit Headers
```http
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1640995200
```

### Rate Limit Response
```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded",
    "details": {
      "limit": 10,
      "remaining": 0,
      "reset": 1640995200
    }
  }
}
```

## CORS Configuration

### Allowed Origins
- `http://localhost:3001` (Development frontend)
- `https://admin.ai-cv-evaluator.com` (Production admin)
- Custom origins via `CORS_ALLOW_ORIGINS` environment variable

### Allowed Methods
- `GET` - Read operations
- `POST` - Create operations
- `OPTIONS` - CORS preflight

### Allowed Headers
- `*` - All headers allowed

### Credentials
- `true` - Credentials allowed for session management

## File Upload Limits

### Maximum File Size
- **Default**: 10MB per file
- **Configurable**: Via `MAX_UPLOAD_MB` environment variable

### Supported File Types
- **PDF**: `.pdf`
- **Word**: `.doc`, `.docx`
- **Text**: `.txt`
- **Markdown**: `.md`

### File Processing
- **Tika**: Document text extraction
- **Validation**: File type and size validation
- **Storage**: Temporary storage during processing

## Monitoring and Observability

### Health Endpoints
- `/healthz` - Basic health check
- `/readyz` - Detailed readiness check
- `/metrics` - Prometheus metrics

### Metrics Available
- HTTP request metrics
- AI service metrics
- RAG effectiveness metrics
- Circuit breaker metrics
- Score drift metrics

### Tracing
- OpenTelemetry integration
- Request correlation IDs
- Distributed tracing support

## Security Considerations

### Input Validation
- File type validation
- File size limits
- Request size limits
- Parameter validation

### Authentication
- Session-based authentication
- Basic authentication support
- Rate limiting per IP

### Data Protection
- Temporary file storage
- Secure file processing
- Data encryption in transit

## Troubleshooting

### Common Issues

#### 1. File Upload Failures
```bash
# Check file size
ls -lh cv.pdf

# Check file type
file cv.pdf

# Check server logs
docker logs ai-cv-evaluator
```

#### 2. Evaluation Timeouts
```bash
# Check job status
curl http://localhost:8080/v1/result/job-id

# Check system health
curl http://localhost:8080/readyz
```

#### 3. Rate Limit Issues
```bash
# Check rate limit headers
curl -I http://localhost:8080/v1/upload

# Wait for reset
sleep 60
```

### Debug Information
```bash
# Check system status
curl http://localhost:8080/admin/api/status

# Check metrics
curl http://localhost:8080/metrics

# Check OpenAPI spec
curl http://localhost:8080/openapi.yaml
```

## SDK Examples

### JavaScript/TypeScript
```typescript
class AICVEvaluator {
  constructor(private baseURL: string) {}
  
  async uploadFiles(cvFile: File, projectFile: File) {
    const formData = new FormData();
    formData.append('cv_file', cvFile);
    formData.append('project_file', projectFile);
    
    const response = await fetch(`${this.baseURL}/v1/upload`, {
      method: 'POST',
      body: formData
    });
    
    return response.json();
  }
  
  async evaluate(cvId: string, projectId: string) {
    const response = await fetch(`${this.baseURL}/v1/evaluate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cv_id: cvId, project_id: projectId })
    });
    
    return response.json();
  }
  
  async getResult(jobId: string) {
    const response = await fetch(`${this.baseURL}/v1/result/${jobId}`);
    return response.json();
  }
}
```

### Python
```python
import requests

class AICVEvaluator:
    def __init__(self, base_url):
        self.base_url = base_url
    
    def upload_files(self, cv_file, project_file):
        files = {
            'cv_file': cv_file,
            'project_file': project_file
        }
        response = requests.post(f"{self.base_url}/v1/upload", files=files)
        return response.json()
    
    def evaluate(self, cv_id, project_id):
        data = {
            'cv_id': cv_id,
            'project_id': project_id
        }
        response = requests.post(f"{self.base_url}/v1/evaluate", json=data)
        return response.json()
    
    def get_result(self, job_id):
        response = requests.get(f"{self.base_url}/v1/result/{job_id}")
        return response.json()
```

### Go
```go
package main

import (
    "bytes"
    "encoding/json"
    "io"
    "mime/multipart"
    "net/http"
)

type AICVEvaluator struct {
    BaseURL string
    Client  *http.Client
}

func (a *AICVEvaluator) UploadFiles(cvFile, projectFile io.Reader) (map[string]string, error) {
    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)
    
    cvWriter, _ := writer.CreateFormFile("cv_file", "cv.pdf")
    io.Copy(cvWriter, cvFile)
    
    projectWriter, _ := writer.CreateFormFile("project_file", "project.pdf")
    io.Copy(projectWriter, projectFile)
    
    writer.Close()
    
    resp, err := a.Client.Post(a.BaseURL+"/v1/upload", writer.FormDataContentType(), &buf)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result map[string]string
    json.NewDecoder(resp.Body).Decode(&result)
    return result, nil
}
```
