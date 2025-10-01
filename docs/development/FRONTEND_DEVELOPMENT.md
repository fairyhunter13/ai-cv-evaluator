# Frontend Development Guide

This comprehensive guide covers frontend development with Hot Module Replacement (HMR), modern development tools, and the separated frontend architecture.

## Overview

The admin dashboard has been separated into a standalone Vue 3 + Vite frontend application that communicates with the Go backend via API calls. This enables:

- **Hot Module Replacement (HMR)** for instant frontend updates
- **Better debugging** with browser dev tools
- **Independent frontend development** without backend restarts
- **Modern development workflow** with Vite

## Architecture

```
┌─────────────────┐    HTTP API    ┌─────────────────┐
│   Frontend      │◄──────────────►│   Backend       │
│   (Vue 3 + Vite)│                │   (Go API)      │
│   Port: 3001    │                │   Port: 8080    │
└─────────────────┘                └─────────────────┘
```

## 🚀 Quick Start

### Option 1: Using Makefile (Recommended)

```bash
# Show all available frontend commands
make frontend-help

# Install dependencies
make frontend-install

# Start frontend development server (HMR enabled)
make frontend-dev
```

### Option 2: Using Development Script

```bash
# Start full development environment (backend + frontend)
./scripts/dev-frontend.sh
```

### Option 3: Manual Setup

```bash
# 1. Install frontend dependencies
cd admin-frontend
npm install

# 2. Start backend services
docker-compose up -d db redis qdrant tika

# 3. Start backend API (in one terminal)
export FRONTEND_SEPARATED=true
go run cmd/server/main.go

# 4. Start frontend dev server (in another terminal)
cd admin-frontend
npm run dev
```

## 📋 Available Makefile Commands

| Command | Description |
|---------|-------------|
| `make frontend-help` | Show all available frontend commands |
| `make frontend-install` | Install frontend dependencies |
| `make frontend-dev` | Start frontend dev server with HMR |
| `make frontend-build` | Build frontend for production |
| `make frontend-clean` | Clean frontend build artifacts |
| `make dev-full` | Start complete dev environment (backend + frontend) |

## 🌐 Access Points

- **Frontend**: http://localhost:3001 (with HMR!)
- **Backend API**: http://localhost:8080
- **Grafana**: http://localhost:3000
- **Prometheus**: http://localhost:9090
- **Jaeger**: http://localhost:16686

## 🛠️ Development Workflow

### 1. **Frontend Development**
```bash
# Start frontend with HMR
make frontend-dev

# Make changes to Vue components
# Changes appear instantly without page refresh!
```

### 2. **Backend Development**
```bash
# Start backend API
export FRONTEND_SEPARATED=true
go run cmd/server/main.go

# Make changes to Go code
# Restart backend to see changes
```

### 3. **Full Stack Development**
```bash
# Start everything at once
make dev-full

# Or use the script
./scripts/dev-frontend.sh
```

## 🎯 Development Features

### **Hot Module Replacement (HMR)**
- ✅ **Instant Updates** - Changes appear immediately
- ✅ **State Preservation** - Vue component state is maintained
- ✅ **Fast Refresh** - Only changed components are updated
- ✅ **Error Recovery** - Automatic error recovery

### **Modern Development Tools**
- ✅ **Vue DevTools** - Component inspection and debugging
- ✅ **Browser DevTools** - Network, console, performance
- ✅ **TypeScript** - Type safety and IntelliSense
- ✅ **Tailwind CSS** - Utility-first CSS framework
- ✅ **Vite** - Lightning-fast build tool

### **API Integration**
- ✅ **Automatic Proxy** - API calls proxied to backend
- ✅ **CORS Handling** - Cross-origin requests handled
- ✅ **Session Management** - Authentication works seamlessly
- ✅ **Error Handling** - Proper error display and handling

## 📁 Project Structure

```
admin-frontend/
├── src/
│   ├── views/              # Vue page components
│   │   ├── Dashboard.vue   # Main dashboard
│   │   ├── Login.vue       # Authentication
│   │   ├── Upload.vue      # File upload
│   │   ├── Evaluate.vue    # Evaluation form
│   │   └── Result.vue      # Results viewer
│   ├── stores/             # Pinia state management
│   │   └── auth.ts         # Authentication store
│   ├── App.vue             # Main app component
│   ├── main.ts             # Entry point
│   └── style.css           # Global styles
├── public/                 # Static assets
├── package.json            # Dependencies
├── vite.config.ts          # Vite configuration
├── tailwind.config.js      # Tailwind CSS config
├── Dockerfile              # Development container
├── Dockerfile.prod         # Production container
└── nginx.conf              # Production nginx config
```

