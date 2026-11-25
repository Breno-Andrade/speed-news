//! The GNOME Shell extension (apps/desktop/gnome-shell-extension) ships inside the app bundle.
//!
//! - Status: is this a GNOME session, is the extension installed, is its button connected?
//! - One-click install: copy it to ~/.local/share/gnome-shell/extensions/<uuid> and add it to the
//!   enabled list in gsettings, so it is active as soon as GNOME Shell reloads (X11: Alt+F2 → r;
//!   Wayland: log out and in). GNOME Shell cannot load a new extension without that reload.
//! - Silent upgrade: when an app update brings a newer extension, the installed copy is replaced
//!   (it takes effect at the next shell reload; the D-Bus interface stays backward compatible).

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use serde::Serialize;
use tauri::{AppHandle, Manager};

pub const EXTENSION_UUID: &str = "speed-news@speednews.dev";
const BUNDLED_DIRECTORY_NAME: &str = "gnome-shell-extension";

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct GnomeExtensionStatus {
    pub is_gnome_session: bool,
    /// "x11", "wayland" or "unknown": decides how to explain the shell reload.
    pub session_type: String,
    pub is_installed: bool,
    /// The extension's button is connected to the app right now.
    pub is_active: bool,
}

fn is_gnome_session() -> bool {
    cfg!(target_os = "linux")
        && std::env::var("XDG_CURRENT_DESKTOP")
            .map(|current_desktop| current_desktop.to_ascii_uppercase().contains("GNOME"))
            .unwrap_or(false)
}

fn session_type() -> String {
    match std::env::var("XDG_SESSION_TYPE").map(|session_value| session_value.to_ascii_lowercase()) {
        Ok(session_value) if session_value == "x11" || session_value == "wayland" => session_value,
        _ => "unknown".into(),
    }
}

fn installed_extension_directory(application_handle: &AppHandle) -> Option<PathBuf> {
    let user_data_directory = application_handle.path().data_dir().ok()?;
    Some(
        user_data_directory
            .join("gnome-shell")
            .join("extensions")
            .join(EXTENSION_UUID),
    )
}

fn bundled_extension_directory(application_handle: &AppHandle) -> Option<PathBuf> {
    let resource_directory = application_handle.path().resource_dir().ok()?;
    let bundled_directory = resource_directory.join(BUNDLED_DIRECTORY_NAME);
    bundled_directory
        .join("metadata.json")
        .is_file()
        .then_some(bundled_directory)
}

/// The integer "version" field of an extension's metadata.json.
fn read_extension_version(extension_directory: &Path) -> Option<u64> {
    let metadata_text = fs::read_to_string(extension_directory.join("metadata.json")).ok()?;
    serde_json::from_str::<serde_json::Value>(&metadata_text)
        .ok()?
        .get("version")?
        .as_u64()
}

fn copy_directory(source_directory: &Path, target_directory: &Path) -> std::io::Result<()> {
    fs::create_dir_all(target_directory)?;
    for directory_entry in fs::read_dir(source_directory)? {
        let directory_entry = directory_entry?;
        let target_path = target_directory.join(directory_entry.file_name());
        if directory_entry.file_type()?.is_dir() {
            copy_directory(&directory_entry.path(), &target_path)?;
        } else {
            fs::copy(directory_entry.path(), target_path)?;
        }
    }
    Ok(())
}

/// Adds `extension_uuid` to the output of `gsettings get org.gnome.shell enabled-extensions`
/// ("['a@b', 'c@d']" or "@as []"). Returns None when it is already enabled.
fn with_extension_enabled(enabled_extensions_value: &str, extension_uuid: &str) -> Option<String> {
    let enabled_uuids: Vec<String> = enabled_extensions_value
        .trim()
        .trim_start_matches("@as")
        .trim()
        .trim_start_matches('[')
        .trim_end_matches(']')
        .split(',')
        .map(|quoted_uuid| quoted_uuid.trim().trim_matches('\'').trim_matches('"').to_string())
        .filter(|enabled_uuid| !enabled_uuid.is_empty())
        .collect();
    if enabled_uuids.iter().any(|enabled_uuid| enabled_uuid == extension_uuid) {
        return None;
    }
    let quoted_uuids: Vec<String> = enabled_uuids
        .iter()
        .chain(std::iter::once(&extension_uuid.to_string()))
        .map(|enabled_uuid| format!("'{enabled_uuid}'"))
        .collect();
    Some(format!("[{}]", quoted_uuids.join(", ")))
}

