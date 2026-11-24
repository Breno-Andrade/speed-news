// @vitest-environment jsdom
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { CircuitProfile, WeekendForecast } from "../../shared/api/types";
import { createFakeApiClient, lasVegasResult } from "../../test/fakeApiClient";
import { renderWithProviders } from "../../test/renderWithProviders";
import { RaceWeekendCard } from "./RaceWeekendCard";

const marinaBayProfile: CircuitProfile = {
  circuitId: "marina_bay",
  circuitName: "Marina Bay Street Circuit",
  facts: { lengthMeters: 4927, altitudeMeters: 18, firstGrandPrixYear: 2008 },
  recentWinners: [
    { season: 2025, driverName: "George Russell", teamName: "Mercedes" },
    { season: 2024, driverName: "Lando Norris", teamName: "McLaren" },
  ],
  previousEdition: { ...lasVegasResult, season: 2025, eventName: "Singapore Grand Prix", laps: 62 },
};

const rainyForecast: WeekendForecast = {
  attributionText: "Previsão: Open-Meteo (CC BY 4.0)",
  attributionUrl: "https://open-meteo.com",
  sessions: [{ sessionKind: "race", startsAt: "2026-10-11T12:00:00Z", temperatureCelsius: 29.6, precipitationProbability: 70, windSpeedKmh: 9.4, weatherCode: 95 }],
};

function renderCard(fakeApiClient = createFakeApiClient({ circuitProfile: marinaBayProfile, forecast: rainyForecast })) {
  return renderWithProviders(<RaceWeekendCard onChangeTimeZone={vi.fn()} />, fakeApiClient);
}

describe("RaceWeekendCard", () => {
  it("shows the weekend schedule in the chosen time zone, grouped by day", async () => {
    renderCard();
    expect(await screen.findByRole("heading", { name: "GP de Singapura" })).toBeInTheDocument();
    expect(screen.getByText(/Semana de corrida · Etapa 17/)).toBeInTheDocument();

    // Brazilian broadcast times for Singapore 2026 (UTC−3).
    const fridayGroup = screen.getByRole("heading", { name: "Sexta-feira, 9 de outubro" }).closest("li")!;
    expect(within(fridayGroup).getByText("05:30")).toBeInTheDocument();
    expect(within(fridayGroup).getByText("Treino Livre 1")).toBeInTheDocument();
    expect(screen.getByText("Fuso: Horário Padrão de Brasília (GMT-3)", { exact: false })).toBeInTheDocument();
    // Track drawing with its license attribution.
    expect(screen.getByRole("img", { name: "Marina Bay Street Circuit" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "f1-circuits (MIT)" })).toBeInTheDocument();
  });

  it("shows the last Grand Prix podium with localized gaps", async () => {
    const userInteraction = userEvent.setup();
    renderCard();
    await userInteraction.click(await screen.findByRole("tab", { name: "Último GP" }));

    expect(await screen.findByText("GP de Las Vegas")).toBeInTheDocument();
    expect(screen.getByText("+0,639 s")).toBeInTheDocument();
    expect(screen.getByText(/largou em 17º/)).toBeInTheDocument();
    expect(screen.getByText(/Liam Lawson · 1:35.591/)).toBeInTheDocument();
  });

  it("shows circuit facts, last year's race and recent winners", async () => {
    const userInteraction = userEvent.setup();
    renderCard();
    await userInteraction.click(await screen.findByRole("tab", { name: "Pista" }));

    expect(await screen.findByText("4.927 m")).toBeInTheDocument();
    expect(screen.getByText("62")).toBeInTheDocument();
    expect(screen.getByText("305,5 km")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Em 2025" })).toBeInTheDocument();
    expect(screen.getByText("George Russell")).toBeInTheDocument();
  });

  it("shows the session forecast with a readable weather label", async () => {
    const userInteraction = userEvent.setup();
    renderCard();
    await userInteraction.click(await screen.findByRole("tab", { name: "Clima" }));

    expect(await screen.findByText("30°C")).toBeInTheDocument();
    expect(screen.getByLabelText("Tempestade")).toBeInTheDocument();
    expect(screen.getByText(/70% chuva/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previsão: Open-Meteo (CC BY 4.0)" })).toBeInTheDocument();
  });

  it("explains why there is no forecast yet", async () => {
    const userInteraction = userEvent.setup();
    renderCard(createFakeApiClient({ forecast: null }));
    await userInteraction.click(await screen.findByRole("tab", { name: "Clima" }));
    expect(await screen.findByText("A previsão do tempo aparece a partir de 7 dias antes da corrida.")).toBeInTheDocument();
  });

  it("says so when a source is unavailable instead of spinning forever", async () => {
    const userInteraction = userEvent.setup();
    renderCard(createFakeApiClient({ failingInsights: true }));
    await userInteraction.click(await screen.findByRole("tab", { name: "Último GP" }));
    expect(await screen.findByText("Não foi possível carregar agora. Tente de novo em instantes.")).toBeInTheDocument();
  });

  it("shows a friendly message when the calendar is empty", async () => {
    renderCard(createFakeApiClient({ raceWeekend: null }));
    expect(await screen.findByText("Nenhuma corrida no calendário.")).toBeInTheDocument();
  });
});
