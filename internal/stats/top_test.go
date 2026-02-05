package stats

import (
	"testing"

	"github.com/verte-zerg/tuipe/internal/model"
)

func TestTopCharsByFrequency(t *testing.T) {
	aggs := []model.CharAggregate{
		{Char: "b", Correct: 3, Incorrect: 1},
		{Char: "a", Correct: 2, Incorrect: 2},
		{Char: "c", Correct: 1, Incorrect: 0},
	}
	top := TopCharsByFrequency(aggs, 2)
	if len(top) != 2 {
		t.Fatalf("expected 2 chars, got %d", len(top))
	}
	if top[0] != "a" || top[1] != "b" {
		t.Fatalf("unexpected order: %v", top)
	}
}
