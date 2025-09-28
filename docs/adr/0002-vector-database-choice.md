# ADR-0002: Vector Database Choice

**Date:** 2025-09-28  
**Status:** Accepted  

## Context

The system requires a vector database for RAG (Retrieval-Augmented Generation) functionality. We need to store and search embeddings for:

- Job description corpus for CV matching
- Scoring rubric corpus for project evaluation
- Efficient semantic similarity search during evaluation

Requirements:
- Support for dense vector embeddings (1536 dimensions from OpenAI)
- Fast similarity search (cosine similarity)
- Easy integration with Go applications
- Simple deployment and maintenance
- Support for metadata filtering and payload storage

## Decision

We will use **Qdrant** as our vector database solution.

## Consequences

### Positive
- **Go SDK**: Excellent Go client library with type safety
- **Docker Native**: Easy deployment via Docker containers
- **Performance**: Optimized for similarity search with HNSW indexing
- **Metadata Support**: Rich payload support for storing corpus metadata
- **HTTP API**: RESTful API for debugging and administration
- **Memory Efficient**: Configurable storage options (memory/disk)
- **Filtering**: Advanced filtering capabilities on metadata
- **Open Source**: No vendor lock-in, community-driven

### Negative
- **Additional Service**: Adds another service to deployment stack
- **Memory Usage**: Requires sufficient RAM for optimal performance
- **Single Node**: Currently deployed as single instance (no clustering)
- **Learning Curve**: Team needs to learn Qdrant-specific concepts

### Risks
- Data loss if Qdrant container fails without persistent volumes
- Performance degradation if memory is insufficient
- Service unavailability affects evaluation pipeline

## Alternatives Considered

### Option A: PostgreSQL with pgvector
- **Pros**: Uses existing PostgreSQL, no additional service, SQL familiar
- **Cons**: Limited vector operations, performance issues at scale, complex setup
- **Rejected**: pgvector extension adds complexity and performance limitations

### Option B: Elasticsearch with Dense Vector
- **Pros**: Mature ecosystem, good text search, familiar to many teams
- **Cons**: Heavy resource usage, complex configuration, over-engineered for vectors only
- **Rejected**: Too resource-intensive for VPS deployment

### Option C: Pinecone
- **Pros**: Fully managed, excellent performance, built for vectors
- **Cons**: Vendor lock-in, subscription costs, requires internet connectivity
- **Rejected**: Project specifies VPS deployment, avoiding cloud dependencies

### Option D: Chroma
- **Pros**: Simple setup, Python-friendly, lightweight
- **Cons**: Limited Go support, newer project, fewer production deployments
- **Rejected**: Poor Go ecosystem support

### Option E: Weaviate
- **Pros**: GraphQL API, good Go support, feature-rich
- **Cons**: Complex setup, heavy resource usage, learning curve
- **Rejected**: Over-engineered for simple RAG use case
