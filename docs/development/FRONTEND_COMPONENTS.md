# Frontend Components Documentation

This document provides comprehensive documentation for the Vue.js frontend components in the AI CV Evaluator project.

## Overview

The frontend is built with Vue 3, TypeScript, Tailwind CSS, and Vite, following modern development practices with Hot Module Replacement (HMR) for development.

## Project Structure

```
admin-frontend/
├── src/
│   ├── components/          # Reusable components
│   │   └── NotificationToast.vue
│   ├── views/              # Page components
│   │   ├── Dashboard.vue
│   │   ├── Login.vue
│   │   ├── Upload.vue
│   │   ├── Evaluate.vue
│   │   ├── Result.vue
│   │   └── Jobs.vue
│   ├── stores/             # Pinia state management
│   │   └── auth.ts
│   ├── utils/              # Utility functions
│   │   ├── csrf.ts
│   │   ├── errorHandler.ts
│   │   └── api.ts
│   ├── App.vue             # Root component
│   └── main.ts             # Application entry point
├── public/                 # Static assets
├── package.json           # Dependencies and scripts
├── vite.config.ts         # Vite configuration
├── tailwind.config.js     # Tailwind CSS configuration
└── nginx.conf             # Production Nginx configuration
```

## Core Components

### 1. App.vue (Root Component)

**Purpose**: Main application wrapper with router view and global components.

**Features**:
- Router view container
- Global notification toast
- Application-level styling

**Key Elements**:
```vue
<template>
  <div id="app">
    <router-view />
    <NotificationToast />
  </div>
</template>
```

### 2. NotificationToast.vue

**Purpose**: Global notification system for user feedback.

**Features**:
- Toast notifications
- Auto-dismiss functionality
- Multiple notification types
- Queue management

**Props**:
- `type`: 'success' | 'error' | 'warning' | 'info'
- `message`: string
- `duration`: number (milliseconds)

**Usage**:
```vue
<NotificationToast 
  :type="'success'" 
  :message="'Operation completed'" 
  :duration="3000" 
/>
```

## Page Components

### 1. Dashboard.vue

**Purpose**: Main dashboard with statistics and navigation.

**Features**:
- Statistics overview (uploads, evaluations, completed jobs)
- Navigation sidebar
- User menu
- Quick actions
- Real-time updates

**State Management**:
- Uses `useAuthStore` for authentication
- Fetches dashboard statistics via API
- Manages sidebar and user menu state

**Key Methods**:
```typescript
const loadStats = async () => {
  // Fetch dashboard statistics
}

const toggleSidebar = () => {
  // Toggle mobile sidebar
}

const handleLogout = async () => {
  // Handle user logout
}
```

**API Endpoints**:
- `GET /admin/api/stats` - Dashboard statistics

### 2. Login.vue

**Purpose**: Authentication page for admin access.

**Features**:
- Username/password form
- Form validation
- Error handling
- Redirect after login

**Form Fields**:
- `username`: string (required)
- `password`: string (required)

**Validation Rules**:
- Username: minimum 3 characters
- Password: minimum 6 characters

**State Management**:
- Uses `useAuthStore` for authentication state
- Manages form state and validation
- Handles login errors

### 3. Upload.vue

**Purpose**: File upload interface for CV and project documents.

**Features**:
- Drag-and-drop file upload
- File type validation
- Progress indicators
- Multiple file support
- File preview

**Supported File Types**:
- `.txt` - Plain text files
- `.pdf` - PDF documents
- `.docx` - Word documents

**File Validation**:
- Maximum file size: 10MB (configurable)
- MIME type validation
- Content type checking

**Key Methods**:
```typescript
const handleFileUpload = (files: FileList) => {
  // Process uploaded files
}

const validateFile = (file: File) => {
  // Validate file type and size
}

const uploadFiles = async () => {
  // Upload files to server
}
```

**API Endpoints**:
- `POST /v1/upload` - Upload CV and project files

### 4. Evaluate.vue

**Purpose**: Job evaluation interface with form inputs.

**Features**:
- CV and project selection
- Job description input
- Study case brief input
- Form validation
- Job submission

