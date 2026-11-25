// End-to-end: the real UI (Vite) + the real Go backend + a fixture server standing in for every
// external source. Fixture world: "GP de Singapura" two days ahead (race week), last race in
// Las Vegas, 3 unread English stories + 2 Portuguese ones (+ one old story already read).
import { expect, test, type Page } from "@playwright/test";
import { restoreInitialReadState, publishFixtureStory, resetStack, setFixtureOutage, unreadCountFromApi } from "./stackControl";

test.beforeEach(async ({ request }) => {
  await resetStack(request);
  await restoreInitialReadState(request);
});

async function openHome(page: Page) {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "GP de Singapura" })).toBeVisible();
  await expect(page.locator(".news-item").first()).toBeVisible();
}

test("home shows the race week card, the unread news and their count", async ({ page }) => {
  await openHome(page);

  await expect(page.getByText(/Semana de corrida · Etapa 2/)).toBeVisible();
  await expect(page.locator(".circuit-map svg path").first()).toBeVisible();
  await expect(page.locator(".unread-pill")).toHaveText("5");
  await expect(page.getByText("Fixture English headline one")).toBeVisible();
  await expect(page.getByText("Manchete de teste em português um")).toBeVisible();
  await expect(page.getByText("Old English story")).toBeHidden();
});

test("reading an article updates the count, and previous/next walk the list", async ({ page, request }) => {
  await openHome(page);
  await page.getByText("Fixture English headline one").click();

  await expect(page.getByRole("heading", { level: 2, name: "Fixture English headline one" })).toBeVisible();
  await expect(page.getByText("Resumo publicado por Fixture Racing News")).toBeVisible();
  await expect.poll(() => unreadCountFromApi(request)).toBe(4);

  await page.getByRole("button", { name: /Próxima/ }).click();
  await expect(page.getByRole("heading", { level: 2, name: "Manchete de teste em português um" })).toBeVisible();
  await expect.poll(() => unreadCountFromApi(request)).toBe(3);

  await page.getByRole("button", { name: "Marcar como não lida" }).click();
  await expect.poll(() => unreadCountFromApi(request)).toBe(4);

  await page.getByRole("button", { name: "Voltar" }).click();
  await expect(page.locator(".unread-pill")).toHaveText("4");
});

test("the race card tabs show last GP, circuit and weather", async ({ page }) => {
  await openHome(page);
  const raceCard = page.locator(".race-card");

  await raceCard.getByRole("tab", { name: "Último GP" }).click();
  await expect(raceCard.getByText("GP de Las Vegas")).toBeVisible();
  await expect(raceCard.getByText("+0,639 s")).toBeVisible();

  await raceCard.getByRole("tab", { name: "Pista" }).click();
  await expect(raceCard.getByText("4.928 m")).toBeVisible();
  await expect(raceCard.getByText("2008")).toBeVisible();

  await raceCard.getByRole("tab", { name: "Clima" }).click();
  await expect(raceCard.getByText("28°C").first()).toBeVisible();
  await expect(raceCard.getByLabel("Chuva").first()).toBeVisible();
});

test("choosing only Portuguese news filters the list and the count", async ({ page, request }) => {
  await openHome(page);
  await page.getByRole("button", { name: "Configurações" }).click();
  await page.getByRole("switch", { name: /^Inglês/ }).uncheck();
  await expect(page.getByRole("switch", { name: /^Português/ })).toBeDisabled();
  await page.getByRole("button", { name: "Voltar" }).click();

  await expect(page.locator(".unread-pill")).toHaveText("2");
  await expect(page.getByText("Fixture English headline one")).toBeHidden();
  await expect(page.getByText("Manchete de teste em português um")).toBeVisible();
  expect(await unreadCountFromApi(request)).toBe(2);
});

test("switching the interface to English translates the whole app", async ({ page }) => {
  await openHome(page);
  await page.getByRole("button", { name: "Configurações" }).click();
  // Users click the visible segment label; the native radio underneath is visually hidden.
  await page.getByRole("radiogroup", { name: "Idioma do app" }).getByText("English").click();
  await page.getByRole("button", { name: "Back" }).click();

  await expect(page.getByRole("heading", { name: "Singapore GP" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Unread" })).toBeVisible();
  await expect(page.getByText(/Race weekend · Round 2/)).toBeVisible();
});

test("a story published while the app is open shows up after refreshing", async ({ page, request }) => {
  await openHome(page);
  await publishFixtureStory(request, "Notícia urgente de teste", "pt");

  await page.getByRole("button", { name: "Atualizar" }).click();
  await expect(page.getByText("Notícia urgente de teste")).toBeVisible({ timeout: 10_000 });
  await expect(page.locator(".unread-pill")).toHaveText("6");
});

test("a feed outage keeps the stored news on screen", async ({ page, request }) => {
  await openHome(page);
  await setFixtureOutage(request, "rss", true);

  await page.getByRole("button", { name: "Atualizar" }).click();
  await page.waitForTimeout(3_000);
  await expect(page.getByText("Fixture English headline one")).toBeVisible();
  await expect(page.getByText("Falha ao carregar")).toBeHidden();
});

test("header icons are vertically aligned (regression)", async ({ page }) => {
  await openHome(page);
  const iconTops = await page.locator(".app-header__actions .icon-button svg").evaluateAll((iconElements) =>
    iconElements.map((iconElement) => Math.round(iconElement.getBoundingClientRect().top)),
  );
  expect(new Set(iconTops).size).toBe(1);
});

test("nothing overflows the popover width", async ({ page }) => {
  await openHome(page);
  for (const tabName of ["Programação", "Último GP", "Pista", "Clima"]) {
    await page.locator(".race-card").getByRole("tab", { name: tabName }).click();
    await page.waitForTimeout(300);
    const horizontalOverflow = await page.locator(".app-view").first().evaluate((viewElement) => viewElement.scrollWidth - viewElement.clientWidth);
    expect(horizontalOverflow, `tab ${tabName}`).toBeLessThanOrEqual(0);
  }
});
