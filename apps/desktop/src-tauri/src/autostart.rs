//! Start at login. A tray news app that does not come back after a reboot silently stops
//! delivering news, so installed builds turn it on once, at first run. After that the user's
//! choice (Settings → General) is the only thing that changes it.

use std::fs;

use tauri::{AppHandle, Manager};
use tauri_plugin_autostart::ManagerExt;

const FIRST_RUN_MARKER: &str = "autostart-initialized";

pub fn enable_on_first_run(application_handle: &AppHandle) {
    // Dev builds (`tauri dev`) must never register themselves to start at login.
    if tauri::utils::platform::bundle_type().is_none() {
        return;
    }
    let Ok(config_directory) = application_handle.path().app_config_dir() else {
        return;
    };
    let marker_path = config_directory.join(FIRST_RUN_MARKER);
    if marker_path.exists() {
        return;
    }
    if let Err(autostart_error) = application_handle.autolaunch().enable() {
        eprintln!("[autostart] could not enable: {autostart_error}");
        return; // try again next launch
    }
    let _ = fs::create_dir_all(&config_directory);
    let _ = fs::write(&marker_path, "");
}
