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

**Usage**:
```vue
<NotificationToast 
  :type="notificationType"
  :message="notificationMessage"
  @dismiss="handleDismiss"
/>
```

### 3. Dashboard.vue

**Purpose**: Main dashboard displaying system overview and statistics.

**Features**:
- System statistics display
- Recent activity feed
- Quick action buttons
- Real-time updates

**Key Sections**:
- Statistics cards (uploads, evaluations, success rate)
- Recent jobs table
- System health indicators
- Quick action buttons

**API Integration**:
```typescript
  // Fetch dashboard statistics
const fetchStats = async () => {
  try {
    const response = await api.get('/admin/api/stats');
    stats.value = response.data;
  } catch (error) {
    handleError(error);
  }
};
```

### 4. Login.vue

**Purpose**: User authentication interface.

**Features**:
- Login form with validation
- Error handling
- Loading states
- Redirect after successful login

**Form Fields**:
- Username
- Password
- Remember me checkbox

**Validation Rules**:
- Username: Required, minimum 3 characters
- Password: Required, minimum 6 characters

**Usage**:
```vue
<Login 
  @login="handleLogin"
  @error="handleError"
/>
```

### 5. Upload.vue

**Purpose**: File upload interface for CV and project documents.

**Features**:
- Drag and drop file upload
- File type validation
- Progress indicators
- Multiple file support

**File Validation**:
- Allowed types: PDF, DOCX, TXT
- Maximum file size: 10MB
- File name sanitization

**Upload Process**:
1. File selection/upload
2. Client-side validation
3. Server upload
4. Response handling
5. Success/error feedback

**Usage**:
```vue
<Upload 
  @upload-success="handleUploadSuccess"
  @upload-error="handleUploadError"
/>
```

### 6. Evaluate.vue

**Purpose**: Job evaluation interface.

**Features**:
- CV and project selection
- Job description input
- Study case brief input
- Evaluation submission

**Form Fields**:
- CV ID (from upload)
- Project ID (from upload)
- Job description (optional)
- Study case brief (optional)

**Validation**:
- CV ID and Project ID are required
- Job description and study case brief are optional
- Maximum length validation for text fields

**Usage**:
```vue
<Evaluate 
  :cv-id="selectedCvId"
  :project-id="selectedProjectId"
  @evaluate="handleEvaluate"
  @error="handleError"
/>
```

### 7. Result.vue

**Purpose**: Display evaluation results.

**Features**:
- Result visualization
- Score breakdown
- Feedback display
- Export functionality

**Result Display**:
- CV match rate (0-100%)
- Project score (1-10)
- Detailed feedback
- Overall summary

**Visualization**:
- Progress bars for scores
- Color-coded results
- Responsive design

**Usage**:
```vue
<Result 
  :result="evaluationResult"
  :loading="isLoading"
  @export="handleExport"
/>
```

### 8. Jobs.vue

**Purpose**: Job management interface.

**Features**:
- Job list with pagination
- Status filtering
- Search functionality
- Job details view

**Job Statuses**:
- Queued
- Processing
- Completed
- Failed

**Features**:
- Pagination controls
- Status filters
- Search by job ID
- Sort by date/status

**Usage**:
```vue
<Jobs 
  :jobs="jobList"
  :loading="isLoading"
  @refresh="handleRefresh"
  @filter="handleFilter"
/>
```

## State Management

### 1. Auth Store (auth.ts)

**Purpose**: Manage authentication state and user session.

**State**:
```typescript
interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
  token: string | null;
  loading: boolean;
}
```

**Actions**:
- `login(credentials)`: Authenticate user
- `logout()`: Clear session
- `checkAuth()`: Verify current session
- `refreshToken()`: Refresh authentication token

**Usage**:
```typescript
import { useAuthStore } from '@/stores/auth';

const authStore = useAuthStore();
await authStore.login({ username, password });
```

### 2. API Store

**Purpose**: Manage API communication and data fetching.

**Features**:
- Centralized API calls
- Error handling
- Loading states
- Response caching

**Methods**:
- `get(url, params)`: GET requests
- `post(url, data)`: POST requests
- `put(url, data)`: PUT requests
- `delete(url)`: DELETE requests

**Usage**:
```typescript
import { useApiStore } from '@/stores/api';

const apiStore = useApiStore();
const data = await apiStore.get('/admin/api/stats');
```

## Utility Functions

### 1. API Utilities (api.ts)

**Purpose**: Centralized API communication.

**Features**:
- Base URL configuration
- Request/response interceptors
- Error handling
- CSRF token management

