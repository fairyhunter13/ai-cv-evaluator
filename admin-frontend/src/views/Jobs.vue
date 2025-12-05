<template>
  <AdminLayout page-title="Job Management">
    <div class="max-w-7xl mx-auto">
          <!-- Filters and Search -->
          <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-6 mb-6">
            <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
              <!-- Search -->
              <div class="md:col-span-2">
                <label
                  for="search"
                  class="block text-sm font-medium text-gray-700 mb-2"
                >
                  Search Jobs
                </label>
                <input
                  id="search"
                  v-model="filters.search"
                  type="text"
                  placeholder="Search by job ID, CV ID, or project ID..."
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                  @input="debouncedSearch"
                >
              </div>

              <!-- Status Filter -->
              <div>
                <label
                  for="status"
                  class="block text-sm font-medium text-gray-700 mb-2"
                >
                  Status
                </label>
                <select
                  id="status"
                  v-model="filters.status"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                  @change="loadJobs"
                >
                  <option value="">
                    All Statuses
                  </option>
                  <option value="queued">
                    Queued
                  </option>
                  <option value="processing">
                    Processing
                  </option>
                  <option value="completed">
                    Completed
                  </option>
                  <option value="failed">
                    Failed
                  </option>
                </select>
              </div>

              <!-- Refresh Button -->
              <div class="flex items-end gap-2">
                <LoadingButton
                  :loading="loading"
                  text="Refresh"
                  loading-text="Loading..."
                  variant="primary"
                  size="md"
                  class="flex-1"
                  @click="loadJobs"
                />
                <button
                  :class="[
                    'px-3 py-2 rounded-md text-sm font-medium transition-colors',
                    autoRefreshEnabled 
                      ? 'bg-green-100 text-green-700 hover:bg-green-200' 
                      : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                  ]"
                  :title="autoRefreshEnabled ? 'Auto-refresh enabled' : 'Auto-refresh disabled'"
                  @click="toggleAutoRefresh"
                >
                  <svg
                    class="w-5 h-5"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </div>

          <!-- Jobs Table -->
          <div class="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
            <div class="px-6 py-4 border-b border-gray-200">
              <h3 class="text-lg font-medium text-gray-900">
                Jobs
              </h3>
              <p class="text-sm text-gray-600">
                Manage and monitor evaluation jobs
              </p>
            </div>

            <!-- Loading State -->
            <div v-if="loading && jobs.length === 0">
              <LoadingTable
                title="Jobs"
                subtitle="Manage and monitor evaluation jobs"
                text="Loading jobs..."
              />
            </div>

            <!-- Error State -->
            <div
              v-else-if="error"
              class="p-6"
            >
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
                      Error loading jobs
                    </h3>
                    <p class="mt-1 text-sm text-red-700">
                      {{ error }}
                    </p>
                    <button
                      class="mt-2 text-sm text-red-600 hover:text-red-500"
                      @click="loadJobs"
                    >
                      Try again
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Jobs Table - Desktop -->
            <div
              v-else-if="jobs.length > 0"
              class="hidden md:block overflow-x-auto"
            >
              <table class="min-w-full divide-y divide-gray-200">
                <thead class="bg-gray-50">
                  <tr>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Job ID
                    </th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Status
                    </th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      CV ID
                    </th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Project ID
                    </th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Created
                    </th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Updated
                    </th>
                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody class="bg-white divide-y divide-gray-200">
                  <tr
                    v-for="job in jobs"
                    :key="job.id"
                    class="hover:bg-gray-50"
                  >
                    <td class="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-900">
                      {{ job.id }}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap">
                      <span 
                        class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                        :class="{
                          'bg-yellow-100 text-yellow-800': job.status === 'queued',
                          'bg-blue-100 text-blue-800': job.status === 'processing',
                          'bg-green-100 text-green-800': job.status === 'completed',
                          'bg-red-100 text-red-800': job.status === 'failed'
                        }"
                      >
                        <div 
                          class="w-2 h-2 rounded-full mr-2"
                          :class="{
                            'bg-yellow-500': job.status === 'queued',
                            'bg-blue-500': job.status === 'processing',
                            'bg-green-500': job.status === 'completed',
                            'bg-red-500': job.status === 'failed'
                          }"
                        />
                        {{ job.status?.charAt(0).toUpperCase() + job.status?.slice(1) }}
                      </span>
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 font-mono">
                      {{ job.cv_id }}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 font-mono">
                      {{ job.project_id }}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {{ formatDate(job.created_at) }}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {{ formatDate(job.updated_at) }}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                      <button
                        class="text-primary-600 hover:text-primary-900 mr-3"
                        @click="viewJobDetails(job.id)"
                      >
                        View Details
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Jobs Cards - Mobile -->
            <div
              v-else-if="jobs.length > 0"
              class="md:hidden divide-y divide-gray-200"
            >
              <div
                v-for="job in jobs"
                :key="job.id"
                class="p-4 hover:bg-gray-50"
              >
                <div class="space-y-3">
                  <!-- Job ID and Status -->
                  <div class="flex items-start justify-between">
                    <div class="flex-1 min-w-0">
                      <p class="text-xs text-gray-500 mb-1">
                        Job ID
                      </p>
                      <p class="text-sm font-mono text-gray-900 truncate">
                        {{ job.id }}
                      </p>
                    </div>
                    <span 
                      class="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                      :class="{
                        'bg-yellow-100 text-yellow-800': job.status === 'queued',
                        'bg-blue-100 text-blue-800': job.status === 'processing',
                        'bg-green-100 text-green-800': job.status === 'completed',
                        'bg-red-100 text-red-800': job.status === 'failed'
                      }"
                    >
                      {{ job.status?.charAt(0).toUpperCase() + job.status?.slice(1) }}
                    </span>
                  </div>

                  <!-- CV and Project IDs -->
                  <div class="grid grid-cols-2 gap-3">
                    <div>
                      <p class="text-xs text-gray-500 mb-1">
                        CV ID
                      </p>
                      <p class="text-sm font-mono text-gray-900 truncate">
                        {{ job.cv_id }}
                      </p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 mb-1">
                        Project ID
                      </p>
                      <p class="text-sm font-mono text-gray-900 truncate">
                        {{ job.project_id }}
                      </p>
                    </div>
                  </div>

                  <!-- Timestamps -->
                  <div class="grid grid-cols-2 gap-3">
                    <div>
                      <p class="text-xs text-gray-500 mb-1">
                        Created
                      </p>
                      <p class="text-xs text-gray-700">
                        {{ formatDate(job.created_at) }}
                      </p>
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 mb-1">
                        Updated
                      </p>
                      <p class="text-xs text-gray-700">
                        {{ formatDate(job.updated_at) }}
                      </p>
                    </div>
                  </div>

                  <!-- Action Button -->
                  <button
                    class="w-full mt-2 px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-md hover:bg-primary-700 transition-colors"
                    @click="viewJobDetails(job.id)"
                  >
                    View Details
                  </button>
                </div>
              </div>
            </div>

            <!-- Empty State -->
            <div
              v-else
              class="p-8 text-center"
            >
              <svg
                class="mx-auto h-12 w-12 text-gray-400"
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
              <h3 class="mt-2 text-sm font-medium text-gray-900">
                No jobs found
              </h3>
              <p class="mt-1 text-sm text-gray-500">
                Get started by uploading files and starting an evaluation.
              </p>
            </div>

            <!-- Pagination -->
            <div
              v-if="pagination.total > 0"
              class="px-6 py-4 border-t border-gray-200"
            >
              <div class="flex items-center justify-between">
                <div class="text-sm text-gray-700">
                  Showing {{ (pagination.page - 1) * pagination.limit + 1 }} to {{ Math.min(pagination.page * pagination.limit, pagination.total) }} of {{ pagination.total }} jobs
                </div>
                <div class="flex space-x-2">
                  <button
                    :disabled="pagination.page <= 1"
                    class="px-3 py-1 text-sm border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                    @click="changePage(pagination.page - 1)"
                  >
                    Previous
                  </button>
                  <span class="px-3 py-1 text-sm text-gray-700">
                    Page {{ pagination.page }} of {{ Math.ceil(pagination.total / pagination.limit) }}
                  </span>
                  <button
                    :disabled="pagination.page >= Math.ceil(pagination.total / pagination.limit)"
                    class="px-3 py-1 text-sm border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                    @click="changePage(pagination.page + 1)"
                  >
                    Next
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

    <!-- Job Details Modal -->
    <div
      v-if="selectedJob"
      class="fixed inset-0 z-50 overflow-y-auto"
    >
      <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:block sm:p-0">
        <div
          class="fixed inset-0 transition-opacity"
          @click="closeJobDetails"
        >
          <div class="absolute inset-0 bg-gray-500 opacity-75" />
        </div>

        <div class="inline-block align-bottom bg-white rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-4xl sm:w-full">
          <div class="bg-white px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-lg font-medium text-gray-900">
                Job Details
              </h3>
              <button
                class="text-gray-400 hover:text-gray-600"
                @click="closeJobDetails"
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
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>

            <div
              v-if="jobDetailsLoading"
              class="text-center py-8"
            >
              <LoadingSpinner
                size="lg"
                text="Loading job details..."
              />
            </div>

            <div
              v-else-if="jobDetailsError"
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
                    Error loading job details
                  </h3>
                  <p class="mt-1 text-sm text-red-700">
                    {{ jobDetailsError }}
                  </p>
                </div>
              </div>
            </div>

            <div
              v-else-if="jobDetails"
              class="space-y-4"
            >
              <!-- Basic Info -->
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label class="block text-sm font-medium text-gray-700">Job ID</label>
                  <p class="mt-1 text-sm text-gray-900 font-mono">
                    {{ jobDetails.id }}
                  </p>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700">Status</label>
                  <span 
                    class="mt-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                    :class="{
                      'bg-yellow-100 text-yellow-800': jobDetails.status === 'queued',
                      'bg-blue-100 text-blue-800': jobDetails.status === 'processing',
                      'bg-green-100 text-green-800': jobDetails.status === 'completed',
                      'bg-red-100 text-red-800': jobDetails.status === 'failed'
                    }"
                  >
                    {{ jobDetails.status?.charAt(0).toUpperCase() + jobDetails.status?.slice(1) }}
                  </span>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700">CV ID</label>
                  <p class="mt-1 text-sm text-gray-900 font-mono">
                    {{ jobDetails.cv_id }}
                  </p>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700">Project ID</label>
                  <p class="mt-1 text-sm text-gray-900 font-mono">
                    {{ jobDetails.project_id }}
                  </p>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700">Created</label>
                  <p class="mt-1 text-sm text-gray-900">
                    {{ formatDate(jobDetails.created_at) }}
                  </p>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-700">Updated</label>
                  <p class="mt-1 text-sm text-gray-900">
                    {{ formatDate(jobDetails.updated_at) }}
                  </p>
                </div>
              </div>

              <!-- Error Information -->
              <div
                v-if="jobDetails.error"
                class="rounded-md bg-red-50 p-4"
              >
                <h4 class="text-sm font-medium text-red-800">
                  Error Details
                </h4>
                <p class="mt-1 text-sm text-red-700">
                  {{ jobDetails.error.message }}
                </p>
                <p class="mt-1 text-xs text-red-600">
                  Code: {{ jobDetails.error.code }}
                </p>
              </div>

              <!-- Result Information -->
              <div
                v-if="jobDetails.result"
                class="rounded-md bg-green-50 p-4"
              >
                <h4 class="text-sm font-medium text-green-800">
                  Evaluation Results
                </h4>
                <div class="mt-2 space-y-2">
                  <div v-if="jobDetails.result.cv_match_rate !== undefined">
                    <span class="text-sm font-medium text-green-700">CV Match Rate:</span>
                    <span class="ml-2 text-sm text-green-600">{{ (jobDetails.result.cv_match_rate * 100).toFixed(1) }}%</span>
                  </div>
                  <div v-if="jobDetails.result.project_score !== undefined">
                    <span class="text-sm font-medium text-green-700">Project Score:</span>
                    <span class="ml-2 text-sm text-green-600">{{ jobDetails.result.project_score }}/10</span>
                  </div>
                  <div v-if="jobDetails.result.cv_feedback">
                    <span class="text-sm font-medium text-green-700">CV Feedback:</span>
                    <p class="mt-1 text-sm text-green-600">
                      {{ jobDetails.result.cv_feedback }}
                    </p>
                  </div>
                  <div v-if="jobDetails.result.project_feedback">
                    <span class="text-sm font-medium text-green-700">Project Feedback:</span>
                    <p class="mt-1 text-sm text-green-600">
                      {{ jobDetails.result.project_feedback }}
                    </p>
                  </div>
                  <div v-if="jobDetails.result.overall_summary">
                    <span class="text-sm font-medium text-green-700">Overall Summary:</span>
                    <p class="mt-1 text-sm text-green-600">
                      {{ jobDetails.result.overall_summary }}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, watch } from 'vue'