fn enable_extension_in_settings() -> Result<(), String> {
    let current_output = Command::new("gsettings")
        .args(["get", "org.gnome.shell", "enabled-extensions"])
        .output()
        .map_err(|command_error| format!("gsettings unavailable: {command_error}"))?;
    let current_value = String::from_utf8_lossy(&current_output.stdout);
    if let Some(updated_value) = with_extension_enabled(&current_value, EXTENSION_UUID) {
        let set_status = Command::new("gsettings")
            .args(["set", "org.gnome.shell", "enabled-extensions", &updated_value])
            .status()
            .map_err(|command_error| command_error.to_string())?;
        if !set_status.success() {
            return Err("gsettings could not enable the extension".into());
        }
    }
    // Also works right away when the shell already knows the extension (e.g. reinstall).
    let _ = Command::new("gnome-extensions")
        .args(["enable", EXTENSION_UUID])
        .status();
    Ok(())
}

fn is_panel_button_active(_application_handle: &AppHandle) -> bool {
    #[cfg(target_os = "linux")]
    return crate::panel_bridge::is_panel_button_active(_application_handle);
    #[cfg(not(target_os = "linux"))]
    return false;
}

#[tauri::command]
pub fn gnome_extension_status(application_handle: AppHandle) -> GnomeExtensionStatus {
    GnomeExtensionStatus {
        is_gnome_session: is_gnome_session(),
        session_type: session_type(),
        is_installed: installed_extension_directory(&application_handle)
            .is_some_and(|installed_directory| installed_directory.join("metadata.json").is_file()),
        is_active: is_panel_button_active(&application_handle),
    }
}

#[tauri::command]
pub fn install_gnome_extension(application_handle: AppHandle) -> Result<GnomeExtensionStatus, String> {
    if !is_gnome_session() {
        return Err("not a GNOME session".into());
    }
    let bundled_directory =
        bundled_extension_directory(&application_handle).ok_or("the extension is not bundled with this build")?;
    let installed_directory = installed_extension_directory(&application_handle).ok_or("no user data directory")?;
    copy_directory(&bundled_directory, &installed_directory).map_err(|copy_error| copy_error.to_string())?;
    enable_extension_in_settings()?;
    Ok(gnome_extension_status(application_handle))
}

/// Called at startup: replace an installed extension older than the bundled one.
pub fn upgrade_installed_extension(application_handle: &AppHandle) {
    let (Some(bundled_directory), Some(installed_directory)) = (
        bundled_extension_directory(application_handle),
        installed_extension_directory(application_handle),
    ) else {
        return;
    };
    let (Some(bundled_version), Some(installed_version)) = (
        read_extension_version(&bundled_directory),
        read_extension_version(&installed_directory),
    ) else {
        return; // not installed (the user decides) or unreadable
    };
    if bundled_version > installed_version {
        if let Err(copy_error) = copy_directory(&bundled_directory, &installed_directory) {
            eprintln!("[gnome-extension] upgrade failed: {copy_error}");
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn enabling_appends_to_the_existing_list() {
        assert_eq!(
            with_extension_enabled("['ubuntu-dock@ubuntu.com', 'ding@rastersoft.com']\n", EXTENSION_UUID),
            Some("['ubuntu-dock@ubuntu.com', 'ding@rastersoft.com', 'speed-news@speednews.dev']".into())
        );
    }

    #[test]
    fn enabling_works_on_an_empty_list() {
        assert_eq!(
            with_extension_enabled("@as []", EXTENSION_UUID),
            Some("['speed-news@speednews.dev']".into())
        );
    }

    #[test]
    fn enabling_an_already_enabled_extension_changes_nothing() {
        assert_eq!(
            with_extension_enabled("['speed-news@speednews.dev']", EXTENSION_UUID),
            None
        );
    }

    #[test]
    fn reads_the_metadata_version_and_copies_the_tree() {
        let scratch_directory = std::env::temp_dir().join(format!("speed-news-extension-test-{}", std::process::id()));
        let source_directory = scratch_directory.join("source");
        fs::create_dir_all(source_directory.join("icons")).unwrap();
        fs::write(
            source_directory.join("metadata.json"),
            r#"{"uuid": "x@y", "version": 3}"#,
        )
        .unwrap();
        fs::write(source_directory.join("icons").join("flag.svg"), "<svg/>").unwrap();

        let target_directory = scratch_directory.join("target");
        copy_directory(&source_directory, &target_directory).unwrap();

        assert_eq!(read_extension_version(&target_directory), Some(3));
        assert!(target_directory.join("icons").join("flag.svg").is_file());
        assert_eq!(read_extension_version(&scratch_directory.join("missing")), None);
        let _ = fs::remove_dir_all(&scratch_directory);
    }
}
