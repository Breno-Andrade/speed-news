import { defineConfig, devices } from "@playwright/test";
import { E2E_API_TOKEN, E2E_BACKEND_PORT, E2E_UI_ORIGIN, E2E_UI_PORT } from "./e2e/stackPorts.mjs";

const isContinuousIntegration = Boolean(process.env.CI);

export default defineConfig({
  testDir: "./e2e",
  // The backend is shared by all tests (like the real app): run them one at a time.
  workers: 1,
  fullyParallel: false,
  retries: isContinuousIntegration ? 1 : 0,
  forbidOnly: isContinuousIntegration,
  reporter: isContinuousIntegration ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: E2E_UI_ORIGIN,
    locale: "pt-BR",
    timezoneId: "America/Sao_Paulo",
    viewport: { width: 380, height: 640 },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 380, height: 640 } } },
    // The desktop app runs on WebKitGTK on Linux: CI also checks WebKit (it needs system libraries
    // that CI installs with "playwright install --with-deps webkit").
    ...(isContinuousIntegration ? [{ name: "webkit", use: { ...devices["Desktop Safari"], viewport: { width: 380, height: 640 } } }] : []),
  ],
  webServer: [
    {
      command: "node e2e/startBackendStack.mjs",
      url: `http://127.0.0.1:${E2E_BACKEND_PORT}/health`,
      timeout: 120_000,
      reuseExistingServer: false,
      stdout: "ignore",
    },
    {
      command: `npx vite --port ${E2E_UI_PORT} --strictPort`,
      url: E2E_UI_ORIGIN,
      timeout: 60_000,
      reuseExistingServer: false,
      env: { VITE_API_BASE_URL: `http://127.0.0.1:${E2E_BACKEND_PORT}`, VITE_API_TOKEN: E2E_API_TOKEN },
    },
  ],
});
