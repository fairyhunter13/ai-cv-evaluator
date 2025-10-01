# Test Data Structure and Helpers

This document provides comprehensive documentation for the test data structure, helper functions, and testing utilities in the AI CV Evaluator project.

## Overview

The project uses a well-organized test data structure with helper functions to support unit tests, integration tests, and end-to-end (E2E) tests. The test infrastructure is designed for reliability, maintainability, and performance.

## Test Data Organization

### Directory Structure

```
test/
├── e2e/                    # End-to-end tests
│   ├── helpers_e2e_test.go
│   ├── helpers_smoke_e2e_test.go
│   ├── happy_path_e2e_test.go
│   ├── smoke_random_e2e_test.go
│   └── rfc_real_responses_e2e_test.go
├── testdata/              # Test data files
│   ├── cv_01.txt          # CV test data 1
│   ├── cv_02.txt          # CV test data 2
│   ├── ...
│   ├── cv_10.txt          # CV test data 10
│   ├── project_01.txt     # Project test data 1
│   ├── project_02.txt     # Project test data 2
│   ├── ...
│   ├── project_10.txt     # Project test data 10
│   ├── cv_optimized_2025.md
│   └── project_repo_report.txt
└── dump/                  # Test output dumps
    ├── happy_path_upload_response.json
    ├── happy_path_evaluate_response.json
    └── happy_path_result_response.json
```

## Test Data Files

### CV Test Data (`testdata/cv_*.txt`)

**Purpose**: Sample CV documents for testing evaluation functionality.

**Content Structure**:
```
Name: John Doe
Email: john.doe@example.com
Phone: +1-555-0123

EXPERIENCE
Senior Software Engineer - Tech Corp (2020-Present)
- Developed microservices using Go and Docker
- Implemented CI/CD pipelines with GitHub Actions
- Led team of 5 developers

Software Engineer - StartupXYZ (2018-2020)
- Built REST APIs using Node.js and Express
- Worked with PostgreSQL and Redis
- Implemented authentication and authorization

EDUCATION
Bachelor of Computer Science - University of Technology (2014-2018)
- GPA: 3.8/4.0
- Relevant Coursework: Data Structures, Algorithms, Database Systems

SKILLS
Programming Languages: Go, JavaScript, Python, Java
Frameworks: Express.js, React, Vue.js
Databases: PostgreSQL, MongoDB, Redis
Tools: Docker, Kubernetes, Git, Linux
```

**Test Data Variations**:
- `cv_01.txt` - Standard software engineer CV
- `cv_02.txt` - Data scientist profile
- `cv_03.txt` - DevOps engineer CV
- `cv_04.txt` - Frontend developer CV
- `cv_05.txt` - Backend developer CV
- `cv_06.txt` - Full-stack developer CV
- `cv_07.txt` - Machine learning engineer CV
- `cv_08.txt` - Cloud architect CV
- `cv_09.txt` - Security engineer CV
- `cv_10.txt` - Product manager CV

### Project Test Data (`testdata/project_*.txt`)

**Purpose**: Sample project reports for testing evaluation functionality.

**Content Structure**:
```
PROJECT TITLE: AI-Powered CV Evaluation System

OVERVIEW
This project implements an AI-powered system for evaluating CVs and project reports against job descriptions and study case briefs. The system uses machine learning models to provide structured feedback and scoring.

TECHNICAL IMPLEMENTATION
- Backend: Go with Clean Architecture
- Database: PostgreSQL with connection pooling
- Queue System: Redpanda (Kafka-compatible)
- AI Integration: OpenRouter and OpenAI APIs
- Vector Database: Qdrant for RAG functionality
- Text Extraction: Apache Tika
- Observability: OpenTelemetry, Prometheus, Grafana

KEY FEATURES
1. File Upload and Processing
   - Support for PDF, DOCX, and TXT files
   - Text extraction using Apache Tika
   - File validation and size limits

2. AI-Powered Evaluation
   - Two-pass prompting for consistency
   - RAG (Retrieval-Augmented Generation)
   - Structured JSON output
   - Retry logic with exponential backoff

3. Queue-Based Processing
   - Asynchronous job processing
   - Exactly-once semantics
   - Horizontal scaling with worker replicas

4. Observability and Monitoring
   - Comprehensive metrics collection
   - Distributed tracing
   - Health checks and readiness probes
   - Performance monitoring

CHALLENGES AND SOLUTIONS
1. Performance Optimization
   - Implemented container pooling for tests
   - Optimized database queries
   - Used connection pooling
   - Implemented caching strategies

2. Reliability and Error Handling
   - Comprehensive error handling
   - Retry mechanisms with backoff
   - Graceful degradation
   - Circuit breaker patterns

3. Scalability
   - Horizontal scaling with worker replicas
   - Queue-based processing
   - Database connection pooling
   - Load balancing

RESULTS AND IMPACT
- Reduced evaluation time from 5 minutes to 30 seconds
- Improved accuracy through RAG integration
- Enhanced reliability with exactly-once processing
- Better observability with comprehensive monitoring

TECHNOLOGIES USED
- Go, PostgreSQL, Redpanda, Qdrant
- OpenRouter, OpenAI, Apache Tika
- Docker, Kubernetes, GitHub Actions
- OpenTelemetry, Prometheus, Grafana
```

