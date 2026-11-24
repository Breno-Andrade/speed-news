import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, createApiClient } from "./apiClient";

const apiConnection = { baseUrl: "http://127.0.0.1:4317", token: "secret-token" };

function mockFetch(responseStatus: number, responseBody?: unknown) {
  const fetchMock = vi.fn<(requestUrl: string, requestInit?: RequestInit) => Promise<Response>>(
    async () => new Response(responseBody === undefined ? null : JSON.stringify(responseBody), { status: responseStatus }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => vi.unstubAllGlobals());

describe("createApiClient", () => {
  it("sends the token and builds article URLs", async () => {
    const fetchMock = mockFetch(200, { articles: [{ id: "first" }] });
    const loadedArticles = await createApiClient(apiConnection).listArticles("unread", 25);

    expect(loadedArticles).toEqual([{ id: "first" }]);
    const [requestUrl, requestInit] = fetchMock.mock.calls[0]!;
    expect(requestUrl).toBe("http://127.0.0.1:4317/articles?status=unread&limit=25");
    expect((requestInit?.headers as Record<string, string>).Authorization).toBe("Bearer secret-token");
  });

  it("escapes article ids in paths", async () => {
    const fetchMock = mockFetch(204);
    await createApiClient(apiConnection).markArticleRead("a/b c");
    expect(fetchMock.mock.calls[0]![0]).toBe("http://127.0.0.1:4317/articles/a%2Fb%20c/read");
    expect(fetchMock.mock.calls[0]![1]?.method).toBe("POST");
  });

  it("turns error answers into ApiError with the backend message", async () => {
    mockFetch(400, { error: "limit must be between 1 and 200" });
    const failedRequest = createApiClient(apiConnection).listArticles("all", 500);
    await expect(failedRequest).rejects.toBeInstanceOf(ApiError);
    await expect(failedRequest).rejects.toMatchObject({ statusCode: 400, message: "limit must be between 1 and 200" });
  });

  it("accepts empty success answers (204/202)", async () => {
    mockFetch(202);
    await expect(createApiClient(apiConnection).requestRefresh()).resolves.toBeUndefined();
  });

  it("unwraps the race card payloads", async () => {
    mockFetch(200, { forecast: null });
    await expect(createApiClient(apiConnection).fetchWeekendForecast()).resolves.toBeNull();
  });

  it("opens the event stream with the token in the query (EventSource cannot send headers)", () => {
    const eventSourceMock = vi.fn();
    vi.stubGlobal("EventSource", eventSourceMock);
    createApiClient({ ...apiConnection, token: "a b" }).openEventStream();
    expect(eventSourceMock).toHaveBeenCalledWith("http://127.0.0.1:4317/events?token=a%20b");
  });
});
