import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'
import { useAuthStore } from './stores/auth'
import axios from 'axios'
import { initCsrfProtection } from './utils/csrf'

// Import pages
import Dashboard from './views/Dashboard.vue'
import Upload from './views/Upload.vue'
import Evaluate from './views/Evaluate.vue'
import Result from './views/Result.vue'
import Jobs from './views/Jobs.vue'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'Dashboard', component: Dashboard },
  { path: '/upload', name: 'Upload', component: Upload },
  { path: '/evaluate', name: 'Evaluate', component: Evaluate },
  { path: '/result', name: 'Result', component: Result },
  { path: '/jobs', name: 'Jobs', component: Jobs },
]

const router = createRouter({
  history: createWebHistory('/app/'),
  routes,
})

const pinia = createPinia()
const app = createApp(App)

// Configure axios base URL and credentials from environment
axios.defaults.baseURL = ''
axios.defaults.withCredentials = true
axios.defaults.headers.common['Accept'] = 'application/json'

app.use(pinia)
app.use(router)

// After Pinia is ready, attach Authorization header if token exists (SPA reload)
try {
  const authStore = useAuthStore()
  if (authStore.token) {
    axios.defaults.headers.common['Authorization'] = `Bearer ${authStore.token}`
  }
} catch (e) {}

// Router guard: rely on SSO forward-auth at Nginx (e.g. oauth2-proxy); don't redirect SPA
router.beforeEach(async (_to, _from, next) => {
  const authStore = useAuthStore()
  await authStore.checkAuth()
  next()
})

initCsrfProtection()

app.mount('#app')
