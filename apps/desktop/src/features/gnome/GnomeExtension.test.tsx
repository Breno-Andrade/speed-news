// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "../../shared/i18n/LocaleProvider";
import type { GnomeExtensionStatus } from "../../shared/platform/tauriBridge";
import { GnomeExtensionNotice } from "./GnomeExtensionNotice";
import { GnomeExtensionRow } from "./GnomeExtensionRow";
import { gnomeExtensionStage } from "./useGnomeExtension";

const bridgeMocks = vi.hoisted(() => ({
  getGnomeExtensionStatus: vi.fn<() => Promise<GnomeExtensionStatus | null>>(),
  installGnomeExtension: vi.fn<() => Promise<GnomeExtensionStatus>>(),
}));
vi.mock("../../shared/platform/tauriBridge", () => bridgeMocks);

const notInstalledOnWayland: GnomeExtensionStatus = { isGnomeSession: true, sessionType: "wayland", isInstalled: false, isActive: false };

async function renderInLocale(component: React.ReactNode) {
  render(<LocaleProvider preferredLocale="pt-BR">{component}</LocaleProvider>);
  await act(async () => undefined);
}

beforeEach(() => {
  window.localStorage.clear();
  bridgeMocks.getGnomeExtensionStatus.mockResolvedValue(notInstalledOnWayland);
  bridgeMocks.installGnomeExtension.mockResolvedValue({ ...notInstalledOnWayland, isInstalled: true });
});

describe("gnomeExtensionStage", () => {
  it("maps the native status to what the UI should offer", () => {
    expect(gnomeExtensionStage(null)).toBe("notApplicable");
    expect(gnomeExtensionStage({ ...notInstalledOnWayland, isGnomeSession: false })).toBe("notApplicable");
    expect(gnomeExtensionStage(notInstalledOnWayland)).toBe("notInstalled");
    expect(gnomeExtensionStage({ ...notInstalledOnWayland, isInstalled: true })).toBe("needsShellReload");
    expect(gnomeExtensionStage({ ...notInstalledOnWayland, isInstalled: true, isActive: true })).toBe("active");
  });
});

describe("GnomeExtensionNotice", () => {
  it("renders nothing outside GNOME or in the browser", async () => {
    bridgeMocks.getGnomeExtensionStatus.mockResolvedValue(null);
    await renderInLocale(<GnomeExtensionNotice />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("installs with one click, then explains the reload for the session type", async () => {
    const user = userEvent.setup();
    await renderInLocale(<GnomeExtensionNotice />);

    await user.click(screen.getByRole("button", { name: "Instalar" }));

    expect(bridgeMocks.installGnomeExtension).toHaveBeenCalledOnce();
    expect(screen.getByRole("status")).toHaveTextContent("recarregue o GNOME");
    expect(screen.getByRole("status")).toHaveTextContent("Saia da sessão e entre de novo.");
  });

  it("shows the X11 shortcut on X11 sessions", async () => {
    bridgeMocks.getGnomeExtensionStatus.mockResolvedValue({ ...notInstalledOnWayland, sessionType: "x11", isInstalled: true });
    await renderInLocale(<GnomeExtensionNotice />);
    expect(screen.getByRole("status")).toHaveTextContent("Alt+F2");
  });

  it("keeps the offer and reports a failed install", async () => {
    bridgeMocks.installGnomeExtension.mockRejectedValue("gsettings unavailable");
    const user = userEvent.setup();
    await renderInLocale(<GnomeExtensionNotice />);

    await user.click(screen.getByRole("button", { name: "Instalar" }));

    expect(screen.getByText(/Não foi possível instalar/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Instalar" })).toBeEnabled();
  });

  it("stays dismissed once closed", async () => {
    const user = userEvent.setup();
    await renderInLocale(<GnomeExtensionNotice />);
    await user.click(screen.getByRole("button", { name: "Fechar aviso" }));
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(window.localStorage.getItem("speed-news:gnome-extension-notice-dismissed")).toBe("true");
  });

  it("disappears once the panel button is connected", async () => {
    bridgeMocks.getGnomeExtensionStatus.mockResolvedValue({ ...notInstalledOnWayland, isInstalled: true, isActive: true });
    await renderInLocale(<GnomeExtensionNotice />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});

describe("GnomeExtensionRow", () => {
  it("offers the install in settings even after the notice was dismissed", async () => {
    window.localStorage.setItem("speed-news:gnome-extension-notice-dismissed", "true");
    await renderInLocale(<GnomeExtensionRow />);
    expect(screen.getByText("Botão na barra do GNOME")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Instalar" })).toBeInTheDocument();
  });

  it("shows when the button is active", async () => {
    bridgeMocks.getGnomeExtensionStatus.mockResolvedValue({ ...notInstalledOnWayland, isInstalled: true, isActive: true });
    await renderInLocale(<GnomeExtensionRow />);
    expect(screen.getByText("Ativo")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("refreshes the status when the popover regains focus", async () => {
    await renderInLocale(<GnomeExtensionRow />);
    bridgeMocks.getGnomeExtensionStatus.mockResolvedValue({ ...notInstalledOnWayland, isInstalled: true, isActive: true });
    await act(async () => {
      window.dispatchEvent(new Event("focus"));
    });
    expect(screen.getByText("Ativo")).toBeInTheDocument();
  });
});
