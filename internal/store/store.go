// Package store handles SQLite persistence.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/verte-zerg/tuipe/internal/model"

	_ "modernc.org/sqlite" // SQLite driver.
)

// Store wraps SQLite access for session data.
type Store struct {
	db *sql.DB
}

// Open opens or creates the SQLite database and applies migrations.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		if cerr := db.Close(); cerr != nil {
			// Best-effort close on migration failure.
			_ = cerr
		}
		return nil, err
	}
	return store, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY,
			started_at TEXT NOT NULL,
			ended_at TEXT NOT NULL,
			lang TEXT NOT NULL,
			words INTEGER NOT NULL,
			caps_pct REAL NOT NULL,
			punct_pct REAL NOT NULL,
			punct_set TEXT NOT NULL,
			wordlist_path TEXT NOT NULL,
			correct_nonspace INTEGER NOT NULL,
			incorrect_nonspace INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS session_char_stats (
			session_id INTEGER NOT NULL,
			char TEXT NOT NULL,
			correct INTEGER NOT NULL,
			incorrect INTEGER NOT NULL,
			latency_sum_ms INTEGER NOT NULL,
			latency_count INTEGER NOT NULL,
			PRIMARY KEY (session_id, char)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_ended_at ON sessions(ended_at);`,
		`CREATE INDEX IF NOT EXISTS idx_session_char_stats_char ON session_char_stats(char);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// InsertSession stores a completed session and its per-character stats.
func (s *Store) InsertSession(ctx context.Context, stats model.SessionStats, chars []model.CharStats) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			if rerr := tx.Rollback(); rerr != nil {
				// Best-effort rollback.
				_ = rerr
			}
		}
	}()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO sessions (started_at, ended_at, lang, words, caps_pct, punct_pct, punct_set, wordlist_path, correct_nonspace, incorrect_nonspace, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		stats.StartedAt.Format(time.RFC3339Nano),
		stats.EndedAt.Format(time.RFC3339Nano),
		stats.Lang,
		stats.Words,
		stats.CapsPct,
		stats.PunctPct,
		stats.PunctSet,
		stats.WordListPath,
		stats.CorrectNonSpace,
		stats.IncorrectNonSpace,
		stats.DurationMs,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if len(chars) > 0 {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO session_char_stats (session_id, char, correct, incorrect, latency_sum_ms, latency_count)
			 VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return 0, err
		}
		defer func() {
			if cerr := stmt.Close(); cerr != nil {
				// Best-effort statement close.
				_ = cerr
			}
		}()
		for _, cs := range chars {
			if _, err := stmt.ExecContext(ctx, id, cs.Char, cs.Correct, cs.Incorrect, cs.LatencySumMs, cs.LatencyCount); err != nil {
				return 0, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// GetWeakChars aggregates character stats over the most recent sessions.
func (s *Store) GetWeakChars(ctx context.Context, window int, lang string) ([]model.CharAggregate, error) {
	if window <= 0 {
		return nil, nil
	}
	query := `WITH recent_sessions AS (
		SELECT id FROM sessions
		WHERE (? = '' OR lang = ?)
		ORDER BY ended_at DESC
		LIMIT ?
	)
	SELECT cs.char, SUM(cs.correct) AS correct, SUM(cs.incorrect) AS incorrect,
		SUM(cs.latency_sum_ms) AS latency_sum_ms, SUM(cs.latency_count) AS latency_count
	FROM session_char_stats cs
	JOIN recent_sessions r ON r.id = cs.session_id
	GROUP BY cs.char`

	rows, err := s.db.QueryContext(ctx, query, lang, lang, window)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			// Best-effort rows close.
			_ = cerr
		}
	}()

	var result []model.CharAggregate
	for rows.Next() {
		var agg model.CharAggregate
		if err := rows.Scan(&agg.Char, &agg.Correct, &agg.Incorrect, &agg.LatencySumMs, &agg.LatencyCount); err != nil {
			return nil, err
		}
		result = append(result, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ListSessions returns session aggregates filtered by stats config.
func (s *Store) ListSessions(ctx context.Context, cfg model.StatsConfig) ([]model.SessionAggregate, error) {
	clauses := []string{"1=1"}
	args := []any{}
	if cfg.Lang != "" {
		clauses = append(clauses, "lang = ?")
		args = append(args, cfg.Lang)
	}
	if cfg.Since != nil {
		clauses = append(clauses, "ended_at >= ?")
		args = append(args, cfg.Since.Format(time.RFC3339Nano))
	}
	query := fmt.Sprintf(`SELECT id, ended_at, correct_nonspace, incorrect_nonspace, duration_ms
		FROM sessions
		WHERE %s
		ORDER BY ended_at ASC`, strings.Join(clauses, " AND "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			// Best-effort rows close.
			_ = cerr
		}
	}()

	var sessions []model.SessionAggregate
	for rows.Next() {
		var agg model.SessionAggregate
		var endedAt string
		if err := rows.Scan(&agg.SessionID, &endedAt, &agg.Correct, &agg.Incorrect, &agg.DurationMs); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, endedAt)
		if err != nil {
			return nil, err
		}
		agg.EndedAt = parsed
		sessions = append(sessions, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

// ListCharAggregatesForSessions aggregates per-character stats across sessions.
func (s *Store) ListCharAggregatesForSessions(ctx context.Context, sessionIDs []int64) ([]model.CharAggregate, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(sessionIDs))
	args := make([]any, len(sessionIDs))
	for i, id := range sessionIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT char, SUM(correct) AS correct, SUM(incorrect) AS incorrect,
		SUM(latency_sum_ms) AS latency_sum_ms, SUM(latency_count) AS latency_count
		FROM session_char_stats
		WHERE session_id IN (%s)
		GROUP BY char`, strings.Join(placeholders, ","))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			// Best-effort rows close.
			_ = cerr
		}
	}()

	var result []model.CharAggregate
	for rows.Next() {
		var agg model.CharAggregate
		if err := rows.Scan(&agg.Char, &agg.Correct, &agg.Incorrect, &agg.LatencySumMs, &agg.LatencyCount); err != nil {
			return nil, err
		}
		result = append(result, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ListCharStatsForSessions returns per-session stats for selected characters.
func (s *Store) ListCharStatsForSessions(ctx context.Context, sessionIDs []int64, chars []string) (map[int64]map[string]model.CharAggregate, error) {
	if len(sessionIDs) == 0 || len(chars) == 0 {
		return map[int64]map[string]model.CharAggregate{}, nil
	}
	idPlaceholders := make([]string, len(sessionIDs))
	args := make([]any, 0, len(sessionIDs)+len(chars))
	for i, id := range sessionIDs {
		idPlaceholders[i] = "?"
		args = append(args, id)
	}
	charPlaceholders := make([]string, len(chars))
	for i, ch := range chars {
		charPlaceholders[i] = "?"
		args = append(args, ch)
	}

	query := fmt.Sprintf(`SELECT session_id, char, correct, incorrect, latency_sum_ms, latency_count
		FROM session_char_stats
		WHERE session_id IN (%s) AND char IN (%s)`, strings.Join(idPlaceholders, ","), strings.Join(charPlaceholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			// Best-effort rows close.
			_ = cerr
		}
	}()

	result := map[int64]map[string]model.CharAggregate{}
	for rows.Next() {
		var sessionID int64
		var agg model.CharAggregate
		if err := rows.Scan(&sessionID, &agg.Char, &agg.Correct, &agg.Incorrect, &agg.LatencySumMs, &agg.LatencyCount); err != nil {
			return nil, err
		}
		if _, ok := result[sessionID]; !ok {
			result[sessionID] = map[string]model.CharAggregate{}
		}
		result[sessionID][agg.Char] = agg
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