**Test Data Variations**:
- `project_01.txt` - AI/ML project report
- `project_02.txt` - Web application project
- `project_03.txt` - Microservices architecture
- `project_04.txt` - Data pipeline project
- `project_05.txt` - Mobile application project
- `project_06.txt` - DevOps automation project
- `project_07.txt` - Blockchain application
- `project_08.txt` - IoT system project
- `project_09.txt` - Security tool development
- `project_10.txt` - Analytics platform

### Special Test Data

#### `cv_optimized_2025.md`
**Purpose**: Real CV data for RFC testing and demonstrations.

**Content**: Optimized CV format with modern structure and comprehensive experience.

#### `project_repo_report.txt`
**Purpose**: Repository-based project report for RFC testing.

**Content**: Detailed project report based on the actual repository implementation.

## E2E Test Helpers

### Core Helper Functions (`helpers_e2e_test.go`)

#### `uploadTestFiles(t, client, cvContent, projectContent)`
**Purpose**: Upload CV and project files for testing.

**Parameters**:
- `t`: Testing.T instance
- `client`: HTTP client
- `cvContent`: CV text content
- `projectContent`: Project text content

**Returns**: `map[string]string` with `cv_id` and `project_id`

**Implementation**:
```go
func uploadTestFiles(t *testing.T, client *http.Client, cvContent, projectContent string) map[string]string {
    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)

    cvWriter, err := writer.CreateFormFile("cv", "test_cv.txt")
    require.NoError(t, err)
    _, _ = cvWriter.Write([]byte(cvContent))

    projWriter, err := writer.CreateFormFile("project", "test_project.txt")
    require.NoError(t, err)
    _, _ = projWriter.Write([]byte(projectContent))

    _ = writer.Close()

    // Retry logic for reliability
    var lastStatus int
    for i := 0; i < 6; i++ {
        req, err := http.NewRequest("POST", baseURL+"/upload", &buf)
        require.NoError(t, err)
        req.Header.Set("Content-Type", writer.FormDataContentType())
        maybeBasicAuth(req)
        
        resp, err := client.Do(req)
        require.NoError(t, err)
        lastStatus = resp.StatusCode
        
        if resp.StatusCode == http.StatusOK {
            defer resp.Body.Close()
            var result map[string]string
            err = json.NewDecoder(resp.Body).Decode(&result)
            require.NoError(t, err)
            return result
        }
        
        resp.Body.Close()
        if resp.StatusCode == http.StatusTooManyRequests {
            time.Sleep(500 * time.Millisecond)
            continue
        }
        break
    }
    
    require.Equal(t, http.StatusOK, lastStatus)
    return map[string]string{}
}
```

#### `evaluateFiles(t, client, cvID, projectID)`
**Purpose**: Submit evaluation job for uploaded files.

**Parameters**:
- `t`: Testing.T instance
- `client`: HTTP client
- `cvID`: CV identifier
- `projectID`: Project identifier

**Returns**: `map[string]interface{}` with job information

