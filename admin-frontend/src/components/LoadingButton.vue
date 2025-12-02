<template>
  <button
    :disabled="loading || disabled"
    :class="buttonClass"
    @click="$emit('click')"
  >
    <LoadingSpinner 
      v-if="loading" 
      size="sm" 
      :class="spinnerClass"
    />
    <slot v-if="!loading" />
    <span
      v-if="loading && loadingText"
      class="ml-2"
    >{{ loadingText }}</span>
    <span v-else-if="!loading && text">{{ text }}</span>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import LoadingSpinner from './LoadingSpinner.vue'
interface Props {
  loading?: boolean
  disabled?: boolean
  text?: string
  loadingText?: string
  variant?: 'primary' | 'secondary' | 'danger' | 'success'
  size?: 'sm' | 'md' | 'lg'
  fullWidth?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  disabled: false,
  text: '',
  loadingText: '',
  variant: 'primary',
  size: 'md',
  fullWidth: false
})

defineEmits<{
  click: []
}>()

const buttonClass = computed(() => {
  const baseClass = 'inline-flex items-center justify-center font-medium rounded-md transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed'
  
  const variantClasses = {
    primary: 'bg-primary-600 text-white hover:bg-primary-700 focus:ring-primary-500',
    secondary: 'bg-gray-600 text-white hover:bg-gray-700 focus:ring-gray-500',
    danger: 'bg-red-600 text-white hover:bg-red-700 focus:ring-red-500',
    success: 'bg-green-600 text-white hover:bg-green-700 focus:ring-green-500'
  }
  
  const sizeClasses = {
    sm: 'px-3 py-1.5 text-sm',
    md: 'px-4 py-2 text-sm',
    lg: 'px-6 py-3 text-base'
  }
  
  const widthClass = props.fullWidth ? 'w-full' : ''
  
  return [
    baseClass,
    variantClasses[props.variant],
    sizeClasses[props.size],
    widthClass
  ].join(' ')
})

const spinnerClass = computed(() => {
  return props.variant === 'primary' ? 'text-white' : 'text-current'
})
</script>
