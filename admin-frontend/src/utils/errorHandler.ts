import axios from 'axios'

export interface ApiError {
  code: string
  message: string
  details?: any
}

export interface ErrorResponse {
  error: ApiError
}

/**
 * Standardizes error handling for API responses
 * Handles both the backend error format and generic errors
 */
export const handleApiError = (error: any): string => {
  // Handle specific HTTP status codes with user-friendly messages
  if (error.response?.status) {
    switch (error.response.status) {
      case 400:
        return 'Invalid request. Please check your input and try again.'
      case 401:
        return 'Authentication required. Please log in again.'
      case 403:
        return 'Access denied. You do not have permission to perform this action.'
      case 404:
        return 'The requested resource was not found.'
      case 422:
        return 'Validation error. Please check your input and try again.'
      case 429:
        return 'Too many requests. Please wait a moment and try again.'
      case 500:
        return 'Server error. Please try again later.'
      case 502:
      case 503:
      case 504:
        return 'Service temporarily unavailable. Please try again later.'
    }
  }

  // Handle API error format
  if (error.response?.data?.error) {
    const apiError = error.response.data.error
    
    // Provide user-friendly messages for common error codes
    switch (apiError.code) {
      case 'VALIDATION_ERROR':
        return 'Please check your input and try again.'
      case 'JOB_NOT_FOUND':
        return 'The requested job was not found.'
      case 'DATABASE_ERROR':
        return 'Database error. Please try again later.'
      case 'AUTHENTICATION_ERROR':
        return 'Authentication failed. Please check your credentials.'
      case 'AUTHORIZATION_ERROR':
        return 'You do not have permission to perform this action.'
      default:
        return apiError.message || 'An error occurred while processing your request.'
    }
  }
  
  if (error.response?.data?.message) {
    return error.response.data.message
  }
  
  if (error.message) {
    return error.message
  }
  
  return 'An unexpected error occurred. Please try again.'
}

/**
 * Checks if the error is an authentication error
 */
export const isAuthError = (error: any): boolean => {
  return error.response?.status === 401 || error.response?.status === 403
}

/**
 * Checks if the error is a network error
 */
export const isNetworkError = (error: any): boolean => {
  return !error.response && (error.code === 'NETWORK_ERROR' || error.message?.includes('Network Error'))
}

/**
 * Creates a standardized error handler for axios requests
 */
export const createErrorHandler = () => {
  return (error: any) => {
    if (isAuthError(error)) {
      // Handle authentication errors
      window.location.href = '/login'
      return
    }
    
    if (isNetworkError(error)) {
      return 'Network error. Please check your connection.'
    }
    
    return handleApiError(error)
  }
}

/**
 * Retry configuration for different types of errors
 */
export const getRetryConfig = (error: any) => {
  if (isNetworkError(error)) {
    return { maxRetries: 3, delay: 1000 }
  }
  
  if (error.response?.status >= 500) {
    return { maxRetries: 2, delay: 2000 }
  }
  
  return { maxRetries: 0, delay: 0 }
}

/**
 * Enhanced error handler with retry logic
 */
export const createRetryableErrorHandler = () => {
  return async (error: any, retryCount = 0) => {
    const config = getRetryConfig(error)
    
    if (retryCount < config.maxRetries) {
      await new Promise(resolve => setTimeout(resolve, config.delay * (retryCount + 1)))
      return { shouldRetry: true, retryCount: retryCount + 1 }
    }
    
    return { shouldRetry: false, error: handleApiError(error) }
  }
}

/**
 * Error severity levels
 */
export enum ErrorSeverity {
  LOW = 'low',
  MEDIUM = 'medium',
  HIGH = 'high',
  CRITICAL = 'critical'
}

/**
 * Get error severity based on error type
 */
export const getErrorSeverity = (error: any): ErrorSeverity => {
  if (isAuthError(error)) {
    return ErrorSeverity.HIGH
  }
  
  if (error.response?.status >= 500) {
    return ErrorSeverity.CRITICAL
  }
  
  if (error.response?.status >= 400) {
    return ErrorSeverity.MEDIUM
  }
  
  if (isNetworkError(error)) {
    return ErrorSeverity.HIGH
  }
  
  return ErrorSeverity.LOW
}
