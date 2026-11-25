// Package sqlite persists articles, the race calendar, circuit drawings and user settings.
// All timestamps are stored as UTC unix seconds.
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure Go driver, no CGO
)

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS articles (
		id            TEXT PRIMARY KEY,
		series_slug   TEXT NOT NULL,
		source_slug   TEXT NOT NULL,
		source_name   TEXT NOT NULL,
		language_code TEXT NOT NULL,
		title         TEXT NOT NULL,
		summary       TEXT NOT NULL,
		article_url   TEXT NOT NULL,
		image_url     TEXT NOT NULL DEFAULT '',
		published_at  INTEGER NOT NULL,
		fetched_at    INTEGER NOT NULL,
		read_at       INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS articles_published_at_index ON articles (published_at DESC)`,
	`CREATE INDEX IF NOT EXISTS articles_unread_index ON articles (read_at) WHERE read_at IS NULL`,
	`CREATE TABLE IF NOT EXISTS race_events (
		id           TEXT PRIMARY KEY,
		series_slug  TEXT NOT NULL,
		season       INTEGER NOT NULL,
		round        INTEGER NOT NULL,
		event_name   TEXT NOT NULL,
		circuit_id   TEXT NOT NULL,
		circuit_name TEXT NOT NULL,
		locality     TEXT NOT NULL,
		country      TEXT NOT NULL,
		latitude     REAL NOT NULL,
		longitude    REAL NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS race_sessions (
		race_event_id        TEXT NOT NULL REFERENCES race_events (id) ON DELETE CASCADE,
		session_kind         TEXT NOT NULL,
		starts_at            INTEGER NOT NULL,
		start_time_confirmed INTEGER NOT NULL,
		PRIMARY KEY (race_event_id, session_kind)
	)`,
	`CREATE TABLE IF NOT EXISTS circuit_layouts (
		circuit_id       TEXT PRIMARY KEY,
		layout_id        TEXT NOT NULL,
		svg_path         TEXT NOT NULL,
		svg_view_box     TEXT NOT NULL,
		attribution_text TEXT NOT NULL,
		attribution_url  TEXT NOT NULL,
		fetched_at       INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS settings (
		setting_key   TEXT PRIMARY KEY,
		setting_value TEXT NOT NULL
	)`,
	// Responses of slow or flaky external APIs (results, circuit facts, weather), as JSON.
	`CREATE TABLE IF NOT EXISTS api_cache (
		cache_key  TEXT PRIMARY KEY,
		payload    TEXT NOT NULL,
		fetched_at INTEGER NOT NULL
	)`,
}

// Store wraps the SQLite database.
type Store struct {
	database *sql.DB
}

// Open creates (if needed) and migrates the database inside dataDirectory.
func Open(dataDirectory string) (*Store, error) {
	if err := os.MkdirAll(dataDirectory, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	databasePath := filepath.Join(dataDirectory, "speed-news.db")
	// journal_size_limit keeps the WAL file from staying at its peak size after a checkpoint;
	// cache_size (negative = KiB) caps SQLite's page cache: the database is small and mostly idle.
	connectionString := "file:" + databasePath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)" +
		"&_pragma=journal_size_limit(4194304)&_pragma=cache_size(-1024)"
	return openWithConnectionString(connectionString)
}

// OpenInMemory is used by tests.
func OpenInMemory() (*Store, error) {
	return openWithConnectionString("file::memory:?_pragma=foreign_keys(ON)")
}

func openWithConnectionString(connectionString string) (*Store, error) {
	database, err := sql.Open("sqlite", connectionString)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite allows a single writer; one connection avoids "database is locked" and keeps in-memory DBs shared.
	database.SetMaxOpenConns(1)

	if err := enableIncrementalAutoVacuum(database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("enable auto vacuum: %w", err)
	}

	for _, schemaStatement := range schemaStatements {
		if _, err := database.Exec(schemaStatement); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	for _, addedColumn := range addedColumns {
		if err := ensureColumn(database, addedColumn.tableName, addedColumn.columnName, addedColumn.columnDefinition); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	return &Store{database: database}, nil
}

// Columns added after the first release: CREATE TABLE IF NOT EXISTS does not touch existing
// databases, so they are added here when missing.
var addedColumns = []struct {
	tableName        string
	columnName       string
	columnDefinition string
}{
	// 1 once the article page was checked for a share image (found or not): never retried.
	{"articles", "image_lookup_done", "INTEGER NOT NULL DEFAULT 0"},
}

// SQLite's PRAGMA auto_vacuum value for INCREMENTAL.
const autoVacuumIncremental = 2

// enableIncrementalAutoVacuum lets Maintain give space back to the file system after deletions,
// so years of articles coming and going do not leave a file the size of its busiest month.
// Databases created before this setting need one VACUUM to switch; VACUUM keeps every row.
func enableIncrementalAutoVacuum(database *sql.DB) error {
	var autoVacuumMode int
	if err := database.QueryRow(`PRAGMA auto_vacuum`).Scan(&autoVacuumMode); err != nil {
		return err
	}
	if autoVacuumMode == autoVacuumIncremental {
		return nil
	}
	if _, err := database.Exec(`PRAGMA auto_vacuum = INCREMENTAL`); err != nil {
		return err
	}
	_, err := database.Exec(`VACUUM`)
	return err
}

func ensureColumn(database *sql.DB, tableName string, columnName string, columnDefinition string) error {
	columnRows, err := database.Query(`SELECT name FROM pragma_table_info(?)`, tableName)
	if err != nil {
		return err
	}
	defer columnRows.Close()
	for columnRows.Next() {
		var existingColumn string
		if err := columnRows.Scan(&existingColumn); err != nil {
			return err
		}
		if existingColumn == columnName {
			return nil
		}
	}
	if err := columnRows.Err(); err != nil {
		return err
	}
	// Names come from the constant list above, never from input.
	_, err = database.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableName, columnName, columnDefinition))
	return err
}

func (store *Store) Close() error {
	return store.database.Close()
}
