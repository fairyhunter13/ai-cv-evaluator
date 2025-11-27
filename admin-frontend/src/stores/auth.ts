import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

export const useAuthStore = defineStore('auth', () => {
  const isAuthenticated = ref(false)
  const user = ref<{ username: string } | null>(null)
  const loading = ref(false)
  const token = ref<string | null>(null)

  const login = async (_username: string, _password: string) => {
    // Backwards-compatible signature; delegate to SSO flow
    return loginWithSSO()
  }

  const loginWithSSO = async (redirectTo?: string) => {
    // Trigger oauth2-proxy start endpoint; rd determines post-login redirect.
    // Default to the portal root so SSO always lands on the main page first.
    const rd = redirectTo || window.location.origin + '/'
    window.location.href = `/oauth2/start?rd=${encodeURIComponent(rd)}`
    return true
  }

  const logout = async () => {
    try {
      // Redirect to oauth2-proxy logout which invalidates SSO and returns to portal
      window.location.href = '/oauth2/sign_out?rd=/'
    } catch (error) {
      console.error('Logout error:', error)
    } finally {
      isAuthenticated.value = false
      user.value = null
      token.value = null
      delete axios.defaults.headers.common['Authorization']
    }
  }

  const checkAuth = async () => {
    try {
      // Try Bearer first if token in memory
      if (token.value) {
        axios.defaults.headers.common['Authorization'] = `Bearer ${token.value}`
      }
      // Access a protected endpoint
      const response = await axios.get('/admin/api/status', { withCredentials: true })
      
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
      token.value = null
      delete axios.defaults.headers.common['Authorization']
    }
    return false
  }

  return {
    isAuthenticated,
    user,
    loading,
    token,
    login,
    loginWithSSO,
    logout,
    checkAuth,
  }
})
