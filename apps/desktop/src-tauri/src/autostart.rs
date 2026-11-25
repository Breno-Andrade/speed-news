//! Start at login. A tray news app that does not come back after a reboot silently stops
//! delivering news, so installed builds turn it on once, at first run. After that the user's
//! choice (Settings → General) is the only thing that changes it.

use std::fs;

use tauri::{AppHandle, Manager};
use tauri_plugin_autostart::ManagerExt;

const FIRST_RUN_MARKER: &str = "autostart-initialized";

/// Enables start at login, then fixes the Linux autostart entry (see `quote_exec_path`).
fn enable_launch_at_login(application_handle: &AppHandle) -> Result<(), String> {
    application_handle
        .autolaunch()
        .enable()
        .map_err(|autostart_error| autostart_error.to_string())?;
    #[cfg(target_os = "linux")]
    quote_linux_autostart_entry(application_handle);
    Ok(())
}

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
    if let Err(autostart_error) = enable_launch_at_login(application_handle) {
        eprintln!("[autostart] could not enable: {autostart_error}");
        return; // try again next launch
    }
    let _ = fs::create_dir_all(&config_directory);
    let _ = fs::write(&marker_path, "");
}

#[tauri::command]
pub fn is_launch_at_login_enabled(application_handle: AppHandle) -> Result<bool, String> {
    application_handle
        .autolaunch()
        .is_enabled()
        .map_err(|autostart_error| autostart_error.to_string())
}

#[tauri::command]
pub fn set_launch_at_login(application_handle: AppHandle, is_enabled: bool) -> Result<(), String> {
    if is_enabled {
        enable_launch_at_login(&application_handle)
    } else {
        application_handle
            .autolaunch()
            .disable()
            .map_err(|autostart_error| autostart_error.to_string())
    }
}

/// The autostart crate writes `Exec=<path> ` unquoted, so a path with a space (an AppImage kept
/// in "~/Meus Apps", for example) never starts. Desktop entries require quoting such paths.
#[cfg(target_os = "linux")]
fn quote_linux_autostart_entry(application_handle: &AppHandle) {
    let Ok(config_directory) = application_handle.path().config_dir() else {
        return;
    };
    let product_name = application_handle
        .config()
        .product_name
        .clone()
        .unwrap_or_else(|| "Speed News".into());
    let entry_path = config_directory
        .join("autostart")
        .join(format!("{product_name}.desktop"));
    let Ok(entry_text) = fs::read_to_string(&entry_path) else {
        return;
    };
    let quoted_text = quote_exec_path(&entry_text);
    if quoted_text != entry_text {
        let _ = fs::write(&entry_path, quoted_text);
    }
}

/// `Exec=/a b/App.AppImage ` → `Exec="/a b/App.AppImage"`. Lines already quoted are left alone.
#[cfg_attr(not(target_os = "linux"), allow(dead_code))]
fn quote_exec_path(entry_text: &str) -> String {
    entry_text
        .lines()
        .map(|entry_line| match entry_line.strip_prefix("Exec=") {
            Some(exec_value) if !exec_value.trim_start().starts_with('"') && exec_value.trim().contains(' ') => {
                let escaped_path = exec_value
                    .trim()
                    .replace('\\', "\\\\")
                    .replace('"', "\\\"")
                    .replace('`', "\\`")
                    .replace('$', "\\$");
                format!("Exec=\"{escaped_path}\"")
            }
            _ => entry_line.to_string(),
        })
        .collect::<Vec<_>>()
        .join("\n")
        + if entry_text.ends_with('\n') { "\n" } else { "" }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn quotes_an_exec_path_with_spaces() {
        let entry_text =
            "[Desktop Entry]\nName=Speed News\nExec=/home/user/Meus Apps/Speed.News.AppImage \nTerminal=false";
        assert_eq!(
            quote_exec_path(entry_text),
            "[Desktop Entry]\nName=Speed News\nExec=\"/home/user/Meus Apps/Speed.News.AppImage\"\nTerminal=false"
        );
    }

    #[test]
    fn leaves_simple_and_already_quoted_paths_alone() {
        let simple_entry = "Exec=/opt/speed-news \n";
        assert_eq!(quote_exec_path(simple_entry), simple_entry);
        let quoted_entry = "Exec=\"/a b/app\"\n";
        assert_eq!(quote_exec_path(quoted_entry), quoted_entry);
    }
}
