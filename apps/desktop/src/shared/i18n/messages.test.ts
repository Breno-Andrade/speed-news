import { describe, expect, it } from "vitest";
import { describeLanguage, resolveLocalizedContent, resolvePreferredLocale } from "./messages";

describe("resolvePreferredLocale", () => {
  it("follows the operating system in automatic mode", () => {
    expect(resolvePreferredLocale("system", "en-GB")).toBe("en-GB");
  });
  it("uses the explicit choice otherwise", () => {
    expect(resolvePreferredLocale("pt-BR", "en-GB")).toBe("pt-BR");
    expect(resolvePreferredLocale("en", "pt-BR")).toBe("en");
  });
});

describe("resolveLocalizedContent", () => {
  it("translates race names and session labels in Portuguese", () => {
    const localizedContent = resolveLocalizedContent("pt-BR");
    expect(localizedContent.translateRaceName("Singapore Grand Prix")).toBe("GP de Singapura");
    expect(localizedContent.messages.sessionLabels.sprint_qualifying).toBe("Classificação sprint");
  });
  it("keeps official names in English", () => {
    const localizedContent = resolveLocalizedContent("en");
    expect(localizedContent.translateRaceName("Singapore Grand Prix")).toBe("Singapore GP");
    expect(localizedContent.messages.trayTooltip(3)).toBe("Speed News — 3 unread");
  });
  it("pluralizes the Portuguese tray tooltip", () => {
    const { messages } = resolveLocalizedContent("pt-BR");
    expect(messages.trayTooltip(0)).toBe("Speed News");
    expect(messages.trayTooltip(1)).toBe("Speed News — 1 não lida");
    expect(messages.trayTooltip(5)).toBe("Speed News — 5 não lidas");
  });
});

describe("describeLanguage", () => {
  it("names a language in the active interface language", () => {
    expect(describeLanguage("pt", "pt-BR")).toBe("Português");
    expect(describeLanguage("en", "pt-BR")).toBe("Inglês");
    expect(describeLanguage("pt", "en")).toBe("Portuguese");
  });
});
