<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Sidebar -->
    <div 
      class="fixed inset-y-0 left-0 z-50 w-64 bg-white shadow-lg transform transition-transform duration-300 ease-in-out"
      :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'"
    >
      <div class="flex items-center justify-center h-16 bg-gradient-to-r from-primary-600 to-primary-700">
        <h1 class="text-xl font-bold text-white">
          AI CV Evaluator
        </h1>
      </div>
      
      <nav class="mt-8">
        <div class="px-4 space-y-2">
          <router-link 
            v-for="item in navItems" 
            :key="item.to"
            :to="item.to" 
            :class="[
              'flex items-center px-4 py-3 text-sm font-medium rounded-lg transition-colors',
              isActive(item.to) 
                ? 'text-primary-600 bg-primary-50' 
                : 'text-gray-600 hover:text-primary-600 hover:bg-primary-50'
            ]"
          >
            <component
              :is="item.icon"
              class="w-5 h-5 mr-3"
            />
            {{ item.label }}
          </router-link>
        </div>
      </nav>
    </div>

    <!-- Main Content -->
    <div class="lg:ml-64">
      <!-- Top Navigation -->
      <header class="bg-white shadow-sm border-b border-gray-200">
        <div class="flex items-center justify-between px-6 py-4">
          <div class="flex items-center">
            <button 
              class="lg:hidden p-2 rounded-md text-gray-400 hover:text-gray-500 hover:bg-gray-100" 
              @click="toggleSidebar"
            >
              <svg
                class="w-6 h-6"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 6h16M4 12h16M4 18h16"
                />
              </svg>
            </button>
            <h1 class="text-2xl font-semibold text-gray-900 ml-4 lg:ml-0">
              {{ pageTitle }}
            </h1>
          </div>
          
          <div class="flex items-center space-x-4">
            <div class="flex items-center space-x-3">
              <div class="text-right">
                <p class="text-sm font-medium text-gray-900">
                  {{ authStore.user?.username }}
                </p>
                <p class="text-xs text-gray-500">
                  Administrator
                </p>
              </div>
              <div class="relative">
                <button 
                  class="flex items-center space-x-2 text-sm rounded-full focus:outline-none focus:ring-2 focus:ring-primary-500" 
                  @click="toggleUserMenu"
                >
                  <div class="w-8 h-8 bg-primary-100 rounded-full flex items-center justify-center">
                    <span class="text-primary-600 font-medium text-sm">
                      {{ authStore.user?.username?.charAt(0).toUpperCase() || 'A' }}
                    </span>
                  </div>
                </button>
                
                <div 
                  v-if="userMenuOpen" 
                  class="absolute right-0 mt-2 w-48 bg-white rounded-md shadow-lg py-1 z-50"
                >
                  <button 
                    class="flex items-center w-full px-4 py-2 text-sm text-gray-700 hover:bg-gray-100"
                    @click="handleLogout"
                  >
                    <svg
                      class="w-4 h-4 mr-3"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                      />
                    </svg>
                    Sign out
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </header>

      <!-- Page Content -->
      <main class="p-6">
        <slot />
      </main>
    </div>

    <!-- Mobile Sidebar Overlay -->
    <div 
      v-if="sidebarOpen" 
      class="fixed inset-0 z-40 bg-black bg-opacity-50 lg:hidden" 
      @click="toggleSidebar"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h, type FunctionalComponent } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Props
defineProps<{
  pageTitle: string
}>()

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const sidebarOpen = ref(false)
const userMenuOpen = ref(false)

// Icon components as functional components
const DashboardIcon: FunctionalComponent = () => h('svg', { 
  fill: 'none', 
  stroke: 'currentColor', 
  viewBox: '0 0 24 24' 
}, [
  h('path', { 
    'stroke-linecap': 'round', 
    'stroke-linejoin': 'round', 
    'stroke-width': '2', 
    d: 'M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2H5a2 2 0 00-2-2z' 
  }),
  h('path', { 
    'stroke-linecap': 'round', 
    'stroke-linejoin': 'round', 
    'stroke-width': '2', 
    d: 'M8 5a2 2 0 012-2h4a2 2 0 012 2v2H8V5z' 
  })
])

const UploadIcon: FunctionalComponent = () => h('svg', { 
  fill: 'none', 
  stroke: 'currentColor', 
  viewBox: '0 0 24 24' 
}, [
  h('path', { 
    'stroke-linecap': 'round', 
    'stroke-linejoin': 'round', 
    'stroke-width': '2', 
    d: 'M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12' 
  })
])

const EvaluateIcon: FunctionalComponent = () => h('svg', { 
  fill: 'none', 
  stroke: 'currentColor', 
  viewBox: '0 0 24 24' 
}, [
  h('path', { 
    'stroke-linecap': 'round', 
    'stroke-linejoin': 'round', 
    'stroke-width': '2', 
    d: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z' 
  })
])

const ResultIcon: FunctionalComponent = () => h('svg', { 
  fill: 'none', 
  stroke: 'currentColor', 
  viewBox: '0 0 24 24' 
}, [
  h('path', { 
    'stroke-linecap': 'round', 
    'stroke-linejoin': 'round', 
    'stroke-width': '2', 
    d: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z' 
  })
])

const JobsIcon: FunctionalComponent = () => h('svg', { 
  fill: 'none', 
  stroke: 'currentColor', 
  viewBox: '0 0 24 24' 
}, [
  h('path', { 
    'stroke-linecap': 'round', 
    'stroke-linejoin': 'round', 
    'stroke-width': '2', 
    d: 'M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2' 
  })
])

// Navigation items
const navItems = [
  { to: '/dashboard', label: 'Dashboard', icon: DashboardIcon },
  { to: '/upload', label: 'Upload Files', icon: UploadIcon },
  { to: '/evaluate', label: 'Start Evaluation', icon: EvaluateIcon },
  { to: '/result', label: 'View Results', icon: ResultIcon },
  { to: '/jobs', label: 'Job Management', icon: JobsIcon },
]

// Check if a nav item is active
const isActive = (path: string) => {
  return route.path === path
}

const toggleSidebar = () => {
  sidebarOpen.value = !sidebarOpen.value
}

const toggleUserMenu = () => {
  userMenuOpen.value = !userMenuOpen.value
}

const handleLogout = async () => {
  await authStore.logout()
  router.push('/login')
}

onMounted(() => {
  // Close user menu when clicking outside
  document.addEventListener('click', (e) => {
    if (!(e.target as Element).closest('.relative')) {
      userMenuOpen.value = false
    }
  })
})
</script>
