package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"speed-news/apps/api/internal/domain"
)

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

// Circuit layouts (track drawings) are stored next to the calendar that references them.

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
