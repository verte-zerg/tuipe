package stats

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/verte-zerg/tuipe/internal/model"
	"github.com/verte-zerg/tuipe/internal/store"
)

func TestBuildReport(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tuipe.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	ctx := context.Background()
	var ids []int64
	for i := 0; i < 3; i++ {
		start := time.Unix(0, 0).Add(time.Duration(i) * time.Minute)
		end := start.Add(30 * time.Second)
		stats := model.SessionStats{
			StartedAt:         start,
			EndedAt:           end,
			Lang:              "en",
			Words:             10,
			CapsPct:           0,
			PunctPct:          0,
			PunctSet:          ".,?!",
			WordListPath:      "dummy",
			CorrectNonSpace:   10,
			IncorrectNonSpace: 1,
			DurationMs:        end.Sub(start).Milliseconds(),
		}
		charStats := []model.CharStats{
			{Char: "a", Correct: 5, Incorrect: 0},
			{Char: "b", Correct: 4, Incorrect: 1},
		}
		id, err := st.InsertSession(ctx, stats, charStats)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
		ids = append(ids, id)
	}

	cfg := model.StatsConfig{
		Lang:        "en",
		Last:        2,
		CurveWindow: 2,
		Chars:       "a,b",
	}
	report, err := BuildReport(ctx, st, cfg)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if len(report.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(report.Sessions))
	}
	if report.Sessions[0].SessionID != ids[1] || report.Sessions[1].SessionID != ids[2] {
		t.Fatalf("unexpected session ids: %+v", report.Sessions)
	}
	if len(report.WindowSessionIDs) != 2 {
		t.Fatalf("expected 2 window session ids, got %d", len(report.WindowSessionIDs))
	}
	if len(report.CharAggsAll) == 0 {
		t.Fatalf("expected char aggregates for all sessions")
	}
	if len(report.CharAggsWindow) == 0 {
		t.Fatalf("expected char aggregates for window sessions")
	}
}
