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
            to="/dashboard" 
            class="flex items-center px-4 py-3 text-sm font-medium text-gray-600 hover:text-primary-600 hover:bg-primary-50 rounded-lg transition-colors"
          >
            <svg
              class="w-5 h-5 mr-3"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2H5a2 2 0 00-2-2z"
              />
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M8 5a2 2 0 012-2h4a2 2 0 012 2v2H8V5z"
              />
            </svg>
            Dashboard
          </router-link>
          <router-link 
            to="/upload" 
            class="flex items-center px-4 py-3 text-sm font-medium text-gray-600 hover:text-primary-600 hover:bg-primary-50 rounded-lg transition-colors"
          >
            <svg
              class="w-5 h-5 mr-3"
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
            Upload Files
          </router-link>
          <router-link 
            to="/evaluate" 
            class="flex items-center px-4 py-3 text-sm font-medium text-primary-600 bg-primary-50 rounded-lg"
          >
            <svg
              class="w-5 h-5 mr-3"
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
            Start Evaluation
          </router-link>
          <router-link 
            to="/result" 
            class="flex items-center px-4 py-3 text-sm font-medium text-gray-600 hover:text-primary-600 hover:bg-primary-50 rounded-lg transition-colors"
          >
            <svg
              class="w-5 h-5 mr-3"
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
            View Results
          </router-link>
          <router-link 
            to="/jobs" 
            class="flex items-center px-4 py-3 text-sm font-medium text-gray-600 hover:text-primary-600 hover:bg-primary-50 rounded-lg transition-colors"
          >
            <svg
              class="w-5 h-5 mr-3"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 5H7a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
              />
            </svg>
            Job Management
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
              Start Evaluation
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

      <!-- Evaluate Content -->
      <main class="p-6">
        <div class="max-w-4xl mx-auto">
          <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-8">
            <div class="text-center mb-8">
              <h2 class="text-2xl font-bold text-gray-900 mb-2">
                Start AI Evaluation
              </h2>
              <p class="text-gray-600">
                Configure and start the AI-powered evaluation process
              </p>
            </div>

            <form
              class="space-y-6"
              @submit.prevent="handleEvaluate"
            >
              <!-- CV ID -->
              <div>
                <label
                  for="cv_id"
                  class="block text-sm font-medium text-gray-700 mb-2"
                >
                  CV ID
                </label>
                <input
                  id="cv_id"
                  v-model="form.cv_id"
                  type="text"
                  required
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                  placeholder="Enter CV ID from upload"
                >
                <p class="mt-1 text-sm text-gray-500">
                  The ID returned from the upload process
                </p>
              </div>

              <!-- Project ID -->
              <div>
                <label
                  for="project_id"
                  class="block text-sm font-medium text-gray-700 mb-2"
                >
                  Project ID
                </label>
                <input
                  id="project_id"
                  v-model="form.project_id"
                  type="text"
                  required
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                  placeholder="Enter Project ID from upload"
                >
                <p class="mt-1 text-sm text-gray-500">
                  The ID returned from the upload process
                </p>
              </div>

              <!-- Job Description -->
              <div>
                <label
                  for="job_description"
                  class="block text-sm font-medium text-gray-700 mb-2"
                >
                  Job Description <span class="text-gray-400">(Optional)</span>
                </label>
                <textarea
                  id="job_description"
                  v-model="form.job_description"
                  rows="4"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                  placeholder="Enter the job description for evaluation (leave empty to use default)..."
                />
                <p class="mt-1 text-sm text-gray-500">
                  Describe the role and requirements (optional - will use default if empty)
                </p>
              </div>

              <!-- Study Case Brief -->
              <div>
                <label
                  for="study_case_brief"
                  class="block text-sm font-medium text-gray-700 mb-2"
                >
                  Study Case Brief <span class="text-gray-400">(Optional)</span>
                </label>
                <textarea
                  id="study_case_brief"
                  v-model="form.study_case_brief"
                  rows="4"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                  placeholder="Enter the study case brief for evaluation (leave empty to use default)..."
                />
                <p class="mt-1 text-sm text-gray-500">
                  Provide context for the evaluation (optional - will use default if empty)
                </p>
              </div>

              <!-- Scoring Rubric -->
              <div>
                <label
                  for="scoring_rubric"
                  class="block text-sm font-medium text-gray-700 mb-2"
                >
                  Scoring Rubric <span class="text-gray-400">(Optional)</span>
                </label>
                <textarea
                  id="scoring_rubric"
                  v-model="form.scoring_rubric"
                  rows="4"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                  placeholder="Provide a custom scoring rubric to override the default (leave empty to use default)..."
                />
                <p class="mt-1 text-sm text-gray-500">
                  Define how the evaluation should be scored (optional - will use default if empty)
                </p>
              </div>

              <!-- Error Message -->
              <div
                v-if="error"
                class="rounded-md bg-red-50 p-4"
              >
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
                      {{ error }}
                    </h3>
                  </div>
                </div>
              </div>

              <!-- Success Message -->
              <div
                v-if="success"
                class="rounded-md bg-green-50 p-4"
              >
                <div class="flex">
                  <div class="flex-shrink-0">
                    <svg
                      class="h-5 w-5 text-green-400"
                      viewBox="0 0 20 20"
                      fill="currentColor"
                    >
                      <path
                        fill-rule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </div>
                  <div class="ml-3">
                    <h3 class="text-sm font-medium text-green-800">
                      {{ success }}
                    </h3>
                    <div
                      v-if="jobId"
                      class="mt-2"
                    >
                      <p class="text-sm text-green-700">
                        Job ID: <code class="bg-green-100 px-2 py-1 rounded">{{ jobId }}</code>
                      </p>
                      <p class="text-sm text-green-700 mt-1">
                        You can use this ID to check results later.
                      </p>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Submit Button -->
              <div class="flex justify-end">
                <LoadingButton
                  :loading="evaluating"
                  text="Start Evaluation"
                  loading-text="Starting Evaluation..."
                  variant="primary"
                  size="lg"
                  @click="handleEvaluate"
                />
              </div>
            </form>
          </div>
        </div>
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
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import axios from 'axios'
import { handleApiError } from '@/utils/errorHandler'
import { success as showSuccess, error as showError } from '@/utils/notifications'
import LoadingButton from '@/components/LoadingButton.vue'

