import { useEffect, useState } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { updateTray } from "../../shared/platform/tauriBridge";

/**
 * Listens to the backend event stream and mirrors the unread count on the tray icon.
 * The webview stays alive while the popover is hidden, so the tray keeps updating.
 * Tray texts (tooltip, menu) follow the interface language.
 */
export function useUnreadCount(): number {
  const apiClient = useApiClient();
  const { messages } = useLocale();
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
      unreadCount,
      tooltipText: messages.trayTooltip(unreadCount),
      openMenuLabel: messages.trayOpenMenuLabel,
      quitMenuLabel: messages.trayQuitMenuLabel,
    }).catch(() => undefined);
  }, [unreadCount, messages]);

  return unreadCount;
}
