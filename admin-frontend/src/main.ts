import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'
import { initCsrfProtection } from './utils/csrf'

// Import pages
import Dashboard from './views/Dashboard.vue'
import Upload from './views/Upload.vue'
import Evaluate from './views/Evaluate.vue'
import Result from './views/Result.vue'
import Jobs from './views/Jobs.vue'
import Login from './views/Login.vue'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/login', name: 'Login', component: Login },
  { path: '/dashboard', name: 'Dashboard', component: Dashboard },
  { path: '/upload', name: 'Upload', component: Upload },
  { path: '/evaluate', name: 'Evaluate', component: Evaluate },
  { path: '/result', name: 'Result', component: Result },
  { path: '/jobs', name: 'Jobs', component: Jobs },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

const pinia = createPinia()
const app = createApp(App)

// Initialize CSRF protection
initCsrfProtection()

app.use(pinia)
app.use(router)
app.mount('#app')
