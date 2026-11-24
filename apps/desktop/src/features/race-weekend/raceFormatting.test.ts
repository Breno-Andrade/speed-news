import { describe, expect, it } from "vitest";
import type { DriverResult } from "../../shared/api/types";
import { resolveLocalizedContent } from "../../shared/i18n/messages";
import { describeWeatherCode, formatMeters, formatRaceDistance, formatRaceTime } from "./raceFormatting";

const portugueseMessages = resolveLocalizedContent("pt-BR").messages;

function buildDriverResult(position: number, raceTime: string): DriverResult {
  return { position, raceTime, driverCode: "VER", driverName: "Max Verstappen", teamName: "Red Bull", gridPosition: 1 };
}

describe("formatRaceTime", () => {
  it("keeps the winner's total time", () => {
    expect(formatRaceTime(buildDriverResult(1, "1:27:39.624"), "pt-BR", portugueseMessages)).toBe("1:27:39.624");
  });
  it("localizes gaps in seconds", () => {
    expect(formatRaceTime(buildDriverResult(2, "+0.639"), "pt-BR", portugueseMessages)).toBe("+0,639 s");
    expect(formatRaceTime(buildDriverResult(2, "+12.5"), "en", portugueseMessages)).toBe("+12.500 s");
  });
  it("translates lapped drivers", () => {
    expect(formatRaceTime(buildDriverResult(3, "+1 Lap"), "pt-BR", portugueseMessages)).toBe("+1 volta");
    expect(formatRaceTime(buildDriverResult(3, "+2 Laps"), "pt-BR", portugueseMessages)).toBe("+2 voltas");
  });
  it("keeps unknown statuses as published", () => {
    expect(formatRaceTime(buildDriverResult(3, "Retired"), "pt-BR", portugueseMessages)).toBe("Retired");
  });
});

describe("circuit numbers", () => {
  it("formats length and race distance for Lusail", () => {
    expect(formatMeters(5380, "pt-BR")).toBe("5.380 m");
    expect(formatRaceDistance(5380, 57, "pt-BR")).toBe("306,7 km");
  });
});

describe("describeWeatherCode", () => {
  it.each([
    [0, "clear"],
    [2, "partlyCloudy"],
    [63, "rain"],
    [81, "showers"],
    [96, "thunderstorm"],
  ])("maps WMO code %d to %s", (weatherCode, expectedCondition) => {
    expect(describeWeatherCode(weatherCode).weatherCondition).toBe(expectedCondition);
  });
});
