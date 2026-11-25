//! Tray icon and unread count.
//!
//! - macOS: the count is native text next to the menu bar icon (`TrayIcon::set_title`).
//! - Linux and Windows: the icon switches to a red-dot variant when there is something unread;
//!   the exact count is shown in the popover header and in the tooltip. (Linux uses the ksni
//!   backend for left clicks, and ksni cannot show text next to the icon.)
//!
//! Texts (tooltip and menu) come from the UI already translated, so the tray follows the
//! interface language chosen in the settings.
//!
//! Clicks: left click toggles the popover, right click shows the menu. On Linux this needs the
//! `ksni` tray backend (see Cargo.toml): the default AppIndicator backend turns every click into
//! "open the menu" and never tells the app.

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;
use std::time::{Duration, Instant};

use tauri::image::Image;
use tauri::menu::{Menu, MenuItem};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{AppHandle, Emitter, Manager, PhysicalPosition, Wry};

const TRAY_ID: &str = "speed-news-tray";
const MAIN_WINDOW_LABEL: &str = "main";
const OPEN_MENU_ID: &str = "open";
const QUIT_MENU_ID: &str = "quit";

const TRAY_ICON_DEFAULT: &[u8] = include_bytes!("../icons/tray/tray-default.png");
#[cfg(not(target_os = "macos"))]
const TRAY_ICON_UNREAD: &[u8] = include_bytes!("../icons/tray/tray-unread.png");

/// Clicking the tray icon while the popover is open first steals its focus (which hides it) and
/// only then delivers the click. Without this guard that click would reopen it immediately.
const FOCUS_LOSS_CLICK_WINDOW: Duration = Duration::from_millis(400);

pub struct PopoverState {
    hidden_by_focus_loss_at: Mutex<Option<Instant>>,
    /// User setting "close when clicking outside"; mirrored from the UI at startup and on change.
    close_on_focus_loss: AtomicBool,
}

impl Default for PopoverState {
    fn default() -> Self {
        Self {
            hidden_by_focus_loss_at: Mutex::new(None),
            close_on_focus_loss: AtomicBool::new(true),
        }
    }
}

impl PopoverState {
    pub fn closes_on_focus_loss(&self) -> bool {
        self.close_on_focus_loss.load(Ordering::Relaxed)
    }

    pub fn remember_hidden_by_focus_loss(&self) {
        if let Ok(mut hidden_at_guard) = self.hidden_by_focus_loss_at.lock() {
            *hidden_at_guard = Some(Instant::now());
        }
    }

    /// True when the popover was hidden by focus loss a moment ago, i.e. this click meant "close".
    fn was_just_hidden_by_focus_loss(&self) -> bool {
        self.hidden_by_focus_loss_at
            .lock()
            .ok()
            .and_then(|mut hidden_at_guard| hidden_at_guard.take())
            .is_some_and(|hidden_at| hidden_at.elapsed() < FOCUS_LOSS_CLICK_WINDOW)
    }
}

/// Kept so the UI can rename the entries when the interface language changes.
struct TrayMenuItems {
    open_menu_item: MenuItem<Wry>,
    quit_menu_item: MenuItem<Wry>,
}

