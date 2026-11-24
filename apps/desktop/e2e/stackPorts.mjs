// Fixed ports for the E2E stack, away from the dev ones (1420 UI, 4317 backend).
export const E2E_FIXTURE_PORT = 4430;
export const E2E_BACKEND_PORT = 4431;
export const E2E_UI_PORT = 4432;
export const E2E_UI_ORIGIN = `http://localhost:${E2E_UI_PORT}`;
export const E2E_API_TOKEN = "e2e-token";
