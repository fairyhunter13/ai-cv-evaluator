import { registerAuthBasicTests } from './sso-gate/auth-basic.ts';
import { registerBackendApiAndHealthTests } from './sso-gate/backend-api-health.ts';
import { registerLogoutTests } from './sso-gate/logout.ts';
import { registerPortalBackendApiLinksTests } from './sso-gate/portal-backend-api-links.ts';
import { registerPortalLoginTests } from './sso-gate/portal-login.ts';
import { registerUnauthenticatedAccessTests } from './sso-gate/unauthenticated.ts';

registerUnauthenticatedAccessTests();
registerAuthBasicTests();
registerPortalLoginTests();
registerPortalBackendApiLinksTests();
registerBackendApiAndHealthTests();
registerLogoutTests();
