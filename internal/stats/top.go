// Package stats contains statistics calculations and reporting.
package stats

import (
	"sort"

	"github.com/verte-zerg/tuipe/internal/model"
)

// TopCharsByFrequency returns the top N characters by total frequency.
func TopCharsByFrequency(aggs []model.CharAggregate, n int) []string {
	if n <= 0 || len(aggs) == 0 {
		return nil
	}
	type item struct {
		ch    string
		total int
	}
	items := make([]item, 0, len(aggs))
	for _, agg := range aggs {
		items = append(items, item{
			ch:    agg.Char,
			total: agg.Correct + agg.Incorrect,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].total == items[j].total {
			return items[i].ch < items[j].ch
		}
		return items[i].total > items[j].total
	})
	if n > len(items) {
		n = len(items)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, items[i].ch)
	}
	return out
}
