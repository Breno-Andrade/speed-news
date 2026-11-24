//! Starts apps/api as a sidecar on a random loopback port with a random token,
//! and hands both to the UI through the `get_api_connection` command.

use std::sync::Mutex;

use serde::Serialize;
use tauri::{AppHandle, Manager, State};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

/// Printed by the Go binary on stdout once it is listening (see cmd/news-api/main.go).
const READY_LINE_PREFIX: &str = "SPEED_NEWS_API_READY port=";

/// Webview origins allowed by the backend CORS policy (dev server, Linux/macOS, Windows).
const ALLOWED_ORIGINS: &str = "http://localhost:1420,tauri://localhost,http://tauri.localhost";

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ApiConnection {
    base_url: String,
    token: String,
}

#[derive(Default)]
pub struct SidecarState {
    api_connection: Mutex<Option<ApiConnection>>,
    sidecar_child: Mutex<Option<CommandChild>>,
}

/// Returns `None` while the sidecar is still starting; the UI polls until it is ready.
#[tauri::command]
pub fn get_api_connection(sidecar_state: State<'_, SidecarState>) -> Option<ApiConnection> {
    sidecar_state
        .api_connection
        .lock()
        .ok()
        .and_then(|connection_guard| connection_guard.clone())
}

pub fn start_sidecar(application_handle: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    let api_token = uuid::Uuid::new_v4().simple().to_string();
    let data_directory = application_handle.path().app_data_dir()?;

    let sidecar_arguments: Vec<String> = vec![
        "--exit-on-stdin-close".into(),
        "--data-dir".into(),
        data_directory.to_string_lossy().into_owned(),
        "--allowed-origins".into(),
        ALLOWED_ORIGINS.into(),
    ];

    let (mut event_receiver, sidecar_child) = application_handle
        .shell()
        .sidecar("news-api")?
        .args(sidecar_arguments)
        .env("SPEED_NEWS_API_TOKEN", &api_token)
        .spawn()?;

    if let Ok(mut child_guard) = application_handle.state::<SidecarState>().sidecar_child.lock() {
        *child_guard = Some(sidecar_child);
    }

    let event_application_handle = application_handle.clone();
    tauri::async_runtime::spawn(async move {
        while let Some(command_event) = event_receiver.recv().await {
            match command_event {
                CommandEvent::Stdout(output_bytes) => {
                    let output_line = String::from_utf8_lossy(&output_bytes);
                    if let Some(port_text) = output_line.trim().strip_prefix(READY_LINE_PREFIX) {
                        let api_connection = ApiConnection {
                            base_url: format!("http://127.0.0.1:{}", port_text.trim()),
                            token: api_token.clone(),
                        };
                        if let Ok(mut connection_guard) =
                            event_application_handle.state::<SidecarState>().api_connection.lock()
                        {
                            *connection_guard = Some(api_connection);
                        }
                    }
                }
                CommandEvent::Stderr(output_bytes) => {
                    eprintln!("[news-api] {}", String::from_utf8_lossy(&output_bytes).trim_end());
                }
                CommandEvent::Terminated(termination_payload) => {
                    eprintln!("[news-api] exited with code {:?}", termination_payload.code);
                    if let Ok(mut connection_guard) =
                        event_application_handle.state::<SidecarState>().api_connection.lock()
                    {
                        *connection_guard = None;
                    }
                }
                _ => {}
            }
        }
    });

    Ok(())
}

pub fn stop_sidecar(application_handle: &AppHandle) {
    let sidecar_child = application_handle
        .state::<SidecarState>()
        .sidecar_child
        .lock()
        .ok()
        .and_then(|mut child_guard| child_guard.take());
    if let Some(sidecar_child) = sidecar_child {
        let _ = sidecar_child.kill();
    }
}
