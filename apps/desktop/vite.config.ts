import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// Tauri expects a fixed port; the same server is used for browser-only development.
const developmentPort = 1420;

export default defineConfig({
  plugins: [react()],
  clearScreen: false,
  server: {
    port: developmentPort,
    strictPort: true,
    watch: { ignored: ["**/src-tauri/**"] },
  },
  build: {
    target: "es2022",
  },
  test: {
    // Component tests switch to jsdom per file with "// @vitest-environment jsdom".
    environment: "node",
    setupFiles: ["./src/test/setupDom.ts"],
    exclude: ["node_modules", "dist", "e2e/**"],
    coverage: {
      provider: "v8",
      include: ["src/**"],
      exclude: ["src/test/**", "**/*.test.*", "src/main.tsx", "src/vite-env.d.ts"],
      reporter: ["text-summary", "html"],
      // Floors a bit below today's numbers (lines 85%, branches 78%): CI fails when coverage drops.
      thresholds: { lines: 80, statements: 78, branches: 72, functions: 70 },
    },
  },
});
