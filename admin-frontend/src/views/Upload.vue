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
                d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
              />
            </svg>
            Upload Files
          </router-link>
          <router-link 
            to="/evaluate" 
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
              Upload Files
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

      <!-- Upload Content -->
      <main class="p-6">
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

onMounted(() => {
  // Close user menu when clicking outside
  document.addEventListener('click', (e) => {
    if (!(e.target as Element).closest('.relative')) {
      userMenuOpen.value = false
    }
  })
})
</script>
