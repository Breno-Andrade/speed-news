//! The GNOME Shell extension (apps/desktop/gnome-shell-extension) ships inside the app bundle.
//!
//! - Status: is this a GNOME session, is the extension installed, is its button connected?
//! - One-click install: copy it to ~/.local/share/gnome-shell/extensions/<uuid> and add it to the
//!   enabled list in gsettings, so it is active as soon as GNOME Shell reloads (X11: Alt+F2 → r;
//!   Wayland: log out and in). GNOME Shell cannot load a new extension without that reload.
//! - Silent upgrade: when an app update brings a newer extension, the installed copy is replaced
//!   (it takes effect at the next shell reload; the D-Bus interface stays backward compatible).

use std::collections::BTreeMap;
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

/// Every file of an extension directory (relative path → bytes), to compare two copies.
fn read_directory_contents(extension_directory: &Path) -> BTreeMap<PathBuf, Vec<u8>> {
    fn collect_files(root_directory: &Path, current_directory: &Path, file_contents: &mut BTreeMap<PathBuf, Vec<u8>>) {
        let Ok(directory_entries) = fs::read_dir(current_directory) else {
            return;
        };
        for directory_entry in directory_entries.flatten() {
            let entry_path = directory_entry.path();
            if entry_path.is_dir() {
                collect_files(root_directory, &entry_path, file_contents);
            } else if let (Ok(relative_path), Ok(file_bytes)) =
                (entry_path.strip_prefix(root_directory), fs::read(&entry_path))
            {
                file_contents.insert(relative_path.to_path_buf(), file_bytes);
            }
        }
    }
    let mut file_contents = BTreeMap::new();
    collect_files(extension_directory, extension_directory, &mut file_contents);
    file_contents
}

/// Replaces the installed extension when the bundled one is newer, or has the same version but
/// different files (a stale copy from a build that forgot to bump "version": without this, an old
/// extension.js could stay installed forever, e.g. one that never focuses the popover, so
/// "close when clicking outside" never fires). Never downgrades. Returns whether it copied.
fn upgrade_extension_directory(bundled_directory: &Path, installed_directory: &Path) -> bool {
    let (Some(bundled_version), Some(installed_version)) = (
        read_extension_version(bundled_directory),
        read_extension_version(installed_directory),
    ) else {
        return false; // not installed (the user decides) or unreadable
    };
    let needs_upgrade = bundled_version > installed_version
        || (bundled_version == installed_version
            && read_directory_contents(bundled_directory) != read_directory_contents(installed_directory));
    if !needs_upgrade {
        return false;
    }
    match copy_directory(bundled_directory, installed_directory) {
        Ok(()) => true,
        Err(copy_error) => {
            eprintln!("[gnome-extension] upgrade failed: {copy_error}");
            false
        }
    }
}

/// Called at startup: refresh an installed extension that differs from the bundled one.
pub fn upgrade_installed_extension(application_handle: &AppHandle) {
    if let (Some(bundled_directory), Some(installed_directory)) = (
        bundled_extension_directory(application_handle),
        installed_extension_directory(application_handle),
    ) {
        upgrade_extension_directory(&bundled_directory, &installed_directory);
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

    /// Writes an extension directory with the given metadata version and extension.js body.
    fn write_extension(extension_directory: &Path, version: u64, script_body: &str) {
        fs::create_dir_all(extension_directory).unwrap();
        fs::write(
            extension_directory.join("metadata.json"),
            format!(r#"{{"uuid": "speed-news@speednews.dev", "version": {version}}}"#),
        )
        .unwrap();
        fs::write(extension_directory.join("extension.js"), script_body).unwrap();
    }

    fn installed_script(extension_directory: &Path) -> String {
        fs::read_to_string(extension_directory.join("extension.js")).unwrap()
    }

    fn scratch_directory(test_name: &str) -> PathBuf {
        let scratch_directory =
            std::env::temp_dir().join(format!("speed-news-upgrade-{test_name}-{}", std::process::id()));
        let _ = fs::remove_dir_all(&scratch_directory);
        scratch_directory
    }

    /// Regression: an old extension.js (without the code that focuses the popover) stayed installed
    /// because the bundled copy kept the same metadata version, so clicking outside the popover
    /// right after opening it never closed it.
    #[test]
    fn a_stale_copy_with_the_same_version_is_replaced() {
        let scratch_directory = scratch_directory("stale");
        let bundled_directory = scratch_directory.join("bundled");
        let installed_directory = scratch_directory.join("installed");
        write_extension(&bundled_directory, 1, "toggle(); focusAppWindowWhenShown();");
        write_extension(&installed_directory, 1, "toggle();");

        assert!(upgrade_extension_directory(&bundled_directory, &installed_directory));
        assert_eq!(
            installed_script(&installed_directory),
            "toggle(); focusAppWindowWhenShown();"
        );
        // Identical copies are left alone.
        assert!(!upgrade_extension_directory(&bundled_directory, &installed_directory));
        let _ = fs::remove_dir_all(&scratch_directory);
    }

    #[test]
    fn a_newer_bundled_version_replaces_the_installed_one() {
        let scratch_directory = scratch_directory("newer");
        let bundled_directory = scratch_directory.join("bundled");
        let installed_directory = scratch_directory.join("installed");
        write_extension(&bundled_directory, 2, "new");
        write_extension(&installed_directory, 1, "old");

        assert!(upgrade_extension_directory(&bundled_directory, &installed_directory));
        assert_eq!(read_extension_version(&installed_directory), Some(2));
        let _ = fs::remove_dir_all(&scratch_directory);
    }

    #[test]
    fn a_newer_installed_extension_is_never_downgraded() {
        let scratch_directory = scratch_directory("downgrade");
        let bundled_directory = scratch_directory.join("bundled");
        let installed_directory = scratch_directory.join("installed");
        write_extension(&bundled_directory, 1, "old");
        write_extension(&installed_directory, 2, "new");

        assert!(!upgrade_extension_directory(&bundled_directory, &installed_directory));
        assert_eq!(installed_script(&installed_directory), "new");
        let _ = fs::remove_dir_all(&scratch_directory);
    }

    #[test]
    fn an_extension_that_is_not_installed_is_not_installed_silently() {
        let scratch_directory = scratch_directory("missing");
        let bundled_directory = scratch_directory.join("bundled");
        write_extension(&bundled_directory, 1, "script");

        assert!(!upgrade_extension_directory(
            &bundled_directory,
            &scratch_directory.join("installed")
        ));
        assert!(!scratch_directory.join("installed").exists());
        let _ = fs::remove_dir_all(&scratch_directory);
    }

    /// The bundled extension must keep focusing the popover after opening it: GNOME does not focus
    /// a window shown from a D-Bus call, and an unfocused window never reports focus loss.
    #[test]
    fn the_bundled_extension_focuses_the_popover_after_opening_it() {
        let extension_script = include_str!("../../gnome-shell-extension/extension.js");
        assert!(extension_script.contains("if (isVisible) this._focusAppWindowWhenShown()"));
        assert!(extension_script.contains("Main.activateWindow(appWindow)"));
        assert!(extension_script.contains(r#"<arg type="b" name="is_visible" direction="out"/>"#));
    }
}
