// Package generator builds typing text sequences.
package generator

import (
	"math/rand"
	"time"
	"unicode"
)

// Generator produces randomized typing text.
type Generator struct {
	rnd *rand.Rand
}

// New returns a Generator seeded with the current time.
func New() *Generator {
	return &Generator{rnd: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// Generate selects words uniformly and applies caps/punctuation rules.
func (g *Generator) Generate(words []string, count int, capsPct, punctPct float64, punctSet []rune) []string {
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		word := words[g.rnd.Intn(len(words))]
		word = applyCaps(g.rnd, word, capsPct)
		word = applyPunct(g.rnd, word, punctPct, punctSet)
		result = append(result, word)
	}
	return result
}

// GenerateWeighted selects words with a bias toward weak characters.
func (g *Generator) GenerateWeighted(words []string, count int, capsPct, punctPct float64, punctSet []rune, weakSet map[rune]struct{}, factor float64) []string {
	weights := make([]float64, len(words))
	total := 0.0
	for i, word := range words {
		weakCount := 0
		for _, r := range word {
			if _, ok := weakSet[r]; ok {
				weakCount++
			}
		}
		w := 1.0 + float64(weakCount)*factor
		weights[i] = w
		total += w
	}

	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		r := g.rnd.Float64() * total
		acc := 0.0
		idx := 0
		for j, w := range weights {
			acc += w
			if r <= acc {
				idx = j
				break
			}
		}
		word := words[idx]
		word = applyCaps(g.rnd, word, capsPct)
		word = applyPunct(g.rnd, word, punctPct, punctSet)
		result = append(result, word)
	}
	return result
}

func applyCaps(rnd *rand.Rand, word string, capsPct float64) string {
	if capsPct <= 0 {
		return word
	}
	if rnd.Float64() > capsPct {
		return word
	}
	runes := []rune(word)
	if len(runes) == 0 {
		return word
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func applyPunct(rnd *rand.Rand, word string, punctPct float64, punctSet []rune) string {
	if punctPct <= 0 || len(punctSet) == 0 {
		return word
	}
	if rnd.Float64() > punctPct {
		return word
	}
	punct := punctSet[rnd.Intn(len(punctSet))]
	return word + string(punct)
}
