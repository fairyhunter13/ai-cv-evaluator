# API Examples and Integration Guide

This document provides comprehensive examples and integration patterns for the AI CV Evaluator API.

## Overview

The AI CV Evaluator API provides endpoints for:
- **File Upload**: Upload CV and project documents
- **Job Evaluation**: Enqueue AI-powered evaluation jobs
- **Result Retrieval**: Get evaluation results and status
- **Admin Operations**: Dashboard and job management

## Base Configuration

### Base URL
```
Development: http://localhost:8080
Production: https://api.ai-cv-evaluator.com
```

### Headers
```http
Content-Type: application/json
Accept: application/json
Idempotency-Key: unique-request-id (optional)
```

## Core API Workflow

### 1. Upload Documents

**Endpoint**: `POST /v1/upload`

**Request**:
```bash
curl -X POST http://localhost:8080/v1/upload \
  -F "cv=@resume.pdf" \
  -F "project=@project_report.docx"
```

**Response**:
```json
{
  "cv_id": "cv_1234567890",
  "project_id": "proj_1234567890"
}
```

**JavaScript Example**:
```javascript
const formData = new FormData();
formData.append('cv', cvFile);
formData.append('project', projectFile);

const response = await fetch('/v1/upload', {
  method: 'POST',
  body: formData
});

const result = await response.json();
console.log('Upload IDs:', result);
```

**Python Example**:
```python
import requests

files = {
    'cv': open('resume.pdf', 'rb'),
    'project': open('project_report.docx', 'rb')
}

response = requests.post('http://localhost:8080/v1/upload', files=files)
result = response.json()

print(f"CV ID: {result['cv_id']}")
print(f"Project ID: {result['project_id']}")
```

### 2. Enqueue Evaluation

**Endpoint**: `POST /v1/evaluate`

**Request**:
```bash
curl -X POST http://localhost:8080/v1/evaluate \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: eval-$(date +%s)" \
  -d '{
    "cv_id": "cv_1234567890",
    "project_id": "proj_1234567890",
    "job_description": "Senior Software Engineer position requiring Go, microservices, and AI/ML experience",
    "study_case_brief": "Design a scalable evaluation system for CV assessment using AI"
  }'
```

**Response**:
```json
{
  "id": "job_1234567890",
  "status": "queued"
}
```

**JavaScript Example**:
```javascript
const evaluationRequest = {
  cv_id: 'cv_1234567890',
  project_id: 'proj_1234567890',
  job_description: 'Senior Software Engineer position...',
  study_case_brief: 'Design a scalable evaluation system...'
};

const response = await fetch('/v1/evaluate', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Idempotency-Key': `eval-${Date.now()}`
  },
  body: JSON.stringify(evaluationRequest)
});

const result = await response.json();
console.log('Job ID:', result.id);
```

**Python Example**:
```python
import requests
import time

evaluation_data = {
    'cv_id': 'cv_1234567890',
    'project_id': 'proj_1234567890',
    'job_description': 'Senior Software Engineer position...',
    'study_case_brief': 'Design a scalable evaluation system...'
}

headers = {
    'Content-Type': 'application/json',
    'Idempotency-Key': f'eval-{int(time.time())}'
}

response = requests.post('http://localhost:8080/v1/evaluate', 
                        json=evaluation_data, 
                        headers=headers)
result = response.json()

print(f"Job ID: {result['id']}")
```

### 3. Check Job Status

**Endpoint**: `GET /v1/result/{id}`

**Request**:
```bash
curl -X GET http://localhost:8080/v1/result/job_1234567890
```

**Response (Queued)**:
```json
{
  "id": "job_1234567890",
  "status": "queued"
}
```

**Response (Processing)**:
```json
{
  "id": "job_1234567890",
  "status": "processing"
}
```