pub fn create_tray(application_handle: &AppHandle) -> tauri::Result<()> {
    // On Linux a left click usually does not reach the app (AppIndicator limitation),
    // so the menu always offers an explicit "open" entry. Initial labels are replaced by the
    // UI's translated ones as soon as it loads.
    let open_menu_item = MenuItem::with_id(application_handle, OPEN_MENU_ID, "Abrir notícias", true, None::<&str>)?;
    let quit_menu_item = MenuItem::with_id(application_handle, QUIT_MENU_ID, "Sair", true, None::<&str>)?;
    let tray_menu = Menu::with_items(application_handle, &[&open_menu_item, &quit_menu_item])?;
    application_handle.manage(TrayMenuItems {
        open_menu_item: open_menu_item.clone(),
        quit_menu_item: quit_menu_item.clone(),
    });

    TrayIconBuilder::with_id(TRAY_ID)
        .icon(Image::from_bytes(TRAY_ICON_DEFAULT)?)
        .tooltip("Speed News")
        .menu(&tray_menu)
        .show_menu_on_left_click(false)
        .on_menu_event(|application_handle, menu_event| match menu_event.id().as_ref() {
            OPEN_MENU_ID => show_main_window(application_handle, None),
            QUIT_MENU_ID => application_handle.exit(0),
            _ => {}
        })
        .on_tray_icon_event(|tray_icon, tray_event| {
            // Lets the positioner know where the tray is (Windows/macOS).
            tauri_plugin_positioner::on_tray_event(tray_icon.app_handle(), &tray_event);
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = tray_event
            {
                toggle_main_window(tray_icon.app_handle(), None);
            }
        })
        .build(application_handle)?;
    Ok(())
}

/// Left click on the icon (or GNOME panel button): open if closed, close if open.
/// `anchor` is the bottom center of the clicked button in screen pixels, when known.
/// Returns whether the popover is visible afterwards.
pub fn toggle_main_window(application_handle: &AppHandle, anchor: Option<PhysicalPosition<i32>>) -> bool {
    let Some(main_window) = application_handle.get_webview_window(MAIN_WINDOW_LABEL) else {
        return false;
    };
    if main_window.is_visible().unwrap_or(false) {
        let _ = main_window.hide();
        return false;
    }
    if application_handle
        .state::<PopoverState>()
        .was_just_hidden_by_focus_loss()
    {
        return false;
    }
    show_main_window(application_handle, anchor);
    true
}

/// "Open" from a menu: always shows the popover.
pub fn show_main_window(application_handle: &AppHandle, anchor: Option<PhysicalPosition<i32>>) {
    let Some(main_window) = application_handle.get_webview_window(MAIN_WINDOW_LABEL) else {
        return;
    };
    match anchor {
        Some(anchor_position) => position_below_anchor(&main_window, anchor_position),
        None => position_near_tray(&main_window),
    }
    show_and_focus(&main_window);
    // The UI plays its entrance animation on this event.
    let _ = main_window.emit("popover-shown", ());
}

/// A popover that never gets keyboard focus never loses it, so "close when clicking outside"
/// would not fire until the user clicked inside it.
///
/// Linux: `show()` + `set_focus()` does not focus. tao queues `show()` to the GTK thread and
/// `set_focus()` only acts when the window is already visible, which it is not yet (we run on
/// the tray/D-Bus thread), so it is a silent no-op. Presenting right after `show_all()` is not
/// enough either: GTK flags the window as mapped at once, but the window manager has not managed
/// it yet and ignores the request. So, on the GTK thread, show it ourselves and present it on the
/// X map event, when the window manager has really put it on screen.
#[cfg(target_os = "linux")]
fn show_and_focus(main_window: &tauri::WebviewWindow) {
    use gtk::prelude::*;
    use std::cell::Cell;
    use std::rc::Rc;

    let shown_window = main_window.clone();
    let _ = main_window.run_on_main_thread(move || {
        let Ok(gtk_window) = shown_window.gtk_window() else {
            let _ = shown_window.show();
            return;
        };
        if gtk_window.is_visible() {
            gtk_window.present_with_time(gtk::gdk::ffi::GDK_CURRENT_TIME as u32);
            return;
        }
        // Connected before showing, so the map event cannot be missed. One-shot.
        let map_handler_slot = Rc::new(Cell::new(None));
        let handler_slot_in_callback = map_handler_slot.clone();
        let map_handler_id = gtk_window.connect_map_event(move |mapped_window, _map_event| {
            mapped_window.present_with_time(gtk::gdk::ffi::GDK_CURRENT_TIME as u32);
            if let Some(handler_id) = handler_slot_in_callback.take() {
                mapped_window.disconnect(handler_id);
            }
            gtk::glib::Propagation::Proceed
        });
        map_handler_slot.set(Some(map_handler_id));
        gtk_window.show_all();
    });
}

