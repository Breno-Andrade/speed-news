// @vitest-environment jsdom
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { createFakeApiClient } from "../../test/fakeApiClient";
import { renderWithProviders } from "../../test/renderWithProviders";
import { SettingsPanel } from "./SettingsPanel";

describe("SettingsPanel", () => {
  it("groups the settings into General, News, Times and About", async () => {
    renderWithProviders(<SettingsPanel />);
    for (const groupName of ["Geral", "Notícias", "Horários", "Sobre"]) {
      expect(await screen.findByRole("region", { name: groupName })).toBeInTheDocument();
    }
  });

  it("never lets the user switch off every news language", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<SettingsPanel />);

    const englishSwitch = await screen.findByRole("switch", { name: /^Inglês/ });
    const portugueseSwitch = screen.getByRole("switch", { name: /^Português/ });
    expect(englishSwitch).toBeChecked();
    expect(screen.getByRole("switch", { name: /^Português/ }).closest("label")).toHaveTextContent("1 fonte");

    await userInteraction.click(englishSwitch);
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ newsLanguageCodes: ["pt"] }));
    expect(portugueseSwitch).toBeDisabled();

    // Re-selecting everything saves "all" (empty list), so languages added later show up by themselves.
    await userInteraction.click(englishSwitch);
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ newsLanguageCodes: [] }));
    expect(portugueseSwitch).toBeEnabled();
  });

  it("switches the interface language immediately", async () => {
    const userInteraction = userEvent.setup();
    renderWithProviders(<SettingsPanel />);

    const languageGroup = await screen.findByRole("radiogroup", { name: "Idioma do app" });
    await userInteraction.click(within(languageGroup).getByRole("radio", { name: "English" }));
    expect(await screen.findByRole("region", { name: "General" })).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: /Close when clicking outside/ })).toBeInTheDocument();
  });

  it("saves the 'close when clicking outside' choice", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<SettingsPanel />);

    const closeSwitch = await screen.findByRole("switch", { name: /Fechar ao clicar fora/ });
    await waitFor(() => expect(closeSwitch).toBeChecked());
    await userInteraction.click(closeSwitch);
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ closeOnFocusLoss: false }));
    expect(closeSwitch).not.toBeChecked();
    expect(await screen.findByRole("status")).toHaveTextContent("Salvo");
  });

  it("reverts the optimistic change and tells the user when saving fails", async () => {
    const userInteraction = userEvent.setup();
    const fakeApiClient = createFakeApiClient();
    fakeApiClient.saveSettings.mockRejectedValueOnce(new Error("backend down"));
    renderWithProviders(<SettingsPanel />, fakeApiClient);

    const closeSwitch = await screen.findByRole("switch", { name: /Fechar ao clicar fora/ });
    await waitFor(() => expect(closeSwitch).toBeChecked());
    await userInteraction.click(closeSwitch);
    await waitFor(() => expect(closeSwitch).toBeChecked());
    expect(await screen.findByRole("alert")).toHaveTextContent("Não foi possível salvar");
  });

  it("shows the current time zone in a readable form", async () => {
    renderWithProviders(<SettingsPanel />);
    const timeZoneRow = await screen.findByRole("button", { name: /Fuso horário/ });
    // The fake settings use America/Sao_Paulo chosen manually.
    expect(timeZoneRow).toHaveTextContent("São Paulo · GMT−3");
    expect(timeZoneRow).toHaveTextContent("Escolhido por você");
  });

  it("finds a time zone by the name of its time, not only the city id", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<SettingsPanel />, createFakeApiClient({ settings: { timeZonePreference: { mode: "system" } } }));

    await userInteraction.click(await screen.findByRole("button", { name: /Fuso horário/ }));
    await userInteraction.type(screen.getByRole("searchbox"), "brasilia");
    await userInteraction.click(await screen.findByRole("button", { name: /^São Paulo/ }));

    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(
      expect.objectContaining({ timeZonePreference: { mode: "manual", timeZoneId: "America/Sao_Paulo" } }),
    );
    // Choosing closes the picker and the row shows the choice.
    expect(await screen.findByRole("button", { name: /Fuso horário/ })).toHaveTextContent("São Paulo");
  });

  it("finds zones by GMT offset and says when nothing matches", async () => {
    const userInteraction = userEvent.setup();
    renderWithProviders(<SettingsPanel />);
    await userInteraction.click(await screen.findByRole("button", { name: /Fuso horário/ }));
    const searchBox = screen.getByRole("searchbox");

    await userInteraction.type(searchBox, "gmt-3");
    expect(screen.getByRole("heading", { name: "GMT−3" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "GMT+1" })).not.toBeInTheDocument();

    await userInteraction.clear(searchBox);
    await userInteraction.type(searchBox, "cidade-que-nao-existe");
    expect(screen.getByText("Nenhum fuso encontrado.")).toBeInTheDocument();
  });

  it("goes back to the computer's time zone", async () => {
    const userInteraction = userEvent.setup();
    const { fakeApiClient } = renderWithProviders(<SettingsPanel />);
    await userInteraction.click(await screen.findByRole("button", { name: /Fuso horário/ }));
    await userInteraction.click(screen.getByRole("button", { name: /Usar o fuso do computador/ }));
    expect(fakeApiClient.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ timeZonePreference: { mode: "system" } }));
  });

  it("credits every data source and states the trademark notice", async () => {
    const userInteraction = userEvent.setup();
    renderWithProviders(<SettingsPanel />);
    const aboutGroup = await screen.findByRole("region", { name: "Sobre" });

    expect(within(aboutGroup).getByText("desenvolvimento")).toBeInTheDocument(); // no app version in the browser
    expect(within(aboutGroup).getByText(/licença GPL-3.0/)).toBeInTheDocument();
    expect(screen.getByText(/sem vínculo com a Fórmula 1/)).toBeInTheDocument();

    await userInteraction.click(within(aboutGroup).getByText("Fontes de dados"));
    for (const sourceName of ["Motorsport.com Brasil", "The Race", "Jolpica F1", "f1-circuits", "Open-Meteo"]) {
      expect(await within(aboutGroup).findByRole("button", { name: sourceName })).toBeInTheDocument();
    }
  });
});