const router = useRouter()
const authStore = useAuthStore()

const sidebarOpen = ref(false)
const userMenuOpen = ref(false)
const evaluating = ref(false)
const error = ref('')
const success = ref('')
const jobId = ref('')

const form = reactive({
  cv_id: '',
  project_id: '',
  job_description: '',
  study_case_brief: '',
  scoring_rubric: ''
})

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

const handleEvaluate = async () => {
  evaluating.value = true
  error.value = ''
  success.value = ''
  jobId.value = ''

  try {
    const response = await axios.post('/v1/evaluate', {
      cv_id: form.cv_id,
      project_id: form.project_id,
      job_description: form.job_description,
      study_case_brief: form.study_case_brief,
      scoring_rubric: form.scoring_rubric
    }, {
      withCredentials: true,
    })

    if (response.status === 200) {
      success.value = 'Evaluation started successfully!'
      // Backend returns 'id' not 'job_id'
      jobId.value = response.data.id || 'N/A'
      showSuccess('Evaluation Started', `Job ID: ${jobId.value}. You can check the status in the Jobs page.`)
      
      // Reset form
      form.cv_id = ''
      form.project_id = ''
      form.job_description = ''
      form.study_case_brief = ''
      form.scoring_rubric = ''
    }
  } catch (err: any) {
    const errorMessage = handleApiError(err)
    error.value = errorMessage
    showError('Evaluation Failed', errorMessage)
  } finally {
    evaluating.value = false
  }
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
