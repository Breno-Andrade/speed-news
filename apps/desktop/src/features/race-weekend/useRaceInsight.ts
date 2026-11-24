import { useEffect, useState } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import type { ApiClient } from "../../shared/api/apiClient";

export type InsightState<Payload> =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "loaded"; payload: Payload }
  | { status: "failed" };

/**
 * Loads a race card panel only once its tab is opened (some sources are slow), then refreshes it
 * whenever the popover regains focus. `loadInsight` must be a stable, module-level function.
 */
export function useRaceInsight<Payload>(loadInsight: (apiClient: ApiClient) => Promise<Payload>, isRequested: boolean): InsightState<Payload> {
  const apiClient = useApiClient();
  const [insightState, setInsightState] = useState<InsightState<Payload>>({ status: "idle" });

  useEffect(() => {
    if (!isRequested) return;
    let isCancelled = false;
    const refreshInsight = () => {
      setInsightState((previousState) => (previousState.status === "loaded" ? previousState : { status: "loading" }));
      loadInsight(apiClient)
        .then((payload) => {
          if (!isCancelled) setInsightState({ status: "loaded", payload });
        })
        .catch(() => {
          // Keep what is already on screen; only show the error when there is nothing to show.
          if (!isCancelled) setInsightState((previousState) => (previousState.status === "loaded" ? previousState : { status: "failed" }));
        });
    };

    refreshInsight();
    window.addEventListener("focus", refreshInsight);
    return () => {
      isCancelled = true;
      window.removeEventListener("focus", refreshInsight);
    };
  }, [apiClient, loadInsight, isRequested]);

  return insightState;
}
