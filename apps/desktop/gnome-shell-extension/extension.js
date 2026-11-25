// Speed News panel button for GNOME Shell 45+.
//
// Why it exists: on Ubuntu's GNOME, the AppIndicator extension opens the tray menu on every
// single left click, so a tray icon can never open the app with one click. This button talks to
// the running app over D-Bus (service implemented in src-tauri/src/panel_bridge.rs):
//   left click  → ToggleWindow(anchor)   right click → menu (Open / Quit)
//   label       ← UnreadCount property
// The button only appears while the app is running; the app hides its own tray icon meanwhile.

import Clutter from "gi://Clutter";
import Gio from "gi://Gio";
import GLib from "gi://GLib";
import GObject from "gi://GObject";
import ShellToolkit from "gi://St"; // St: GNOME Shell Toolkit

import { Extension } from "resource:///org/gnome/shell/extensions/extension.js";
import * as Main from "resource:///org/gnome/shell/ui/main.js";
import * as PanelMenu from "resource:///org/gnome/shell/ui/panelMenu.js";
import * as PopupMenu from "resource:///org/gnome/shell/ui/popupMenu.js";

const BUS_NAME = "dev.speednews.Desktop";
const OBJECT_PATH = "/dev/speednews/Desktop";
const MAXIMUM_SHOWN_COUNT = 99;
// Must match "title" of the main window in src-tauri/tauri.conf.json.
const APP_WINDOW_TITLE = "Speed News";
const FOCUS_RETRY_INTERVAL_MS = 50;
// The app creates its window on demand (it is destroyed while hidden to save memory), so the
// first open after a while takes longer to map than a reused window: keep trying for 5 s.
const FOCUS_RETRY_ATTEMPTS = 100;

const desktopInterfaceXml = `
<node>
  <interface name="dev.speednews.Desktop1">
    <method name="ToggleWindow"><arg type="i" name="anchor_x" direction="in"/><arg type="i" name="anchor_y" direction="in"/><arg type="b" name="is_visible" direction="out"/></method>
    <method name="ShowWindow"><arg type="i" name="anchor_x" direction="in"/><arg type="i" name="anchor_y" direction="in"/></method>
    <method name="Quit"/>
    <method name="RegisterPanelButton"/>
    <method name="UnregisterPanelButton"/>
    <property name="UnreadCount" type="u" access="read"/>
    <property name="OpenLabel" type="s" access="read"/>
    <property name="QuitLabel" type="s" access="read"/>
  </interface>
</node>`;

const DesktopProxy = Gio.DBusProxy.makeProxyWrapper(desktopInterfaceXml);