import axios from 'axios'
import { handleApiError } from '@/utils/errorHandler'
import { success, error as showError } from '@/utils/notifications'
import config from '@/config'
import AdminLayout from '@/layouts/AdminLayout.vue'
import LoadingButton from '@/components/LoadingButton.vue'
import LoadingTable from '@/components/LoadingTable.vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'

const loading = ref(false)
const error = ref('')
const jobs = ref<any[]>([])
const pagination = reactive({
  page: 1,
  limit: 10,
  total: 0
})

const filters = reactive({
  search: '',
  status: ''
})

const selectedJob = ref<string | null>(null)
const jobDetails = ref<any>(null)
const jobDetailsLoading = ref(false)
const jobDetailsError = ref('')

// Auto-refresh
const autoRefreshEnabled = ref(true)
const autoRefreshInterval = ref(config.autoRefreshInterval) // From environment config
let refreshInterval: NodeJS.Timeout | null = null

// Debounced search
let searchTimeout: NodeJS.Timeout | null = null
const debouncedSearch = () => {
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadJobs()
  }, 500)
}

// Start auto-refresh
const startAutoRefresh = () => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
  if (autoRefreshEnabled.value) {
    refreshInterval = setInterval(() => {
      // Only refresh if not currently loading and no modal is open
      if (!loading.value && !selectedJob.value) {
        loadJobs(true) // Silent refresh
      }
    }, autoRefreshInterval.value)
  }
}

