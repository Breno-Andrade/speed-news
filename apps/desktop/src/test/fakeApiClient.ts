// In-memory stand-in for the Go backend, for component tests. Every method is a vi.fn spy, so
// tests can assert calls; state (articles, settings) behaves like the real API.
import { vi } from "vitest";
import type { ApiClient } from "../shared/api/apiClient";
import type { Article, CircuitProfile, NewsSource, RaceResultSummary, RaceWeekendOverview, UserSettings, WeekendForecast } from "../shared/api/types";

export function buildArticle(articleOverrides: Partial<Article> & Pick<Article, "id" | "title">): Article {
  return {
    seriesSlug: "f1",
    sourceSlug: "motorsport-br-f1",
    sourceName: "Motorsport.com Brasil",
    languageCode: "pt",
    summary: `Resumo de ${articleOverrides.title}`,
    articleUrl: `https://br.motorsport.com/f1/news/${articleOverrides.id}`,
    publishedAt: new Date(Date.now() - 60 * 60 * 1000).toISOString(),
    fetchedAt: new Date().toISOString(),
    ...articleOverrides,
  };
}

export const singaporeWeekend: RaceWeekendOverview = {
  isRaceWeekendNow: true,
  circuitLayout: {
    circuitId: "marina_bay",
    layoutId: "sg-2008",
    svgPath: "M40 960 L960 960 L960 40 Z",
    svgViewBox: "0 0 1000 1000",
    attributionText: "f1-circuits (MIT)",
    attributionUrl: "https://github.com/bacinger/f1-circuits",
  },
  raceEvent: {
    id: "f1-2026-17",
    seriesSlug: "f1",
    season: 2026,
    round: 17,
    eventName: "Singapore Grand Prix",
    circuitId: "marina_bay",
    circuitName: "Marina Bay Street Circuit",
    locality: "Marina Bay",
    country: "Singapore",
    sessions: [
      { sessionKind: "practice_1", startsAt: "2026-10-09T08:30:00Z", startTimeConfirmed: true },
      { sessionKind: "sprint_qualifying", startsAt: "2026-10-09T12:30:00Z", startTimeConfirmed: true },
      { sessionKind: "sprint", startsAt: "2026-10-10T09:00:00Z", startTimeConfirmed: true },
      { sessionKind: "qualifying", startsAt: "2026-10-10T13:00:00Z", startTimeConfirmed: true },
      { sessionKind: "race", startsAt: "2026-10-11T12:00:00Z", startTimeConfirmed: true },
    ],
  },
};

export const lasVegasResult: RaceResultSummary = {
  season: 2026,
  round: 21,
  eventName: "Las Vegas Grand Prix",
  circuitId: "vegas",
  circuitName: "Las Vegas Strip Street Circuit",
  raceDate: "2026-11-22",
  laps: 50,
  podium: [
    { position: 1, driverCode: "VER", driverName: "Max Verstappen", teamName: "Red Bull", gridPosition: 1, raceTime: "1:27:39.624" },
    { position: 2, driverCode: "NOR", driverName: "Lando Norris", teamName: "McLaren", gridPosition: 3, raceTime: "+0.639" },
    { position: 3, driverCode: "PIA", driverName: "Oscar Piastri", teamName: "McLaren", gridPosition: 17, raceTime: "+1.529" },
  ],
  poleSitter: { position: 1, driverCode: "VER", driverName: "Max Verstappen", teamName: "Red Bull", gridPosition: 1, raceTime: "1:27:39.624" },
  fastestLap: { driverCode: "LAW", driverName: "Liam Lawson", teamName: "Racing Bulls", lapTime: "1:35.591", lapNumber: 50 },
};

type FakeApiOptions = {
  articles?: Article[];
  settings?: Partial<UserSettings>;
  sources?: NewsSource[];
  raceWeekend?: RaceWeekendOverview | null;
  lastRace?: RaceResultSummary | null;
  circuitProfile?: CircuitProfile | null;
  forecast?: WeekendForecast | null;
  failingInsights?: boolean;
};

export function createFakeApiClient(fakeApiOptions: FakeApiOptions = {}) {
  let storedArticles = [...(fakeApiOptions.articles ?? [])];
  let storedSettings: UserSettings = {
    timeZonePreference: { mode: "manual", timeZoneId: "America/Sao_Paulo" },
    interfaceLanguage: "pt-BR",
    newsLanguageCodes: [],
    closeOnFocusLoss: true,
    ...fakeApiOptions.settings,
  };
  const insight = async <Payload>(payload: Payload) => {
    if (fakeApiOptions.failingInsights) throw new Error("source unavailable");
    return payload;
  };

  const fakeApiClient = {
    listArticles: vi.fn(async (statusFilter: "unread" | "all") =>
      statusFilter === "unread" ? storedArticles.filter((article) => !article.readAt) : storedArticles,
    ),
    markArticleRead: vi.fn(async (articleId: string) => {
      storedArticles = storedArticles.map((article) => (article.id === articleId ? { ...article, readAt: new Date().toISOString() } : article));
    }),
    markArticleUnread: vi.fn(async (articleId: string) => {
      storedArticles = storedArticles.map((article) => (article.id === articleId ? { ...article, readAt: undefined } : article));
    }),
    markAllArticlesRead: vi.fn(async () => {
      storedArticles = storedArticles.map((article) => ({ ...article, readAt: article.readAt ?? new Date().toISOString() }));
    }),
    requestRefresh: vi.fn(async () => undefined),
    fetchCurrentRaceWeekend: vi.fn(async () => (fakeApiOptions.raceWeekend === undefined ? singaporeWeekend : fakeApiOptions.raceWeekend)),
    fetchLastRace: vi.fn(async () => insight(fakeApiOptions.lastRace === undefined ? lasVegasResult : fakeApiOptions.lastRace)),
    fetchCircuitProfile: vi.fn(async () => insight(fakeApiOptions.circuitProfile ?? null)),
    fetchWeekendForecast: vi.fn(async () => insight(fakeApiOptions.forecast ?? null)),
    listSources: vi.fn(async () => fakeApiOptions.sources ?? [
      { slug: "motorsport-br-f1", displayName: "Motorsport.com Brasil", seriesSlug: "f1", languageCode: "pt", homepageUrl: "" },
      { slug: "the-race-f1", displayName: "The Race", seriesSlug: "f1", languageCode: "en", homepageUrl: "" },
    ]),
    fetchSettings: vi.fn(async () => storedSettings),
    saveSettings: vi.fn(async (userSettings: UserSettings) => {
      storedSettings = userSettings;
      return storedSettings;
    }),
    openEventStream: vi.fn(() => createFakeEventStream()),
  };
  return fakeApiClient;
}

/** Minimal EventSource: tests push unread counts with emitUnreadCount. */
export function createFakeEventStream() {
  const listeners = new Set<(messageEvent: MessageEvent<string>) => void>();
  const fakeEventStream = {
    addEventListener: (_eventName: string, listener: (messageEvent: MessageEvent<string>) => void) => listeners.add(listener),
    removeEventListener: (_eventName: string, listener: (messageEvent: MessageEvent<string>) => void) => listeners.delete(listener),
    close: vi.fn(),
    emitUnreadCount: (unreadCount: number) => {
      for (const listener of listeners) listener(new MessageEvent("unread-count", { data: JSON.stringify({ unreadCount }) }));
    },
  };
  return fakeEventStream;
}

export type FakeApiClient = ReturnType<typeof createFakeApiClient>;

export function asApiClient(fakeApiClient: FakeApiClient): ApiClient {
  return fakeApiClient as unknown as ApiClient;
}