**Form Fields**:
- `cv_id`: string (required)
- `project_id`: string (required)
- `job_description`: string (optional)
- `study_case_brief`: string (optional)

**Validation Rules**:
- CV and project IDs are required
- Job description: maximum 2000 characters
- Study case brief: maximum 2000 characters

**Key Methods**:
```typescript
const loadUploads = async () => {
  // Load available uploads
}

const submitEvaluation = async () => {
  // Submit evaluation job
}

const validateForm = () => {
  // Validate form inputs
}
```

**API Endpoints**:
- `GET /admin/api/uploads` - Get available uploads
- `POST /v1/evaluate` - Submit evaluation job

### 5. Result.vue

**Purpose**: Display evaluation results and job status.

**Features**:
- Job status display
- Result visualization
- Progress tracking
- Error handling
- Result export

**Job Statuses**:
- `queued` - Job is queued for processing
- `processing` - Job is being processed
- `completed` - Job completed successfully
- `failed` - Job failed with error

**Result Display**:
- CV match rate (0-1 scale)
- Project score (1-10 scale)
- Feedback text
- Overall summary

**Key Methods**:
```typescript
const loadResult = async () => {
  // Load job result
}

const pollStatus = async () => {
  // Poll job status
}

const exportResult = () => {
  // Export result data
}
```

**API Endpoints**:
- `GET /v1/result/{id}` - Get job result

### 6. Jobs.vue

**Purpose**: Job management interface with listing and filtering.

**Features**:
- Job listing with pagination
- Status filtering
- Search functionality
- Job details view
- Bulk operations

**Filtering Options**:
- Status: queued, processing, completed, failed
- Date range
- Search by ID or description

**Pagination**:
- Page size: 10, 25, 50, 100
- Page navigation
- Total count display

**Key Methods**:
```typescript
const loadJobs = async () => {
  // Load job list
}

const applyFilters = () => {
  // Apply search and filter criteria
}

const viewJobDetails = (jobId: string) => {
  // View job details
}
```

**API Endpoints**:
- `GET /admin/api/jobs` - Get job list
- `GET /admin/api/jobs/{id}` - Get job details

## State Management

### Auth Store (`stores/auth.ts`)

**Purpose**: Authentication state management using Pinia.

**State**:
```typescript
interface AuthState {
  isAuthenticated: boolean
  user: User | null
  loading: boolean
}
```

**Actions**:
```typescript
const login = async (username: string, password: string) => {
  // Authenticate user
}

const logout = async () => {
  // Logout user
}

const checkAuth = async () => {
  // Check authentication status
}
```

**Getters**:
```typescript
const isLoggedIn = computed(() => isAuthenticated.value)
const currentUser = computed(() => user.value)
```

## Utility Functions

### 1. CSRF Protection (`utils/csrf.ts`)

**Purpose**: CSRF token management for secure requests.

**Functions**:
```typescript
export const initCsrfProtection = () => {
  // Initialize CSRF protection
}

export const getCsrfToken = () => {
  // Get CSRF token
}
```

### 2. Error Handling (`utils/errorHandler.ts`)

**Purpose**: Centralized error handling and user feedback.

**Functions**:
```typescript
export const handleApiError = (error: any) => {
  // Handle API errors
}

export const isAuthError = (error: any) => {
  // Check if error is authentication related
}
```

### 3. API Client (`utils/api.ts`)

**Purpose**: HTTP client with authentication and error handling.

**Features**:
- Automatic authentication headers
- Request/response interceptors
- Error handling
- Retry logic

**Methods**:
```typescript
export const apiClient = {
  get: (url: string) => Promise<Response>,
  post: (url: string, data: any) => Promise<Response>,
  put: (url: string, data: any) => Promise<Response>,
  delete: (url: string) => Promise<Response>
}
```

## Styling and Theming

### Tailwind CSS Configuration

**Custom Colors**:
```javascript
// tailwind.config.js
module.exports = {
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eff6ff',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8'
        }
      }
    }
  }
}
```

