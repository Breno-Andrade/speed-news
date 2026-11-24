import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { createApiClient, type ApiClient } from "./apiClient";
import { resolveApiConnection } from "./apiConnection";

const ApiClientContext = createContext<ApiClient | null>(null);

type ConnectionState = { status: "connecting" } | { status: "ready"; apiClient: ApiClient } | { status: "failed"; errorMessage: string };

export function ApiProvider({ children, connectingFallback }: { children: ReactNode; connectingFallback: (errorMessage?: string) => ReactNode }) {
  const [connectionState, setConnectionState] = useState<ConnectionState>({ status: "connecting" });

  useEffect(() => {
    let isCancelled = false;
    resolveApiConnection()
      .then((apiConnection) => {
        if (!isCancelled) setConnectionState({ status: "ready", apiClient: createApiClient(apiConnection) });
      })
      .catch((connectionError: unknown) => {
        if (!isCancelled) setConnectionState({ status: "failed", errorMessage: String(connectionError) });
      });
    return () => {
      isCancelled = true;
    };
  }, []);

  if (connectionState.status === "connecting") return connectingFallback();
  if (connectionState.status === "failed") return connectingFallback(connectionState.errorMessage);
  return <ApiClientProvider apiClient={connectionState.apiClient}>{children}</ApiClientProvider>;
}

/** Provides an already built client. Used by ApiProvider and by component tests (fake client). */
export function ApiClientProvider({ apiClient, children }: { apiClient: ApiClient; children: ReactNode }) {
  return <ApiClientContext.Provider value={apiClient}>{children}</ApiClientContext.Provider>;
}

export function useApiClient(): ApiClient {
  const apiClient = useContext(ApiClientContext);
  if (!apiClient) throw new Error("useApiClient must be used inside <ApiProvider>");
  return apiClient;
}