**Response (Completed)**:
```json
{
  "id": "job_1234567890",
  "status": "completed",
  "result": {
    "cv_match_rate": 0.85,
    "cv_feedback": "Strong technical background with relevant experience in Go, microservices, and distributed systems. Excellent problem-solving skills demonstrated through project work.",
    "project_score": 8.5,
    "project_feedback": "Well-structured project with clear architecture. Good use of modern technologies and best practices. Demonstrates strong system design skills.",
    "overall_summary": "Highly qualified candidate with strong technical skills and relevant experience. Excellent fit for the Senior Software Engineer position."
  }
}
```

**Response (Failed)**:
```json
{
  "id": "job_1234567890",
  "status": "failed",
  "error": {
    "code": "AI_SERVICE_ERROR",
    "message": "AI service temporarily unavailable"
  }
}
```

**JavaScript Polling Example**:
```javascript
async function pollJobStatus(jobId) {
  const maxAttempts = 30; // 5 minutes max
  const interval = 10000; // 10 seconds
  
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const response = await fetch(`/v1/result/${jobId}`);
    const result = await response.json();
    
    if (result.status === 'completed') {
      return result;
    } else if (result.status === 'failed') {
      throw new Error(`Job failed: ${result.error.message}`);
    }
    
    // Wait before next poll
    await new Promise(resolve => setTimeout(resolve, interval));
  }
  
  throw new Error('Job timeout');
}

// Usage
try {
  const result = await pollJobStatus('job_1234567890');
  console.log('Evaluation completed:', result.result);
} catch (error) {
  console.error('Evaluation failed:', error.message);
}
```

**Python Polling Example**:
```python
import time
import requests

def poll_job_status(job_id, max_attempts=30, interval=10):
    for attempt in range(max_attempts):
        response = requests.get(f'http://localhost:8080/v1/result/{job_id}')
        result = response.json()
        
        if result['status'] == 'completed':
            return result
        elif result['status'] == 'failed':
            raise Exception(f"Job failed: {result['error']['message']}")
        
        time.sleep(interval)
    
    raise Exception('Job timeout')

# Usage
try:
    result = poll_job_status('job_1234567890')
    print('Evaluation completed:', result['result'])
except Exception as e:
    print('Evaluation failed:', str(e))
```

## Complete Integration Examples

### JavaScript/Node.js Integration

```javascript
class AICVEvaluator {
  constructor(baseUrl = 'http://localhost:8080') {
    this.baseUrl = baseUrl;
  }
  
  async uploadDocuments(cvFile, projectFile) {
    const formData = new FormData();
    formData.append('cv', cvFile);
    formData.append('project', projectFile);
    
    const response = await fetch(`${this.baseUrl}/v1/upload`, {
      method: 'POST',
      body: formData
    });
    
    if (!response.ok) {
      throw new Error(`Upload failed: ${response.statusText}`);
    }
    
    return await response.json();
  }
  
  async evaluateDocuments(cvId, projectId, jobDescription, studyCaseBrief) {
    const response = await fetch(`${this.baseUrl}/v1/evaluate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': `eval-${Date.now()}`
      },
      body: JSON.stringify({
        cv_id: cvId,
        project_id: projectId,
        job_description: jobDescription,
        study_case_brief: studyCaseBrief
      })
    });
    
    if (!response.ok) {
      throw new Error(`Evaluation failed: ${response.statusText}`);
    }
    
    return await response.json();
  }
  
  async getResult(jobId) {
    const response = await fetch(`${this.baseUrl}/v1/result/${jobId}`);
    
    if (!response.ok) {
      throw new Error(`Result fetch failed: ${response.statusText}`);
    }
    
    return await response.json();
  }
  
  async evaluateComplete(cvFile, projectFile, jobDescription, studyCaseBrief) {
    // Step 1: Upload documents
    const uploadResult = await this.uploadDocuments(cvFile, projectFile);
    
    // Step 2: Start evaluation
    const evaluationResult = await this.evaluateDocuments(
      uploadResult.cv_id,
      uploadResult.project_id,
      jobDescription,
      studyCaseBrief
    );
    
    // Step 3: Poll for completion
    const maxAttempts = 30;
    const interval = 10000;
    
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      const result = await this.getResult(evaluationResult.id);
      
      if (result.status === 'completed') {
        return result;
      } else if (result.status === 'failed') {
        throw new Error(`Evaluation failed: ${result.error.message}`);
      }
      
      await new Promise(resolve => setTimeout(resolve, interval));
    }
    
    throw new Error('Evaluation timeout');
  }
}

