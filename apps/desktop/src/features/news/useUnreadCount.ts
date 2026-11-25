import { useEffect, useState } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { updateTray } from "../../shared/platform/tauriBridge";
import { useSettings } from "../settings/SettingsProvider";

/**
 * Listens to the backend event stream and mirrors the unread count on the tray icon.
 * The webview stays alive while the popover is hidden, so the tray keeps updating.
 * Tray texts (tooltip, menu) follow the interface language.
 * With "show new stories on the icon" off, the tray and panel button get 0 (no count, no dot);
 * the returned count (popover header) and the tooltip keep the real number.
 */
export function useUnreadCount(): number {
  const apiClient = useApiClient();
  const { messages } = useLocale();
  const { userSettings } = useSettings();
  const { showUnreadIndicator } = userSettings;
  const [unreadCount, setUnreadCount] = useState(0);

  useEffect(() => {
    const eventStream = apiClient.openEventStream();
    const handleUnreadCount = (messageEvent: MessageEvent<string>) => {
      const { unreadCount: receivedCount } = JSON.parse(messageEvent.data) as { unreadCount: number };
      setUnreadCount(receivedCount);
    };
    eventStream.addEventListener("unread-count", handleUnreadCount);
    return () => {
      eventStream.removeEventListener("unread-count", handleUnreadCount);
      eventStream.close();
    };
  }, [apiClient]);

  useEffect(() => {
    updateTray({
      unreadCount: showUnreadIndicator ? unreadCount : 0,
      tooltipText: messages.trayTooltip(unreadCount),
      openMenuLabel: messages.trayOpenMenuLabel,
      quitMenuLabel: messages.trayQuitMenuLabel,
    }).catch(() => undefined);
  }, [unreadCount, showUnreadIndicator, messages]);

  return unreadCount;
}
