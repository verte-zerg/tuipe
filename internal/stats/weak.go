package stats

import (
	"sort"

	"github.com/verte-zerg/tuipe/internal/model"
)

// SelectWeakChars selects the lowest-accuracy characters from aggregates.
func SelectWeakChars(aggs []model.CharAggregate, top int) map[rune]struct{} {
	weakSet := map[rune]struct{}{}
	if len(aggs) == 0 {
		return weakSet
	}
	candidates := make([]model.CharAggregate, len(aggs))
	copy(candidates, aggs)
	sort.Slice(candidates, func(i, j int) bool {
		ai := accuracy(candidates[i])
		aj := accuracy(candidates[j])
		if ai == aj {
			return candidates[i].Char < candidates[j].Char
		}
		return ai < aj
	})
	if top <= 0 || top > len(candidates) {
		top = len(candidates)
	}
	for i := 0; i < top; i++ {
		runes := []rune(candidates[i].Char)
		if len(runes) > 0 {
			weakSet[runes[0]] = struct{}{}
		}
	}
	return weakSet
}

func accuracy(agg model.CharAggregate) float64 {
	total := agg.Correct + agg.Incorrect
	if total == 0 {
		return 1.0
	}
	return float64(agg.Correct) / float64(total)
}
