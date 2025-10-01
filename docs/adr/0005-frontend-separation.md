# Frontend Separation Decision
Date: 2024-12-19
Status: Accepted

## Context

The AI CV Evaluator project initially used Go templates to serve the admin dashboard directly from the backend server. This approach had several limitations:

1. **Development Experience**: No Hot Module Replacement (HMR) for frontend development
2. **Debugging**: Limited browser dev tools integration
3. **Technology Stack**: Go templates are less modern compared to Vue/React ecosystems
4. **Scalability**: Frontend and backend are tightly coupled
5. **Deployment**: Frontend changes require backend restarts

The admin dashboard serves as a testing and management interface for the API, allowing users to:
- Upload CV and project files
- Submit evaluation requests
- View results and job status
- Monitor system health

## Decision

We will separate the frontend from the backend by:

1. **Create a standalone Vue 3 + Vite frontend application** in `admin-frontend/` directory
2. **Convert Go templates to Vue components** with modern development tooling
3. **Implement API-only backend** (now default)
4. **Use Nginx as reverse proxy** in production to serve frontend and proxy API calls
5. **Maintain backward compatibility** with traditional backend-only deployment

### Technology Choices

- **Frontend Framework**: Vue 3 with Composition API
- **Build Tool**: Vite for HMR and fast builds
- **Styling**: Tailwind CSS for utility-first styling
- **State Management**: Pinia for Vue state management
- **Routing**: Vue Router for SPA navigation
- **TypeScript**: For type safety and better development experience

### Architecture Changes

```
Before:
┌─────────────────┐
│   Go Server     │
│   (Templates)   │
└─────────────────┘

After:
┌─────────────────┐    HTTP API    ┌─────────────────┐
│   Vue Frontend  │◄──────────────►│   Go Backend    │
│   (Port 3001)   │                │   (Port 8080)   │
└─────────────────┘                └─────────────────┘
```

## Consequences

### Positive

1. **Development Experience**
   - Hot Module Replacement (HMR) for instant updates
   - Better debugging with browser dev tools
   - Modern development workflow with Vite
   - Independent frontend development

2. **Scalability**
   - Frontend and backend can be scaled independently
   - CDN deployment for frontend static files
   - Better resource utilization

3. **Technology Benefits**
   - Modern Vue 3 ecosystem
   - Better TypeScript support
   - Improved developer productivity
   - Easier testing and maintenance

4. **Deployment Flexibility**
   - Independent deployment cycles
   - Frontend can be deployed to CDN
   - Backend remains API-only

### Negative

1. **Complexity**
   - Additional Docker container for frontend
   - More complex deployment pipeline
   - Need to manage CORS configuration

2. **Development Setup**
   - Requires Node.js for frontend development
   - More dependencies to manage
   - Need to run both frontend and backend

3. **Network Overhead**
   - Additional HTTP requests between frontend and backend
   - Need to handle authentication across services

### Risks

1. **Authentication Complexity**
   - Session management across services
   - CORS configuration for cross-origin requests
   - Security considerations for API endpoints

2. **Deployment Complexity**
   - More containers to manage
   - Nginx configuration for reverse proxy
   - SSL certificate management

## Alternatives Considered

### Option A: Keep Go Templates
- **Pros**: Simple deployment, no additional complexity
- **Cons**: Poor development experience, limited modern tooling
- **Decision**: Rejected due to development experience limitations

### Option B: React + Vite
- **Pros**: Popular framework, good ecosystem
- **Cons**: Larger bundle size, more complex state management
- **Decision**: Rejected in favor of Vue 3 for simplicity

### Option C: Svelte + Vite
- **Pros**: Smaller bundle size, good performance
- **Cons**: Smaller ecosystem, less familiar to team
- **Decision**: Rejected due to ecosystem concerns

### Option D: Server-Side Rendering (SSR)
- **Pros**: Better SEO, faster initial load
- **Cons**: More complex deployment, not needed for admin dashboard
- **Decision**: Rejected as not necessary for internal admin tool

## Implementation Plan

1. **Phase 1**: Create Vue 3 frontend application
   - Set up Vite + Vue 3 project structure
   - Convert Go templates to Vue components
   - Implement basic routing and state management

2. **Phase 2**: Backend API modifications
   - Remove `FRONTEND_SEPARATED` configuration flag (now default)
   - Remove template rendering from backend
   - Add CORS configuration for frontend

3. **Phase 3**: Development workflow
   - Create Makefile targets for frontend development
   - Update Docker Compose for development
   - Add frontend to CI/CD pipeline

4. **Phase 4**: Production deployment
   - Create production Docker image for frontend
   - Configure Nginx reverse proxy
   - Update deployment documentation

## Monitoring and Metrics

- Frontend build time and bundle size
- API response times from frontend
- User experience metrics
- Development productivity improvements

## Success Criteria

1. **Development Experience**
   - HMR working for frontend changes
   - Faster development iteration
   - Better debugging capabilities

2. **Functionality**
   - All admin features working via API
   - Authentication working across services
   - No regression in user experience

3. **Performance**
   - Frontend load time < 2 seconds
   - API response times maintained
   - No significant performance degradation

## Future Considerations

1. **Mobile Responsiveness**: Ensure admin dashboard works on mobile devices
2. **PWA Features**: Consider adding Progressive Web App capabilities
3. **Testing**: Add frontend unit and integration tests
4. **Monitoring**: Add frontend error tracking and performance monitoring
5. **Accessibility**: Ensure admin dashboard is accessible

---

**Decision Made By**: Development Team  
**Review Date**: 2024-12-19  
**Next Review**: 2025-03-19
