<template>
  <div class="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
    <div class="px-6 py-4 border-b border-gray-200">
      <h3 class="text-lg font-medium text-gray-900">
        {{ title }}
      </h3>
      <p class="text-sm text-gray-600">
        {{ subtitle }}
      </p>
    </div>
    
    <div class="p-8 text-center">
      <LoadingSpinner
        size="lg"
        :text="text"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  title?: string
  subtitle?: string
  text?: string
}

withDefaults(defineProps<Props>(), {
  title: 'Loading',
  subtitle: 'Please wait...',
  text: 'Loading data...'
})
</script>