**Implementation**:
```go
func evaluateFiles(t *testing.T, client *http.Client, cvID, projectID string) map[string]interface{} {
    payload := map[string]interface{}{
        "cv_id":             cvID,
        "project_id":        projectID,
        "job_description":   defaultJobDescription,
        "study_case_brief":  defaultStudyCaseBrief,
    }

    jsonData, err := json.Marshal(payload)
    require.NoError(t, err)

    req, err := http.NewRequest("POST", baseURL+"/evaluate", bytes.NewBuffer(jsonData))
    require.NoError(t, err)
    req.Header.Set("Content-Type", "application/json")
    maybeBasicAuth(req)

    resp, err := client.Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    require.Equal(t, http.StatusOK, resp.StatusCode)

    var result map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&result)
    require.NoError(t, err)

    return result
}
```

#### `waitForCompleted(t, client, jobID, timeout)`
**Purpose**: Poll job status until completion or timeout.

**Parameters**:
- `t`: Testing.T instance
- `client`: HTTP client
- `jobID`: Job identifier
- `timeout`: Maximum wait time

**Returns**: Final job result

**Implementation**:
```go
func waitForCompleted(t *testing.T, client *http.Client, jobID string, timeout time.Duration) map[string]interface{} {
    start := time.Now()
    for time.Since(start) < timeout {
        resp, err := client.Get(baseURL + "/result/" + jobID)
        require.NoError(t, err)
        defer resp.Body.Close()

        var result map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&result)
        require.NoError(t, err)

        status, _ := result["status"].(string)
        if status == "completed" || status == "failed" {
            return result
        }

        time.Sleep(2 * time.Second)
    }

    t.Fatalf("Job %s did not complete within %v", jobID, timeout)
    return nil
}
```

### Smoke Test Helpers (`helpers_smoke_e2e_test.go`)

#### `availablePairs()`
**Purpose**: Get available CV/Project test data pairs.

**Returns**: `[]pair` with CV and Project content

**Implementation**:
```go
func availablePairs() []pair {
    var pairs []pair
    
    for i := 1; i <= 10; i++ {
        cvPath := fmt.Sprintf("../testdata/cv_%02d.txt", i)
        projPath := fmt.Sprintf("../testdata/project_%02d.txt", i)
        
        cvBytes, err := os.ReadFile(cvPath)
        if err != nil {
            continue
        }
        
        projBytes, err := os.ReadFile(projPath)
        if err != nil {
            continue
        }
        
        pairs = append(pairs, pair{
            CV:      cvBytes,
            Project: projBytes,
        })
    }
    
    return pairs
}
```

### Utility Functions

#### `dumpJSON(t, filename, data)`
**Purpose**: Save test responses to files for debugging.

**Parameters**:
- `t`: Testing.T instance
- `filename`: Output filename
- `data`: Data to save

**Implementation**:
```go
func dumpJSON(t *testing.T, filename string, data interface{}) {
    if os.Getenv("E2E_CLEAR_DUMP") == "true" {
        return
    }
    
    jsonData, err := json.MarshalIndent(data, "", "  ")
    require.NoError(t, err)
    
    err = os.WriteFile(filepath.Join("test/dump", filename), jsonData, 0644)
    require.NoError(t, err)
}
```

#### `maybeBasicAuth(req)`
**Purpose**: Add basic authentication if configured.

**Parameters**:
- `req`: HTTP request

**Implementation**:
```go
func maybeBasicAuth(req *http.Request) {
    if username := os.Getenv("E2E_USERNAME"); username != "" {
        if password := os.Getenv("E2E_PASSWORD"); password != "" {
            req.SetBasicAuth(username, password)
        }
    }
}
```

## Test Configuration

### Environment Variables

**E2E Test Configuration**:
```bash
# Base URL for E2E tests
E2E_BASE_URL=http://localhost:8080/v1

# Authentication (optional)
E2E_USERNAME=admin
E2E_PASSWORD=changeme

# Test behavior
E2E_CLEAR_DUMP=true          # Clear dump directory
E2E_START_SERVICES=false     # Don't start services
E2E_TIMEOUT=5m              # Test timeout
```

### Test Data Generation

**Automated Test Data Creation**:
```go
func generateTestData() {
    // Generate CV test data
    for i := 1; i <= 10; i++ {
        cvContent := generateCVContent(i)
        err := os.WriteFile(fmt.Sprintf("testdata/cv_%02d.txt", i), []byte(cvContent), 0644)
        require.NoError(t, err)
    }
    
    // Generate project test data
    for i := 1; i <= 10; i++ {
        projectContent := generateProjectContent(i)
        err := os.WriteFile(fmt.Sprintf("testdata/project_%02d.txt", i), []byte(projectContent), 0644)
        require.NoError(t, err)
    }
}
```