## 🔧 Configuration

### **Vite Configuration**
```typescript
// vite.config.ts
export default defineConfig({
  server: {
    port: 3001,
    proxy: {
      '/v1': 'http://localhost:8080',
      '/admin/api': 'http://localhost:8080',
    },
  },
})
```

### **Environment Variables**
```bash
# Backend configuration
export FRONTEND_SEPARATED=true
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD=your_password
export ADMIN_SESSION_SECRET=your_session_secret
```

## 🐛 Debugging

### **Frontend Debugging**
1. **Browser DevTools**
   - Console for JavaScript errors
   - Network tab for API calls
   - Vue DevTools for component inspection

2. **Vite Dev Server**
   - Hot reload status
   - Build errors in terminal
   - HMR status in browser

3. **Common Issues**
   - CORS errors → Check backend CORS configuration
   - API connection → Verify backend is running on port 8080
   - HMR not working → Check Vite dev server logs

### **Backend Debugging**
1. **Go Server Logs**
   - Check terminal output for errors
   - API endpoint responses
   - Database connection status

2. **Database Debugging**
   - Check PostgreSQL logs
   - Verify Redis connectivity
   - Monitor Qdrant vector database

## 🚀 Production Build

### **Build for Production**
```bash
# Build frontend
make frontend-build

# Build creates optimized files in admin-frontend/dist/
```

### **Docker Production Build**
```bash
# Build production image
docker build -f admin-frontend/Dockerfile.prod -t ai-cv-evaluator-frontend:latest ./admin-frontend
```

## 📦 Dependencies

### **Frontend Dependencies**
- **Vue 3** - Progressive JavaScript framework
- **Vite** - Build tool and dev server
- **TypeScript** - Type safety
- **Tailwind CSS** - Utility-first CSS
- **Pinia** - State management
- **Vue Router** - Client-side routing
- **Axios** - HTTP client

### **Development Dependencies**
- **ESLint** - Code linting
- **Vue TSC** - TypeScript checking
- **Autoprefixer** - CSS vendor prefixes

## 🔄 CI/CD Integration

The frontend is fully integrated into the CI/CD pipeline:

- ✅ **Automated Testing** - Frontend build verification
- ✅ **Security Scanning** - Trivy security scans
- ✅ **Multi-arch Builds** - AMD64 and ARM64 support
- ✅ **Production Deployment** - Automated deployment to production
- ✅ **SSL/TLS Setup** - Automatic certificate management

## 🎉 Benefits

### **Developer Experience**
- ⚡ **Instant Feedback** - Changes appear immediately
- 🛠️ **Better Debugging** - Full browser dev tools
- 🔄 **Independent Development** - Frontend/backend can be developed separately
- 📱 **Modern Tooling** - Latest development tools and practices

### **Production Benefits**
- 🚀 **Performance** - Optimized production builds
- 🔒 **Security** - Proper CORS and security headers
- 📈 **Scalability** - Frontend can be deployed independently
- 🌐 **CDN Ready** - Static assets can be served from CDN

## 🆘 Troubleshooting

### **Common Issues**

1. **Frontend not loading**
   ```bash
   # Check if frontend is running
   curl http://localhost:3001
   
   # Check Vite dev server logs
   make frontend-dev
   ```

2. **API connection issues**
   ```bash
   # Check backend is running
   curl http://localhost:8080/healthz
   
   # Check CORS configuration
   export FRONTEND_SEPARATED=true
   ```

3. **HMR not working**
   ```bash
   # Clear Vite cache
   make frontend-clean
   
   # Restart dev server
   make frontend-dev
   ```

4. **Dependencies issues**
   ```bash
   # Reinstall dependencies
   make frontend-clean
   make frontend-install
   ```

### **Getting Help**

- Check the terminal output for error messages
- Use browser DevTools to inspect network requests
- Verify all services are running on correct ports
- Check the Makefile help: `make frontend-help`

## 🎯 Next Steps

1. **Start Development**: `make frontend-dev`
2. **Make Changes**: Edit Vue components in `admin-frontend/src/`
3. **See Changes**: Changes appear instantly with HMR
4. **Debug**: Use browser DevTools and Vue DevTools
5. **Deploy**: Use CI/CD pipeline for production deployment

Happy coding! 🚀