// Usage
const evaluator = new AICVEvaluator();

// Complete evaluation workflow
try {
  const result = await evaluator.evaluateComplete(
    cvFile,
    projectFile,
    'Senior Software Engineer position...',
    'Design a scalable evaluation system...'
  );
  
  console.log('CV Match Rate:', result.result.cv_match_rate);
  console.log('Project Score:', result.result.project_score);
  console.log('Overall Summary:', result.result.overall_summary);
} catch (error) {
  console.error('Evaluation failed:', error.message);
}
```

### Python Integration

```python
import requests
import time
from typing import Dict, Any, Optional

class AICVEvaluator:
    def __init__(self, base_url: str = 'http://localhost:8080'):
        self.base_url = base_url
    
    def upload_documents(self, cv_file_path: str, project_file_path: str) -> Dict[str, str]:
        """Upload CV and project documents."""
        files = {
            'cv': open(cv_file_path, 'rb'),
            'project': open(project_file_path, 'rb')
        }
        
        response = requests.post(f'{self.base_url}/v1/upload', files=files)
        response.raise_for_status()
        
        return response.json()
    
    def evaluate_documents(self, cv_id: str, project_id: str, 
                          job_description: str, study_case_brief: str) -> Dict[str, Any]:
        """Start evaluation job."""
        data = {
            'cv_id': cv_id,
            'project_id': project_id,
            'job_description': job_description,
            'study_case_brief': study_case_brief
        }
        
        headers = {
            'Content-Type': 'application/json',
            'Idempotency-Key': f'eval-{int(time.time())}'
        }
        
        response = requests.post(f'{self.base_url}/v1/evaluate', 
                               json=data, headers=headers)
        response.raise_for_status()
        
        return response.json()
    
    def get_result(self, job_id: str) -> Dict[str, Any]:
        """Get job result."""
        response = requests.get(f'{self.base_url}/v1/result/{job_id}')
        response.raise_for_status()
        
        return response.json()
    
    def evaluate_complete(self, cv_file_path: str, project_file_path: str,
                         job_description: str, study_case_brief: str,
                         max_attempts: int = 30, interval: int = 10) -> Dict[str, Any]:
        """Complete evaluation workflow."""
        # Step 1: Upload documents
        upload_result = self.upload_documents(cv_file_path, project_file_path)
        
        # Step 2: Start evaluation
        evaluation_result = self.evaluate_documents(
            upload_result['cv_id'],
            upload_result['project_id'],
            job_description,
            study_case_brief
        )
        
        # Step 3: Poll for completion
        for attempt in range(max_attempts):
            result = self.get_result(evaluation_result['id'])
            
            if result['status'] == 'completed':
                return result
            elif result['status'] == 'failed':
                raise Exception(f"Evaluation failed: {result['error']['message']}")
            
            time.sleep(interval)
        
        raise Exception('Evaluation timeout')

# Usage
evaluator = AICVEvaluator()

try:
    result = evaluator.evaluate_complete(
        'resume.pdf',
        'project_report.docx',
        'Senior Software Engineer position...',
        'Design a scalable evaluation system...'
    )
    
    print(f"CV Match Rate: {result['result']['cv_match_rate']}")
    print(f"Project Score: {result['result']['project_score']}")
    print(f"Overall Summary: {result['result']['overall_summary']}")
except Exception as e:
    print(f"Evaluation failed: {str(e)}")
```

## Admin API Examples

### Authentication

```bash
# Login
curl -X POST http://localhost:8080/admin/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=admin&password=your_password"
```

### Dashboard Statistics

```bash
curl -X GET http://localhost:8080/admin/api/stats \
  -H "Cookie: session=your_session_cookie"
