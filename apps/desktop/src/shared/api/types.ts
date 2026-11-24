// Mirrors apps/api/internal/domain. All dates are ISO-8601 strings in UTC.

export type Article = {
  id: string;
  seriesSlug: string;
  sourceSlug: string;
  sourceName: string;
  languageCode: string;
  title: string;
  summary: string;
  articleUrl: string;
  imageUrl?: string;
  publishedAt: string;
  fetchedAt: string;
  readAt?: string;
};

export type SessionKind = "practice_1" | "practice_2" | "practice_3" | "sprint_qualifying" | "sprint" | "qualifying" | "race";

export type RaceSession = {
  sessionKind: SessionKind;
  startsAt: string;
  startTimeConfirmed: boolean;
};

export type RaceEvent = {
  id: string;
  seriesSlug: string;
  season: number;
  round: number;
  eventName: string;
  circuitId: string;
  circuitName: string;
  locality: string;
  country: string;
  sessions: RaceSession[];
};

export type CircuitLayout = {
  circuitId: string;
  layoutId: string;
  svgPath: string;
  svgViewBox: string;
  attributionText: string;
  attributionUrl: string;
};

export type DriverResult = {
  position: number;
  driverCode: string;
  driverName: string;
  teamName: string;
  gridPosition: number;
  /** Winner: total time ("1:27:39.624"). Others: gap ("+0.639"), "+1 Lap" or a status, as published. */
  raceTime: string;
};

export type FastestLap = {
  driverCode: string;
  driverName: string;
  teamName: string;
  lapTime: string;
  lapNumber: number;
};

export type RaceResultSummary = {
  season: number;
  round: number;
  eventName: string;
  circuitId: string;
  circuitName: string;
  raceDate: string;
  laps: number;
  podium: DriverResult[];
  poleSitter?: DriverResult;
  fastestLap?: FastestLap;
};

export type CircuitProfile = {
  circuitId: string;
  circuitName: string;
  facts?: { lengthMeters: number; altitudeMeters: number; firstGrandPrixYear: number };
  recentWinners: { season: number; driverName: string; teamName: string }[];
  previousEdition?: RaceResultSummary;
};

export type SessionForecast = {
  sessionKind: SessionKind;
  startsAt: string;
  temperatureCelsius: number;
  precipitationProbability: number;
  windSpeedKmh: number;
  /** WMO weather code (0 clear, 61 rain, 95 thunderstorm...). */
  weatherCode: number;
};

export type WeekendForecast = {
  sessions: SessionForecast[];
  attributionText: string;
  attributionUrl: string;
};

export type RaceWeekendOverview = {
  raceEvent: RaceEvent;
  circuitLayout?: CircuitLayout;
  isRaceWeekendNow: boolean;
};

export type TimeZonePreference = { mode: "system" } | { mode: "manual"; timeZoneId: string };

/** "system" follows the operating system language. */
export type InterfaceLanguage = "system" | "pt-BR" | "en";

export type UserSettings = {
  timeZonePreference: TimeZonePreference;
  interfaceLanguage: InterfaceLanguage;
  /** Source languages to show ("pt", "en"). Empty means all languages. */
  newsLanguageCodes: string[];
  /** Hide the popover when clicking anywhere else. */
  closeOnFocusLoss: boolean;
};

export type NewsSource = {
  slug: string;
  displayName: string;
  seriesSlug: string;
  languageCode: string;
  homepageUrl: string;
};

export type ApiConnection = {
  baseUrl: string;
  token: string;
};
