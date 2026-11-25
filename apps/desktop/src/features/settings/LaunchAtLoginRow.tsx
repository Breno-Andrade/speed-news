import { useEffect, useState } from "react";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { isLaunchAtLoginEnabled, setLaunchAtLogin } from "../../shared/platform/tauriBridge";

/** Settings → General: start at login. Desktop app only; the OS stores the choice. */
export function LaunchAtLoginRow() {
  const { messages } = useLocale();
  const [isEnabled, setIsEnabled] = useState<boolean | null>(null);

  useEffect(() => {
    isLaunchAtLoginEnabled()
      .then(setIsEnabled)
      .catch(() => setIsEnabled(null));
  }, []);

  async function toggle(shouldEnable: boolean) {
    setIsEnabled(shouldEnable);
    try {
      await setLaunchAtLogin(shouldEnable);
    } catch {
      setIsEnabled(!shouldEnable); // the OS refused: show the real state again
    }
  }

  if (isEnabled === null) return null;
  return (
    <label className="settings-row">
      <span className="settings-row__text">
        <span className="settings-row__label">{messages.launchAtLogin}</span>
        <span className="settings-row__description">{messages.launchAtLoginHint}</span>
      </span>
      <input type="checkbox" role="switch" className="toggle-switch" checked={isEnabled} onChange={(changeEvent) => toggle(changeEvent.target.checked)} />
    </label>
  );
}
