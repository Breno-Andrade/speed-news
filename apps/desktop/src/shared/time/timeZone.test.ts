import { describe, expect, it } from "vitest";
import type { RaceSession } from "../api/types";
import { describeTimeZone, formatCountdown, formatTimeOfDay, groupSessionsByDay, resolveEffectiveTimeZone } from "./timeZone";

// Singapore 2026 as returned by Jolpica (UTC).
const singaporeSessions: RaceSession[] = [
  { sessionKind: "race", startsAt: "2026-10-11T12:00:00Z", startTimeConfirmed: true },
  { sessionKind: "practice_1", startsAt: "2026-10-09T08:30:00Z", startTimeConfirmed: true },
  { sessionKind: "sprint_qualifying", startsAt: "2026-10-09T12:30:00Z", startTimeConfirmed: true },
  { sessionKind: "sprint", startsAt: "2026-10-10T09:00:00Z", startTimeConfirmed: true },
  { sessionKind: "qualifying", startsAt: "2026-10-10T13:00:00Z", startTimeConfirmed: true },
];

describe("resolveEffectiveTimeZone", () => {
  it("follows the computer in system mode", () => {
    expect(resolveEffectiveTimeZone({ mode: "system" }, "America/Sao_Paulo")).toBe("America/Sao_Paulo");
  });
  it("uses the chosen zone in manual mode", () => {
    expect(resolveEffectiveTimeZone({ mode: "manual", timeZoneId: "Europe/Lisbon" }, "America/Sao_Paulo")).toBe("Europe/Lisbon");
  });
});

describe("groupSessionsByDay", () => {
  it("reproduces the Brazilian broadcast schedule for Singapore", () => {
    const dayGroups = groupSessionsByDay(singaporeSessions, "America/Sao_Paulo", "pt-BR");
    const renderedSchedule = dayGroups.map((dayGroup) => ({
      dayLabel: dayGroup.dayLabel,
      sessions: dayGroup.sessions.map((raceSession) => `${formatTimeOfDay(raceSession.startsAt, "America/Sao_Paulo", "pt-BR")} ${raceSession.sessionKind}`),
    }));
    expect(renderedSchedule).toEqual([
      { dayLabel: "Sexta-feira, 9 de outubro", sessions: ["05:30 practice_1", "09:30 sprint_qualifying"] },
      { dayLabel: "Sábado, 10 de outubro", sessions: ["06:00 sprint", "10:00 qualifying"] },
      { dayLabel: "Domingo, 11 de outubro", sessions: ["09:00 race"] },
    ]);
  });

  it("moves sessions to another day when the time zone crosses midnight", () => {
    // New Zealand in October is NZDT (UTC+13): sprint qualifying slides to Saturday, qualifying to Sunday
    // and the race (12:00 UTC Sunday) to 01:00 on Monday.
    const dayGroups = groupSessionsByDay(singaporeSessions, "Pacific/Auckland", "en-US");
    expect(dayGroups.map((dayGroup) => [dayGroup.dayKey, dayGroup.sessions.map((raceSession) => raceSession.sessionKind)])).toEqual([
      ["2026-10-09", ["practice_1"]],
      ["2026-10-10", ["sprint_qualifying", "sprint"]],
      ["2026-10-11", ["qualifying"]],
      ["2026-10-12", ["race"]],
    ]);
  });
});

describe("describeTimeZone", () => {
  it("shows a readable name and the offset", () => {
    expect(describeTimeZone("America/Sao_Paulo", "pt-BR", new Date("2026-10-09T12:00:00Z"))).toBe("Horário Padrão de Brasília (GMT-3)");
  });
});

describe("formatCountdown", () => {
  it.each([
    [(2 * 24 + 4) * 3_600_000 + 59_000, "2d 4h"],
    [(3 * 60 + 20) * 60_000, "3h 20min"],
    [12 * 60_000, "12min"],
    [-5_000, "0min"],
  ])("formats %d ms as %s", (remainingMs, expectedText) => {
    expect(formatCountdown(remainingMs)).toBe(expectedText);
  });
});
