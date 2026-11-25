//! D-Bus service used by the Speed News GNOME Shell extension (apps/desktop/gnome-shell-extension).
//!
//! On Ubuntu's GNOME, the AppIndicator extension opens the tray menu on every single left click,
//! so a tray icon can never open the popover with one click. Our own panel button can: it calls
//! `ToggleWindow` here and reads the unread count from the `UnreadCount` property.
//!
//! While the button is registered, the tray icon is hidden to avoid showing the app twice. If the
//! extension goes away without unregistering (GNOME Shell restart or crash), the tray comes back.

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;
use std::thread;

use tauri::{AppHandle, Manager, PhysicalPosition};
use zbus::blocking::connection::Builder as ConnectionBuilder;
use zbus::blocking::fdo::DBusProxy;
use zbus::blocking::Connection;
use zbus::interface;
use zbus::message::Header;

use crate::tray;

pub const BUS_NAME: &str = "dev.speednews.Desktop";
pub const OBJECT_PATH: &str = "/dev/speednews/Desktop";

pub struct PanelBridge {
    application_handle: AppHandle,
    unread_count: u32,
    open_label: String,
    quit_label: String,
}

#[interface(name = "dev.speednews.Desktop1")]
impl PanelBridge {
    /// Click on the panel button. The anchor is the bottom center of the button, in screen pixels.
    /// Returns true when the popover ended up visible: the extension then gives it keyboard focus.
    /// (GNOME's focus-stealing prevention does not focus a window shown from a D-Bus call; without
    /// focus, "close when clicking outside" never triggers.)
    fn toggle_window(&self, anchor_x: i32, anchor_y: i32) -> bool {
        tray::toggle_main_window(
            &self.application_handle,
            Some(PhysicalPosition::new(anchor_x, anchor_y)),
        )
    }

    /// "Open" in the panel button menu.
    fn show_window(&self, anchor_x: i32, anchor_y: i32) {
        tray::show_main_window(
            &self.application_handle,
            Some(PhysicalPosition::new(anchor_x, anchor_y)),
        );
    }

    fn quit(&self) {
        self.application_handle.exit(0);
    }

    /// Called by the extension when its button is on the panel: hide the duplicate tray icon,
    /// and bring it back if the caller disappears from the bus without unregistering.
    fn register_panel_button(&self, #[zbus(header)] message_header: Header<'_>) {
        tray::set_tray_visible(&self.application_handle, false);
        set_panel_button_active(&self.application_handle, true);

        let Some(caller_name) = message_header.sender().map(|sender_name| sender_name.to_string()) else {
            return;
        };
        let watcher_application_handle = self.application_handle.clone();
        thread::spawn(move || {
            restore_tray_when_caller_leaves(watcher_application_handle, caller_name);
        });
    }

    fn unregister_panel_button(&self) {
        tray::set_tray_visible(&self.application_handle, true);
        set_panel_button_active(&self.application_handle, false);
    }

    #[zbus(property)]
    fn unread_count(&self) -> u32 {
        self.unread_count
    }

    #[zbus(property)]
    fn open_label(&self) -> String {
        self.open_label.clone()
    }

    #[zbus(property)]
    fn quit_label(&self) -> String {
        self.quit_label.clone()
    }
}

fn restore_tray_when_caller_leaves(application_handle: AppHandle, caller_name: String) {
    let Some(connection) = bridge_connection(&application_handle) else {
        return;
    };
    let Ok(bus_proxy) = DBusProxy::new(&connection) else {
        return;
    };
    let Ok(name_owner_changes) = bus_proxy.receive_name_owner_changed_with_args(&[(0, caller_name.as_str())]) else {
        return;
    };
    for name_owner_change in name_owner_changes {
        let caller_is_gone = name_owner_change
            .args()
            .map(|change_arguments| change_arguments.new_owner().is_none())
            .unwrap_or(false);
        if caller_is_gone {
            tray::set_tray_visible(&application_handle, true);
            set_panel_button_active(&application_handle, false);
            return;
        }
    }
}

/// Holds the session bus connection; None when D-Bus is unavailable (the tray keeps working).
#[derive(Default)]
pub struct PanelBridgeState {
    connection: Mutex<Option<Connection>>,
    /// True while the GNOME extension's button is registered (the app then hides its tray icon).
    is_panel_button_active: AtomicBool,
}

fn bridge_connection(application_handle: &AppHandle) -> Option<Connection> {
    application_handle
        .state::<PanelBridgeState>()
        .connection
        .lock()
        .ok()
        .and_then(|connection_guard| connection_guard.clone())
}

pub fn start(application_handle: &AppHandle) {
    let panel_bridge = PanelBridge {
        application_handle: application_handle.clone(),
        unread_count: 0,
        open_label: "Abrir notícias".into(),
        quit_label: "Sair".into(),
    };
    let connection_result = ConnectionBuilder::session()
        .and_then(|connection_builder| connection_builder.name(BUS_NAME))
        .and_then(|connection_builder| connection_builder.serve_at(OBJECT_PATH, panel_bridge))
        .and_then(|connection_builder| connection_builder.build());

    match connection_result {
        Ok(connection) => {
            if let Ok(mut connection_guard) = application_handle.state::<PanelBridgeState>().connection.lock() {
                *connection_guard = Some(connection);
            }
        }
        Err(connection_error) => eprintln!("[panel-bridge] D-Bus unavailable, tray only: {connection_error}"),
    }
}

/// Mirrors what the tray shows, so the panel button stays in sync (count and translated labels).
pub fn publish_tray_state(application_handle: &AppHandle, unread_count: u32, open_label: &str, quit_label: &str) {
    let Some(connection) = bridge_connection(application_handle) else {
        return;
    };
    let Ok(interface_reference) = connection.object_server().interface::<_, PanelBridge>(OBJECT_PATH) else {
        return;
    };
    let mut panel_bridge = interface_reference.get_mut();
    let signal_emitter = interface_reference.signal_emitter();

    if panel_bridge.unread_count != unread_count {
        panel_bridge.unread_count = unread_count;
        let _ = zbus::block_on(panel_bridge.unread_count_changed(signal_emitter));
    }
    if panel_bridge.open_label != open_label {
        panel_bridge.open_label = open_label.to_string();
        let _ = zbus::block_on(panel_bridge.open_label_changed(signal_emitter));
    }
    if panel_bridge.quit_label != quit_label {
        panel_bridge.quit_label = quit_label.to_string();
        let _ = zbus::block_on(panel_bridge.quit_label_changed(signal_emitter));
    }
}

fn set_panel_button_active(application_handle: &AppHandle, is_active: bool) {
    application_handle
        .state::<PanelBridgeState>()
        .is_panel_button_active
        .store(is_active, Ordering::Relaxed);
}

/// Whether the GNOME panel button is currently connected to this app.
pub fn is_panel_button_active(application_handle: &AppHandle) -> bool {
    application_handle
        .state::<PanelBridgeState>()
        .is_panel_button_active
        .load(Ordering::Relaxed)
}