// Stop auto-refresh
const stopAutoRefresh = () => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }
}

// Toggle auto-refresh
const toggleAutoRefresh = () => {
  autoRefreshEnabled.value = !autoRefreshEnabled.value
  if (autoRefreshEnabled.value) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
}

const loadJobs = async (silent = false) => {
  loading.value = true
  error.value = ''

  try {
    const params = new URLSearchParams({
      page: pagination.page.toString(),
      limit: pagination.limit.toString()
    })

    if (filters.search) {
      params.append('search', filters.search)
    }
    if (filters.status) {
      params.append('status', filters.status)
    }

    const response = await axios.get(`/admin/api/jobs?${params.toString()}`, {
      withCredentials: true,
    })

    if (response.status === 200) {
      jobs.value = response.data.jobs || []
      pagination.total = response.data.pagination?.total || 0
      // Only show success notification on manual refresh
      if (!silent) {
        success('Jobs loaded', `Found ${jobs.value.length} jobs`)
      }
    }
  } catch (err: any) {
    const errorMessage = handleApiError(err)
    error.value = errorMessage
    // Only show error on manual refresh
    if (!silent) {
      showError('Failed to load jobs', errorMessage)
    }
  } finally {
    loading.value = false
  }
}

const changePage = (page: number) => {
  if (page >= 1 && page <= Math.ceil(pagination.total / pagination.limit)) {
    pagination.page = page
    loadJobs()
  }
}

const viewJobDetails = async (jobId: string) => {
  selectedJob.value = jobId
  jobDetailsLoading.value = true
  jobDetailsError.value = ''
  jobDetails.value = null

  try {
    const response = await axios.get(`/admin/api/jobs/${jobId}`, {
      withCredentials: true,
    })

    if (response.status === 200) {
      jobDetails.value = response.data
      success('Job details loaded', `Details for job ${jobId} loaded successfully`)
    }
  } catch (err: any) {
    const errorMessage = handleApiError(err)
    jobDetailsError.value = errorMessage
    showError('Failed to load job details', errorMessage)
  } finally {
    jobDetailsLoading.value = false
  }
}

const closeJobDetails = () => {
  selectedJob.value = null
  jobDetails.value = null
  jobDetailsError.value = ''
}

const formatDate = (dateString: string) => {
  if (!dateString) return 'N/A'
  try {
    const date = new Date(dateString)
    return date.toLocaleString()
  } catch {
    return dateString
  }
}

// Watch for filter changes
watch([() => filters.status], () => {
  pagination.page = 1
  loadJobs()
})

onMounted(() => {
  loadJobs()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>
