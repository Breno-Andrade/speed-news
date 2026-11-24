import { useEffect, useState } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import type { RaceWeekendOverview } from "../../shared/api/types";

const refreshIntervalMs = 10 * 60_000;

type RaceWeekendState = { status: "loading" } | { status: "loaded"; raceWeekend: RaceWeekendOverview | null } | { status: "failed" };

/** Loads the current/next race; refreshes periodically and when the popover regains focus. */
export function useRaceWeekend(): RaceWeekendState {
  const apiClient = useApiClient();
  const [raceWeekendState, setRaceWeekendState] = useState<RaceWeekendState>({ status: "loading" });

  useEffect(() => {
    let isCancelled = false;
    const loadRaceWeekend = () => {
      apiClient
        .fetchCurrentRaceWeekend()
        .then((raceWeekend) => {
          if (!isCancelled) setRaceWeekendState({ status: "loaded", raceWeekend });
        })
        .catch(() => {
          if (!isCancelled) setRaceWeekendState((previousState) => (previousState.status === "loaded" ? previousState : { status: "failed" }));
        });
    };

    loadRaceWeekend();
    const refreshTimer = window.setInterval(loadRaceWeekend, refreshIntervalMs);
    window.addEventListener("focus", loadRaceWeekend);
    return () => {
      isCancelled = true;
      window.clearInterval(refreshTimer);
      window.removeEventListener("focus", loadRaceWeekend);
    };
  }, [apiClient]);

  return raceWeekendState;
}

/** Re-renders every tickIntervalMs so countdowns stay accurate. */
export function useCurrentInstant(tickIntervalMs = 30_000): Date {
  const [currentInstant, setCurrentInstant] = useState(() => new Date());
  useEffect(() => {
    const tickTimer = window.setInterval(() => setCurrentInstant(new Date()), tickIntervalMs);
    return () => window.clearInterval(tickTimer);
  }, [tickIntervalMs]);
  return currentInstant;
}
