import type { DriverResult } from "../../shared/api/types";
import type { Messages, WeatherCondition } from "../../shared/i18n/messages";

const gapInSecondsPattern = /^\+(\d+(?:\.\d+)?)$/;
const lapsBehindPattern = /^\+(\d+) Laps?$/;

/** Winner keeps the total time; "+0.639" → "+0,639 s" (pt); "+1 Lap" → "+1 volta"; statuses stay as published. */
export function formatRaceTime(driverResult: DriverResult, locale: string, messages: Messages): string {
  if (driverResult.position === 1) return driverResult.raceTime;

  const gapMatch = gapInSecondsPattern.exec(driverResult.raceTime);
  if (gapMatch?.[1]) {
    const gapSeconds = Number(gapMatch[1]);
    return `+${new Intl.NumberFormat(locale, { minimumFractionDigits: 3, maximumFractionDigits: 3 }).format(gapSeconds)} s`;
  }
  const lapsMatch = lapsBehindPattern.exec(driverResult.raceTime);
  if (lapsMatch?.[1]) return messages.lapsBehind(Number(lapsMatch[1]));
  return driverResult.raceTime;
}

export function formatMeters(distanceMeters: number, locale: string): string {
  return `${new Intl.NumberFormat(locale).format(distanceMeters)} m`;
}

/** Race distance from track length and number of laps: 5380 m × 57 → "306,7 km". */
export function formatRaceDistance(lengthMeters: number, raceLaps: number, locale: string): string {
  const distanceKilometers = (lengthMeters * raceLaps) / 1000;
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(distanceKilometers)} km`;
}

export type WeatherDescription = { weatherCondition: WeatherCondition; weatherEmoji: string };

/** WMO weather codes as used by Open-Meteo. */
export function describeWeatherCode(weatherCode: number): WeatherDescription {
  if (weatherCode === 0) return { weatherCondition: "clear", weatherEmoji: "☀️" };
  if (weatherCode === 1) return { weatherCondition: "mainlyClear", weatherEmoji: "🌤️" };
  if (weatherCode === 2) return { weatherCondition: "partlyCloudy", weatherEmoji: "⛅" };
  if (weatherCode === 3) return { weatherCondition: "cloudy", weatherEmoji: "☁️" };
  if (weatherCode === 45 || weatherCode === 48) return { weatherCondition: "fog", weatherEmoji: "🌫️" };
  if (weatherCode >= 51 && weatherCode <= 57) return { weatherCondition: "drizzle", weatherEmoji: "🌦️" };
  if (weatherCode >= 61 && weatherCode <= 67) return { weatherCondition: "rain", weatherEmoji: "🌧️" };
  if ((weatherCode >= 71 && weatherCode <= 77) || weatherCode === 85 || weatherCode === 86) return { weatherCondition: "snow", weatherEmoji: "❄️" };
  if (weatherCode >= 80 && weatherCode <= 82) return { weatherCondition: "showers", weatherEmoji: "🌦️" };
  if (weatherCode >= 95) return { weatherCondition: "thunderstorm", weatherEmoji: "⛈️" };
  return { weatherCondition: "cloudy", weatherEmoji: "☁️" };
}