**Configuration**:
```typescript
const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});
```

**Usage**:
```typescript
import { api } from '@/utils/api';

const response = await api.get('/v1/result/123');
```

### 2. Error Handler (errorHandler.ts)

**Purpose**: Centralized error handling and user feedback.

**Features**:
- Error classification
- User-friendly messages
- Logging
- Notification display

**Error Types**:
- Network errors
- Validation errors
- Authentication errors
- Server errors

**Usage**:
```typescript
import { handleError } from '@/utils/errorHandler';

try {
  await api.post('/v1/evaluate', data);
} catch (error) {
  handleError(error);
}
```

### 3. CSRF Utilities (csrf.ts)

**Purpose**: CSRF token management for secure requests.

**Features**:
- Token fetching
- Token validation
- Automatic token inclusion
- Token refresh

**Usage**:
```typescript
import { getCsrfToken } from '@/utils/csrf';

const token = await getCsrfToken();
```

## Styling and Theming

### 1. Tailwind CSS Configuration

**Configuration**: `tailwind.config.js`

**Custom Colors**:
```javascript
module.exports = {
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eff6ff',
          500: '#3b82f6',
          900: '#1e3a8a',
        },
        success: {
          50: '#f0fdf4',
          500: '#22c55e',
          900: '#14532d',
        },
        error: {
          50: '#fef2f2',
          500: '#ef4444',
          900: '#7f1d1d',
        },
      },
    },
  },
};
```

### 2. Component Styling

**Base Styles**:
```css
/* Base component styles */
.component-base {
  @apply bg-white rounded-lg shadow-sm border border-gray-200;
}

.component-header {
  @apply px-6 py-4 border-b border-gray-200;
}

.component-content {
  @apply px-6 py-4;
}

.component-footer {
  @apply px-6 py-4 border-t border-gray-200;
}
```

**Button Styles**:
```css
.btn-primary {
  @apply bg-primary-500 text-white px-4 py-2 rounded-md hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500;
}

.btn-secondary {
  @apply bg-gray-500 text-white px-4 py-2 rounded-md hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-500;
}

.btn-danger {
  @apply bg-error-500 text-white px-4 py-2 rounded-md hover:bg-error-600 focus:outline-none focus:ring-2 focus:ring-error-500;
}
```

## Component Development

### 1. Component Structure

**Template**:
```vue
<template>
  <div class="component-base">
    <header class="component-header">
      <h2 class="text-lg font-semibold">{{ title }}</h2>
    </header>
    <main class="component-content">
      <slot />
    </main>
    <footer class="component-footer" v-if="$slots.footer">
      <slot name="footer" />
    </footer>
  </div>
</template>
```

**Script**:
```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';

// Props
interface Props {
  title: string;
  loading?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
});

// Emits
const emit = defineEmits<{
  submit: [data: any];
  cancel: [];
}>();

// State
const data = ref({});

// Computed
const isValid = computed(() => {
  return Object.keys(data.value).length > 0;
});

// Methods
const handleSubmit = () => {
  emit('submit', data.value);
};

// Lifecycle
onMounted(() => {
  // Component initialization
});
</script>
```

**Styles**:
```vue
<style scoped>
.component-base {
  @apply bg-white rounded-lg shadow-sm border border-gray-200;
}

.component-header {
  @apply px-6 py-4 border-b border-gray-200;
}

.component-content {
  @apply px-6 py-4;
}
</style>
```

### 2. Component Testing

**Unit Tests**:
```typescript
import { mount } from '@vue/test-utils';
import { describe, it, expect } from 'vitest';
import Component from './Component.vue';

describe('Component', () => {
  it('renders correctly', () => {
    const wrapper = mount(Component, {
      props: {
        title: 'Test Title',
      },
    });
    
    expect(wrapper.text()).toContain('Test Title');
  });

  it('emits submit event', async () => {
    const wrapper = mount(Component);
    
    await wrapper.find('button').trigger('click');
    
    expect(wrapper.emitted('submit')).toBeTruthy();
  });
});
```

**Integration Tests**:
```typescript
import { mount } from '@vue/test-utils';
import { createPinia } from 'pinia';
import Component from './Component.vue';

describe('Component Integration', () => {
  it('works with store', () => {
    const pinia = createPinia();
    const wrapper = mount(Component, {
      global: {
        plugins: [pinia],
      },
    });
    
    // Test component with store
  });
});
```

### 3. Component Documentation

