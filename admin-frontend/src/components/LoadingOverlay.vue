<template>
  <div 
    v-if="show" 
    class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
  >
    <div class="bg-white rounded-lg p-6 max-w-sm w-full mx-4">
      <div class="text-center">
        <LoadingSpinner :size="size" :text="text" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  show: boolean
  text?: string
  size?: 'sm' | 'md' | 'lg' | 'xl'
}

withDefaults(defineProps<Props>(), {
  text: 'Loading...',
  size: 'lg'
})
</script>