const SpeedNewsButton = GObject.registerClass(
  class SpeedNewsButton extends PanelMenu.Button {
    _init(extensionPath) {
      super._init(0.5, "Speed News", false);
      this._desktopProxy = null;

      const buttonBox = new ShellToolkit.BoxLayout({ style_class: "panel-status-menu-box speed-news-button" });
      buttonBox.add_child(
        new ShellToolkit.Icon({
          gicon: Gio.icon_new_for_string(`${extensionPath}/icons/speed-news-flag.svg`),
          style_class: "system-status-icon",
        }),
      );
      this._countLabel = new ShellToolkit.Label({ style_class: "speed-news-count", y_align: Clutter.ActorAlign.CENTER, visible: false });
      buttonBox.add_child(this._countLabel);
      this.add_child(buttonBox);

      this._focusTimerId = null;
      this.connect("destroy", () => this._cancelFocusTimer());

      this._openMenuItem = new PopupMenu.PopupMenuItem("Abrir notícias");
      this._openMenuItem.connect("activate", () => {
        this._desktopProxy
          ?.ShowWindowAsync(...this._windowAnchor())
          .then(() => this._focusAppWindowWhenShown())
          .catch(console.error);
      });
      this._quitMenuItem = new PopupMenu.PopupMenuItem("Sair");
      this._quitMenuItem.connect("activate", () => this._desktopProxy?.QuitAsync().catch(console.error));
      this.menu.addMenuItem(this._openMenuItem);
      this.menu.addMenuItem(this._quitMenuItem);
    }

    setDesktopProxy(desktopProxy) {
      this._desktopProxy = desktopProxy;
      this.refreshFromProxy();
    }

    refreshFromProxy() {
      if (!this._desktopProxy) return;
      const unreadCount = this._desktopProxy.UnreadCount ?? 0;
      this._countLabel.text = unreadCount > MAXIMUM_SHOWN_COUNT ? `${MAXIMUM_SHOWN_COUNT}+` : String(unreadCount);
      this._countLabel.visible = unreadCount > 0;
      if (this._desktopProxy.OpenLabel) this._openMenuItem.label.text = this._desktopProxy.OpenLabel;
      if (this._desktopProxy.QuitLabel) this._quitMenuItem.label.text = this._desktopProxy.QuitLabel;
    }

    /** Bottom center of the button in stage (screen) pixels: the popover opens right below it. */
    _windowAnchor() {
      const [buttonLeft, buttonTop] = this.get_transformed_position();
      const [buttonWidth, buttonHeight] = this.get_transformed_size();
      return [Math.round(buttonLeft + buttonWidth / 2), Math.round(buttonTop + buttonHeight)];
    }

    // PanelMenu.Button opens its menu on any click; here only the right click does.
    vfunc_event(event) {
      const eventType = event.type();
      if (eventType !== Clutter.EventType.BUTTON_PRESS && eventType !== Clutter.EventType.TOUCH_BEGIN) {
        return Clutter.EVENT_PROPAGATE;
      }
      if (eventType === Clutter.EventType.BUTTON_PRESS && event.get_button() === Clutter.BUTTON_SECONDARY) {
        this.menu.toggle();
        return Clutter.EVENT_STOP;
      }
      if (this.menu.isOpen) this.menu.close();
      this._desktopProxy
        ?.ToggleWindowAsync(...this._windowAnchor())
        .then(([isVisible]) => {
          if (isVisible) this._focusAppWindowWhenShown();
        })
        .catch(console.error);
      return Clutter.EVENT_STOP;
    }

    /**
     * GNOME's focus-stealing prevention does not focus a window that an app shows because of a
     * D-Bus call, and a window that never had focus never loses it: "close when clicking outside"
     * would never fire. The shell itself may grant focus, so activate the window from here once
     * it is mapped (it appears a few milliseconds after ToggleWindow returns).
     */
    _focusAppWindowWhenShown() {
      this._cancelFocusTimer();
      let remainingAttempts = FOCUS_RETRY_ATTEMPTS;
      this._focusTimerId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, FOCUS_RETRY_INTERVAL_MS, () => {
        const appWindow = global
          .get_window_actors()
          .map((windowActor) => windowActor.get_meta_window())
          .find((metaWindow) => metaWindow?.get_title() === APP_WINDOW_TITLE);
        remainingAttempts -= 1;
        if (appWindow) {
          Main.activateWindow(appWindow);
        }
        if (appWindow || remainingAttempts <= 0) {
          this._focusTimerId = null;
          return GLib.SOURCE_REMOVE;
        }
        return GLib.SOURCE_CONTINUE;
      });
    }

    _cancelFocusTimer() {
      if (this._focusTimerId) GLib.source_remove(this._focusTimerId);
      this._focusTimerId = null;
    }
  },
);

export default class SpeedNewsExtension extends Extension {
  enable() {
    this._panelButton = new SpeedNewsButton(this.path);
    this._panelButton.visible = false;
    Main.panel.addToStatusArea(this.uuid, this._panelButton);

    // Show the button only while the app is running; it may start before or after the shell.
    this._busWatchId = Gio.bus_watch_name(
      Gio.BusType.SESSION,
      BUS_NAME,
      Gio.BusNameWatcherFlags.NONE,
      () => this._connectToApp(),
      () => this._disconnectFromApp(),
    );
  }

  disable() {
    if (this._busWatchId) Gio.bus_unwatch_name(this._busWatchId);
    this._busWatchId = null;
    // Give the tray icon back to the app when the user disables the extension.
    this._desktopProxy?.UnregisterPanelButtonAsync().catch(() => undefined);
    this._disconnectFromApp();
    this._panelButton?.destroy();
    this._panelButton = null;
  }

  _connectToApp() {
    this._disconnectFromApp();
    new DesktopProxy(Gio.DBus.session, BUS_NAME, OBJECT_PATH, (desktopProxy, proxyError) => {
      if (proxyError || !this._panelButton) {
        if (proxyError) console.error(`Speed News: ${proxyError.message}`);
        return;
      }
      this._desktopProxy = desktopProxy;
      this._propertiesChangedId = desktopProxy.connect("g-properties-changed", () => this._panelButton?.refreshFromProxy());
      this._panelButton.setDesktopProxy(desktopProxy);
      this._panelButton.visible = true;
      desktopProxy.RegisterPanelButtonAsync().catch(console.error);
    });
  }

  _disconnectFromApp() {
    if (this._desktopProxy && this._propertiesChangedId) this._desktopProxy.disconnect(this._propertiesChangedId);
    this._desktopProxy = null;
    this._propertiesChangedId = null;
    if (this._panelButton) {
      this._panelButton.setDesktopProxy(null);
      this._panelButton.visible = false;
    }
  }
}
