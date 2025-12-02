<template>
  <div
    class="flex items-center justify-center"
    :class="containerClass"
  >
    <svg 
      class="animate-spin text-primary-600" 
      :class="spinnerClass"
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
    <span
      v-if="text"
      class="ml-2 text-sm text-gray-600"
    >{{ text }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
interface Props {
  size?: 'sm' | 'md' | 'lg' | 'xl'
  text?: string
  containerClass?: string
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  text: '',
  containerClass: ''
})

const spinnerClass = computed(() => {
  const sizeMap = {
    sm: 'h-4 w-4',
    md: 'h-6 w-6',
    lg: 'h-8 w-8',
    xl: 'h-12 w-12'
  }
  return sizeMap[props.size]
})
</script>
