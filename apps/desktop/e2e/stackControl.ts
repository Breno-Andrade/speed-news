import type { APIRequestContext } from "@playwright/test";
import { E2E_API_TOKEN, E2E_BACKEND_PORT, E2E_FIXTURE_PORT } from "./stackPorts.mjs";

const backendUrl = `http://127.0.0.1:${E2E_BACKEND_PORT}`;
const fixtureUrl = `http://127.0.0.1:${E2E_FIXTURE_PORT}`;
const authorizationHeaders = { Authorization: `Bearer ${E2E_API_TOKEN}` };

export const defaultSettings = {
  timeZonePreference: { mode: "system" },
  interfaceLanguage: "system",
  newsLanguageCodes: [],
  closeOnFocusLoss: true,
};

/** Every test starts from the same world: default settings and the initial fixture feeds. */
export async function resetStack(requestContext: APIRequestContext) {
  await requestContext.post(`${fixtureUrl}/control/reset`);
  await requestContext.put(`${backendUrl}/settings`, { headers: authorizationHeaders, data: defaultSettings });
}

export async function unreadCountFromApi(requestContext: APIRequestContext): Promise<number> {
  const unreadResponse = await requestContext.get(`${backendUrl}/articles/unread-count`, { headers: authorizationHeaders });
  return ((await unreadResponse.json()) as { unreadCount: number }).unreadCount;
}

/** The fixture stories that start unread (see apps/api/internal/fixtures). */
const initiallyUnreadTitles = new Set([
  "Fixture English headline one",
  "Fixture English headline two",
  "Fixture English headline three",
  "Manchete de teste em português um",
  "Manchete de teste em português dois",
]);

/**
 * Restores the initial read state: exactly the 5 base stories unread. Everything else (the old
 * story, stories published by earlier tests and still stored) is marked read, so no test depends
 * on which tests ran before it.
 */
export async function restoreInitialReadState(requestContext: APIRequestContext) {
  const articlesResponse = await requestContext.get(`${backendUrl}/articles?status=all&limit=200`, { headers: authorizationHeaders });
  const { articles } = (await articlesResponse.json()) as { articles: { id: string; title: string }[] };
  for (const article of articles) {
    const readStateAction = initiallyUnreadTitles.has(article.title) ? "unread" : "read";
    await requestContext.post(`${backendUrl}/articles/${article.id}/${readStateAction}`, { headers: authorizationHeaders });
  }
}

export async function publishFixtureStory(requestContext: APIRequestContext, storyTitle: string, languageCode: "pt" | "en") {
  await requestContext.post(`${fixtureUrl}/control/publish?language=${languageCode}&title=${encodeURIComponent(storyTitle)}`);
}

export async function setFixtureOutage(requestContext: APIRequestContext, sourceName: "rss" | "jolpica" | "circuits" | "openmeteo", isDown: boolean) {
  await requestContext.post(`${fixtureUrl}/control/outage?source=${sourceName}&down=${isDown}`);
}
