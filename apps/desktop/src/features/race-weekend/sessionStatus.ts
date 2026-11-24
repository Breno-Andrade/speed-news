import type { RaceSession, SessionKind } from "../../shared/api/types";

// Approximate durations, only used to show "now" while a session is running.
const estimatedDurationMinutesBySession: Record<SessionKind, number> = {
  practice_1: 60,
  practice_2: 60,
  practice_3: 60,
  sprint_qualifying: 45,
  sprint: 60,
  qualifying: 60,
  race: 120,
};

export type SessionStatus = "finished" | "live" | "next" | "upcoming";

/** Finished, live, the next one to start (gets the countdown) or later. */
export function classifySessions(raceSessions: RaceSession[], currentInstant: Date): Map<RaceSession, SessionStatus> {
  const currentTimeMs = currentInstant.getTime();
  const statusBySession = new Map<RaceSession, SessionStatus>();
  let nextSessionAssigned = false;

  const sortedSessions = [...raceSessions].sort((firstSession, secondSession) => firstSession.startsAt.localeCompare(secondSession.startsAt));
  for (const raceSession of sortedSessions) {
    const startsAtMs = new Date(raceSession.startsAt).getTime();
    const endsAtMs = startsAtMs + estimatedDurationMinutesBySession[raceSession.sessionKind] * 60_000;
    if (currentTimeMs >= endsAtMs) {
      statusBySession.set(raceSession, "finished");
    } else if (currentTimeMs >= startsAtMs) {
      statusBySession.set(raceSession, "live");
    } else if (!nextSessionAssigned) {
      statusBySession.set(raceSession, "next");
      nextSessionAssigned = true;
    } else {
      statusBySession.set(raceSession, "upcoming");
    }
  }
  return statusBySession;
}
