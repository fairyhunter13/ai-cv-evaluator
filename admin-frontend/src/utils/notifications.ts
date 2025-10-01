import { ref, reactive } from 'vue'

export interface Notification {
  id: string
  type: 'success' | 'error' | 'warning' | 'info'
  title: string
  message: string
  duration?: number
  persistent?: boolean
}

const notifications = ref<Notification[]>([])

// Helper function to add notifications
export const addNotification = (notification: Omit<Notification, 'id'>) => {
  const id = Math.random().toString(36).substr(2, 9)
  const newNotification: Notification = {
    id,
    duration: 5000,
    persistent: false,
    ...notification
  }
  
  notifications.value.push(newNotification)
  
  // Auto-remove non-persistent notifications
  if (!newNotification.persistent && newNotification.duration) {
    setTimeout(() => {
      removeNotification(id)
    }, newNotification.duration)
  }
  
  return id
}

export const removeNotification = (id: string) => {
  const index = notifications.value.findIndex(n => n.id === id)
  if (index > -1) {
    notifications.value.splice(index, 1)
  }
}

export const clearAll = () => {
  notifications.value = []
}

export const success = (title: string, message: string, options?: Partial<Notification>) => {
  return addNotification({
    type: 'success',
    title,
    message,
    ...options
  })
}

export const error = (title: string, message: string, options?: Partial<Notification>) => {
  return addNotification({
    type: 'error',
    title,
    message,
    persistent: true, // Errors should be persistent by default
    ...options
  })
}

export const warning = (title: string, message: string, options?: Partial<Notification>) => {
  return addNotification({
    type: 'warning',
    title,
    message,
    ...options
  })
}

export const info = (title: string, message: string, options?: Partial<Notification>) => {
  return addNotification({
    type: 'info',
    title,
    message,
    ...options
  })
}

// Global notification store
export const notificationStore = reactive({
  notifications: notifications.value,
  add: addNotification,
  remove: removeNotification,
  clear: clearAll,
  success,
  error,
  warning,
  info
})

export const useNotifications = () => {
  return {
    notifications: notifications.value,
    addNotification,
    removeNotification,
    clearAll,
    success,
    error,
    warning,
    info
  }
}
