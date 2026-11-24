import { describe, expect, it } from "vitest";
import { displayDomain, sourceHue, sourceInitial } from "./sourceIdentity";

describe("sourceHue", () => {
  it("is stable for the same source and spreads different sources", () => {
    expect(sourceHue("motorsport-f1")).toBe(sourceHue("motorsport-f1"));
    const distinctHues = new Set(["formula1-com", "motorsport-f1", "autosport-f1", "the-race-f1", "f1mania", "gptotal"].map(sourceHue));
    expect(distinctHues.size).toBeGreaterThanOrEqual(5);
    for (const sourceSlug of ["", "a-very-long-source-slug-to-check-overflow-handling"]) {
      expect(sourceHue(sourceSlug)).toBeGreaterThanOrEqual(0);
      expect(sourceHue(sourceSlug)).toBeLessThan(360);
    }
  });
});

describe("sourceInitial", () => {
  it("takes the first letter or digit", () => {
    expect(sourceInitial("Motorsport.com")).toBe("M");
    expect(sourceInitial("  the Race")).toBe("T");
    expect(sourceInitial("Ó Notícias")).toBe("Ó");
    expect(sourceInitial("1000 Milhas")).toBe("1");
    expect(sourceInitial("")).toBe("?");
  });
});

describe("displayDomain", () => {
  it("shows a clean host name", () => {
    expect(displayDomain("https://www.motorsport.com/f1/news/abc")).toBe("motorsport.com");
    expect(displayDomain("https://br.motorsport.com/f1/")).toBe("br.motorsport.com");
    expect(displayDomain("not a url")).toBe("");
  });
});
