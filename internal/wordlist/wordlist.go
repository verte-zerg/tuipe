// Package wordlist loads word lists from files.
package wordlist

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadWords reads one word per line from the provided file path.
func LoadWords(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			// Best-effort close for read-only word list.
			_ = cerr
		}
	}()

	var words []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		words = append(words, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("word list is empty")
	}
	return words, nil
}
