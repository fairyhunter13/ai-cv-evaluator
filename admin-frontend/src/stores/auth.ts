import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'
import { handleApiError, isAuthError } from '@/utils/errorHandler'

export const useAuthStore = defineStore('auth', () => {
  const isAuthenticated = ref(false)
  const user = ref<{ username: string } | null>(null)
  const loading = ref(false)

  const login = async (username: string, password: string) => {
    loading.value = true
    
    try {
      const formData = new URLSearchParams()
      formData.append('username', username)
      formData.append('password', password)
      
      const response = await axios.post('/admin/login', formData, {
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        withCredentials: true,
      })
      
      // Check if we got redirected to dashboard (successful login)
      if (response.status === 200 || response.status === 302) {
        isAuthenticated.value = true
        // Use the provided username or extract from response if available
        user.value = { username }
        return true
      }
      
      throw new Error('Login failed')
    } catch (error: any) {
      if (isAuthError(error)) {
        throw new Error('Invalid credentials')
      }
      throw new Error(handleApiError(error))
    } finally {
      loading.value = false
    }
  }

  const logout = async () => {
    try {
      await axios.post('/admin/logout', {}, {
        withCredentials: true,
      })
    } catch (error) {
      console.error('Logout error:', error)
    } finally {
      isAuthenticated.value = false
      user.value = null
    }
  }

  const checkAuth = async () => {
    try {
      // Try to access a protected endpoint to check if we're authenticated
      const response = await axios.get('/admin/api/status', {
        withCredentials: true,
      })
      
      if (response.status === 200) {
        isAuthenticated.value = true
        // Extract user info from the API response
        if (response.data && response.data.username) {
          user.value = { username: response.data.username }
        } else {
          // Fallback to admin if username not provided
          user.value = { username: 'admin' }
        }
        return true
      }
    } catch (error) {
      isAuthenticated.value = false
      user.value = null
    }
    return false
  }

  return {
    isAuthenticated,
    user,
    loading,
    login,
    logout,
    checkAuth,
  }
})
