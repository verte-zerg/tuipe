// Package model defines shared data structures.
package model

import "time"

// Config defines practice settings.
type Config struct {
	Lang       string
	Words      int
	CapsPct    float64
	PunctPct   float64
	PunctSet   string
	FocusWeak  bool
	WeakTop    int
	WeakFactor float64
	WeakWindow int
}

// StatsConfig defines filters and options for stats output.
type StatsConfig struct {
	Lang        string
	Since       *time.Time
	Last        int
	CurveWindow int
	Chars       string
}

// SessionStats captures a completed typing session.
type SessionStats struct {
	StartedAt         time.Time
	EndedAt           time.Time
	Lang              string
	Words             int
	CapsPct           float64
	PunctPct          float64
	PunctSet          string
	WordListPath      string
	CorrectNonSpace   int
	IncorrectNonSpace int
	DurationMs        int64
}

// CharStats stores per-character stats for a session.
type CharStats struct {
	Char         string
	Correct      int
	Incorrect    int
	LatencySumMs int64
	LatencyCount int64
}

// Aggregated per-char stats for selection or reporting.

// CharAggregate aggregates character stats across sessions.
type CharAggregate struct {
	Char         string
	Correct      int
	Incorrect    int
	LatencySumMs int64
	LatencyCount int64
}

// SessionAggregate summarizes a session for reporting.
type SessionAggregate struct {
	SessionID  int64
	EndedAt    time.Time
	Correct    int
	Incorrect  int
	DurationMs int64
}
