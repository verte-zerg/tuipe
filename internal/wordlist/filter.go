// Package wordlist provides word list filtering helpers.
package wordlist

import "strings"

// FilterFunc returns true when a word should be kept.
type FilterFunc func(string) bool

// FilterForLang returns a language-specific filter for word lists.
func FilterForLang(lang string) FilterFunc {
	switch strings.ToLower(lang) {
	case "en":
		return filterEnglishASCII
	default:
		return func(string) bool { return true }
	}
}

func filterEnglishASCII(word string) bool {
	if word == "" {
		return false
	}
	for i := 0; i < len(word); i++ {
		ch := word[i]
		if ch < 'a' || ch > 'z' {
			return false
		}
	}
	return true
}
