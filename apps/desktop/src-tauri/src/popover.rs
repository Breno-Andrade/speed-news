//! The popover window: created on demand, hidden on close, destroyed after a while hidden.
//!
//! A hidden webview still costs its whole engine (WebKitGTK / WebView2 processes, JS heap,
//! decoded images). The tray no longer depends on it (the Go sidecar reports the tray state, see
//! sidecar.rs), so the window is destroyed `DESTROY_AFTER_HIDDEN` after being hidden and built
//! again from tauri.conf.json (`"create": false`) the next time it is opened. Reopening within
//! that delay is instant, as before.

use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Mutex;
use std::time::{Duration, Instant};

use tauri::{AppHandle, Emitter, Manager, WebviewWindow, WebviewWindowBuilder};

use crate::placement::{place_popover, MonitorArea, Placement, ScreenRect};

pub const MAIN_WINDOW_LABEL: &str = "main";

/// How long a hidden popover keeps its webview before it is destroyed.
const DESTROY_AFTER_HIDDEN: Duration = Duration::from_secs(60);

/// Clicking the tray icon while the popover is open first steals its focus (which hides it) and
/// only then delivers the click. Without this guard that click would reopen it immediately.
const FOCUS_LOSS_CLICK_WINDOW: Duration = Duration::from_millis(400);

pub struct PopoverState {
    hidden_by_focus_loss_at: Mutex<Option<Instant>>,
    /// User setting "close when clicking outside", reported by the sidecar with the tray state.
    close_on_focus_loss: AtomicBool,
    /// Bumped on every show and hide: a pending destroy only runs if nothing happened since.
    visibility_generation: AtomicU64,
    /// True while a creation thread is building the window (clicks meanwhile are ignored).
    is_creating_window: AtomicBool,
    /// Where the tray icon was last seen (Windows/macOS report it on every tray event), so
    /// "Open" from the menu or a second launch still opens next to it.
    last_tray_icon_area: Mutex<Option<ScreenRect>>,
}

impl Default for PopoverState {
    fn default() -> Self {
        Self {
            hidden_by_focus_loss_at: Mutex::new(None),
            close_on_focus_loss: AtomicBool::new(true),
            visibility_generation: AtomicU64::new(0),
            is_creating_window: AtomicBool::new(false),
            last_tray_icon_area: Mutex::new(None),
        }
    }
}

impl PopoverState {
    pub fn closes_on_focus_loss(&self) -> bool {
        self.close_on_focus_loss.load(Ordering::Relaxed)
    }

    pub fn set_close_on_focus_loss(&self, is_enabled: bool) {
        self.close_on_focus_loss.store(is_enabled, Ordering::Relaxed);
    }

