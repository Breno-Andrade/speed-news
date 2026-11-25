import { useEffect, useState } from "react";
import { useApiClient } from "../../shared/api/ApiProvider";
import { mirrorUnreadCountInTitle } from "../../shared/platform/tauriBridge";
import { useSettings } from "../settings/SettingsProvider";

/**
 * Listens to the backend event stream for the popover header count.
 * The native tray does not depend on this hook: the Go sidecar reports the tray state to the
 * shell directly, because the webview is destroyed while the popover is hidden.
 * In the browser (no tray) the tab title mirrors what the tray would show: nothing when
 * "show new stories on the icon" is off.
 */
export function useUnreadCount(): number {
  const apiClient = useApiClient();
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
    mirrorUnreadCountInTitle(showUnreadIndicator ? unreadCount : 0);
  }, [unreadCount, showUnreadIndicator]);

  return unreadCount;
}
