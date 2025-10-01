import axios from 'axios'

// CSRF token management
let csrfToken: string | null = null

export const getCsrfToken = (): string | null => {
  return csrfToken
}

export const setCsrfToken = (token: string): void => {
  csrfToken = token
}

export const clearCsrfToken = (): void => {
  csrfToken = null
}

// Extract CSRF token from meta tag or cookie
export const extractCsrfToken = (): string | null => {
  // Try to get from meta tag first
  const metaTag = document.querySelector('meta[name="csrf-token"]')
  if (metaTag) {
    const token = metaTag.getAttribute('content')
    if (token) {
      setCsrfToken(token)
      return token
    }
  }

  // Try to get from cookie
  const cookies = document.cookie.split(';')
  for (const cookie of cookies) {
    const [name, value] = cookie.trim().split('=')
    if (name === 'csrf-token' || name === '_token') {
      setCsrfToken(value)
      return value
    }
  }

  return null
}

// Add CSRF token to axios requests
export const setupCsrfInterceptor = (): void => {
  // Request interceptor to add CSRF token
  axios.interceptors.request.use(
    (config) => {
      const token = getCsrfToken()
      if (token) {
        // Add CSRF token to headers
        config.headers['X-CSRF-Token'] = token
        config.headers['X-Requested-With'] = 'XMLHttpRequest'
      }
      return config
    },
    (error) => {
      return Promise.reject(error)
    }
  )

  // Response interceptor to handle CSRF token refresh
  axios.interceptors.response.use(
    (response) => {
      // Check if server sent a new CSRF token
      const newToken = response.headers['x-csrf-token']
      if (newToken) {
        setCsrfToken(newToken)
      }
      return response
    },
    (error) => {
      // Handle CSRF token mismatch
      if (error.response?.status === 419 || error.response?.status === 403) {
        // Clear the token and try to get a new one
        clearCsrfToken()
        extractCsrfToken()
      }
      return Promise.reject(error)
    }
  )
}

// Initialize CSRF protection
export const initCsrfProtection = (): void => {
  extractCsrfToken()
  setupCsrfInterceptor()
}
