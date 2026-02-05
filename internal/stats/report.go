// Package stats contains statistics calculations and reporting.
package stats

import (
	"context"

	"github.com/verte-zerg/tuipe/internal/model"
	"github.com/verte-zerg/tuipe/internal/store"
)

// Report contains precomputed data for stats rendering.
type Report struct {
	Sessions         []model.SessionAggregate
	WindowSessionIDs []int64
	CharAggsAll      []model.CharAggregate
	CharAggsWindow   []model.CharAggregate
}

// BuildReport loads and prepares data for stats rendering.
func BuildReport(ctx context.Context, st *store.Store, cfg model.StatsConfig) (Report, error) {
	sessions, err := st.ListSessions(ctx, cfg)
	if err != nil {
		return Report{}, err
	}
	if cfg.Last > 0 && len(sessions) > cfg.Last {
		sessions = sessions[len(sessions)-cfg.Last:]
	}

	allIDs := sessionIDs(sessions)
	windowIDs := lastSessionIDs(sessions, cfg.CurveWindow)
	charAggsAll, err := st.ListCharAggregatesForSessions(ctx, allIDs)
	if err != nil {
		return Report{}, err
	}
	charAggsWindow, err := st.ListCharAggregatesForSessions(ctx, windowIDs)
	if err != nil {
		return Report{}, err
	}

	return Report{
		Sessions:         sessions,
		WindowSessionIDs: windowIDs,
		CharAggsAll:      charAggsAll,
		CharAggsWindow:   charAggsWindow,
	}, nil
}

func sessionIDs(sessions []model.SessionAggregate) []int64 {
	ids := make([]int64, len(sessions))
	for i, s := range sessions {
		ids[i] = s.SessionID
	}
	return ids
}

func lastSessionIDs(sessions []model.SessionAggregate, window int) []int64 {
	if window <= 0 || len(sessions) <= window {
		return sessionIDs(sessions)
	}
	return sessionIDs(sessions[len(sessions)-window:])
}