```

**Response**:
```json
{
  "uploads": 150,
  "evaluations": 120,
  "completed": 115,
  "avg_time": 45.2,
  "failed": 5
}
```

### Job Management

```bash
# Get job list
curl -X GET "http://localhost:8080/admin/api/jobs?page=1&limit=10" \
  -H "Cookie: session=your_session_cookie"

# Get job details
curl -X GET http://localhost:8080/admin/api/jobs/job_1234567890 \
  -H "Cookie: session=your_session_cookie"
```

## Error Handling

### Common Error Responses

**400 Bad Request**:
```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "validation failed",
    "details": {
      "cv_id": "required"
    }
  }
}
```

**413 Payload Too Large**:
```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "payload too large",
    "details": {
      "max_mb": 10
    }
  }
}
```

**429 Too Many Requests**:
```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "rate limit exceeded",
    "details": {
      "retry_after": 60
    }
  }
}
```

### Error Handling Patterns

**JavaScript**:
```javascript
async function handleApiCall(apiCall) {
  try {
    const response = await apiCall();
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(`API Error: ${error.error.message}`);
    }
    
    return await response.json();
  } catch (error) {
    if (error.name === 'TypeError') {
      throw new Error('Network error - check connection');
    }
    throw error;
  }
}
```

**Python**:
```python
def handle_api_call(api_call):
    try:
        response = api_call()
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        if e.response is not None:
            error_data = e.response.json()
            raise Exception(f"API Error: {error_data['error']['message']}")
        else:
            raise Exception(f"Network error: {str(e)}")
```

## Rate Limiting

### Rate Limit Headers
```http
X-RateLimit-Limit: 30
X-RateLimit-Remaining: 29
X-RateLimit-Reset: 1640995200
```

### Handling Rate Limits
```javascript
async function makeRequestWithRetry(requestFn, maxRetries = 3) {
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      return await requestFn();
    } catch (error) {
      if (error.status === 429) {
        const retryAfter = error.headers.get('Retry-After');
        const delay = retryAfter ? parseInt(retryAfter) * 1000 : 60000;
        
        if (attempt < maxRetries - 1) {
          await new Promise(resolve => setTimeout(resolve, delay));
          continue;
        }
      }
      throw error;
    }
  }
}
```

## Caching with ETags

### ETag Support
```bash
# First request
curl -X GET http://localhost:8080/v1/result/job_1234567890

# Response includes ETag
# ETag: "abc123def456"

# Subsequent request with If-None-Match
curl -X GET http://localhost:8080/v1/result/job_1234567890 \
  -H "If-None-Match: abc123def456"

# Response: 304 Not Modified (no body)
```

### ETag Implementation
```javascript
class ResultCache {
  constructor() {
    this.cache = new Map();
  }
  
  async getResult(jobId, etag = null) {
    const response = await fetch(`/v1/result/${jobId}`, {
      headers: etag ? { 'If-None-Match': etag } : {}
    });
    
    if (response.status === 304) {
      return this.cache.get(jobId);
    }
    
    const result = await response.json();
    const newEtag = response.headers.get('ETag');
    
    this.cache.set(jobId, { result, etag: newEtag });
    return result;
  }
}
```

## Best Practices

### 1. Idempotency
- Always use unique Idempotency-Key headers
- Handle duplicate requests gracefully
- Implement retry logic with exponential backoff

### 2. Error Handling
- Check HTTP status codes
- Parse error responses
- Implement proper retry logic
- Log errors for debugging

### 3. Performance
- Use ETags for caching
- Implement connection pooling
- Set appropriate timeouts
- Monitor rate limits

### 4. Security
- Use HTTPS in production
- Validate all inputs
- Implement proper authentication
- Handle sensitive data carefully

---

*This guide provides comprehensive examples for integrating with the AI CV Evaluator API across different programming languages and use cases.*
