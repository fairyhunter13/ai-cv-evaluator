<template>
  <AdminLayout page-title="Upload Files">
    <div class="max-w-4xl mx-auto">
          <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-8">
            <div class="text-center mb-8">
              <h2 class="text-2xl font-bold text-gray-900 mb-2">
                Upload Documents
              </h2>
              <p class="text-gray-600">
                Upload your CV and project files for AI evaluation
              </p>
            </div>

            <form
              class="space-y-8"
              @submit.prevent="handleUpload"
            >
              <!-- CV Upload -->
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-2">
                  CV File (PDF, DOC, DOCX)
                </label>
                <div 
                  class="border-2 border-dashed border-gray-300 rounded-lg p-6 text-center hover:border-primary-400 transition-colors"
                  :class="{ 'border-primary-400 bg-primary-50': dragOver.cv }"
                  @drop="handleDrop($event, 'cv')"
                  @dragover.prevent
                  @dragenter.prevent
                >
                  <input
                    ref="cvInput"
                    type="file"
                    accept=".pdf,.doc,.docx"
                    class="hidden"
                    @change="handleFileSelect($event, 'cv')"
                  >
                  <div
                    v-if="!files.cv"
                    class="space-y-4"
                  >
                    <svg
                      class="mx-auto h-12 w-12 text-gray-400"
                      stroke="currentColor"
                      fill="none"
                      viewBox="0 0 48 48"
                    >
                      <path
                        d="M28 8H12a4 4 0 00-4 4v20m32-12v8m0 0v8a4 4 0 01-4 4H12a4 4 0 01-4-4v-4m32-4l-3.172-3.172a4 4 0 00-5.656 0L28 28M8 32l9.172-9.172a4 4 0 015.656 0L28 28m0 0l4 4m4-24h8m-4-4v8m-12 4h.02"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                    <div>
                      <button
                        type="button"
                        class="text-primary-600 hover:text-primary-500 font-medium"
                        @click="cvInput?.click()"
                      >
                        Click to upload
                      </button>
                      <p class="text-gray-500">
                        or drag and drop
                      </p>
                    </div>
                    <p class="text-sm text-gray-500">
                      PDF, DOC, DOCX up to 10MB
                    </p>
                  </div>
                  <div
                    v-else
                    class="space-y-2"
                  >
                    <svg
                      class="mx-auto h-8 w-8 text-green-500"
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
                    <p class="text-sm font-medium text-gray-900">
                      {{ files.cv.name }}
                    </p>
                    <p class="text-sm text-gray-500">
                      {{ formatFileSize(files.cv.size) }}
                    </p>
                    <button
                      type="button"
                      class="text-red-600 hover:text-red-500 text-sm"
                      @click="removeFile('cv')"
                    >
                      Remove
                    </button>
                  </div>
                </div>
              </div>

              <!-- Project Upload -->
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-2">
                  Project File (PDF, DOC, DOCX)
                </label>
                <div 
                  class="border-2 border-dashed border-gray-300 rounded-lg p-6 text-center hover:border-primary-400 transition-colors"
                  :class="{ 'border-primary-400 bg-primary-50': dragOver.project }"
                  @drop="handleDrop($event, 'project')"
                  @dragover.prevent
                  @dragenter.prevent
                >
                  <input
                    ref="projectInput"
                    type="file"
                    accept=".pdf,.doc,.docx"
                    class="hidden"
                    @change="handleFileSelect($event, 'project')"
                  >
                  <div
                    v-if="!files.project"
                    class="space-y-4"
                  >
                    <svg
                      class="mx-auto h-12 w-12 text-gray-400"
                      stroke="currentColor"
                      fill="none"
                      viewBox="0 0 48 48"
                    >
                      <path
                        d="M28 8H12a4 4 0 00-4 4v20m32-12v8m0 0v8a4 4 0 01-4 4H12a4 4 0 01-4-4v-4m32-4l-3.172-3.172a4 4 0 00-5.656 0L28 28M8 32l9.172-9.172a4 4 0 015.656 0L28 28m0 0l4 4m4-24h8m-4-4v8m-12 4h.02"
                        stroke-width="2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                    <div>
                      <button
                        type="button"
                        class="text-primary-600 hover:text-primary-500 font-medium"
                        @click="projectInput?.click()"
                      >
                        Click to upload
                      </button>
                      <p class="text-gray-500">
                        or drag and drop
                      </p>
                    </div>
                    <p class="text-sm text-gray-500">
                      PDF, DOC, DOCX up to 10MB
                    </p>
                  </div>
                  <div
                    v-else
                    class="space-y-2"
                  >
                    <svg
                      class="mx-auto h-8 w-8 text-green-500"
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
                    <p class="text-sm font-medium text-gray-900">
                      {{ files.project.name }}
                    </p>
                    <p class="text-sm text-gray-500">
                      {{ formatFileSize(files.project.size) }}
                    </p>
                    <button
                      type="button"
                      class="text-red-600 hover:text-red-500 text-sm"
                      @click="removeFile('project')"
                    >
                      Remove
                    </button>
                  </div>
                </div>
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
                  </div>
                </div>
              </div>

              <!-- Submit Button -->
              <div class="flex justify-end">
                <LoadingButton
                  :loading="uploading"
                  :disabled="!files.cv || !files.project"
                  text="Upload Files"
                  loading-text="Uploading..."
                  variant="primary"
                  size="lg"
                  @click="handleUpload"
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

const uploading = ref(false)
const error = ref('')
const success = ref('')
const cvInput = ref<HTMLInputElement | null>(null)
const projectInput = ref<HTMLInputElement | null>(null)

const files = reactive({
  cv: null as File | null,
  project: null as File | null
})

const dragOver = reactive({
  cv: false,
  project: false
})

const handleFileSelect = (event: Event, type: 'cv' | 'project') => {
  const target = event.target as HTMLInputElement
  if (target.files && target.files[0]) {
    files[type] = target.files[0]
  }
}

const handleDrop = (event: DragEvent, type: 'cv' | 'project') => {
  event.preventDefault()
  dragOver[type] = false
  
  if (event.dataTransfer?.files && event.dataTransfer.files[0]) {
    files[type] = event.dataTransfer.files[0]
  }
}

const removeFile = (type: 'cv' | 'project') => {
  files[type] = null
}

const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const handleUpload = async () => {
  if (!files.cv || !files.project) {
    error.value = 'Please select both CV and project files'
    return
  }

  uploading.value = true
  error.value = ''
  success.value = ''

  try {
    const formData = new FormData()
    formData.append('cv', files.cv)
    formData.append('project', files.project)

    const response = await axios.post('/v1/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      withCredentials: true,
    })

    if (response.status === 200) {
      success.value = 'Files uploaded successfully!'
      // Display the returned IDs
      if (response.data.cv_id && response.data.project_id) {
        success.value += ` CV ID: ${response.data.cv_id}, Project ID: ${response.data.project_id}`
        showSuccess('Upload Successful', `CV ID: ${response.data.cv_id}, Project ID: ${response.data.project_id}`)
      } else {
        showSuccess('Upload Successful', 'Files uploaded successfully!')
      }
      // Reset form
      files.cv = null
      files.project = null
    }
  } catch (err: any) {
    const errorMessage = handleApiError(err)
    error.value = errorMessage
    showError('Upload Failed', errorMessage)
  } finally {
    uploading.value = false
  }
}
</script>