## Test Execution

### Running E2E Tests

**Basic E2E Test**:
```bash
make test-e2e
```

**E2E Test with Services**:
```bash
make run-e2e-tests E2E_START_SERVICES=true
```

**E2E Test with Custom URL**:
```bash
make run-e2e-tests E2E_BASE_URL=http://localhost:8080/v1
```

**E2E Test with Logging**:
```bash
make run-e2e-tests E2E_LOG_DIR=artifacts/test-logs
```

### Test Categories

#### 1. Happy Path Tests (`happy_path_e2e_test.go`)
**Purpose**: Test the complete workflow from upload to result.

**Test Flow**:
1. Upload CV and project files
2. Submit evaluation job
3. Poll for completion
4. Verify result structure

#### 2. Smoke Tests (`smoke_random_e2e_test.go`)
**Purpose**: Random testing with various test data combinations.

**Test Flow**:
1. Select random CV/Project pair
2. Upload and evaluate
3. Verify completion
4. Check result validity

#### 3. RFC Tests (`rfc_real_responses_e2e_test.go`)
**Purpose**: Generate real responses for RFC documentation.

**Test Flow**:
1. Use real CV and project data
2. Generate evaluation responses
3. Save responses for documentation
4. Verify response structure

## Test Data Maintenance

### Adding New Test Data

**CV Test Data**:
1. Create new file: `testdata/cv_11.txt`
2. Follow CV format structure
3. Include realistic content
4. Update test data index

**Project Test Data**:
1. Create new file: `testdata/project_11.txt`
2. Follow project format structure
3. Include technical details
4. Update test data index

### Test Data Validation

**Content Validation**:
```go
func validateTestData() error {
    // Check CV files
    for i := 1; i <= 10; i++ {
        cvPath := fmt.Sprintf("testdata/cv_%02d.txt", i)
        content, err := os.ReadFile(cvPath)
        if err != nil {
            return fmt.Errorf("missing CV file: %s", cvPath)
        }
        
        if len(content) < 100 {
            return fmt.Errorf("CV file too short: %s", cvPath)
        }
    }
    
    // Check project files
    for i := 1; i <= 10; i++ {
        projPath := fmt.Sprintf("testdata/project_%02d.txt", i)
        content, err := os.ReadFile(projPath)
        if err != nil {
            return fmt.Errorf("missing project file: %s", projPath)
        }
        
        if len(content) < 200 {
            return fmt.Errorf("project file too short: %s", projPath)
        }
    }
    
    return nil
}
```

## Best Practices

### 1. Test Data Design
- Use realistic content
- Include edge cases
- Maintain consistency
- Document variations

### 2. Helper Functions
- Keep functions focused
- Use descriptive names
- Handle errors gracefully
- Provide clear feedback

### 3. Test Organization
- Group related tests
- Use descriptive test names
- Include setup and teardown
- Clean up resources

### 4. Performance
- Use container pooling
- Optimize test execution
- Parallel test execution
- Resource cleanup

### 5. Reliability
- Implement retry logic
- Handle timeouts gracefully
- Use proper assertions
- Log test execution

## Troubleshooting

### Common Issues

1. **Test Data Not Found**
   ```bash
   # Check test data files
   ls -la test/testdata/
   
   # Verify file permissions
   chmod 644 test/testdata/*
   ```

2. **Upload Failures**
   ```bash
   # Check file size limits
   # Verify MIME types
   # Check network connectivity
   ```

3. **Evaluation Timeouts**
   ```bash
   # Increase timeout values
   # Check AI service availability
   # Verify queue system
   ```

4. **Test Environment Issues**
   ```bash
   # Check environment variables
   echo $E2E_BASE_URL
   
   # Verify service availability
   curl -f $E2E_BASE_URL/healthz
   ```

### Debug Tools

**Test Output Analysis**:
```bash
# View test dumps
ls -la test/dump/

# Analyze JSON responses
cat test/dump/happy_path_result_response.json | jq .

# Check test logs
tail -f artifacts/test-logs/compose.follow.log
```

**Performance Analysis**:
```bash
# Monitor resource usage
docker stats

# Check queue metrics
curl http://localhost:8090/api/topics

# View application logs
docker-compose logs app worker
```

---

*This documentation should be updated when new test data is added or test helpers are modified.*
