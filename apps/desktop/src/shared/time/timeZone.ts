// Everything arrives from the backend in UTC. Conversion to the user's time zone happens only here, at display time.
import type { RaceSession, TimeZonePreference } from "../api/types";

export function detectSystemTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

export function resolveEffectiveTimeZone(timeZonePreference: TimeZonePreference, systemTimeZone: string): string {
  return timeZonePreference.mode === "manual" ? timeZonePreference.timeZoneId : systemTimeZone;
}

export function listSupportedTimeZones(): string[] {
  return Intl.supportedValuesOf("timeZone");
}

export function formatTimeOfDay(isoInstant: string, effectiveTimeZone: string, locale: string): string {
  return new Intl.DateTimeFormat(locale, { timeZone: effectiveTimeZone, hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).format(
    new Date(isoInstant),
  );
}

export type SessionDayGroup = {
  dayKey: string;
  /** Full label for screen readers: "Sexta-feira, 9 de outubro". */
  dayLabel: string;
  /** Compact column label: "SEX" / "FRI". */
  weekdayShort: string;
  /** "9". */
  dayOfMonth: string;
  sessions: RaceSession[];
};

/**
 * Groups sessions by calendar day *in the chosen time zone*: a session at 23:00 UTC belongs
 * to the next day for someone in Asia, and the heading must reflect that.
 */
export function groupSessionsByDay(raceSessions: RaceSession[], effectiveTimeZone: string, locale: string): SessionDayGroup[] {
  const dayKeyFormatter = new Intl.DateTimeFormat("en-CA", { timeZone: effectiveTimeZone, year: "numeric", month: "2-digit", day: "2-digit" });
  const dayLabelFormatter = new Intl.DateTimeFormat(locale, { timeZone: effectiveTimeZone, weekday: "long", day: "numeric", month: "long" });
  const weekdayShortFormatter = new Intl.DateTimeFormat(locale, { timeZone: effectiveTimeZone, weekday: "short" });
  const dayOfMonthFormatter = new Intl.DateTimeFormat(locale, { timeZone: effectiveTimeZone, day: "numeric" });

  const sortedSessions = [...raceSessions].sort((firstSession, secondSession) => firstSession.startsAt.localeCompare(secondSession.startsAt));
  const dayGroups: SessionDayGroup[] = [];
  for (const raceSession of sortedSessions) {
    const sessionInstant = new Date(raceSession.startsAt);
    const dayKey = dayKeyFormatter.format(sessionInstant);
    const lastGroup = dayGroups.at(-1);
    if (lastGroup && lastGroup.dayKey === dayKey) {
      lastGroup.sessions.push(raceSession);
    } else {
      dayGroups.push({
        dayKey,
        dayLabel: capitalizeFirstLetter(dayLabelFormatter.format(sessionInstant)),
        // pt-BR yields "sex." — drop the abbreviation dot, the column is uppercased by CSS.
        weekdayShort: weekdayShortFormatter.format(sessionInstant).replace(/\.$/, ""),
        dayOfMonth: dayOfMonthFormatter.format(sessionInstant),
        sessions: [raceSession],
      });
    }
  }
  return dayGroups;
}

/** "Horário de Brasília (GMT-3)" / "Central European Time (GMT+1)". */
export function describeTimeZone(effectiveTimeZone: string, locale: string, referenceInstant = new Date()): string {
  const readTimeZoneName = (timeZoneNameStyle: Intl.DateTimeFormatOptions["timeZoneName"]) =>
    new Intl.DateTimeFormat(locale, { timeZone: effectiveTimeZone, timeZoneName: timeZoneNameStyle })
      .formatToParts(referenceInstant)
      .find((datePart) => datePart.type === "timeZoneName")?.value ?? effectiveTimeZone;

  const genericName = readTimeZoneName("longGeneric");
  const offsetName = readTimeZoneName("shortOffset");
  return genericName === offsetName ? offsetName : `${genericName} (${offsetName})`;
}

/** "2d 4h", "3h 20min", "12min". */
export function formatCountdown(remainingMs: number): string {
  const totalMinutes = Math.max(0, Math.floor(remainingMs / 60_000));
  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  const minutes = totalMinutes % 60;
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}min`;
  return `${minutes}min`;
}

export function formatRelativeTime(isoInstant: string, locale: string, currentInstant = new Date()): string {
  const elapsedSeconds = Math.round((new Date(isoInstant).getTime() - currentInstant.getTime()) / 1000);
  const relativeFormatter = new Intl.RelativeTimeFormat(locale, { numeric: "auto", style: "short" });
  const unitSteps: [Intl.RelativeTimeFormatUnit, number][] = [
    ["day", 86_400],
    ["hour", 3_600],
    ["minute", 60],
  ];
  for (const [timeUnit, secondsPerUnit] of unitSteps) {
    if (Math.abs(elapsedSeconds) >= secondsPerUnit) {
      return relativeFormatter.format(Math.round(elapsedSeconds / secondsPerUnit), timeUnit);
    }
  }
  return relativeFormatter.format(0, "minute");
}

function capitalizeFirstLetter(sourceText: string): string {
  return sourceText.charAt(0).toLocaleUpperCase() + sourceText.slice(1);
}
