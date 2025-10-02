# AI and LLM Pipeline

This document describes the AI pipeline design, prompt engineering, RAG implementation, and resilience patterns for the AI CV Evaluator service.

## 🎯 Overview

The AI pipeline processes CV and project evaluations using a combination of:
- **Large Language Models (LLM)** for text analysis and scoring
- **Retrieval Augmented Generation (RAG)** for context injection
- **Vector embeddings** for semantic similarity
- **Structured output validation** for consistent results

## 🔧 Provider Configuration

### Chat Provider (OpenRouter)
- **Default**: `openrouter`
- **Environment Variables**:
  - `OPENROUTER_API_KEY` (required for live)
  - `OPENROUTER_BASE_URL` (default: `https://openrouter.ai/api/v1`)
  - `CHAT_MODEL` (not used - free models auto-selected)

### Embeddings Provider (OpenAI)
- **Default**: `openai`
- **Environment Variables**:
  - `OPENAI_API_KEY` (required for embeddings)
  - `OPENAI_BASE_URL` (default: `https://api.openai.com/v1`)
  - `EMBEDDINGS_MODEL` (default: `text-embedding-3-small`)

### Endpoints
- **Chat**: `POST ${OPENROUTER_BASE_URL}/chat/completions`
- **Embeddings**: `POST ${OPENAI_BASE_URL}/embeddings`

## 🔄 Pipeline Stages

### 1. Text Extraction and Normalization
- **Extract text** from uploaded documents (PDF, DOCX, TXT)
- **Normalize content**: Strip control characters, preserve headings
- **Chunk text**: 512–1024 tokens with 10–20% overlap
- **Store metadata**: Source, section, weight for filtering

### 2. CV Analysis
- **Extract structured info**: Skills, experience, projects
- **Compare against job description** using RAG
- **Produce match rate** and CV feedback
- **Score parameters**: Technical skills, experience level, achievements, cultural fit

### 3. Project Evaluation
- **Evaluate project report** against scoring rubric
- **Use RAG** for context from study case brief
- **Score parameters**: Correctness, code quality, resilience, documentation, creativity
- **Generate project score** and feedback

### 4. Result Aggregation
- **Combine CV and project scores**
- **Generate overall summary**
- **Apply scoring weights** and normalization
- **Validate final output** against schema

## 🧠 RAG (Retrieval Augmented Generation)

### Vector Database (Qdrant)
**Collections**:
- `job_description` (payload: title, section, text)
- `scoring_rubric` (payload: parameter, weight, description)

**Configuration**:
- **Distance metric**: Cosine or dot product
- **Vector size**: Consistent across collections
- **Payload indexes**: For frequent filters
- **Idempotent creation**: Skip if exists

### Embeddings Strategy
- **Model**: `text-embedding-3-small` (configurable)
- **Batch processing**: 8–16 items with concurrency limits
- **Caching**: Hash-based deduplication
- **Token limits**: Truncate long texts appropriately

### Retrieval Process
- **Similarity search**: Top-k tuned (k=4–8)
- **Re-ranking**: Optional for better precision
- **Context injection**: Merge static and ephemeral content
- **Metadata filtering**: Use source, section, weight

## 📝 Prompting & Schema Design

### System Prompt Principles
- **JSON-only output**: No freeform text
- **Structured responses**: Enforce schema compliance
- **Numeric scales**: 1–5 for sub-scores
- **Concise feedback**: 1–3 sentences for feedback fields
- **No chain-of-thought**: Explicit instruction to avoid reasoning in output

### Two-Pass Strategy
1. **Pass 1**: Raw scoring with retrieved context
2. **Pass 2**: Normalization and validation
   - Fix ranges and fill required fields
   - Ensure consistency across parameters
   - Validate against JSON schema

### Chain-of-Thought Prevention
- **Explicit instruction**: "You may reason privately but must only output valid JSON that matches the schema. Do not include chain-of-thought or internal reasoning in the output."
- **Validation patterns**: Reject outputs containing "Step 1", "First,", "I think", lengthy numbered lists
- **Retry logic**: Retry with clarifying instructions on validation failures
- **Field length limits**: Enforce concise feedback (1–3 sentences)

## 📊 JSON Schemas

### CV Evaluation Schema
```json
{
  "cv": {
    "technical_skills": {"score": 1, "feedback": ""},
    "experience_level": {"score": 1, "feedback": ""},
    "achievements": {"score": 1, "feedback": ""},
    "cultural_fit": {"score": 1, "feedback": ""}
  },
  "cv_match_rate": 0.0,
  "cv_feedback": ""
}
```

### Project Evaluation Schema
```json
{
  "project": {
    "correctness": {"score": 1, "feedback": ""},
    "code_quality": {"score": 1, "feedback": ""},
    "resilience": {"score": 1, "feedback": ""},
    "documentation": {"score": 1, "feedback": ""},
    "creativity": {"score": 1, "feedback": ""}
  },
  "project_score": 0.0,
  "project_feedback": "",
  "overall_summary": ""
}
```

