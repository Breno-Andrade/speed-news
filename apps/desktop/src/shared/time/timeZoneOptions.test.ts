import { describe, expect, it } from "vitest";
import {
  buildTimeZoneOptions,
  cityNameFromZoneId,
  describeTimeZoneOption,
  filterTimeZoneOptions,
  formatOffsetLabel,
  groupByCountry,
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

describe("countries", () => {
  it("knows the country of every zone the browser offers, legacy names included", () => {
    const zonesWithoutCountry = allTimeZoneOptions.filter((timeZoneOption) => timeZoneOption.countryCode === null);
    expect(zonesWithoutCountry.map((timeZoneOption) => timeZoneOption.timeZoneId)).toEqual([]);
    expect(describeTimeZoneOption("Asia/Calcutta", "pt-BR", referenceInstant).countryCode).toBe("IN");
  });

  it("names countries in the interface language", () => {
    expect(describeTimeZoneOption("America/Sao_Paulo", "pt-BR", referenceInstant).countryName).toBe("Brasil");
    expect(describeTimeZoneOption("America/New_York", "pt-BR", referenceInstant).countryName).toBe("Estados Unidos");
    expect(describeTimeZoneOption("America/New_York", "en", referenceInstant).countryName).toBe("United States");
  });

  it("finds zones by country name", () => {
    expect(findCities("brasil")).toEqual(expect.arrayContaining(["America/Sao_Paulo", "America/Manaus", "America/Noronha"]));
    expect(findCities("portugal").sort()).toEqual(["Atlantic/Azores", "Atlantic/Madeira", "Europe/Lisbon"]);
  });
});

describe("grouping by country", () => {
  const countryGroups = groupByCountry(allTimeZoneOptions, "pt-BR", "BR");

  it("puts the computer's country first, then the rest alphabetically", () => {
    expect(countryGroups[0]?.countryName).toBe("Brasil");
    const otherCountryNames = countryGroups.slice(1).map((countryGroup) => countryGroup.countryName);
    expect(otherCountryNames).toEqual([...otherCountryNames].sort((firstName, secondName) => firstName.localeCompare(secondName, "pt-BR")));
  });

  it("orders a country's zones west to east", () => {
    const brazilCities = countryGroups[0]!.timeZoneOptions.map((timeZoneOption) => timeZoneOption.cityName);
    // Rio Branco (GMT−5) before Manaus (GMT−4) before São Paulo (GMT−3) before Noronha (GMT−2).
    expect(brazilCities.indexOf("Rio Branco")).toBeLessThan(brazilCities.indexOf("Manaus"));
    expect(brazilCities.indexOf("Manaus")).toBeLessThan(brazilCities.indexOf("São Paulo"));
    expect(brazilCities.indexOf("São Paulo")).toBeLessThan(brazilCities.indexOf("Noronha"));
  });

  it("has one group per country", () => {
    const countryNames = countryGroups.map((countryGroup) => countryGroup.countryName);
    expect(new Set(countryNames).size).toBe(countryNames.length);
  });

  it("works without a preferred country", () => {
    const alphabeticalGroups = groupByCountry(allTimeZoneOptions, "pt-BR");
    expect(alphabeticalGroups[0]?.countryName.localeCompare("B", "pt-BR")).toBeLessThan(0);
  });
});
