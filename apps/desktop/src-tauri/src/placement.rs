//! Where the popover goes, as pure functions (all values in physical screen pixels).
//!
//! Rules, valid for one or many monitors, any panel position and mixed scale factors:
//! - the target monitor is the one containing the anchor (else the nearest one);
//! - the popover is centered horizontally on the anchor;
//! - it opens below the anchor when the anchor is in the top half of the monitor (GNOME top bar,
//!   macOS menu bar) and above it otherwise (Windows taskbar, bottom KDE panel);
//! - it stays inside the monitor's work area (panels and taskbars excluded), with a margin;
//! - its size is the configured logical size times the *target* monitor's scale factor. The
//!   window's own `outer_size()` is not used: on Linux it reads 0×0 until the window has been
//!   mapped once, which pushed a freshly created popover against the screen edge.

/// A rectangle in physical screen pixels.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ScreenRect {
    pub left: i32,
    pub top: i32,
    pub width: i32,
    pub height: i32,
}

impl ScreenRect {
    pub fn new(left: i32, top: i32, width: i32, height: i32) -> Self {
        Self {
            left,
            top,
            width,
            height,
        }
    }

    /// A zero-size rectangle at a point (click position, cursor, GNOME button anchor).
    pub fn point(horizontal: i32, vertical: i32) -> Self {
        Self::new(horizontal, vertical, 0, 0)
    }

    fn right(&self) -> i32 {
        self.left + self.width
    }

    fn bottom(&self) -> i32 {
        self.top + self.height
    }

    fn center(&self) -> (i32, i32) {
        (self.left + self.width / 2, self.top + self.height / 2)
    }

    fn contains(&self, horizontal: i32, vertical: i32) -> bool {
        (self.left..self.right()).contains(&horizontal) && (self.top..self.bottom()).contains(&vertical)
    }

    fn squared_distance_to(&self, horizontal: i32, vertical: i32) -> i64 {
        let horizontal_gap = i64::from((self.left - horizontal).max(horizontal - (self.right() - 1)).max(0));
        let vertical_gap = i64::from((self.top - vertical).max(vertical - (self.bottom() - 1)).max(0));
        horizontal_gap * horizontal_gap + vertical_gap * vertical_gap
    }

    fn intersection(&self, other: &ScreenRect) -> Option<ScreenRect> {
        let left = self.left.max(other.left);
        let top = self.top.max(other.top);
        let right = self.right().min(other.right());
        let bottom = self.bottom().min(other.bottom());
        (right > left && bottom > top).then(|| ScreenRect::new(left, top, right - left, bottom - top))
    }
}

/// What placement needs to know about a monitor.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct MonitorArea {
    pub bounds: ScreenRect,
    pub work_area: ScreenRect,
    pub scale_factor: f64,
}

/// Gap between the anchor and the popover; also kept from the work area's top and bottom, so a
/// popover under a top bar sits exactly `ANCHOR_GAP` below it.
const ANCHOR_GAP: i32 = 4;
/// Kept from the work area's left and right edges.
const SCREEN_MARGIN: i32 = 8;

/// Top-left corner and physical size of the popover.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Placement {
    pub left: i32,
    pub top: i32,
    pub width: i32,
    pub height: i32,
}

pub fn place_popover(anchor: ScreenRect, logical_size: (f64, f64), monitors: &[MonitorArea]) -> Placement {
    let (anchor_center_x, anchor_center_y) = anchor.center();
    let Some(target_monitor) = monitor_for_point(monitors, anchor_center_x, anchor_center_y) else {
        // No monitor information at all: center under the anchor, nothing to clamp against.
        let (width, height) = physical_size(logical_size, 1.0);
        return Placement {
            left: anchor_center_x - width / 2,
            top: anchor.bottom() + ANCHOR_GAP,
            width,
            height,
        };
    };
    let (width, height) = physical_size(logical_size, target_monitor.scale_factor);
    // Some X11 setups report one work area spanning every monitor: never leave this monitor.
    let usable_area = target_monitor
        .work_area
        .intersection(&target_monitor.bounds)
        .unwrap_or(target_monitor.bounds);

    let opens_upward = anchor_center_y > target_monitor.bounds.top + target_monitor.bounds.height / 2;
    let preferred_top = if opens_upward {
        anchor.top - height - ANCHOR_GAP
    } else {
        anchor.bottom() + ANCHOR_GAP
    };
    Placement {
        left: clamp_into(
            anchor_center_x - width / 2,
            width,
            usable_area.left,
            usable_area.right(),
            SCREEN_MARGIN,
        ),
        top: clamp_into(preferred_top, height, usable_area.top, usable_area.bottom(), ANCHOR_GAP),
        width,
        height,
    }
}

