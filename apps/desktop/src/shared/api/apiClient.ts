import type {
  ApiConnection,
  Article,
  CircuitProfile,
  NewsSource,
  RaceResultSummary,
  RaceWeekendOverview,
  UserSettings,
  WeekendForecast,
} from "./types";

export class ApiError extends Error {
  constructor(
    message: string,
    readonly statusCode: number,
  ) {
    super(message);
  }
}

export type ArticleStatusFilter = "unread" | "all";

/** Typed client for apps/api. */
export function createApiClient(apiConnection: ApiConnection) {
  async function requestJson<ResponseBody>(path: string, requestInit: RequestInit = {}): Promise<ResponseBody> {
    const httpResponse = await fetch(`${apiConnection.baseUrl}${path}`, {
      ...requestInit,
      headers: {
        Authorization: `Bearer ${apiConnection.token}`,
        "Content-Type": "application/json",
        ...requestInit.headers,
      },
    });
    if (!httpResponse.ok) {
      const errorBody = (await httpResponse.json().catch(() => ({}))) as { error?: string };
      throw new ApiError(errorBody.error ?? httpResponse.statusText, httpResponse.status);
    }
    if (httpResponse.status === 204 || httpResponse.status === 202) {
      return undefined as ResponseBody;
    }
    return (await httpResponse.json()) as ResponseBody;
  }

  return {
    async listArticles(statusFilter: ArticleStatusFilter, articleLimit = 60): Promise<Article[]> {
      const responseBody = await requestJson<{ articles: Article[] }>(`/articles?status=${statusFilter}&limit=${articleLimit}`);
      return responseBody.articles;
    },
    markArticleRead(articleId: string): Promise<void> {
      return requestJson(`/articles/${encodeURIComponent(articleId)}/read`, { method: "POST" });
    },
    markArticleUnread(articleId: string): Promise<void> {
      return requestJson(`/articles/${encodeURIComponent(articleId)}/unread`, { method: "POST" });
    },
    markAllArticlesRead(): Promise<void> {
      return requestJson("/articles/read-all", { method: "POST" });
    },
    requestRefresh(): Promise<void> {
      return requestJson("/articles/refresh", { method: "POST" });
    },
    async fetchCurrentRaceWeekend(): Promise<RaceWeekendOverview | null> {
      const responseBody = await requestJson<{ raceWeekend: RaceWeekendOverview | null }>("/race-weekend/current");
      return responseBody.raceWeekend;
    },
    async fetchLastRace(): Promise<RaceResultSummary | null> {
      return (await requestJson<{ lastRace: RaceResultSummary | null }>("/race-weekend/last-race")).lastRace;
    },
    async fetchCircuitProfile(): Promise<CircuitProfile | null> {
      return (await requestJson<{ circuit: CircuitProfile | null }>("/race-weekend/circuit")).circuit;
    },
    /** null until the race weekend is less than a week away. */
    async fetchWeekendForecast(): Promise<WeekendForecast | null> {
      return (await requestJson<{ forecast: WeekendForecast | null }>("/race-weekend/forecast")).forecast;
    },
    async listSources(): Promise<NewsSource[]> {
      const responseBody = await requestJson<{ sources: NewsSource[] }>("/sources");
      return responseBody.sources;
    },
    fetchSettings(): Promise<UserSettings> {
      return requestJson("/settings");
    },
    saveSettings(userSettings: UserSettings): Promise<UserSettings> {
      return requestJson("/settings", { method: "PUT", body: JSON.stringify(userSettings) });
    },
    /** EventSource cannot send headers, so the token goes in the query string (loopback only). */
    openEventStream(): EventSource {
      return new EventSource(`${apiConnection.baseUrl}/events?token=${encodeURIComponent(apiConnection.token)}`);
    },
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;
