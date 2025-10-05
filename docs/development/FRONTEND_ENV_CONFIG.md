# Environment Configuration

This document explains how to configure the admin frontend using environment variables.

## Environment Files

The application supports different environment configurations:

- `.env.development` - Development environment (used when running `npm run dev`)
- `.env.production` - Production environment (used when running `npm run build`)
- `.env.example` - Example configuration file (copy this to create your own)

## Available Variables

### API Configuration

- `VITE_API_BASE_URL` - Base URL for the API server
  - Development: `http://localhost:8080`
  - Production: Your production API URL

### Auto-refresh Configuration

- `VITE_AUTO_REFRESH_INTERVAL` - Interval for auto-refreshing jobs page (in milliseconds)
  - Development: `5000` (5 seconds)
  - Production: `10000` (10 seconds recommended)

### Application Configuration

- `VITE_APP_NAME` - Application name displayed in the UI
- `VITE_APP_VERSION` - Application version

## Setup Instructions

### Development

1. Copy `.env.example` to `.env.development`:
   ```bash
   cp .env.example .env.development
   ```

2. Update the values as needed for your development environment

3. Run the development server:
   ```bash
   npm run dev
   ```

### Production

1. Copy `.env.production.example` to `.env.production`:
   ```bash
   cp .env.production.example .env.production
   ```

2. Update the values for your production environment:
   - Set `VITE_API_BASE_URL` to your production API URL
   - Adjust `VITE_AUTO_REFRESH_INTERVAL` as needed

3. Build for production:
   ```bash
   npm run build
   ```

## Using Environment Variables in Code

Import the config object:

```typescript
import config from '@/config'

// Access configuration
const apiUrl = config.apiBaseUrl
const refreshInterval = config.autoRefreshInterval
const appName = config.appName
```

## Notes

- All environment variables must be prefixed with `VITE_` to be exposed to the client
- Changes to environment files require restarting the dev server
- Never commit `.env.production` or `.env.development` files with sensitive data
- Use `.env.example` and `.env.production.example` as templates
