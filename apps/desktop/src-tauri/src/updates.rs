//! Automatic updates from GitHub Releases (tauri-plugin-updater, signed with the key whose public
//! half is in tauri.conf.json; unsigned or tampered downloads are rejected by the plugin).
//!
//! - AppImage, Windows and macOS: download in the background, install while the popover is
//!   hidden, restart. The user only sees "Updated to vX" next time they open the app.
//! - .deb / .rpm: installing a system package needs the admin password, so a sudden prompt would
//!   come out of nowhere. The update is downloaded and the UI offers "Update (asks for password)".
//! - Development builds (`make dev`, no bundle): no update checks at all.

use std::sync::Mutex;
use std::time::Duration;

use serde::Serialize;
use tauri::utils::config::BundleType;
use tauri::{AppHandle, Emitter, Manager};
use tauri_plugin_updater::{Update, UpdaterExt};

const FIRST_CHECK_DELAY: Duration = Duration::from_secs(60);
const CHECK_INTERVAL: Duration = Duration::from_secs(6 * 60 * 60);
const HIDDEN_WINDOW_POLL_INTERVAL: Duration = Duration::from_secs(30);

/// An update downloaded for a .deb/.rpm install, waiting for the user's go-ahead.
#[derive(Default)]
pub struct PendingUpdateState {
    pending_update: Mutex<Option<(Update, Vec<u8>)>>,
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateReadyPayload {
    version: String,
    requires_password: bool,
}

/// How an update can be applied for the way the app was installed.
#[derive(Debug, PartialEq, Eq)]
enum InstallStrategy {
    /// Replace the app silently while the popover is hidden.
    Silent,
    /// Ask first: installing needs the admin password.
    AskFirst,
}

fn install_strategy(bundle_type: Option<BundleType>) -> Option<InstallStrategy> {
    match bundle_type? {
        BundleType::Deb | BundleType::Rpm => Some(InstallStrategy::AskFirst),
        BundleType::AppImage | BundleType::Msi | BundleType::Nsis | BundleType::App | BundleType::Dmg => {
            Some(InstallStrategy::Silent)
        }
    }
}

pub fn start_background_updates(application_handle: &AppHandle) {
    let Some(strategy) = install_strategy(tauri::utils::platform::bundle_type()) else {
        return; // development build or unknown bundle: nothing to update
    };
    let update_application_handle = application_handle.clone();
    tauri::async_runtime::spawn(async move {
        tokio_sleep(FIRST_CHECK_DELAY).await;
        loop {
            if let Err(update_error) = check_and_apply(&update_application_handle, &strategy).await {
                // Offline, GitHub down, bad signature...: try again at the next check.
                eprintln!("[updates] {update_error}");
            }
            tokio_sleep(CHECK_INTERVAL).await;
        }
    });
}

async fn check_and_apply(application_handle: &AppHandle, strategy: &InstallStrategy) -> Result<(), String> {
    let updater = application_handle
        .updater()
        .map_err(|updater_error| updater_error.to_string())?;
    let Some(update) = updater.check().await.map_err(|check_error| check_error.to_string())? else {
        return Ok(());
    };
    let update_bytes = update
        .download(|_, _| {}, || {})
        .await
        .map_err(|download_error| download_error.to_string())?;

    match strategy {
        InstallStrategy::Silent => {
            wait_until_popover_hidden(application_handle).await;
            update
                .install(update_bytes)
                .map_err(|install_error| install_error.to_string())?;
            application_handle.restart();
        }
        InstallStrategy::AskFirst => {
            let ready_payload = ready_payload(&update.version);
            if let Ok(mut pending_guard) = application_handle.state::<PendingUpdateState>().pending_update.lock() {
                *pending_guard = Some((update, update_bytes));
            }
            let _ = application_handle.emit("update-ready", ready_payload);
            Ok(())
        }
    }
}

/// Never replace the app under the user's eyes: wait until the popover is closed (or destroyed).
async fn wait_until_popover_hidden(application_handle: &AppHandle) {
    while crate::popover::is_popover_visible(application_handle) {
        tokio_sleep(HIDDEN_WINDOW_POLL_INTERVAL).await;
    }
}

async fn tokio_sleep(sleep_duration: Duration) {
    tauri::async_runtime::spawn_blocking(move || std::thread::sleep(sleep_duration))
        .await
        .ok();
}

/// The update downloaded for a .deb/.rpm install, if any. The UI asks on every mount: the
/// "update-ready" event is lost when it fires while the popover webview does not exist.
#[tauri::command]
pub fn pending_update(pending_update_state: tauri::State<'_, PendingUpdateState>) -> Option<UpdateReadyPayload> {
    pending_update_state
        .pending_update
        .lock()
        .ok()
        .and_then(|pending_guard| pending_guard.as_ref().map(|(update, _)| ready_payload(&update.version)))
}

fn ready_payload(version: &str) -> UpdateReadyPayload {
    UpdateReadyPayload {
        version: version.to_string(),
        requires_password: true,
    }
}

/// "Update (asks for password)" in the UI, for .deb/.rpm installs.
#[tauri::command]
pub fn install_pending_update(application_handle: AppHandle) -> Result<(), String> {
    let pending_update = application_handle
        .state::<PendingUpdateState>()
        .pending_update
        .lock()
        .map_err(|lock_error| lock_error.to_string())?
        .take();
    let Some((update, update_bytes)) = pending_update else {
        return Err("no update is waiting".into());
    };
    update
        .install(update_bytes)
        .map_err(|install_error| install_error.to_string())?;
    application_handle.restart();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn self_contained_bundles_update_silently() {
        for bundle_type in [BundleType::AppImage, BundleType::Msi, BundleType::Nsis, BundleType::App] {
            assert_eq!(install_strategy(Some(bundle_type)), Some(InstallStrategy::Silent));
        }
    }

    #[test]
    fn system_packages_ask_before_the_password_prompt() {
        assert_eq!(install_strategy(Some(BundleType::Deb)), Some(InstallStrategy::AskFirst));
        assert_eq!(install_strategy(Some(BundleType::Rpm)), Some(InstallStrategy::AskFirst));
    }

    #[test]
    fn development_builds_never_update() {
        assert_eq!(install_strategy(None), None);
    }
}
