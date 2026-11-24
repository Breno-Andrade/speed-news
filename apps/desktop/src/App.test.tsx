// @vitest-environment jsdom
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { buildArticle, createFakeApiClient, type FakeApiClient } from "./test/fakeApiClient";
import { renderWithProviders } from "./test/renderWithProviders";

const articles = [
  buildArticle({ id: "first", title: "Verstappen vence em Las Vegas", imageUrl: "https://cdn.example/first.jpg" }),
  buildArticle({ id: "second", title: "Norris comenta disputa", sourceSlug: "the-race-f1", sourceName: "The Race", languageCode: "en" }),
  buildArticle({ id: "third", title: "Ferrari confirma atualização" }),
];

function emitUnreadCount(fakeApiClient: FakeApiClient, unreadCount: number) {
  const eventStream = fakeApiClient.openEventStream.mock.results[0]?.value as { emitUnreadCount: (count: number) => void };
  act(() => eventStream.emitUnreadCount(unreadCount));
}

describe("App", () => {
  it("shows the race card, the unread list and the count pushed by the backend", async () => {
    const { fakeApiClient } = renderWithProviders(<App />, createFakeApiClient({ articles }));

    expect(await screen.findByRole("heading", { name: "GP de Singapura" })).toBeInTheDocument();
    expect(await screen.findByText("Verstappen vence em Las Vegas")).toBeInTheDocument();
    // English article in a Portuguese UI carries a language tag.
    expect(within(screen.getByText("Norris comenta disputa").closest("button")!).getByText("EN")).toBeInTheDocument();

    emitUnreadCount(fakeApiClient, 3);
    expect(await screen.findByText("3", { selector: ".unread-pill" })).toBeInTheDocument();
    expect(document.title).toBe("(3) Speed News");
  });

  it("opens an article, marks it read and navigates with previous/next", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<App />, createFakeApiClient({ articles }));

    await userInteraction.click(await screen.findByText("Norris comenta disputa"));

    expect(fakeApiClient.markArticleRead).toHaveBeenCalledWith("second");
    const articleHeading = await screen.findByRole("heading", { level: 2, name: "Norris comenta disputa" });
    expect(articleHeading).toBeInTheDocument();
    expect(screen.getByText("Resumo publicado por The Race")).toBeInTheDocument();
    expect(screen.getByText("br.motorsport.com")).toBeInTheDocument();

    // Neighbours come from the list as it was when the article was opened.
    await userInteraction.click(screen.getByRole("button", { name: /Próxima/ }));
    expect(await screen.findByRole("heading", { level: 2, name: "Ferrari confirma atualização" })).toBeInTheDocument();
    expect(fakeApiClient.markArticleRead).toHaveBeenCalledWith("third");

    await userInteraction.click(screen.getByRole("button", { name: /Anterior/ }));
    expect(await screen.findByRole("heading", { level: 2, name: "Norris comenta disputa" })).toBeInTheDocument();
  });

  it("marks an article as unread again and copies its link", async () => {
    const userInteraction = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, "writeText");
    const { fakeApiClient } = renderWithProviders(<App />, createFakeApiClient({ articles }));

    await userInteraction.click(await screen.findByText("Ferrari confirma atualização"));
    await userInteraction.click(await screen.findByRole("button", { name: "Marcar como não lida" }));
    expect(fakeApiClient.markArticleUnread).toHaveBeenCalledWith("third");
    expect(screen.getByRole("button", { name: "Marcar como lida" })).toHaveAttribute("aria-pressed", "true");

    await userInteraction.click(screen.getByRole("button", { name: "Copiar link" }));
    expect(writeText).toHaveBeenCalledWith("https://br.motorsport.com/f1/news/third");
    expect(await screen.findByText("Link copiado")).toBeInTheDocument();
  });

  it("comes back to the same list (home stays mounted) after reading", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<App />, createFakeApiClient({ articles }));

    await userInteraction.click(await screen.findByText("Verstappen vence em Las Vegas"));
    const listLoadsBeforeBack = fakeApiClient.listArticles.mock.calls.length;
    await userInteraction.click(screen.getByRole("button", { name: "Voltar" }));

    expect(await screen.findByRole("heading", { name: "GP de Singapura" })).toBeVisible();
    // Returning does not remount home: no new race card request, no list reset to "loading".
    expect(fakeApiClient.fetchCurrentRaceWeekend).toHaveBeenCalledTimes(1);
    expect(fakeApiClient.listArticles.mock.calls.length).toBe(listLoadsBeforeBack);
  });

  it("marks everything read from the toolbar", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<App />, createFakeApiClient({ articles }));
    await screen.findByText("Verstappen vence em Las Vegas");
    emitUnreadCount(fakeApiClient, 3);

    await userInteraction.click(screen.getByRole("button", { name: /Marcar todas como lidas/ }));
    expect(fakeApiClient.markAllArticlesRead).toHaveBeenCalledOnce();
  });

  it("shows the news skeleton only when loading takes longer than 150 ms", async () => {
    let resolveArticles: (loadedArticles: typeof articles) => void = () => undefined;
    const fakeApiClient = createFakeApiClient({ articles });
    fakeApiClient.listArticles.mockImplementation(() => new Promise((resolve) => (resolveArticles = resolve)));

    renderWithProviders(<App />, fakeApiClient);
    expect(screen.queryByRole("status", { name: "Carregando notícias…" })).not.toBeInTheDocument();
    expect(await screen.findByRole("status", { name: "Carregando notícias…" })).toBeInTheDocument();

    act(() => resolveArticles(articles));
    expect(await screen.findByText("Verstappen vence em Las Vegas")).toBeInTheDocument();
    expect(screen.queryByRole("status", { name: "Carregando notícias…" })).not.toBeInTheDocument();
  });

  it("shows the empty state when everything is read", async () => {
    renderWithProviders(<App />, createFakeApiClient({ articles: [] }));
    expect(await screen.findByText("Tudo em dia! Nenhuma notícia nova.")).toBeInTheDocument();
  });

  it("explains a failed list load instead of staying blank", async () => {
    const fakeApiClient = createFakeApiClient({ articles });
    fakeApiClient.listArticles.mockRejectedValue(new Error("backend down"));
    renderWithProviders(<App />, fakeApiClient);
    await waitFor(() => expect(screen.getByText("Falha ao carregar. Tente atualizar.")).toBeInTheDocument());
  });
});