    fn remember_hidden_by_focus_loss(&self) {
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

    /// Starts a new visibility period and returns its ticket.
    fn next_visibility_generation(&self) -> u64 {
        self.visibility_generation.fetch_add(1, Ordering::SeqCst) + 1
    }

    /// A destroy scheduled with `ticket` may run only if the popover was neither shown nor hidden
    /// again since.
    fn is_current_generation(&self, ticket: u64) -> bool {
        self.visibility_generation.load(Ordering::SeqCst) == ticket
    }

    /// Claims the window creation; false when another thread is already creating it.
    fn begin_window_creation(&self) -> bool {
        self.is_creating_window
            .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
            .is_ok()
    }

    fn finish_window_creation(&self) {
        self.is_creating_window.store(false, Ordering::SeqCst);
    }

    pub fn remember_tray_icon_area(&self, tray_icon_area: ScreenRect) {
        if let Ok(mut area_guard) = self.last_tray_icon_area.lock() {
            *area_guard = Some(tray_icon_area);
        }
    }

    fn last_tray_icon_area(&self) -> Option<ScreenRect> {
        self.last_tray_icon_area.lock().ok().and_then(|area_guard| *area_guard)
    }
}

/// Left click on the icon (or GNOME panel button): open if closed, close if open.
/// `anchor` is what was clicked, in physical screen pixels, when known: the tray icon's area,
/// the click point, or the bottom center of the GNOME panel button.
/// Returns whether the popover is visible afterwards.
pub fn toggle_popover(application_handle: &AppHandle, anchor: Option<ScreenRect>) -> bool {
    if is_popover_visible(application_handle) {
        hide_popover(application_handle);
        return false;
    }
    if application_handle
        .state::<PopoverState>()
        .was_just_hidden_by_focus_loss()
    {
        return false;
    }
    show_popover(application_handle, anchor);
    true
}

/// "Open" from a menu, a second launch or the panel button menu: always shows the popover.
pub fn show_popover(application_handle: &AppHandle, anchor: Option<ScreenRect>) {
    let popover_state = application_handle.state::<PopoverState>();
    popover_state.next_visibility_generation(); // cancels a pending destroy
    if let Some(main_window) = application_handle.get_webview_window(MAIN_WINDOW_LABEL) {
        present(&main_window, anchor);
        return;
    }
    if !popover_state.begin_window_creation() {
        return; // a click while the window is being built: it shows up on its own
    }
    // Building a webview window from an event handler (tray click) deadlocks on Windows
    // (WebView2 needs the event loop the handler is blocking), so it is built on its own thread.
    let creating_application_handle = application_handle.clone();
    std::thread::spawn(move || {
        let created_window = create_window(&creating_application_handle);
        creating_application_handle
            .state::<PopoverState>()
            .finish_window_creation();
        if let Some(main_window) = created_window {
            present(&main_window, anchor);
        }
    });
}

fn present(main_window: &WebviewWindow, anchor: Option<ScreenRect>) {
    let application_handle = main_window.app_handle();
    let monitors = monitor_areas(application_handle);
    let resolved_anchor = anchor
        .or_else(|| application_handle.state::<PopoverState>().last_tray_icon_area())
        .or_else(|| {
            // Just clicked a menu entry or a launcher: the pointer is the best hint left.
            application_handle
                .cursor_position()
                .ok()
                .map(|cursor| ScreenRect::point(cursor.x as i32, cursor.y as i32))
        })
        .unwrap_or_else(|| fallback_anchor(&monitors));
    show_and_focus(
        main_window,
        place_popover(resolved_anchor, configured_logical_size(application_handle), &monitors),
    );
    // The UI plays its entrance animation on this event (a brand-new window just mounts).
    let _ = main_window.emit("popover-shown", ());
}

/// Hides the popover and schedules the destruction of its webview.
pub fn hide_popover(application_handle: &AppHandle) {
    if let Some(main_window) = application_handle.get_webview_window(MAIN_WINDOW_LABEL) {
        let _ = main_window.hide();
    }
    schedule_destroy(application_handle);
}

/// Window lost focus: hide it when the user wants "close when clicking outside".
pub fn handle_focus_lost(main_window: &tauri::Window) {
    let popover_state = main_window.state::<PopoverState>();
    if popover_state.closes_on_focus_loss() && main_window.is_visible().unwrap_or(false) {
        popover_state.remember_hidden_by_focus_loss();
        hide_popover(main_window.app_handle());
    }
}

/// False when the window does not exist (destroyed or not created yet).
pub fn is_popover_visible(application_handle: &AppHandle) -> bool {
    window_counts_as_visible(
        application_handle
            .get_webview_window(MAIN_WINDOW_LABEL)
            .map(|main_window| main_window.is_visible().unwrap_or(false)),
    )
}

/// `window_visibility` is None when there is no window at all.
fn window_counts_as_visible(window_visibility: Option<bool>) -> bool {
    window_visibility.unwrap_or(false)
}

/// Destroying the popover's window is not "the user closed the last window": the app lives in
/// the tray. Only an explicit exit (menu, quit button, updater restart) carries an exit code.
pub fn should_keep_running(exit_code: Option<i32>) -> bool {
    exit_code.is_none()
}

fn schedule_destroy(application_handle: &AppHandle) {
    let destroy_ticket = application_handle.state::<PopoverState>().next_visibility_generation();
    let destroying_application_handle = application_handle.clone();
    tauri::async_runtime::spawn(async move {
        // A sleeping blocking thread, as in updates.rs: no extra runtime features needed.
        let _ = tauri::async_runtime::spawn_blocking(|| std::thread::sleep(DESTROY_AFTER_HIDDEN)).await;
        if !destroying_application_handle
            .state::<PopoverState>()
            .is_current_generation(destroy_ticket)
        {
            return; // shown (or hidden again) meanwhile: that event owns the next decision
        }
        if let Some(main_window) = destroying_application_handle.get_webview_window(MAIN_WINDOW_LABEL) {
            if !main_window.is_visible().unwrap_or(false) {
                let _ = main_window.destroy();
            }
        }
    });
}

/// Builds the popover from its tauri.conf.json entry (`"create": false` keeps Tauri from doing it
/// at startup). Never call it from an event handler or a sync command (Windows deadlock).
fn create_window(application_handle: &AppHandle) -> Option<WebviewWindow> {
    let window_config = application_handle
        .config()
        .app
        .windows
        .iter()
        .find(|window_config| window_config.label == MAIN_WINDOW_LABEL)?
        .clone();
    match WebviewWindowBuilder::from_config(application_handle, &window_config)
        .and_then(|window_builder| window_builder.build())
    {
        Ok(main_window) => {
            // The webview paints its default white until index.html arrives: start in the app's
            // own background (same values as --color-background in src/styles.css).
            let window_theme = main_window.theme().unwrap_or(tauri::Theme::Light);
            let _ = main_window.set_background_color(Some(app_background_color(window_theme)));
            Some(main_window)
        }
        Err(build_error) => {
            eprintln!("[popover] could not create the window: {build_error}");
            None
        }
    }
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
///
/// The move happens in the same GTK closure, right before showing: tao's `set_position` travels
/// on another channel and could arrive after the window was already mapped somewhere else.
#[cfg(target_os = "linux")]
fn show_and_focus(main_window: &WebviewWindow, placement: Placement) {
    use gtk::prelude::*;
    use std::cell::Cell;
    use std::rc::Rc;

    let shown_window = main_window.clone();
    let _ = main_window.run_on_main_thread(move || {
        let Ok(gtk_window) = shown_window.gtk_window() else {
            let _ = shown_window.set_position(tauri::PhysicalPosition::new(placement.left, placement.top));
            let _ = shown_window.show();
            return;
        };
        // GTK positions in logical pixels (its scale factor is a whole number on X11).
        let gtk_scale_factor = gtk_window.scale_factor().max(1);
        gtk_window.move_(placement.left / gtk_scale_factor, placement.top / gtk_scale_factor);
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
fn show_and_focus(main_window: &WebviewWindow, placement: Placement) {
    let _ = main_window.set_position(tauri::PhysicalPosition::new(placement.left, placement.top));
    let _ = main_window.show();
    let _ = main_window.set_focus();
}

/// Esc key in the UI.
#[tauri::command]
pub fn hide_main_window(application_handle: AppHandle) {
    hide_popover(&application_handle);
}

/// `--color-background` of src/styles.css for each theme (a test in StartupSkeleton.test.tsx
/// checks index.html against styles.css; this pair is checked against this file's tests).
fn app_background_color(window_theme: tauri::Theme) -> tauri::window::Color {
    match window_theme {
        tauri::Theme::Dark => tauri::window::Color(0x15, 0x15, 0x1e, 0xff),
        _ => tauri::window::Color(0xff, 0xff, 0xff, 0xff),
    }
}

fn monitor_areas(application_handle: &AppHandle) -> Vec<MonitorArea> {
    application_handle
        .available_monitors()
        .unwrap_or_default()
        .iter()
        .map(|monitor| {
            let work_area = monitor.work_area();
            MonitorArea {
                bounds: ScreenRect::new(
                    monitor.position().x,
                    monitor.position().y,
                    monitor.size().width as i32,
                    monitor.size().height as i32,
                ),
                work_area: ScreenRect::new(
                    work_area.position.x,
                    work_area.position.y,
                    work_area.size.width as i32,
                    work_area.size.height as i32,
                ),
                scale_factor: monitor.scale_factor(),
            }
        })
        .collect()
}

/// The popover's logical size from tauri.conf.json (it is not resizable).
fn configured_logical_size(application_handle: &AppHandle) -> (f64, f64) {
    application_handle
        .config()
        .app
        .windows
        .iter()
        .find(|window_config| window_config.label == MAIN_WINDOW_LABEL)
        .map(|window_config| (window_config.width, window_config.height))
        .unwrap_or((380.0, 620.0))
}

/// Nothing known about the click and no pointer (e.g. Wayland): top right of the first monitor,
/// where GNOME and KDE keep their tray area.
fn fallback_anchor(monitors: &[MonitorArea]) -> ScreenRect {
    monitors
        .first()
        .map(|monitor| {
            ScreenRect::point(
                monitor.work_area.left + monitor.work_area.width - 1,
                monitor.work_area.top,
            )
        })
        .unwrap_or(ScreenRect::point(0, 0))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn window_background_matches_the_stylesheet_tokens() {
        let stylesheet = include_str!("../../src/styles.css");
        let token_colors: Vec<&str> = stylesheet
            .match_indices("--color-background: #")
            .map(|(token_index, token)| &stylesheet[token_index + token.len()..token_index + token.len() + 6])
            .collect();
        let as_hex = |background_color: tauri::window::Color| {
            format!(
                "{:02x}{:02x}{:02x}",
                background_color.0, background_color.1, background_color.2
            )
        };
        assert_eq!(
            token_colors,
            vec![
                as_hex(app_background_color(tauri::Theme::Light)),
                as_hex(app_background_color(tauri::Theme::Dark))
            ]
        );
    }

    #[test]
    fn the_last_seen_tray_icon_is_remembered_for_menu_opens() {
        let popover_state = PopoverState::default();
        assert_eq!(popover_state.last_tray_icon_area(), None);
        popover_state.remember_tray_icon_area(ScreenRect::new(1700, 1040, 24, 24));
        assert_eq!(
            popover_state.last_tray_icon_area(),
            Some(ScreenRect::new(1700, 1040, 24, 24))
        );
    }

    #[test]
    fn fallback_anchor_is_the_top_right_of_the_first_monitor() {
        let monitors = [MonitorArea {
            bounds: ScreenRect::new(0, 0, 1920, 1080),
            work_area: ScreenRect::new(0, 32, 1920, 1048),
            scale_factor: 1.0,
        }];
        assert_eq!(fallback_anchor(&monitors), ScreenRect::point(1919, 32));
        assert_eq!(fallback_anchor(&[]), ScreenRect::point(0, 0));
    }

    #[test]
    fn closing_on_focus_loss_is_on_by_default() {
        assert!(PopoverState::default().closes_on_focus_loss());
    }

    #[test]
    fn closing_on_focus_loss_follows_the_reported_setting() {
        let popover_state = PopoverState::default();
        popover_state.set_close_on_focus_loss(false);
        assert!(!popover_state.closes_on_focus_loss());
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

    #[test]
    fn a_pending_destroy_runs_when_nothing_happened_since_the_hide() {
        let popover_state = PopoverState::default();
        let destroy_ticket = popover_state.next_visibility_generation();
        assert!(popover_state.is_current_generation(destroy_ticket));
    }

    #[test]
    fn reopening_cancels_the_pending_destroy() {
        let popover_state = PopoverState::default();
        let destroy_ticket = popover_state.next_visibility_generation(); // hidden
        popover_state.next_visibility_generation(); // shown again before the delay
        assert!(!popover_state.is_current_generation(destroy_ticket));
    }

    #[test]
    fn only_the_latest_hide_may_destroy() {
        let popover_state = PopoverState::default();
        let first_ticket = popover_state.next_visibility_generation(); // hidden
        popover_state.next_visibility_generation(); // shown
        let second_ticket = popover_state.next_visibility_generation(); // hidden again
        assert!(!popover_state.is_current_generation(first_ticket));
        assert!(popover_state.is_current_generation(second_ticket));
    }

    #[test]
    fn a_second_click_while_the_window_is_built_does_not_build_another() {
        let popover_state = PopoverState::default();
        assert!(popover_state.begin_window_creation());
        assert!(!popover_state.begin_window_creation());
        popover_state.finish_window_creation();
        // Built (or failed): the next open may create it again.
        assert!(popover_state.begin_window_creation());
    }

    #[test]
    fn a_missing_window_counts_as_hidden() {
        // Updater: installs while the popover is not on screen, destroyed windows included.
        assert!(!window_counts_as_visible(None));
        assert!(!window_counts_as_visible(Some(false)));
        assert!(window_counts_as_visible(Some(true)));
    }

    #[test]
    fn destroying_the_popover_window_does_not_quit_the_app() {
        assert!(should_keep_running(None));
        assert!(!should_keep_running(Some(0)));
        assert!(!should_keep_running(Some(i32::MAX))); // updater restart
    }
}