## 🎯 Scoring Aggregation

### CV Match Rate Calculation
- **Weights**: 40/25/20/15 (technical_skills/experience_level/achievements/cultural_fit)
- **Formula**: `weighted_average(1-5) ÷ 5 × 0.2`
- **Range**: [0.0, 1.0]
- **Display**: Multiply by 100 for percentage

### Project Score Calculation
- **Weights**: 30/25/20/15/10 (correctness/code_quality/resilience/documentation/creativity)
- **Formula**: `weighted_average(1-5) × 2`
- **Range**: [1.0, 10.0]
- **Clamp**: Ensure minimum of 1.0

### Final Output Fields
- `cv_match_rate`: Normalized fraction [0.0, 1.0]
- `cv_feedback`: Concise feedback (1–3 sentences)
- `project_score`: Numeric score [1.0, 10.0]
- `project_feedback`: Concise feedback (1–3 sentences)
- `overall_summary`: Comprehensive summary (3–5 sentences)

## 🔄 Asynchronous Processing

### Job Lifecycle
1. **Enqueue**: `/evaluate` creates job with status "queued"
2. **Processing**: Worker picks up job, status "processing"
3. **AI Pipeline**: Run RAG + LLM chain
4. **Aggregation**: Combine scores and generate summary
5. **Completion**: Store result, status "completed"
6. **Polling**: `/result/{id}` returns current status

### Worker Implementation
- **Context-aware**: Respect cancellation from queue
- **Bounded concurrency**: Worker pools with limits
- **Idempotent**: Safe retries and upserts
- **Observability**: Log progress and metrics

## 🛡️ Resilience Patterns

### Timeout Management
- **LLM calls**: 10–15s timeout
- **Embeddings**: 5–10s timeout
- **Vector DB**: 3–5s timeout
- **Context propagation**: Respect cancellation

### Retry Strategy
- **Exponential backoff**: With jitter
- **Max retries**: 3 attempts for transient failures
- **Circuit breaker**: For persistent upstream failures
- **Rate limiting**: Token bucket for API calls

### Error Handling
- **Structured errors**: Consistent taxonomy
- **Graceful degradation**: Fallback to cached results
- **Observability**: Log errors with context
- **Recovery**: Automatic retry with backoff

## 💰 Cost Optimization

### Token Management
- **Compact prompts**: Remove irrelevant context
- **Cap max_tokens**: Prevent runaway generation
- **Batch requests**: Group embeddings calls
- **Caching**: Memoize intermediate results

### Concurrency Control
- **Rate limiting**: Per-provider limits
- **Worker pools**: Bounded concurrency
- **Backpressure**: Queue depth monitoring
- **Circuit breaker**: Fail fast on persistent errors

### Caching Strategy
- **Embeddings**: Hash-based deduplication
- **LLM outputs**: Cache by input hash
- **Vector results**: TTL-based caching
- **Intermediate results**: Reuse across stages

## 🔍 Quality Assurance

### Validation Pipeline
- **JSON schema validation**: All LLM responses
- **Range validation**: Scores within expected bounds
- **Required fields**: Ensure all fields present
- **Retry logic**: On validation failures

### Testing Strategy
- **Golden tests**: Stable prompt I/O pairs
- **Schema compliance**: Verify output structure
- **Score distribution**: Monitor for drift
- **E2E validation**: Live provider testing

### Monitoring
- **Token usage**: Track costs per request
- **Latency metrics**: Per-provider timing
- **Error rates**: Success/failure tracking
- **Score drift**: Distribution monitoring

## 🚀 Performance Optimization

### Prompt Engineering
- **Temperature**: 0.2–0.4 for consistency
- **Top-p**: Default value
- **Max tokens**: Bounded for cost control
- **System prompts**: Optimized for schema compliance

### RAG Optimization
- **Chunk size**: 512–1024 tokens optimal
- **Overlap**: 10–20% for context continuity
- **Top-k**: Tuned for accuracy (4–8)
- **Re-ranking**: Optional for precision

### Caching Strategy
- **Stable hashing**: Deterministic cache keys
- **TTL management**: Appropriate expiration
- **Invalidation**: On corpus updates
- **Memory limits**: Prevent unbounded growth

## ✅ Definition of Done (AI Pipeline)

### Implementation Requirements
- **E2E validation** with live providers
- **RAG returns relevant chunks** for both CV and project tasks
- **LLM outputs valid JSON** passing schema validation
- **Retry logic verified** for transient failures
- **Cost controls** implemented and monitored

### Quality Requirements
- **Schema compliance** over representative test set
- **Golden tests** capture stability
- **Score distribution** monitoring
- **Error handling** comprehensive
- **Performance** within acceptable limits

This document serves as the comprehensive guide for AI pipeline implementation, ensuring robust, cost-effective, and high-quality AI-powered CV evaluation.
