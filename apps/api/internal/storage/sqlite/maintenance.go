package sqlite

import (
	"context"
	"time"
)

// CheckpointWAL copies the write-ahead log into the database and truncates it. Cheap; called
// after every refresh so the -wal file does not hold a refresh's worth of pages until the next one.
func (store *Store) CheckpointWAL(ctx context.Context) error {
	_, err := store.database.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// Maintain is the daily housekeeping: it drops cached API responses fetched before cacheCutoff
// (past races and seasons are never asked again), gives free pages back to the file system and
// refreshes the query planner statistics.
func (store *Store) Maintain(ctx context.Context, cacheCutoff time.Time) error {
	if _, err := store.database.ExecContext(ctx, `DELETE FROM api_cache WHERE fetched_at < ?`, cacheCutoff.Unix()); err != nil {
		return err
	}
	if _, err := store.database.ExecContext(ctx, `PRAGMA incremental_vacuum`); err != nil {
		return err
	}
	_, err := store.database.ExecContext(ctx, `PRAGMA optimize`)
	return err
}
