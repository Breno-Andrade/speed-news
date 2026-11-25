import { describe, expect, it } from "vitest";
import rustTrayTexts from "../../../src-tauri/src/tray_texts.rs?raw";
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

// The tray is drawn by the native shell while the popover webview does not exist, so its texts
// are duplicated in Rust. They must stay identical to the ones the popover uses.
describe("tray texts in the native shell", () => {
  it.each(["pt-BR", "en"])("match the %s messages", (locale) => {
    const { messages } = resolveLocalizedContent(locale);
    expect(rustTrayTexts).toContain(`"${messages.trayOpenMenuLabel}"`);
    expect(rustTrayTexts).toContain(`"${messages.trayQuitMenuLabel}"`);
    expect(rustTrayTexts).toContain(`"${messages.trayTooltip(0)}"`);
    // Plural form, with Rust's format! placeholder in place of the number.
    expect(rustTrayTexts).toContain(`"${messages.trayTooltip(7).replace("7", "{unread_count}")}"`);
  });

  it("matches the Portuguese singular", () => {
    expect(rustTrayTexts).toContain(`"${resolveLocalizedContent("pt-BR").messages.trayTooltip(1)}"`);
  });
});
