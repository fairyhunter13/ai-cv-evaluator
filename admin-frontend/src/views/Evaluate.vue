<template>
  <AdminLayout page-title="Start Evaluation">
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
  </AdminLayout>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import axios from 'axios'
import { handleApiError } from '@/utils/errorHandler'
import { success as showSuccess, error as showError } from '@/utils/notifications'
import AdminLayout from '@/layouts/AdminLayout.vue'
import LoadingButton from '@/components/LoadingButton.vue'

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
</script>
