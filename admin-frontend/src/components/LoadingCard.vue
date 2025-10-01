<template>
  <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-8">
    <div class="text-center">
      <LoadingSpinner size="lg" :text="text" />
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  text?: string
}

withDefaults(defineProps<Props>(), {
  text: 'Loading...'
})
</script>
