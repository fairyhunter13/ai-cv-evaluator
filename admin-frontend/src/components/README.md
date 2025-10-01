# Loading Components

This directory contains reusable loading components for the admin-frontend application.

## Components

### LoadingSpinner
A reusable spinner component with different sizes and optional text.

**Props:**
- `size`: 'sm' | 'md' | 'lg' | 'xl' (default: 'md')
- `text`: string (optional)
- `containerClass`: string (optional)

**Usage:**
```vue
<LoadingSpinner size="lg" text="Loading data..." />
```

### LoadingButton
A button component with built-in loading state.

**Props:**
- `loading`: boolean (default: false)
- `disabled`: boolean (default: false)
- `text`: string (optional)
- `loadingText`: string (optional)
- `variant`: 'primary' | 'secondary' | 'danger' | 'success' (default: 'primary')
- `size`: 'sm' | 'md' | 'lg' (default: 'md')
- `fullWidth`: boolean (default: false)

**Events:**
- `click`: Emitted when button is clicked

**Usage:**
```vue
<LoadingButton
  :loading="isLoading"
  text="Submit"
  loading-text="Submitting..."
  variant="primary"
  size="lg"
  @click="handleSubmit"
/>
```

### LoadingCard
A card component with loading spinner for content areas.

**Props:**
- `text`: string (default: 'Loading...')

**Usage:**
```vue
<LoadingCard text="Loading content..." />
```

### LoadingTable
A table component with loading state for data tables.

**Props:**
- `title`: string (default: 'Loading')
- `subtitle`: string (default: 'Please wait...')
- `text`: string (default: 'Loading data...')

**Usage:**
```vue
<LoadingTable
  title="Jobs"
  subtitle="Manage and monitor evaluation jobs"
  text="Loading jobs..."
/>
```

### LoadingOverlay
A full-screen overlay with loading spinner.

**Props:**
- `show`: boolean (required)
- `text`: string (default: 'Loading...')
- `size`: 'sm' | 'md' | 'lg' | 'xl' (default: 'lg')

**Usage:**
```vue
<LoadingOverlay :show="isLoading" text="Processing..." />
```

## Implementation Status

All async operations in the admin-frontend now have comprehensive loading states:

### ✅ Dashboard
- Statistics loading with error handling
- Retry functionality for failed requests

### ✅ Upload
- File upload progress indication
- Button loading state during upload

### ✅ Evaluate
- Evaluation process loading state
- Button disabled during processing

### ✅ Jobs
- Job list loading with table skeleton
- Job details modal loading
- Refresh button loading state
- Pagination loading states

### ✅ Results
- Result fetching loading state
- Empty state with loading card

### ✅ Login
- Authentication loading state
- Button loading during login process

## Best Practices

1. **Always show loading states** for async operations
2. **Provide meaningful text** in loading states
3. **Handle error states** with retry options
4. **Use appropriate sizes** for different contexts
5. **Disable interactive elements** during loading
6. **Provide visual feedback** for user actions

## Accessibility

All loading components include:
- Proper ARIA labels
- Screen reader friendly text
- Keyboard navigation support
- High contrast support
