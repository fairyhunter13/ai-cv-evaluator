// Application configuration from environment variables

export const config = {
  // API Configuration
  apiBaseUrl: (import.meta as any).env?.VITE_API_BASE_URL || '',
  
  // Auto-refresh Configuration
  autoRefreshInterval: parseInt(import.meta.env.VITE_AUTO_REFRESH_INTERVAL || '5000', 10),
  
  // Application Configuration
  appName: import.meta.env.VITE_APP_NAME || 'AI CV Evaluator Admin',
  appVersion: import.meta.env.VITE_APP_VERSION || '1.0.0',
  
  // Environment
  isDevelopment: import.meta.env.DEV,
  isProduction: import.meta.env.PROD,
}

export default config
