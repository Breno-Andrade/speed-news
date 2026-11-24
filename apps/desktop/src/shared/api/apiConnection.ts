import type { ApiConnection } from "./types";
import { fetchSidecarConnection, isRunningInTauri } from "../platform/tauriBridge";

const sidecarPollIntervalMs = 250;
const sidecarStartupTimeoutMs = 15_000;

/**
 * Inside Tauri the shell starts the Go sidecar on a random port with a random token and hands them over.
 * In the browser (npm run dev:web) the values come from .env.development.
 */
export async function resolveApiConnection(): Promise<ApiConnection> {
  if (!isRunningInTauri()) {
    return {
      baseUrl: import.meta.env.VITE_API_BASE_URL ?? "http://127.0.0.1:4317",
      token: import.meta.env.VITE_API_TOKEN ?? "",
    };
  }

  const startedAt = Date.now();
  while (Date.now() - startedAt < sidecarStartupTimeoutMs) {
    const sidecarConnection = await fetchSidecarConnection();
    if (sidecarConnection) {
      return sidecarConnection;
    }
    await new Promise((resolveDelay) => setTimeout(resolveDelay, sidecarPollIntervalMs));
  }
  throw new Error("O serviço de notícias não iniciou a tempo.");
}
