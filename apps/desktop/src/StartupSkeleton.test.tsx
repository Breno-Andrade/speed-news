// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import indexHtml from "../index.html?raw";
import { App } from "./App";
import { StartupSkeleton, STATIC_SKELETON_LABEL } from "./StartupSkeleton";
import { withStartupSkeleton } from "./startupSkeletonMarkup";
import stylesCss from "./styles.css?raw";
import { buildArticle, createFakeApiClient } from "./test/fakeApiClient";
import { renderWithProviders } from "./test/renderWithProviders";

const neverSettles = () => new Promise<never>(() => undefined);
const articles = [buildArticle({ id: "first", title: "Verstappen vence em Las Vegas" })];

// A freshly created popover paints index.html before any script runs: it must show the same
// skeleton React renders next, or the first open flickers between two layouts.
describe("startup skeleton", () => {
  it("is inlined in index.html exactly as React renders it", () => {
    // On failure: run `npm run generate:startup-skeleton` (apps/desktop) and commit index.html.
    expect(withStartupSkeleton(indexHtml) === indexHtml).toBe(true);
  });

  it("paints the first frame in the app's background colors", () => {
    const backgroundTokens = [...stylesCss.matchAll(/--color-background:\s*(#[0-9a-f]{6})/gi)].map((tokenMatch) => tokenMatch[1]);
    const inlineBackgrounds = [...indexHtml.matchAll(/background:\s*(#[0-9a-f]{6})/gi)].map((colorMatch) => colorMatch[1]);
    expect(backgroundTokens).toHaveLength(2); // light, then dark
    expect(inlineBackgrounds).toEqual(backgroundTokens);
  });

  it("shows no words that depend on the language, which is unknown before the scripts run", () => {
    const { container } = render(<StartupSkeleton label={STATIC_SKELETON_LABEL} />);
    expect(container.textContent).toBe("Speed News");
  });

  it("is continued by the app at once: card and news skeletons appear without a delay", async () => {
    const slowApiClient = createFakeApiClient();
    slowApiClient.fetchCurrentRaceWeekend.mockImplementation(neverSettles);
    slowApiClient.listArticles.mockImplementation(neverSettles);
    renderWithProviders(<App />, slowApiClient);
    // As soon as the settings are known, no further delay: a gap here is the blank band the
    // first open used to show.
    expect(await screen.findByRole("status", { name: "Carregando…" })).toBeInTheDocument();
    expect(screen.getByRole("status", { name: "Carregando notícias…" })).toBeInTheDocument();
    // The filters already sit where the skeleton drew them.
    expect(screen.getByRole("tab", { name: "Não lidas" })).toBeInTheDocument();
  });

  // The system language (English in jsdom) is only a guess: showing it until the saved one
  // arrived made the whole interface flip language right after the first paint (regression).
  it("keeps the skeleton until the saved language is known, then shows the app in that language", async () => {
    const pendingSettings: { resolve: () => void } = { resolve: () => undefined };
    const apiClient = createFakeApiClient({ articles });
    const savedSettings = await apiClient.fetchSettings();
    apiClient.fetchSettings.mockImplementation(
      () => new Promise((resolveSettings) => (pendingSettings.resolve = () => resolveSettings(savedSettings))),
    );
    renderWithProviders(<App />, apiClient);

    expect(screen.queryByRole("tab", { name: "Unread" })).toBeNull();
    expect(screen.queryByRole("tab", { name: "Não lidas" })).toBeNull();
    expect(document.querySelector(".race-card[aria-busy='true']")).not.toBeNull();

    await act(async () => pendingSettings.resolve());
    expect(await screen.findByRole("tab", { name: "Não lidas" })).toBeInTheDocument();
  });

  it("opens the app with default settings when they cannot be loaded", async () => {
    const apiClient = createFakeApiClient({ articles });
    apiClient.fetchSettings.mockRejectedValue(new Error("backend restarting"));
    renderWithProviders(<App />, apiClient);
    // Defaults follow the system language: English in jsdom.
    expect(await screen.findByRole("tab", { name: "Unread" })).toBeInTheDocument();
  });

  // The count arrives over the event stream long before the list: it used to pop into the
  // header alone while everything else was still a skeleton (regression).
  it("shows the unread count together with the first list, not ahead of it", async () => {
    const pendingArticles: { resolve: () => void } = { resolve: () => undefined };
    const apiClient = createFakeApiClient({ articles });
    apiClient.listArticles.mockImplementation(
      () => new Promise((resolveArticles) => (pendingArticles.resolve = () => resolveArticles(articles))),
    );
    renderWithProviders(<App />, apiClient);
    await screen.findByRole("tab", { name: "Não lidas" });

    const eventStream = apiClient.openEventStream.mock.results[0]?.value as { emitUnreadCount: (count: number) => void };
    act(() => eventStream.emitUnreadCount(3));
    expect(document.querySelector(".unread-pill")).toBeNull();
    expect(screen.queryByRole("button", { name: /Marcar todas como lidas/ })).toBeNull();

    await act(async () => pendingArticles.resolve());
    expect(await screen.findByText("3", { selector: ".unread-pill" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Marcar todas como lidas/ })).toBeInTheDocument();
  });

  it("does not fade the header in over the skeleton, only when switching views", async () => {
    const userInteraction = userEvent.setup();
    renderWithProviders(<App />, createFakeApiClient());
    await screen.findByRole("tab", { name: "Não lidas" });
    const appTitle = screen.getByRole("heading", { level: 1 });
    expect(appTitle.closest(".fade-in")).toBeNull();

    await userInteraction.click(await screen.findByRole("button", { name: "Configurações" }));
    expect((await screen.findByRole("button", { name: /Voltar/ })).closest(".fade-in")).not.toBeNull();
  });
});
