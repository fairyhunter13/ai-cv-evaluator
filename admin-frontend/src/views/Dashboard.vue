<template>
  <AdminLayout page-title="Dashboard">
    <!-- Welcome Section -->
        <div class="mb-8">
          <div class="bg-gradient-to-r from-primary-600 to-primary-700 rounded-xl p-8 text-white">
            <div class="flex items-center justify-between">
              <div>
                <h2 class="text-3xl font-bold mb-2">
                  Welcome back, {{ authStore.user?.username }}!
                </h2>
                <p class="text-primary-100 text-lg">
                  Manage your AI CV evaluation system with ease
                </p>
              </div>
              <div class="hidden lg:block">
                <div class="w-24 h-24 bg-white bg-opacity-20 rounded-full flex items-center justify-center">
                  <svg
                    class="w-12 h-12 text-white"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Stats Cards -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
          <!-- Loading State -->
          <div
            v-if="statsLoading"
            class="col-span-full"
          >
            <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-8">
              <div class="text-center">
                <LoadingSpinner
                  size="lg"
                  text="Loading statistics..."
                />
              </div>
            </div>
          </div>
          
          <!-- Error State -->
          <div
            v-else-if="statsError"
            class="col-span-full"
          >
            <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
              <div class="rounded-md bg-red-50 p-4">
                <div class="flex">
                  <div class="flex-shrink-0">
                    <svg
                      class="h-5 w-5 text-red-400"
                      viewBox="0 0 20 20"
                      fill="currentColor"
                    >
                      <path
                        fill-rule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </div>
                  <div class="ml-3">
                    <h3 class="text-sm font-medium text-red-800">
                      Error loading statistics
                    </h3>
                    <p class="mt-1 text-sm text-red-700">
                      {{ statsError }}
                    </p>
                    <button
                      class="mt-2 text-sm text-red-600 hover:text-red-500"
                      @click="loadStats"
                    >
                      Try again
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
          
          <!-- Stats Cards -->
          <template v-else>
            <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
              <div class="flex items-center justify-between">
                <div>
                  <p class="text-sm font-medium text-gray-600">
                    Total Uploads
                  </p>
                  <p
                    class="text-3xl font-bold text-gray-900"
                    data-testid="stats-uploads"
                  >
                    {{ stats.uploads }}
                  </p>
                </div>
                <div class="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center">
                  <svg
                    class="w-6 h-6 text-blue-600"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                    />
                  </svg>
                </div>
              </div>
              <div class="mt-4 flex items-center text-sm text-gray-500">
                <svg
                  class="w-4 h-4 mr-1"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 10l7-7m0 0l7 7m-7-7v18"
                  />
                </svg>
                Live total based on all stored uploads
              </div>
            </div>

            <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
              <div class="flex items-center justify-between">
                <div>
                  <p class="text-sm font-medium text-gray-600">
                    Evaluations
                  </p>
                  <p
                    class="text-3xl font-bold text-gray-900"
                    data-testid="stats-evaluations"
                  >
                    {{ stats.evaluations }}
                  </p>
                </div>
                <div class="w-12 h-12 bg-green-100 rounded-lg flex items-center justify-center">
                  <svg
                    class="w-6 h-6 text-green-600"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </div>
              </div>
              <div class="mt-4 flex items-center text-sm text-gray-500">
                <svg
                  class="w-4 h-4 mr-1"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 10l7-7m0 0l7 7m-7-7v18"
                  />
                </svg>
                Live count of evaluation jobs in the system
              </div>
            </div>

            <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
              <div class="flex items-center justify-between">
                <div>
                  <p class="text-sm font-medium text-gray-600">
                    Completed
                  </p>
                  <p
                    class="text-3xl font-bold text-gray-900"
                    data-testid="stats-completed"
                  >
                    {{ stats.completed }}
                  </p>
                </div>
                <div class="w-12 h-12 bg-purple-100 rounded-lg flex items-center justify-center">
                  <svg
                    class="w-6 h-6 text-purple-600"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
                    />
                  </svg>
                </div>
              </div>
              <div class="mt-4 flex items-center text-sm text-gray-500">
                <svg
                  class="w-4 h-4 mr-1"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 10l7-7m0 0l7 7m-7-7v18"
                  />
                </svg>
                Number of jobs that have finished processing
              </div>
            </div>

            <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
              <div class="flex items-center justify-between">
                <div>
                  <p class="text-sm font-medium text-gray-600">
                    Avg Time
                  </p>
                  <p
                    class="text-3xl font-bold text-gray-900"
                    data-testid="stats-avg-time"
                  >
                    {{ stats.avgTime }}
                  </p>
                  <p class="text-sm text-gray-500">
                    seconds
                  </p>
                </div>
                <div class="w-12 h-12 bg-orange-100 rounded-lg flex items-center justify-center">
                  <svg
                    class="w-6 h-6 text-orange-600"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </div>
              </div>
              <div class="mt-4 flex items-center text-sm text-gray-500">
                <svg
                  class="w-4 h-4 mr-1"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 14l-7 7m0 0l-7-7m7 7V3"
                  />
                </svg>
                Average processing time for completed jobs
              </div>
            </div>
          </template>
        </div>

        <!-- Quick Actions -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
          <!-- Quick Actions Card -->
          <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
            <h3 class="text-lg font-semibold text-gray-900 mb-6">
              Quick Actions
            </h3>
            <div class="space-y-4">
              <router-link 
                to="/upload" 
                class="flex items-center p-4 bg-gradient-to-r from-blue-50 to-blue-100 rounded-lg hover:from-blue-100 hover:to-blue-200 transition-all duration-200 group"
              >
                <div class="w-10 h-10 bg-blue-500 rounded-lg flex items-center justify-center mr-4 group-hover:bg-blue-600 transition-colors">
                  <svg
                    class="w-5 h-5 text-white"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                    />
                  </svg>
                </div>
                <div class="flex-1">
                  <h4 class="font-medium text-gray-900">
                    Upload Documents
                  </h4>
                  <p class="text-sm text-gray-600">
                    Upload CV and project files
                  </p>
                </div>
                <svg
                  class="w-5 h-5 text-gray-400 group-hover:text-gray-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 5l7 7-7 7"
                  />
                </svg>
              </router-link>

              <router-link 
                to="/evaluate" 
                class="flex items-center p-4 bg-gradient-to-r from-green-50 to-green-100 rounded-lg hover:from-green-100 hover:to-green-200 transition-all duration-200 group"
              >
                <div class="w-10 h-10 bg-green-500 rounded-lg flex items-center justify-center mr-4 group-hover:bg-green-600 transition-colors">
                  <svg
                    class="w-5 h-5 text-white"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </div>
                <div class="flex-1">
                  <h4 class="font-medium text-gray-900">
                    Start Evaluation
                  </h4>
                  <p class="text-sm text-gray-600">
                    Begin AI-powered analysis
                  </p>
                </div>
                <svg
                  class="w-5 h-5 text-gray-400 group-hover:text-gray-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 5l7 7-7 7"
                  />
                </svg>
              </router-link>

              <router-link 
                to="/result" 
                class="flex items-center p-4 bg-gradient-to-r from-purple-50 to-purple-100 rounded-lg hover:from-purple-100 hover:to-purple-200 transition-all duration-200 group"
              >
                <div class="w-10 h-10 bg-purple-500 rounded-lg flex items-center justify-center mr-4 group-hover:bg-purple-600 transition-colors">
                  <svg
                    class="w-5 h-5 text-white"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
                    />
                  </svg>
                </div>
                <div class="flex-1">
                  <h4 class="font-medium text-gray-900">
                    View Results
                  </h4>
                  <p class="text-sm text-gray-600">
                    Check evaluation reports
                  </p>
                </div>
                <svg
                  class="w-5 h-5 text-gray-400 group-hover:text-gray-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 5l7 7-7 7"
                  />
                </svg>
              </router-link>
            </div>
          </div>

          <!-- System Status Card -->
          <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
            <h3 class="text-lg font-semibold text-gray-900 mb-6">
              System Status
            </h3>
            <div class="space-y-4">
              <div class="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div class="flex items-center">
                  <div class="w-3 h-3 bg-green-500 rounded-full mr-3" />
                  <div>
                    <p class="font-medium text-gray-900">
                      API Server
                    </p>
                    <p class="text-sm text-gray-600">
                      Running normally
                    </p>
                  </div>
                </div>
                <span class="text-sm text-green-600 font-medium">Online</span>
              </div>

              <div class="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div class="flex items-center">
                  <div class="w-3 h-3 bg-green-500 rounded-full mr-3" />
                  <div>
                    <p class="font-medium text-gray-900">
                      Worker Queue
                    </p>
                    <p class="text-sm text-gray-600">
                      Processing tasks
                    </p>
                  </div>
                </div>
                <span class="text-sm text-green-600 font-medium">Active</span>
              </div>

              <div class="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
                <div class="flex items-center">
                  <div class="w-3 h-3 bg-green-500 rounded-full mr-3" />
                  <div>
                    <p class="font-medium text-gray-900">
                      Database
                    </p>
                    <p class="text-sm text-gray-600">
                      Connected
                    </p>
                  </div>
                </div>
                <span class="text-sm text-green-600 font-medium">Healthy</span>
              </div>

              <div class="pt-4 border-t border-gray-200">
                <div class="flex space-x-3">
                  <a 
                    href="/metrics" 
                    target="_blank" 
                    class="flex-1 bg-blue-600 text-white text-center py-2 px-4 rounded-lg hover:bg-blue-700 transition-colors"
                  >
                    View Metrics
                  </a>
                  <a 
                    href="/grafana/" 
                    target="_blank" 
                    class="flex-1 bg-gray-600 text-white text-center py-2 px-4 rounded-lg hover:bg-gray-700 transition-colors"
                  >
                    Grafana
                  </a>
                </div>
              </div>
            </div>
          </div>
        </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import axios from 'axios'