**Component Documentation Template**:
```markdown
## ComponentName

**Purpose**: Brief description of component purpose

**Features**:
- Feature 1
- Feature 2
- Feature 3

**Props**:
| Prop | Type | Default | Description |
|------|------|---------|-------------|
| prop1 | string | '' | Description |
| prop2 | number | 0 | Description |

**Events**:
| Event | Payload | Description |
|-------|---------|-------------|
| event1 | data | Description |
| event2 | - | Description |

**Slots**:
| Slot | Description |
|------|-------------|
| default | Main content |
| header | Header content |
| footer | Footer content |

**Usage**:
```vue
<ComponentName 
  :prop1="value1"
  :prop2="value2"
  @event1="handleEvent1"
  @event2="handleEvent2"
>
  <template #header>
    Header content
  </template>
  
  Main content
  
  <template #footer>
    Footer content
  </template>
</ComponentName>
```

**Examples**:
```vue
<!-- Basic usage -->
<ComponentName :prop1="value" />

<!-- With events -->
<ComponentName 
  :prop1="value"
  @event1="handleEvent"
/>

<!-- With slots -->
<ComponentName>
  <template #header>Header</template>
  Content
  <template #footer>Footer</template>
</ComponentName>
```
```

## Performance Optimization

### 1. Component Optimization

**Lazy Loading**:
```vue
<script setup lang="ts">
import { defineAsyncComponent } from 'vue';

const AsyncComponent = defineAsyncComponent(() => 
  import('./HeavyComponent.vue')
);
</script>
```

**Memoization**:
```vue
<script setup lang="ts">
import { computed, ref } from 'vue';

const data = ref([]);

const expensiveComputation = computed(() => {
  return data.value.map(item => {
    // Expensive computation
    return processItem(item);
  });
});
</script>
```

### 2. Bundle Optimization

**Code Splitting**:
```typescript
// Route-based code splitting
const routes = [
  {
    path: '/dashboard',
    component: () => import('@/views/Dashboard.vue'),
  },
  {
    path: '/upload',
    component: () => import('@/views/Upload.vue'),
  },
];
```

**Tree Shaking**:
```typescript
// Import only what you need
import { ref, computed } from 'vue';
import { debounce } from 'lodash-es';
```

### 3. Performance Monitoring

**Performance Metrics**:
```typescript
// Component performance monitoring
import { onMounted, onUnmounted } from 'vue';

const startTime = performance.now();

onMounted(() => {
  const endTime = performance.now();
  console.log(`Component mounted in ${endTime - startTime}ms`);
});
```

## Accessibility

### 1. ARIA Attributes

**Semantic HTML**:
```vue
<template>
  <div role="main" aria-label="Main content">
    <h1 id="page-title">Page Title</h1>
    <nav aria-label="Main navigation">
      <ul role="list">
        <li><a href="/dashboard" aria-current="page">Dashboard</a></li>
        <li><a href="/upload">Upload</a></li>
      </ul>
    </nav>
  </div>
</template>
```

**Form Accessibility**:
```vue
<template>
  <form @submit="handleSubmit">
    <label for="username">Username</label>
    <input 
      id="username"
      type="text"
      v-model="username"
      aria-describedby="username-error"
      :aria-invalid="hasError"
    />
    <div id="username-error" v-if="hasError" role="alert">
      {{ errorMessage }}
    </div>
  </form>
</template>
```

### 2. Keyboard Navigation

**Keyboard Support**:
```vue
<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue';

const handleKeydown = (event: KeyboardEvent) => {
  switch (event.key) {
    case 'Enter':
      handleSubmit();
      break;
    case 'Escape':
      handleCancel();
      break;
  }
};

onMounted(() => {
  document.addEventListener('keydown', handleKeydown);
});

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown);
});
</script>
```

## Development Workflow

### 1. Component Development

**Development Steps**:
1. Create component file
2. Define component structure
3. Implement functionality
4. Add styling
5. Write tests
6. Document component
7. Review and refactor

### 2. Code Quality

**Linting**:
```bash
# ESLint
npm run lint

# Prettier
npm run format

# Type checking
npm run type-check
```

**Testing**:
```bash
# Unit tests
npm run test

# E2E tests
npm run test:e2e

# Coverage
npm run test:coverage
```

### 3. Deployment

**Build Process**:
```bash
# Development build
npm run dev

# Production build
npm run build

# Preview production build
npm run preview
```

**Docker Build**:
```dockerfile
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

---

*This frontend components documentation ensures consistent development practices and comprehensive component coverage for the AI CV Evaluator project.*