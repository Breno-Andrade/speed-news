// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "../../shared/i18n/LocaleProvider";
import type { UpdateReady } from "../../shared/platform/tauriBridge";
import { UpdateNotices } from "./UpdateNotices";

const bridgeMocks = vi.hoisted(() => ({
  getAppVersion: vi.fn<() => Promise<string | null>>(),
  getPendingUpdate: vi.fn<() => Promise<UpdateReady | null>>(),
  installPendingUpdate: vi.fn<() => Promise<void>>(),
  onUpdateReady: vi.fn<(onReady: (updateReady: UpdateReady) => void) => () => void>(),
}));
vi.mock("../../shared/platform/tauriBridge", () => bridgeMocks);

let announceUpdateReady: (updateReady: UpdateReady) => void = () => undefined;

function renderNotices() {
  return render(
    <LocaleProvider preferredLocale="pt-BR">
      <UpdateNotices />
    </LocaleProvider>,
  );
}

beforeEach(() => {
  window.localStorage.clear();
  bridgeMocks.getAppVersion.mockResolvedValue("0.2.0");
  bridgeMocks.getPendingUpdate.mockResolvedValue(null);
  bridgeMocks.installPendingUpdate.mockResolvedValue(undefined);
  bridgeMocks.onUpdateReady.mockImplementation((onReady) => {
    announceUpdateReady = onReady;
    return () => undefined;
  });
});

describe("UpdateNotices", () => {
  it("stays silent on the very first run and remembers the version", async () => {
    renderNotices();
    await act(async () => undefined);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(window.localStorage.getItem("speed-news:last-seen-version")).toBe("0.2.0");
  });

  it("says once that the app was updated", async () => {
    window.localStorage.setItem("speed-news:last-seen-version", "0.1.0");
    renderNotices();
    expect(await screen.findByText("✓ Atualizado para a versão 0.2.0")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Fechar aviso" }));
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  // The popover webview is destroyed while hidden: an update downloaded meanwhile announced
  // itself to nobody, so a freshly created popover must still offer it.
  it("offers an update that was downloaded while the popover did not exist", async () => {
    bridgeMocks.getPendingUpdate.mockResolvedValue({ version: "0.3.0", requiresPassword: true });
    renderNotices();
    expect(await screen.findByText("Versão 0.3.0 pronta")).toBeInTheDocument();
  });

  it("offers to install a downloaded update on .deb/.rpm, warning about the password", async () => {
    renderNotices();
    await act(async () => announceUpdateReady({ version: "0.3.0", requiresPassword: true }));

    expect(screen.getByText("Versão 0.3.0 pronta")).toBeInTheDocument();
    expect(screen.getByText("O sistema vai pedir sua senha de administrador.")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Atualizar agora" }));
    expect(bridgeMocks.installPendingUpdate).toHaveBeenCalledOnce();
  });

  it("keeps the offer when the password prompt is cancelled", async () => {
    bridgeMocks.installPendingUpdate.mockRejectedValue(new Error("cancelled"));
    renderNotices();
    await act(async () => announceUpdateReady({ version: "0.3.0", requiresPassword: true }));

    await userEvent.click(screen.getByRole("button", { name: "Atualizar agora" }));
    expect(screen.getByRole("button", { name: "Atualizar agora" })).toBeEnabled();
  });

  it("does nothing in the browser (no app version)", async () => {
    bridgeMocks.getAppVersion.mockResolvedValue(null);
    window.localStorage.setItem("speed-news:last-seen-version", "0.1.0");
    renderNotices();
    await act(async () => undefined);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});