/// The monitor containing the point, else the nearest one (a point on a panel edge or on a
/// monitor that was just unplugged must still land somewhere sensible).
fn monitor_for_point(monitors: &[MonitorArea], horizontal: i32, vertical: i32) -> Option<&MonitorArea> {
    monitors
        .iter()
        .find(|monitor| monitor.bounds.contains(horizontal, vertical))
        .or_else(|| {
            monitors
                .iter()
                .min_by_key(|monitor| monitor.bounds.squared_distance_to(horizontal, vertical))
        })
}

fn physical_size(logical_size: (f64, f64), scale_factor: f64) -> (i32, i32) {
    let (logical_width, logical_height) = logical_size;
    (
        (logical_width * scale_factor).round() as i32,
        (logical_height * scale_factor).round() as i32,
    )
}

/// Keeps `[start, start + length)` inside `[area_start, area_end)` with a margin; when it cannot
/// fit, it sticks to the start (the top of a popover taller than the screen stays reachable).
fn clamp_into(start: i32, length: i32, area_start: i32, area_end: i32, margin: i32) -> i32 {
    let lowest = area_start + margin;
    let highest = area_end - length - margin;
    if highest < lowest {
        return lowest;
    }
    start.clamp(lowest, highest)
}

#[cfg(test)]
mod tests {
    use super::*;

    const POPOVER_SIZE: (f64, f64) = (380.0, 620.0);

    fn monitor(left: i32, top: i32, width: i32, height: i32, top_panel: i32, bottom_panel: i32) -> MonitorArea {
        MonitorArea {
            bounds: ScreenRect::new(left, top, width, height),
            work_area: ScreenRect::new(left, top + top_panel, width, height - top_panel - bottom_panel),
            scale_factor: 1.0,
        }
    }

    /// The maintainer's setup: laptop at x 0, external monitor at x 1920 with the GNOME top bar.
    fn laptop_and_external() -> Vec<MonitorArea> {
        vec![monitor(0, 0, 1920, 1080, 0, 0), monitor(1920, 0, 1920, 1080, 32, 0)]
    }

    #[test]
    fn gnome_button_on_the_external_monitor_centers_below_it() {
        let placement = place_popover(ScreenRect::point(3000, 32), POPOVER_SIZE, &laptop_and_external());
        assert_eq!((placement.left, placement.top), (3000 - 190, 36));
    }

    // The first-open bug: the size must come from the configuration, so a button at the far
    // right puts the popover 8 px from the edge, fully on screen, not starting at the button.
    #[test]
    fn button_at_the_far_right_keeps_the_whole_popover_on_screen() {
        let placement = place_popover(ScreenRect::point(3800, 32), POPOVER_SIZE, &laptop_and_external());
        assert_eq!(placement.left, 3840 - 380 - 8);
        assert_eq!(placement.width, 380);
    }

    #[test]
    fn popover_never_spills_onto_the_neighbouring_monitor() {
        // Anchor near the right edge of the laptop screen: stays on the laptop.
        let placement = place_popover(ScreenRect::point(1900, 20), POPOVER_SIZE, &laptop_and_external());
        assert_eq!(placement.left, 1920 - 380 - 8);
        // Anchor near the left edge of the external screen: stays on the external one.
        let placement = place_popover(ScreenRect::point(1930, 32), POPOVER_SIZE, &laptop_and_external());
        assert_eq!(placement.left, 1920 + 8);
    }

