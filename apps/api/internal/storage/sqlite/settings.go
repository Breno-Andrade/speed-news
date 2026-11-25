package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"speed-news/apps/api/internal/domain"
)

const userSettingsKey = "user_settings"

// LoadUserSettings returns the defaults when nothing was saved yet.
func (store *Store) LoadUserSettings(ctx context.Context) (domain.UserSettings, error) {
	var settingValue string
	err := store.database.QueryRowContext(ctx,
		`SELECT setting_value FROM settings WHERE setting_key = ?`, userSettingsKey).Scan(&settingValue)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultUserSettings(), nil
	}
	if err != nil {
		return domain.UserSettings{}, err
	}
	userSettings := domain.DefaultUserSettings()
	if err := json.Unmarshal([]byte(settingValue), &userSettings); err != nil {
		return domain.UserSettings{}, fmt.Errorf("decode user settings: %w", err)
	}
	return userSettings, nil
}

func (store *Store) SaveUserSettings(ctx context.Context, userSettings domain.UserSettings) error {
	encodedSettings, err := json.Marshal(userSettings)
	if err != nil {
		return err
	}
	_, err = store.database.ExecContext(ctx,
		`INSERT OR REPLACE INTO settings (setting_key, setting_value) VALUES (?, ?)`, userSettingsKey, string(encodedSettings))
	return err
}
