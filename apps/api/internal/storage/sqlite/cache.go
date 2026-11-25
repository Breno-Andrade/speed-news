package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// LoadCachedPayload returns the cached JSON and when it was fetched; found is false on a miss.
func (store *Store) LoadCachedPayload(ctx context.Context, cacheKey string) (payload string, fetchedAt time.Time, found bool, err error) {
	var fetchedAtUnix int64
	err = store.database.QueryRowContext(ctx,
		`SELECT payload, fetched_at FROM api_cache WHERE cache_key = ?`, cacheKey).Scan(&payload, &fetchedAtUnix)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, err
	}
	return payload, time.Unix(fetchedAtUnix, 0).UTC(), true, nil
}

func (store *Store) SaveCachedPayload(ctx context.Context, cacheKey string, payload string, fetchedAt time.Time) error {
	_, err := store.database.ExecContext(ctx,
		`INSERT OR REPLACE INTO api_cache (cache_key, payload, fetched_at) VALUES (?, ?, ?)`, cacheKey, payload, fetchedAt.Unix())
	return err
}