**Component Classes**:
- `.btn-primary` - Primary button styling
- `.card` - Card component styling
- `.form-input` - Form input styling
- `.sidebar` - Sidebar navigation styling

### Responsive Design

**Breakpoints**:
- `sm`: 640px
- `md`: 768px
- `lg`: 1024px
- `xl`: 1280px

**Mobile-First Approach**:
- Base styles for mobile
- Progressive enhancement for larger screens
- Touch-friendly interface elements

## Development Workflow

### Hot Module Replacement (HMR)

**Configuration**:
```typescript
// vite.config.ts
export default defineConfig({
  server: {
    hmr: {
      port: 3001
    }
  }
})
```

**Benefits**:
- Instant updates during development
- State preservation across changes
- Fast rebuild times

### TypeScript Integration

**Type Safety**:
- Component props typing
- API response typing
- State management typing
- Event handling typing

**Example**:
```typescript
interface UploadProps {
  files: File[]
  onUpload: (files: File[]) => void
  maxSize: number
}

const UploadComponent: DefineComponent<UploadProps> = {
  props: {
    files: { type: Array as PropType<File[]>, required: true },
    onUpload: { type: Function as PropType<(files: File[]) => void>, required: true },
    maxSize: { type: Number, default: 10 }
  }
}
```

## Testing

### Component Testing

**Test Structure**:
```typescript
import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'

describe('UploadComponent', () => {
  it('should handle file upload', async () => {
    const wrapper = mount(UploadComponent, {
      props: {
        files: [],
        onUpload: vi.fn(),
        maxSize: 10
      }
    })
    
    // Test file upload functionality
  })
})
```

**Testing Utilities**:
- Vue Test Utils for component testing
- Vitest for test runner
- Mock API responses
- Test user interactions

## Performance Optimization

### Code Splitting

**Route-based Splitting**:
```typescript
const routes = [
  {
    path: '/dashboard',
    component: () => import('./views/Dashboard.vue')
  }
]
```

**Component Lazy Loading**:
```typescript
const LazyComponent = defineAsyncComponent(() => import('./LazyComponent.vue'))
```

### Bundle Optimization

**Vite Configuration**:
```typescript
export default defineConfig({
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['vue', 'vue-router', 'pinia'],
          ui: ['@headlessui/vue']
        }
      }
    }
  }
})
```

## Deployment

### Production Build

**Build Process**:
```bash
npm run build
```

**Output**:
- Optimized JavaScript bundles
- Minified CSS
- Static assets
- Source maps (optional)

### Nginx Configuration

**Production Setup**:
```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        root /var/www/admin-frontend/dist;
        try_files $uri $uri/ /index.html;
    }
    
    location /api {
        proxy_pass http://backend:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Best Practices

### 1. Component Design
- Single responsibility principle
- Reusable and composable
- Clear prop interfaces
- Proper event handling

### 2. State Management
- Use Pinia for global state
- Keep component state local when possible
- Avoid prop drilling
- Use computed properties for derived state

### 3. Performance
- Lazy load components
- Use virtual scrolling for large lists
- Optimize images and assets
- Implement proper caching

### 4. Accessibility
- Semantic HTML
- ARIA attributes
- Keyboard navigation
- Screen reader support

### 5. Error Handling
- Graceful error boundaries
- User-friendly error messages
- Retry mechanisms
- Fallback UI components

## Troubleshooting

### Common Issues

1. **HMR Not Working**
   ```bash
   # Check Vite configuration
   # Ensure port 3001 is available
   # Restart development server
   ```

2. **Build Failures**
   ```bash
   # Check TypeScript errors
   npm run type-check
   
   # Check for missing dependencies
   npm install
   ```

3. **API Connection Issues**
   ```bash
   # Check CORS configuration
   # Verify API base URL
   # Check network connectivity
   ```

### Debug Tools

**Vue DevTools**:
- Component inspection
- State debugging
- Performance profiling
- Timeline analysis

**Browser DevTools**:
- Network tab for API calls
- Console for error messages
- Application tab for storage
- Performance tab for optimization

---

*This documentation should be updated when new components are added or existing ones are modified.*