import AdminLayout from '@/layouts/AdminLayout.vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'

const authStore = useAuthStore()

const stats = reactive({
  uploads: 0,
  evaluations: 0,
  completed: 0,
  avgTime: 0
})

const statsLoading = ref(false)
const statsError = ref('')

const loadStats = async () => {
  statsLoading.value = true
  statsError.value = ''
  
  try {
    const response = await axios.get('/admin/api/stats', {
      withCredentials: true,
    })
    
    if (response.status === 200) {
      // Check if response contains error information
      if (response.data.error) {
        console.error('API returned error:', response.data.error)
        statsError.value = 'Failed to load statistics'
        // Set default values instead of mock data
        stats.uploads = 0
        stats.evaluations = 0
        stats.completed = 0
        stats.avgTime = 0
        return
      }
      
      stats.uploads = response.data.uploads || 0
      stats.evaluations = response.data.evaluations || 0
      stats.completed = response.data.completed || 0
      stats.avgTime = response.data.avg_time || 0
    }
  } catch (error) {
    console.error('Failed to load stats:', error)
    statsError.value = 'Failed to load statistics'
    // Set default values instead of mock data
    stats.uploads = 0
    stats.evaluations = 0
    stats.completed = 0
    stats.avgTime = 0
  } finally {
    statsLoading.value = false
  }
}

onMounted(() => {
  loadStats()
})
</script>