#[cfg(not(target_os = "linux"))]
fn show_and_focus(main_window: &tauri::WebviewWindow) {
    let _ = main_window.show();
    let _ = main_window.set_focus();
}

/// Called by the UI with the "close when clicking outside" setting.
#[tauri::command]
pub fn set_close_on_focus_loss(popover_state: tauri::State<'_, PopoverState>, is_enabled: bool) {
    popover_state.close_on_focus_loss.store(is_enabled, Ordering::Relaxed);
}

/// Esc key in the UI.
#[tauri::command]
pub fn hide_main_window(application_handle: AppHandle) {
    if let Some(main_window) = application_handle.get_webview_window(MAIN_WINDOW_LABEL) {
        let _ = main_window.hide();
    }
}

/// Hidden while the GNOME panel button is active, so the app does not appear twice.
pub fn set_tray_visible(application_handle: &AppHandle, is_visible: bool) {
    if let Some(tray_icon) = application_handle.tray_by_id(TRAY_ID) {
        let _ = tray_icon.set_visible(is_visible);
    }
}

/// Centers the popover horizontally under the anchor, kept inside the anchor's monitor.
fn position_below_anchor(main_window: &tauri::WebviewWindow, anchor_position: PhysicalPosition<i32>) {
    let Ok(window_size) = main_window.outer_size() else {
        return;
    };
    let anchor_monitor_span = main_window.available_monitors().ok().and_then(|monitors| {
        monitors.into_iter().find_map(|monitor| {
            let monitor_position = monitor.position();
            let monitor_size = monitor.size();
            let horizontal_span = monitor_position.x..monitor_position.x + monitor_size.width as i32;
            let vertical_span = monitor_position.y..monitor_position.y + monitor_size.height as i32;
            (horizontal_span.contains(&anchor_position.x) && vertical_span.contains(&anchor_position.y))
                .then_some(horizontal_span)
        })
    });
    let window_left = popover_left_edge(anchor_position.x, window_size.width as i32, anchor_monitor_span);
    let _ = main_window.set_position(PhysicalPosition::new(window_left, anchor_position.y + 4));
}

/// Left edge that centers a popover of `window_width` under `anchor_x`, keeping a margin from the
/// monitor edges when the monitor is known. Pure, so it is unit tested.
fn popover_left_edge(anchor_x: i32, window_width: i32, monitor_horizontal_span: Option<std::ops::Range<i32>>) -> i32 {
    const SCREEN_MARGIN: i32 = 8;
    let centered_left = anchor_x - window_width / 2;
    match monitor_horizontal_span {
        Some(monitor_span) => centered_left
            .min(monitor_span.end - window_width - SCREEN_MARGIN)
            .max(monitor_span.start + SCREEN_MARGIN),
        None => centered_left,
    }
}

#[cfg(target_os = "linux")]
fn position_near_tray(main_window: &tauri::WebviewWindow) {
    use tauri_plugin_positioner::{Position, WindowExt};
    // AppIndicator does not report the tray position; GNOME/KDE panels sit at the top right.
    let _ = main_window.move_window(Position::TopRight);
}

#[cfg(not(target_os = "linux"))]
fn position_near_tray(main_window: &tauri::WebviewWindow) {
    use tauri_plugin_positioner::{Position, WindowExt};
    if main_window.move_window(Position::TrayCenter).is_err() {
        let _ = main_window.move_window(Position::TopRight);
    }
}

