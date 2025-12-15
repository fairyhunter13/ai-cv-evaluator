export const PORTAL_PATH = '/';
export const PROTECTED_PATHS = ['/app/', '/grafana/', '/prometheus/', '/jaeger/', '/redpanda/', '/admin/'];

// Environment detection
export const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8088';
export const IS_PRODUCTION = BASE_URL.includes('ai-cv-evaluator.web.id');
export const IS_DEV = !IS_PRODUCTION;

// Credentials: Use env vars, with sensible defaults for dev
// In production CI, set SSO_USERNAME and SSO_PASSWORD secrets
export const SSO_USERNAME = process.env.SSO_USERNAME || (IS_PRODUCTION ? '' : 'admin');
export const SSO_PASSWORD = process.env.SSO_PASSWORD || (IS_PRODUCTION ? '' : 'admin123');

// Authelia Configuration
export const AUTHELIA_URL =
  process.env.AUTHELIA_URL || (IS_PRODUCTION ? 'https://auth.ai-cv-evaluator.web.id' : 'http://localhost:9091');

// Services that may not be available in all environments
export const DEV_ONLY_SERVICES = ['/mailpit/'];
