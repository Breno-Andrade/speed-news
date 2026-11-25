//! Native shell of Speed News: tray icon, popover window and the Go sidecar lifecycle.
//! All business logic lives in apps/api (Go) and in the React UI.

mod autostart;
mod gnome_extension;
#[cfg(target_os = "linux")]
mod panel_bridge;
mod sidecar;
mod tray;
mod updates;

use tauri::{AppHandle, Manager, RunEvent, WindowEvent};

/// Lets the UI offer a "Quit" button. Exiting triggers RunEvent::Exit, which stops the sidecar.
#[tauri::command]
fn quit_app(application_handle: AppHandle) {
    application_handle.exit(0);
}

pub fn run() {
    let application_builder = tauri::Builder::default()
        // Must be the first plugin. Opening the app again (menu, autostart + manual launch) shows the
        // running instance's popover instead of starting a second app and sidecar on the same database.
        .plugin(tauri_plugin_single_instance::init(
            |application_handle, _arguments, _working_directory| {
                tray::show_main_window(application_handle, None);
            },
        ))
        .plugin(tauri_plugin_autostart::Builder::new().build())
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_positioner::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .manage(sidecar::SidecarState::default())
        .manage(tray::PopoverState::default())
        .manage(updates::PendingUpdateState::default());
    #[cfg(target_os = "linux")]
    let application_builder = application_builder.manage(panel_bridge::PanelBridgeState::default());

    let application = application_builder
        .invoke_handler(tauri::generate_handler![
            sidecar::get_api_connection,
            tray::update_tray,
            tray::set_close_on_focus_loss,
            tray::hide_main_window,
            updates::install_pending_update,
            gnome_extension::gnome_extension_status,
            gnome_extension::install_gnome_extension,
            autostart::is_launch_at_login_enabled,
            autostart::set_launch_at_login,
            quit_app
        ])
        .setup(|application| {
            // macOS: live only in the menu bar, no Dock icon.
            #[cfg(target_os = "macos")]
            application.set_activation_policy(tauri::ActivationPolicy::Accessory);

            tray::create_tray(application.handle())?;
            // GNOME panel button service (optional: without D-Bus the tray keeps working).
            #[cfg(target_os = "linux")]
            panel_bridge::start(application.handle());
            sidecar::start_sidecar(application.handle())?;
            updates::start_background_updates(application.handle());
            // An app update may bring a newer panel button: refresh the installed copy.
            gnome_extension::upgrade_installed_extension(application.handle());
            autostart::enable_on_first_run(application.handle());
            Ok(())
        })
        .on_window_event(|window, window_event| match window_event {
            // Popover behaviour: clicking anywhere else hides it (unless the user turned it off).
            WindowEvent::Focused(false) => {
                let popover_state = window.state::<tray::PopoverState>();
                if popover_state.closes_on_focus_loss() && window.is_visible().unwrap_or(false) {
                    popover_state.remember_hidden_by_focus_loss();
                    let _ = window.hide();
                }
            }
            // Closing only hides; the app keeps running in the tray.
            WindowEvent::CloseRequested { api, .. } => {
                api.prevent_close();
                let _ = window.hide();
            }
            _ => {}
        })
        .build(tauri::generate_context!())
        .expect("error while building Speed News");

    application.run(|application_handle, run_event| {
        if let RunEvent::Exit = run_event {
            sidecar::stop_sidecar(application_handle);
        }
    });
}
