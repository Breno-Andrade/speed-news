// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import indexHtml from "../index.html?raw";
import { App } from "./App";
import { StartupSkeleton, STATIC_SKELETON_LABEL } from "./StartupSkeleton";
import { withStartupSkeleton } from "./startupSkeletonMarkup";
import stylesCss from "./styles.css?raw";
import { createFakeApiClient } from "./test/fakeApiClient";
import { renderWithProviders } from "./test/renderWithProviders";

const neverSettles = () => new Promise<never>(() => undefined);

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

  it("is continued by the app at once: card and news skeletons appear without a delay", () => {
    const slowApiClient = createFakeApiClient();
    slowApiClient.fetchCurrentRaceWeekend.mockImplementation(neverSettles);
    slowApiClient.listArticles.mockImplementation(neverSettles);
    renderWithProviders(<App />, slowApiClient);
    // Synchronously, no timers: a gap here is the blank band the first open used to show.
    // (Before the saved settings arrive the texts follow the system language: English in jsdom.)
    expect(screen.getByRole("status", { name: "Loading…" })).toBeInTheDocument();
    expect(screen.getByRole("status", { name: "Loading articles…" })).toBeInTheDocument();
    // The filters already sit where the skeleton drew them.
    expect(screen.getByRole("tab", { name: "Unread" })).toBeInTheDocument();
  });

  it("does not fade the header in over the skeleton, only when switching views", async () => {
    const userInteraction = userEvent.setup();
    renderWithProviders(<App />, createFakeApiClient());
    const appTitle = screen.getByRole("heading", { level: 1 });
    expect(appTitle.closest(".fade-in")).toBeNull();

    await userInteraction.click(await screen.findByRole("button", { name: "Configurações" }));
    expect((await screen.findByRole("button", { name: /Voltar/ })).closest(".fade-in")).not.toBeNull();
  });
});