    #[test]
    fn monitor_left_of_the_primary_with_negative_coordinates() {
        let monitors = vec![monitor(-1920, 0, 1920, 1080, 32, 0), monitor(0, 0, 1920, 1080, 32, 0)];
        let placement = place_popover(ScreenRect::point(-30, 32), POPOVER_SIZE, &monitors);
        assert_eq!((placement.left, placement.top), (-380 - 8, 36));
    }

    #[test]
    fn monitors_stacked_vertically() {
        let monitors = vec![monitor(0, -1080, 1920, 1080, 32, 0), monitor(0, 0, 1920, 1080, 32, 0)];
        let placement = place_popover(ScreenRect::point(1000, -1048), POPOVER_SIZE, &monitors);
        assert_eq!(placement.top, -1048 + 4);
    }

    #[test]
    fn windows_taskbar_at_the_bottom_opens_above_the_icon() {
        let monitors = vec![monitor(0, 0, 1920, 1080, 0, 48)];
        let tray_icon = ScreenRect::new(1700, 1040, 24, 24);
        let placement = place_popover(tray_icon, POPOVER_SIZE, &monitors);
        assert_eq!(placement.left, 1712 - 190);
        // Above the icon, and inside the work area (taskbar excluded).
        assert_eq!(placement.top, 1032 - 620 - 4);
        assert!(placement.top + placement.height <= 1032);
    }

    #[test]
    fn taskbar_on_the_second_monitor_uses_that_monitor() {
        let monitors = vec![monitor(0, 0, 1920, 1080, 0, 48), monitor(1920, 0, 2560, 1440, 0, 48)];
        let tray_icon = ScreenRect::new(4400, 1400, 24, 24);
        let placement = place_popover(tray_icon, POPOVER_SIZE, &monitors);
        assert_eq!(placement.left, 4480 - 380 - 8);
        assert_eq!(placement.top, 1392 - 620 - 4);
    }

    #[test]
    fn mixed_scale_factors_use_the_target_monitor_scale() {
        let mut high_density = monitor(1920, 0, 3840, 2160, 64, 0);
        high_density.scale_factor = 2.0;
        let monitors = vec![monitor(0, 0, 1920, 1080, 32, 0), high_density];
        let placement = place_popover(ScreenRect::point(3000, 64), POPOVER_SIZE, &monitors);
        assert_eq!((placement.width, placement.height), (760, 1240));
        assert_eq!(placement.left, 3000 - 380);
    }

    #[test]
    fn a_popover_taller_than_the_screen_keeps_its_top_visible() {
        let monitors = vec![monitor(0, 0, 1366, 600, 32, 0)];
        let placement = place_popover(ScreenRect::point(1300, 32), POPOVER_SIZE, &monitors);
        assert_eq!(placement.top, 32 + 4);
    }

    #[test]
    fn an_anchor_outside_every_monitor_uses_the_nearest_one() {
        // A monitor unplugged after the click, or a panel reporting its outer edge.
        let placement = place_popover(ScreenRect::point(5000, 10), POPOVER_SIZE, &laptop_and_external());
        assert_eq!(placement.left, 3840 - 380 - 8);
    }

    #[test]
    fn a_work_area_spanning_every_monitor_is_cut_to_the_target_monitor() {
        let mut external = monitor(1920, 0, 1920, 1080, 32, 0);
        external.work_area = ScreenRect::new(0, 32, 3840, 1048); // one _NET_WORKAREA for both
        let monitors = vec![monitor(0, 0, 1920, 1080, 32, 0), external];
        let placement = place_popover(ScreenRect::point(1950, 32), POPOVER_SIZE, &monitors);
        assert_eq!(placement.left, 1920 + 8);
    }

    #[test]
    fn without_monitor_information_it_still_centers_under_the_anchor() {
        let placement = place_popover(ScreenRect::point(1000, 30), POPOVER_SIZE, &[]);
        assert_eq!((placement.left, placement.top), (1000 - 190, 34));
    }
}
