package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"speed-news/apps/api/internal/domain"
)

func (apiServer *server) handleGetSettings(responseWriter http.ResponseWriter, request *http.Request) {
	userSettings, err := apiServer.Store.LoadUserSettings(request.Context())
	if err != nil {
		apiServer.writeInternalError(responseWriter, "load settings", err)
		return
	}
	writeJSON(responseWriter, http.StatusOK, userSettings)
}

func (apiServer *server) handlePutSettings(responseWriter http.ResponseWriter, request *http.Request) {
	// Start from the defaults so a client that omits a field never wipes it with a zero value.
	userSettings := domain.DefaultUserSettings()
	request.Body = http.MaxBytesReader(responseWriter, request.Body, 16*1024)
	if err := json.NewDecoder(request.Body).Decode(&userSettings); err != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := ValidateUserSettings(userSettings, apiServer.availableLanguageCodes()); err != nil {
		writeError(responseWriter, http.StatusBadRequest, err.Error())
		return
	}
	if userSettings.NewsLanguageCodes == nil {
		userSettings.NewsLanguageCodes = []string{}
	}
	if err := apiServer.Store.SaveUserSettings(request.Context(), userSettings); err != nil {
		apiServer.writeInternalError(responseWriter, "save settings", err)
		return
	}
	// The news languages change what the tray counts.
	apiServer.NewsService.PublishUnreadCount(request.Context())
	writeJSON(responseWriter, http.StatusOK, userSettings)
}

func (apiServer *server) availableLanguageCodes() map[string]bool {
	languageCodes := map[string]bool{}
	for _, newsSource := range apiServer.NewsService.NewsSources() {
		languageCodes[newsSource.LanguageCode] = true
	}
	return languageCodes
}

// ValidateUserSettings checks the time zone, the interface language and that every news
// language exists among the configured sources.
func ValidateUserSettings(userSettings domain.UserSettings, availableLanguageCodes map[string]bool) error {
	if err := validateTimeZonePreference(userSettings.TimeZonePreference); err != nil {
		return err
	}

	switch userSettings.InterfaceLanguage {
	case domain.InterfaceLanguageSystem, domain.InterfaceLanguagePortuguese, domain.InterfaceLanguageEnglish:
	default:
		return fmt.Errorf("interfaceLanguage must be %q, %q or %q",
			domain.InterfaceLanguageSystem, domain.InterfaceLanguagePortuguese, domain.InterfaceLanguageEnglish)
	}

	for _, languageCode := range userSettings.NewsLanguageCodes {
		if !availableLanguageCodes[languageCode] {
			return fmt.Errorf("no news source in language %q", languageCode)
		}
	}
	return nil
}

// validateTimeZonePreference accepts mode "system" or mode "manual" with a valid IANA time zone.
func validateTimeZonePreference(timeZonePreference domain.TimeZonePreference) error {
	switch timeZonePreference.Mode {
	case "system":
		return nil
	case "manual":
		if timeZonePreference.TimeZoneID == "" {
			return errors.New("timeZoneId is required when mode is manual")
		}
		if _, err := time.LoadLocation(timeZonePreference.TimeZoneID); err != nil {
			return fmt.Errorf("unknown time zone %q", timeZonePreference.TimeZoneID)
		}
		return nil
	default:
		return fmt.Errorf("timeZonePreference.mode must be \"system\" or \"manual\"")
	}
}
