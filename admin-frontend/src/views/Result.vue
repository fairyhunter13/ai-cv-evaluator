<template>
  <AdminLayout page-title="View Results">
    <div class="max-w-6xl mx-auto">
          <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-8">
            <div class="text-center mb-8">
              <h2 class="text-2xl font-bold text-gray-900 mb-2">
                Evaluation Results
              </h2>
              <p class="text-gray-600">
                View and analyze AI evaluation results
              </p>
            </div>

            <form
              class="space-y-6 mb-8"
              @submit.prevent="handleGetResult"
            >
              <div>
                <label
                  for="job_id"
                  class="block text-sm font-medium text-gray-700 mb-2"
                >
                  Job ID
                </label>
                <div class="flex space-x-4">
                  <input
                    id="job_id"
                    v-model="jobId"
                    type="text"
                    required
                    class="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-primary-500 focus:border-primary-500"
                    placeholder="Enter Job ID from evaluation"
                  >
                  <LoadingButton
                    :loading="loading"
                    text="Get Results"
                    loading-text="Loading..."
                    variant="primary"
                    size="lg"
                    @click="handleGetResult"
                  />
                </div>
                <p class="mt-1 text-sm text-gray-500">
                  Enter the Job ID returned from the evaluation process
                </p>
              </div>
            </form>

            <!-- Error Message -->
            <div
              v-if="error"
              class="rounded-md bg-red-50 p-4 mb-6"
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

            <!-- Results Display -->
            <div
              v-if="result"
              class="space-y-6"
            >
              <!-- Status -->
              <div class="bg-gray-50 rounded-lg p-4">
                <div class="flex items-center justify-between">
                  <div>
                    <h3 class="text-lg font-medium text-gray-900">
                      Status
                    </h3>
                    <p class="text-sm text-gray-600">
                      Current evaluation status
                    </p>
                  </div>
                  <div class="flex items-center">
                    <div 
                      class="w-3 h-3 rounded-full mr-2"
                      :class="{
                        'bg-green-500': result.status === 'completed',
                        'bg-yellow-500': result.status === 'processing',
                        'bg-red-500': result.status === 'failed'
                      }"
                    />
                    <span 
                      class="text-sm font-medium"
                      :class="{
                        'text-green-600': result.status === 'completed',
                        'text-yellow-600': result.status === 'processing',
                        'text-red-600': result.status === 'failed'
                      }"
                    >
                      {{ result.status?.charAt(0).toUpperCase() + result.status?.slice(1) }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- Results Content -->
              <div
                v-if="result.status === 'completed'"
                class="space-y-6"
              >
                <div class="bg-white border border-gray-200 rounded-lg p-6">
                  <h3 class="text-lg font-semibold text-gray-900 mb-4">
                    Evaluation Results
                  </h3>
                  
                  <!-- JSON Display -->
                  <div class="bg-gray-50 rounded-lg p-4">
                    <div class="flex items-center justify-between mb-2">
                      <h4 class="text-sm font-medium text-gray-700">
                        Raw Results (JSON)
                      </h4>
                      <button
                        class="text-xs text-primary-600 hover:text-primary-500"
                        @click="copyToClipboard"
                      >
                        Copy to Clipboard
                      </button>
                    </div>
                    <pre class="text-xs text-gray-600 overflow-x-auto">{{ JSON.stringify(result, null, 2) }}</pre>
                  </div>
                </div>
              </div>

              <!-- Processing Status -->
              <div
                v-else-if="result.status === 'processing'"
                class="text-center py-8"
              >
                <div class="inline-flex items-center px-4 py-2 bg-yellow-100 text-yellow-800 rounded-lg">
                  <svg
                    class="animate-spin -ml-1 mr-3 h-5 w-5 text-yellow-600"
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    />
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    />
                  </svg>
                  Evaluation is still processing. Please check again later.
                </div>
              </div>

              <!-- Failed Status -->
              <div
                v-else-if="result.status === 'failed'"
                class="text-center py-8"
              >
                <div class="inline-flex items-center px-4 py-2 bg-red-100 text-red-800 rounded-lg">
                  <svg
                    class="w-5 h-5 mr-2"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  Evaluation failed. Please try again.
                </div>
              </div>
            </div>

            <!-- No Results -->
            <div v-else-if="!loading && !result">
              <LoadingCard text="Enter a Job ID to view evaluation results" />
            </div>
          </div>
        </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import axios from 'axios'
import AdminLayout from '@/layouts/AdminLayout.vue'
import LoadingButton from '@/components/LoadingButton.vue'
import LoadingCard from '@/components/LoadingCard.vue'

const loading = ref(false)
const error = ref('')
const result = ref<any>(null)
const jobId = ref('')
// Cache ETag per job ID
const etags = ref<Record<string, string>>({})

const handleGetResult = async () => {
  if (!jobId.value.trim()) {
    error.value = 'Please enter a Job ID'
    return
  }

  loading.value = true
  error.value = ''
  result.value = null

  try {
    const currentJob = jobId.value
    const response = await axios.get(`/v1/result/${currentJob}`, {
      withCredentials: true,
      headers: etags.value[currentJob] ? { 'If-None-Match': etags.value[currentJob] } : {},
      validateStatus: (status) => (status >= 200 && status < 300) || status === 304,
    })

    if (response.status === 200) {
      result.value = response.data
      const newETag = response.headers['etag'] || response.headers['ETag']
      if (newETag) {
        etags.value[currentJob] = newETag as string
      }
    } else if (response.status === 304) {
      // Not modified; keep current result as-is
    }
  } catch (err: any) {
    if (err.response?.status === 404) {
      error.value = 'Job not found. Please check the Job ID.'
    } else if (err.response?.data?.error) {
      error.value = err.response.data.error
    } else {
      error.value = 'Failed to fetch results. Please try again.'
    }
  } finally {
    loading.value = false
  }
}

const copyToClipboard = async () => {
  try {
    await navigator.clipboard.writeText(JSON.stringify(result.value, null, 2))
    // You could add a toast notification here
  } catch (err) {
    console.error('Failed to copy to clipboard:', err)
  }
}
</script>
