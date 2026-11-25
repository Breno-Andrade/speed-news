import { describe, expect, it } from "vitest";
import {
  buildTimeZoneOptions,
  cityNameFromZoneId,
  describeTimeZoneOption,
  filterTimeZoneOptions,
  formatOffsetLabel,
  groupByOffset,
  parseOffsetMinutes,
} from "./timeZoneOptions";

// November: no daylight saving in Brazil, standard time in Europe.
const referenceInstant = new Date("2026-11-24T12:00:00Z");
const allTimeZoneOptions = buildTimeZoneOptions(Intl.supportedValuesOf("timeZone"), "pt-BR", referenceInstant);
const findCities = (searchQuery: string) => filterTimeZoneOptions(allTimeZoneOptions, searchQuery).map((timeZoneOption) => timeZoneOption.timeZoneId);

describe("offsets", () => {
  it.each([
    ["GMT-3", -180],
    ["GMT+5:30", 330],
    ["GMT+10:30", 630],
    ["GMT", 0],
  ])("parses %s", (shortOffset, expectedMinutes) => {
    expect(parseOffsetMinutes(shortOffset)).toBe(expectedMinutes);
  });

  it.each([
    [-180, "GMT−3"],
    [345, "GMT+5:45"],
    [0, "GMT"],
  ])("formats %d minutes", (offsetMinutes, expectedLabel) => {
    expect(formatOffsetLabel(offsetMinutes)).toBe(expectedLabel);
  });
});

describe("readable names", () => {
  it("restores native city spelling and handles nested ids", () => {
    expect(cityNameFromZoneId("America/Sao_Paulo")).toBe("São Paulo");
    expect(cityNameFromZoneId("America/Argentina/Buenos_Aires")).toBe("Buenos Aires");
    expect(cityNameFromZoneId("Europe/Lisbon")).toBe("Lisbon");
  });

  it("describes a zone with region, readable name and offset", () => {
    const saoPaulo = describeTimeZoneOption("America/Sao_Paulo", "pt-BR", referenceInstant);
    expect(saoPaulo).toMatchObject({ cityName: "São Paulo", regionName: "Américas", genericName: "Horário Padrão de Brasília", offsetLabel: "GMT−3" });
  });
});

describe("search", () => {
  it("finds a zone by the name people use, not only the IANA id", () => {
    expect(findCities("brasilia")).toContain("America/Sao_Paulo");
    expect(findCities("são paulo")).toContain("America/Sao_Paulo");
    expect(findCities("sao paulo")).toContain("America/Sao_Paulo");
  });

  it("finds zones by offset, in any notation", () => {
    for (const offsetQuery of ["-3", "gmt-3", "utc-3", "GMT−3"]) {
      expect(findCities(offsetQuery), offsetQuery).toContain("America/Sao_Paulo");
    }
    // India is "Asia/Kolkata" or the legacy "Asia/Calcutta" depending on the ICU version: assert the offset.
    const halfHourMatches = filterTimeZoneOptions(allTimeZoneOptions, "+5:30");
    expect(halfHourMatches.length).toBeGreaterThan(0);
    expect(halfHourMatches.every((timeZoneOption) => timeZoneOption.offsetMinutes === 330)).toBe(true);
  });

  it("requires every typed word, in any order", () => {
    expect(findCities("europa lisbon")).toEqual(["Europe/Lisbon"]);
    expect(findCities("lisbon europa")).toEqual(["Europe/Lisbon"]);
    expect(findCities("nowhere-city")).toEqual([]);
  });
});

describe("ordering and grouping", () => {
  it("goes west to east and groups by offset", () => {
    const offsets = allTimeZoneOptions.map((timeZoneOption) => timeZoneOption.offsetMinutes);
    expect(offsets).toEqual([...offsets].sort((firstOffset, secondOffset) => firstOffset - secondOffset));

    const offsetGroups = groupByOffset(allTimeZoneOptions);
    const brasiliaGroup = offsetGroups.find((offsetGroup) => offsetGroup.offsetLabel === "GMT−3");
    expect(brasiliaGroup?.timeZoneOptions.map((timeZoneOption) => timeZoneOption.timeZoneId)).toContain("America/Sao_Paulo");
    expect(new Set(offsetGroups.map((offsetGroup) => offsetGroup.offsetLabel)).size).toBe(offsetGroups.length);
  });
});
