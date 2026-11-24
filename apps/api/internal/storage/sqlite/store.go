// Package sqlite persists articles, the race calendar, circuit drawings and user settings.
// All timestamps are stored as UTC unix seconds.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure Go driver, no CGO

	"speed-news/apps/api/internal/domain"
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
	connectionString := "file:" + databasePath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
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

// ---------- articles ----------

// InsertArticles stores new articles and ignores ones already known.
// Articles published before autoReadBefore are stored as read, so the first run does not show 200 unread items.
func (store *Store) InsertArticles(ctx context.Context, articles []domain.Article, autoReadBefore time.Time) (int, error) {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = transaction.Rollback() }() // no-op after Commit

	// Known articles are kept as they are (read state included), except that a missing image is
	// filled in when a later fetch finds one (e.g. after improving image extraction).
	insertStatement, err := transaction.PrepareContext(ctx, `INSERT INTO articles
		(id, series_slug, source_slug, source_name, language_code, title, summary, article_url, image_url, published_at, fetched_at, read_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET image_url = excluded.image_url
		WHERE articles.image_url = '' AND excluded.image_url != ''`)
	if err != nil {
		return 0, err
	}
	defer insertStatement.Close()

	insertedCount := 0
	for _, article := range articles {
		var readAtValue any
		if article.PublishedAt.Before(autoReadBefore) {
			readAtValue = article.FetchedAt.Unix()
		}
		insertResult, err := insertStatement.ExecContext(ctx,
			article.ID, string(article.SeriesSlug), article.SourceSlug, article.SourceName, article.LanguageCode,
			article.Title, article.Summary, article.ArticleURL, article.ImageURL,
			article.PublishedAt.Unix(), article.FetchedAt.Unix(), readAtValue)
		if err != nil {
			return 0, fmt.Errorf("insert article %s: %w", article.ID, err)
		}
		affectedRows, _ := insertResult.RowsAffected()
		insertedCount += int(affectedRows)
	}
	return insertedCount, transaction.Commit()
}

// ArticleFilter narrows article queries. An empty LanguageCodes means every language.
type ArticleFilter struct {
	OnlyUnread    bool
	LanguageCodes []string
}

// whereClause builds "WHERE ..." with placeholders, or "" when nothing is filtered.
func (articleFilter ArticleFilter) whereClause() (string, []any) {
	conditions := []string{}
	queryArguments := []any{}
	if articleFilter.OnlyUnread {
		conditions = append(conditions, "read_at IS NULL")
	}
	if len(articleFilter.LanguageCodes) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(articleFilter.LanguageCodes)), ", ")
		conditions = append(conditions, "language_code IN ("+placeholders+")")
		for _, languageCode := range articleFilter.LanguageCodes {
			queryArguments = append(queryArguments, languageCode)
		}
	}
	if len(conditions) == 0 {
		return "", queryArguments
	}
	return " WHERE " + strings.Join(conditions, " AND "), queryArguments
}

// ListArticles returns the newest articles first.
func (store *Store) ListArticles(ctx context.Context, articleFilter ArticleFilter, maximumResults int) ([]domain.Article, error) {
	whereClause, queryArguments := articleFilter.whereClause()
	articlesQuery := `SELECT id, series_slug, source_slug, source_name, language_code, title, summary,
		article_url, image_url, published_at, fetched_at, read_at FROM articles` +
		whereClause + ` ORDER BY published_at DESC LIMIT ?`

	articleRows, err := store.database.QueryContext(ctx, articlesQuery, append(queryArguments, maximumResults)...)
	if err != nil {
		return nil, err
	}
	defer articleRows.Close()

	articles := []domain.Article{}
	for articleRows.Next() {
		var article domain.Article
		var seriesSlug string
		var publishedAtUnix, fetchedAtUnix int64
		var readAtUnix sql.NullInt64
		if err := articleRows.Scan(&article.ID, &seriesSlug, &article.SourceSlug, &article.SourceName,
			&article.LanguageCode, &article.Title, &article.Summary, &article.ArticleURL, &article.ImageURL,
			&publishedAtUnix, &fetchedAtUnix, &readAtUnix); err != nil {
			return nil, err
		}
		article.SeriesSlug = domain.SeriesSlug(seriesSlug)
		article.PublishedAt = time.Unix(publishedAtUnix, 0).UTC()
		article.FetchedAt = time.Unix(fetchedAtUnix, 0).UTC()
		if readAtUnix.Valid {
			readAt := time.Unix(readAtUnix.Int64, 0).UTC()
			article.ReadAt = &readAt
		}
		articles = append(articles, article)
	}
	return articles, articleRows.Err()
}

