import { describe, expect, it } from "vitest";
import type { RaceSession } from "../../shared/api/types";
import { classifySessions } from "./sessionStatus";

const qualifying: RaceSession = { sessionKind: "qualifying", startsAt: "2026-10-10T13:00:00Z", startTimeConfirmed: true };
const race: RaceSession = { sessionKind: "race", startsAt: "2026-10-11T12:00:00Z", startTimeConfirmed: true };
const firstPractice: RaceSession = { sessionKind: "practice_1", startsAt: "2026-10-09T08:30:00Z", startTimeConfirmed: true };
// Deliberately out of order: classification must not depend on the input order.
const weekendSessions = [race, firstPractice, qualifying];

function statusesAt(isoInstant: string) {
  const statusBySession = classifySessions(weekendSessions, new Date(isoInstant));
  return [firstPractice, qualifying, race].map((raceSession) => statusBySession.get(raceSession));
}

describe("classifySessions", () => {
  it("marks only the first future session as next", () => {
    expect(statusesAt("2026-10-05T00:00:00Z")).toEqual(["next", "upcoming", "upcoming"]);
  });

  it("is live from the start until the estimated end of the session", () => {
    expect(statusesAt("2026-10-09T08:30:00Z")).toEqual(["live", "next", "upcoming"]);
    expect(statusesAt("2026-10-09T09:29:00Z")).toEqual(["live", "next", "upcoming"]);
    expect(statusesAt("2026-10-09T09:31:00Z")).toEqual(["finished", "next", "upcoming"]);
  });

  it("gives the race two hours before calling it finished", () => {
    expect(statusesAt("2026-10-11T13:59:00Z")).toEqual(["finished", "finished", "live"]);
    expect(statusesAt("2026-10-11T14:01:00Z")).toEqual(["finished", "finished", "finished"]);
  });
});
