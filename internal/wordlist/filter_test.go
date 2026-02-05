package wordlist

import "testing"

func TestFilterEnglishASCII(t *testing.T) {
	filter := FilterForLang("en")
	if !filter("hello") {
		t.Fatalf("expected hello to pass english filter")
	}
	for _, word := range []string{"résumé", "naïve", "don’t", "co-op"} {
		if filter(word) {
			t.Fatalf("expected %q to be rejected", word)
		}
	}
}