// CountUnreadArticles counts unread articles in the given languages (empty = all).
func (store *Store) CountUnreadArticles(ctx context.Context, languageCodes []string) (int, error) {
	whereClause, queryArguments := ArticleFilter{OnlyUnread: true, LanguageCodes: languageCodes}.whereClause()
	var unreadCount int
	err := store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM articles`+whereClause, queryArguments...).Scan(&unreadCount)
	return unreadCount, err
}

// MarkArticleRead returns false when the article does not exist.
func (store *Store) MarkArticleRead(ctx context.Context, articleID string, readAt time.Time) (bool, error) {
	updateResult, err := store.database.ExecContext(ctx,
		`UPDATE articles SET read_at = COALESCE(read_at, ?) WHERE id = ?`, readAt.Unix(), articleID)
	if err != nil {
		return false, err
	}
	affectedRows, err := updateResult.RowsAffected()
	return affectedRows > 0, err
}

// ArticlePage identifies an article whose page should be checked for a share image.
type ArticlePage struct {
	ArticleID  string
	ArticleURL string
}

// ListArticlesAwaitingImageLookup returns the newest articles of a source with no image that
// were never checked.
func (store *Store) ListArticlesAwaitingImageLookup(ctx context.Context, sourceSlug string, maximumResults int) ([]ArticlePage, error) {
	articleRows, err := store.database.QueryContext(ctx, `SELECT id, article_url FROM articles
		WHERE source_slug = ? AND image_url = '' AND image_lookup_done = 0
		ORDER BY published_at DESC LIMIT ?`, sourceSlug, maximumResults)
	if err != nil {
		return nil, err
	}
	defer articleRows.Close()

	articlePages := []ArticlePage{}
	for articleRows.Next() {
		var articlePage ArticlePage
		if err := articleRows.Scan(&articlePage.ArticleID, &articlePage.ArticleURL); err != nil {
			return nil, err
		}
		articlePages = append(articlePages, articlePage)
	}
	return articlePages, articleRows.Err()
}

// SaveImageLookupResult stores the found image (if any) and marks the article as checked.
func (store *Store) SaveImageLookupResult(ctx context.Context, articleID string, imageURL string) error {
	_, err := store.database.ExecContext(ctx, `UPDATE articles
		SET image_url = CASE WHEN image_url = '' THEN ? ELSE image_url END, image_lookup_done = 1
		WHERE id = ?`, imageURL, articleID)
	return err
}

// MarkArticleUnread returns false when the article does not exist.
func (store *Store) MarkArticleUnread(ctx context.Context, articleID string) (bool, error) {
	updateResult, err := store.database.ExecContext(ctx, `UPDATE articles SET read_at = NULL WHERE id = ?`, articleID)
	if err != nil {
		return false, err
	}
	affectedRows, err := updateResult.RowsAffected()
	return affectedRows > 0, err
}

// MarkAllArticlesRead only touches the languages the user sees, so hidden languages keep their state.
func (store *Store) MarkAllArticlesRead(ctx context.Context, readAt time.Time, languageCodes []string) error {
	whereClause, queryArguments := ArticleFilter{OnlyUnread: true, LanguageCodes: languageCodes}.whereClause()
	_, err := store.database.ExecContext(ctx, `UPDATE articles SET read_at = ?`+whereClause,
		append([]any{readAt.Unix()}, queryArguments...)...)
	return err
}

// DeleteArticlesPublishedBefore keeps the database small.
func (store *Store) DeleteArticlesPublishedBefore(ctx context.Context, cutoff time.Time) error {
	_, err := store.database.ExecContext(ctx, `DELETE FROM articles WHERE published_at < ?`, cutoff.Unix())
	return err
}

// ---------- race calendar ----------

// ReplaceSeasonEvents swaps the whole season, so cancelled or moved races disappear.
func (store *Store) ReplaceSeasonEvents(ctx context.Context, seriesSlug domain.SeriesSlug, season int, raceEvents []domain.RaceEvent) error {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }() // no-op after Commit

	if _, err := transaction.ExecContext(ctx,
		`DELETE FROM race_events WHERE series_slug = ? AND season = ?`, string(seriesSlug), season); err != nil {
		return err
	}
	for _, raceEvent := range raceEvents {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO race_events
			(id, series_slug, season, round, event_name, circuit_id, circuit_name, locality, country, latitude, longitude)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			raceEvent.ID, string(raceEvent.SeriesSlug), raceEvent.Season, raceEvent.Round, raceEvent.EventName,
			raceEvent.CircuitID, raceEvent.CircuitName, raceEvent.Locality, raceEvent.Country,
			raceEvent.Coordinates.Latitude, raceEvent.Coordinates.Longitude); err != nil {
			return fmt.Errorf("insert race event %s: %w", raceEvent.ID, err)
		}
		for _, raceSession := range raceEvent.Sessions {
			if _, err := transaction.ExecContext(ctx, `INSERT OR REPLACE INTO race_sessions
				(race_event_id, session_kind, starts_at, start_time_confirmed) VALUES (?, ?, ?, ?)`,
				raceEvent.ID, string(raceSession.SessionKind), raceSession.StartsAt.Unix(), raceSession.StartTimeConfirmed); err != nil {
				return fmt.Errorf("insert session %s/%s: %w", raceEvent.ID, raceSession.SessionKind, err)
			}
		}
	}
	return transaction.Commit()
}

// FindUpcomingRaceEvent returns the first event whose last session started after notBefore.
// Returns nil when the calendar has nothing ahead.
func (store *Store) FindUpcomingRaceEvent(ctx context.Context, seriesSlug domain.SeriesSlug, notBefore time.Time) (*domain.RaceEvent, error) {
	var raceEventID string
	err := store.database.QueryRowContext(ctx, `SELECT race_sessions.race_event_id
		FROM race_sessions JOIN race_events ON race_events.id = race_sessions.race_event_id
		WHERE race_events.series_slug = ?
		GROUP BY race_sessions.race_event_id
		HAVING MAX(race_sessions.starts_at) >= ?
		ORDER BY MIN(race_sessions.starts_at) LIMIT 1`, string(seriesSlug), notBefore.Unix()).Scan(&raceEventID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return store.loadRaceEvent(ctx, raceEventID)
}

func (store *Store) loadRaceEvent(ctx context.Context, raceEventID string) (*domain.RaceEvent, error) {
	var raceEvent domain.RaceEvent
	var seriesSlug string
	err := store.database.QueryRowContext(ctx, `SELECT id, series_slug, season, round, event_name, circuit_id,
		circuit_name, locality, country, latitude, longitude FROM race_events WHERE id = ?`, raceEventID).Scan(
		&raceEvent.ID, &seriesSlug, &raceEvent.Season, &raceEvent.Round, &raceEvent.EventName, &raceEvent.CircuitID,
		&raceEvent.CircuitName, &raceEvent.Locality, &raceEvent.Country,
		&raceEvent.Coordinates.Latitude, &raceEvent.Coordinates.Longitude)
	if err != nil {
		return nil, err
	}
	raceEvent.SeriesSlug = domain.SeriesSlug(seriesSlug)

	sessionRows, err := store.database.QueryContext(ctx, `SELECT session_kind, starts_at, start_time_confirmed
		FROM race_sessions WHERE race_event_id = ? ORDER BY starts_at`, raceEventID)
	if err != nil {
		return nil, err
	}
	defer sessionRows.Close()

	raceEvent.Sessions = []domain.RaceSession{}
	for sessionRows.Next() {
		var raceSession domain.RaceSession
		var sessionKind string
		var startsAtUnix int64
		if err := sessionRows.Scan(&sessionKind, &startsAtUnix, &raceSession.StartTimeConfirmed); err != nil {
			return nil, err
		}
		raceSession.SessionKind = domain.SessionKind(sessionKind)
		raceSession.StartsAt = time.Unix(startsAtUnix, 0).UTC()
		raceEvent.Sessions = append(raceEvent.Sessions, raceSession)
	}
	return &raceEvent, sessionRows.Err()
}

// ---------- circuit layouts ----------

func (store *Store) FindCircuitLayout(ctx context.Context, circuitID string) (*domain.CircuitLayout, error) {
	var circuitLayout domain.CircuitLayout
	err := store.database.QueryRowContext(ctx, `SELECT circuit_id, layout_id, svg_path, svg_view_box,
		attribution_text, attribution_url FROM circuit_layouts WHERE circuit_id = ?`, circuitID).Scan(
		&circuitLayout.CircuitID, &circuitLayout.LayoutID, &circuitLayout.SVGPath, &circuitLayout.SVGViewBox,
		&circuitLayout.AttributionText, &circuitLayout.AttributionURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &circuitLayout, nil
}

func (store *Store) SaveCircuitLayout(ctx context.Context, circuitLayout domain.CircuitLayout, fetchedAt time.Time) error {
	_, err := store.database.ExecContext(ctx, `INSERT OR REPLACE INTO circuit_layouts
		(circuit_id, layout_id, svg_path, svg_view_box, attribution_text, attribution_url, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		circuitLayout.CircuitID, circuitLayout.LayoutID, circuitLayout.SVGPath, circuitLayout.SVGViewBox,
		circuitLayout.AttributionText, circuitLayout.AttributionURL, fetchedAt.Unix())
	return err
}

// ---------- api cache ----------

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

// ---------- settings ----------

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
