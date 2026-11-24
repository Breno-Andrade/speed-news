// @vitest-environment jsdom
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { createFakeApiClient } from "../../test/fakeApiClient";
import { renderWithProviders } from "../../test/renderWithProviders";
import { SettingsPanel } from "./SettingsPanel";

describe("SettingsPanel", () => {
  it("never lets the user deselect every news language", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<SettingsPanel />);

    const englishCheckbox = await screen.findByRole("checkbox", { name: "Inglês" });
    const portugueseCheckbox = screen.getByRole("checkbox", { name: "Português" });
    expect(englishCheckbox).toBeChecked();
    expect(portugueseCheckbox).toBeChecked();

    await userInteraction.click(englishCheckbox);
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ newsLanguageCodes: ["pt"] }));
    expect(portugueseCheckbox).toBeDisabled();

    // Re-selecting everything saves "all" (empty list), so languages added later show up by themselves.
    await userInteraction.click(englishCheckbox);
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ newsLanguageCodes: [] }));
    expect(portugueseCheckbox).toBeEnabled();
  });

  it("switches the interface language immediately", async () => {
    const userInteraction = userEvent.setup();
    renderWithProviders(<SettingsPanel />);

    await userInteraction.click(await screen.findByRole("radio", { name: "English" }));
    expect(await screen.findByRole("heading", { name: "App language" })).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Close when clicking outside" })).toBeInTheDocument();
  });

  it("saves the 'close when clicking outside' choice", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<SettingsPanel />);

    const closeCheckbox = await screen.findByRole("checkbox", { name: "Fechar ao clicar fora" });
    await waitFor(() => expect(closeCheckbox).toBeChecked());
    await userInteraction.click(closeCheckbox);
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ closeOnFocusLoss: false }));
    expect(closeCheckbox).not.toBeChecked();
    expect(await screen.findByRole("status")).toHaveTextContent("Salvo");
  });

  it("reverts the optimistic change when saving fails", async () => {
    const userInteraction = userEvent.setup();
    const fakeApiClient = createFakeApiClient();
    fakeApiClient.saveSettings.mockRejectedValueOnce(new Error("backend down"));
    renderWithProviders(<SettingsPanel />, fakeApiClient);

    const closeCheckbox = await screen.findByRole("checkbox", { name: "Fechar ao clicar fora" });
    await waitFor(() => expect(closeCheckbox).toBeChecked());
    await userInteraction.click(closeCheckbox);
    await waitFor(() => expect(closeCheckbox).toBeChecked());
    // The user is told, instead of an unhandled error in the console.
    expect(await screen.findByRole("alert")).toHaveTextContent("Não foi possível salvar");
  });

  it("finds a time zone by city name, ignoring accents", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<SettingsPanel />);

    await userInteraction.type(await screen.findByPlaceholderText("Buscar cidade ou região…"), "sao paulo");
    await userInteraction.click(await screen.findByRole("option", { name: /America\/Sao Paulo/ }));
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(
      expect.objectContaining({ timeZonePreference: { mode: "manual", timeZoneId: "America/Sao_Paulo" } }),
    );

    const timeZoneGroup = screen.getByRole("radiogroup", { name: "Fuso horário" });
    await userInteraction.click(within(timeZoneGroup).getByRole("radio", { name: /Automático \(do computador/ }));
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ timeZonePreference: { mode: "system" } }));
  });

  it("names each radio group, so the two 'Automatic' options are distinguishable", async () => {
    renderWithProviders(<SettingsPanel />);
    const languageGroup = await screen.findByRole("radiogroup", { name: "Idioma do app" });
    const timeZoneGroup = screen.getByRole("radiogroup", { name: "Fuso horário" });
    expect(within(languageGroup).getAllByRole("radio")).toHaveLength(3);
    expect(within(timeZoneGroup).getAllByRole("radio")).toHaveLength(2);
  });
});
