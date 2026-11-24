// Loaded before every test file (see vite.config.ts). Component tests opt into jsdom with
// "// @vitest-environment jsdom"; pure logic tests keep the faster node environment, so every
// DOM stub below is guarded.
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

if (typeof window !== "undefined") {
  afterEach(() => cleanup());

  // jsdom has no layout engine: ResizeObserver does not exist and every size is 0.
  class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  vi.stubGlobal("ResizeObserver", ResizeObserverStub);

  if (!window.matchMedia) {
    vi.stubGlobal("matchMedia", (mediaQuery: string) => ({
      matches: false,
      media: mediaQuery,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }));
  }

  if (!navigator.clipboard) {
    Object.defineProperty(navigator, "clipboard", { value: { writeText: vi.fn(async () => undefined) }, configurable: true });
  }
}
