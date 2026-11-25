//! Tray tooltip and menu texts. The tray is drawn while the popover webview does not exist, so
//! these few strings live here. Keep them identical to `trayTooltip`, `trayOpenMenuLabel` and
//! `trayQuitMenuLabel` in apps/desktop/src/shared/i18n/messages.ts (same language rule too:
//! a locale starting with "pt" is Portuguese, anything else is English).

#[derive(Debug, PartialEq, Eq)]
pub struct TrayTexts {
    pub tooltip: String,
    pub open_label: &'static str,
    pub quit_label: &'static str,
}

/// `interface_language` is the saved setting ("system", "pt-BR", "en").
pub fn tray_texts(unread_count: u32, interface_language: &str, system_locale: &str) -> TrayTexts {
    let effective_locale = if interface_language == "system" {
        system_locale
    } else {
        interface_language
    };
    if effective_locale.to_lowercase().starts_with("pt") {
        TrayTexts {
            tooltip: match unread_count {
                0 => "Speed News".into(),
                1 => "Speed News — 1 não lida".into(),
                _ => format!("Speed News — {unread_count} não lidas"),
            },
            open_label: "Abrir notícias",
            quit_label: "Sair",
        }
    } else {
        TrayTexts {
            tooltip: match unread_count {
                0 => "Speed News".into(),
                _ => format!("Speed News — {unread_count} unread"),
            },
            open_label: "Open news",
            quit_label: "Quit",
        }
    }
}

/// Locale from the POSIX environment ("pt_BR.UTF-8" → "pt-BR"). `C`/`POSIX` mean "not set".
/// Windows asks the system instead (tests still cover it there).
#[cfg(any(not(target_os = "windows"), test))]
pub fn locale_from_environment(read_variable: impl Fn(&str) -> Option<String>) -> Option<String> {
    ["LC_ALL", "LC_MESSAGES", "LANG"]
        .into_iter()
        .filter_map(read_variable)
        .map(|variable_value| variable_value.trim().to_string())
        .find(|variable_value| !variable_value.is_empty() && variable_value != "C" && variable_value != "POSIX")
        .map(|variable_value| {
            let without_encoding = variable_value.split(['.', '@']).next().unwrap_or_default();
            without_encoding.replace('_', "-")
        })
}

/// The operating system's locale, used for "system" until the UI reports the one it detected.
#[cfg(not(target_os = "windows"))]
pub fn system_locale() -> String {
    locale_from_environment(|variable_name| std::env::var(variable_name).ok()).unwrap_or_else(|| "en".into())
}

#[cfg(target_os = "windows")]
pub fn system_locale() -> String {
    use windows_sys::Win32::Globalization::GetUserDefaultLocaleName;
    // LOCALE_NAME_MAX_LENGTH
    let mut locale_buffer = [0u16; 85];
    // SAFETY: the buffer and its length match; the call writes at most that many UTF-16 units.
    let written_length = unsafe { GetUserDefaultLocaleName(locale_buffer.as_mut_ptr(), locale_buffer.len() as i32) };
    if written_length <= 1 {
        return "en".into();
    }
    // The length includes the terminating NUL.
    String::from_utf16_lossy(&locale_buffer[..(written_length - 1) as usize])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn portuguese_tooltip_matches_the_ui() {
        assert_eq!(tray_texts(0, "pt-BR", "en").tooltip, "Speed News");
        assert_eq!(tray_texts(1, "pt-BR", "en").tooltip, "Speed News — 1 não lida");
        assert_eq!(tray_texts(5, "pt-BR", "en").tooltip, "Speed News — 5 não lidas");
        assert_eq!(tray_texts(5, "pt-BR", "en").open_label, "Abrir notícias");
        assert_eq!(tray_texts(5, "pt-BR", "en").quit_label, "Sair");
    }

    #[test]
    fn english_tooltip_matches_the_ui() {
        assert_eq!(tray_texts(0, "en", "pt-BR").tooltip, "Speed News");
        assert_eq!(tray_texts(3, "en", "pt-BR").tooltip, "Speed News — 3 unread");
        assert_eq!(tray_texts(3, "en", "pt-BR").open_label, "Open news");
        assert_eq!(tray_texts(3, "en", "pt-BR").quit_label, "Quit");
    }

    #[test]
    fn system_language_follows_the_system_locale() {
        assert_eq!(tray_texts(2, "system", "pt-PT").quit_label, "Sair");
        assert_eq!(tray_texts(2, "system", "de-DE").quit_label, "Quit");
    }

    #[test]
    fn environment_locale_is_normalized() {
        let environment = |variable_name: &str| match variable_name {
            "LANG" => Some("pt_BR.UTF-8".to_string()),
            _ => None,
        };
        assert_eq!(locale_from_environment(environment).as_deref(), Some("pt-BR"));
    }

    #[test]
    fn more_specific_environment_variables_win_and_c_means_unset() {
        let environment = |variable_name: &str| match variable_name {
            "LC_ALL" => Some("C".to_string()),
            "LC_MESSAGES" => Some("en_GB@euro".to_string()),
            "LANG" => Some("pt_BR.UTF-8".to_string()),
            _ => None,
        };
        assert_eq!(locale_from_environment(environment).as_deref(), Some("en-GB"));
        assert_eq!(locale_from_environment(|_| None), None);
    }
}