/// Called by the UI whenever the unread count or the interface language changes.
#[tauri::command]
pub fn update_tray(
    application_handle: AppHandle,
    unread_count: u32,
    tooltip_text: String,
    open_menu_label: String,
    quit_menu_label: String,
) -> Result<(), String> {
    let Some(tray_icon) = application_handle.tray_by_id(TRAY_ID) else {
        return Ok(());
    };
    // macOS: native text next to the menu bar icon.
    #[cfg(target_os = "macos")]
    {
        let count_label = (unread_count > 0).then(|| unread_count.to_string());
        tray_icon
            .set_title(count_label)
            .map_err(|tray_error| tray_error.to_string())?;
    }

    // Linux and Windows: only "has unread" vs "all read" on the icon itself.
    #[cfg(not(target_os = "macos"))]
    {
        let icon_bytes = if unread_count > 0 {
            TRAY_ICON_UNREAD
        } else {
            TRAY_ICON_DEFAULT
        };
        let state_icon = Image::from_bytes(icon_bytes).map_err(|image_error| image_error.to_string())?;
        tray_icon
            .set_icon(Some(state_icon))
            .map_err(|tray_error| tray_error.to_string())?;
    }

    tray_icon
        .set_tooltip(Some(tooltip_text))
        .map_err(|tray_error| tray_error.to_string())?;

    // Keep the GNOME panel button (if any) in sync.
    #[cfg(target_os = "linux")]
    crate::panel_bridge::publish_tray_state(&application_handle, unread_count, &open_menu_label, &quit_menu_label);

    let tray_menu_items = application_handle.state::<TrayMenuItems>();
    tray_menu_items
        .open_menu_item
        .set_text(open_menu_label)
        .map_err(|menu_error| menu_error.to_string())?;
    tray_menu_items
        .quit_menu_item
        .set_text(quit_menu_label)
        .map_err(|menu_error| menu_error.to_string())?;

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn popover_is_centered_under_the_anchor() {
        assert_eq!(popover_left_edge(1000, 380, Some(0..1920)), 810);
    }

    #[test]
    fn popover_never_leaves_the_right_edge_of_the_monitor() {
        // Panel button at the far right of a 1920 px screen: the window slides left, keeping 8 px.
        assert_eq!(popover_left_edge(1900, 380, Some(0..1920)), 1920 - 380 - 8);
    }

    #[test]
    fn popover_never_leaves_the_left_edge_of_the_monitor() {
        assert_eq!(popover_left_edge(20, 380, Some(0..1920)), 8);
    }

    #[test]
    fn popover_stays_on_the_anchor_monitor_with_multiple_screens() {
        // Second monitor starting at x = 1920: clamping uses that monitor, not the first one.
        assert_eq!(popover_left_edge(3830, 380, Some(1920..3840)), 3840 - 380 - 8);
        assert_eq!(popover_left_edge(1930, 380, Some(1920..3840)), 1920 + 8);
    }

    #[test]
    fn popover_is_only_centered_when_the_monitor_is_unknown() {
        assert_eq!(popover_left_edge(10, 380, None), 10 - 190);
    }

    #[test]
    fn closing_on_focus_loss_is_on_by_default() {
        assert!(PopoverState::default().closes_on_focus_loss());
    }

    #[test]
    fn a_click_right_after_a_focus_loss_hide_means_close() {
        let popover_state = PopoverState::default();
        popover_state.remember_hidden_by_focus_loss();
        assert!(popover_state.was_just_hidden_by_focus_loss());
        // The marker is consumed: the next click opens the popover again.
        assert!(!popover_state.was_just_hidden_by_focus_loss());
    }

    #[test]
    fn an_old_focus_loss_hide_does_not_swallow_a_click() {
        let popover_state = PopoverState::default();
        if let Ok(mut hidden_at_guard) = popover_state.hidden_by_focus_loss_at.lock() {
            *hidden_at_guard = Some(Instant::now() - FOCUS_LOSS_CLICK_WINDOW - Duration::from_millis(50));
        }
        assert!(!popover_state.was_just_hidden_by_focus_loss());
    }

    #[test]
    fn never_hidden_means_the_click_opens() {
        assert!(!PopoverState::default().was_just_hidden_by_focus_loss());
    }
}
