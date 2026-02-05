// Package wordfreq provides word list extraction from the wordfreq dataset.
package wordfreq

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/verte-zerg/tuipe/internal/wordlist"
)

const pypiEndpoint = "https://pypi.org/pypi/wordfreq/json"

// Wheel describes a cached wordfreq wheel.
type Wheel struct {
	Version  string
	Path     string
	Filename string
	Cached   bool
}
type wordEntry struct {
	word  string
	score float64
}

type pypiResponse struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	URLs []struct {
		URL          string `json:"url"`
		Filename     string `json:"filename"`
		Packagetype  string `json:"packagetype"`
		PythonTarget string `json:"python_version"`
	} `json:"urls"`
}

// DownloadLatestWheel fetches the latest wordfreq wheel into cacheDir.
func DownloadLatestWheel(ctx context.Context, cacheDir string) (Wheel, error) {
	if cacheDir == "" {
		return Wheel{}, fmt.Errorf("cache directory is required")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return Wheel{}, fmt.Errorf("failed to create cache dir: %w", err)
	}

	resp, err := httpRequest(ctx, pypiEndpoint)
	if err != nil {
		return Wheel{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return Wheel{}, fmt.Errorf("unexpected pypi status: %s", resp.Status)
	}

	var payload pypiResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Wheel{}, fmt.Errorf("failed to decode pypi response: %w", err)
	}
	if payload.Info.Version == "" {
		return Wheel{}, fmt.Errorf("missing version in pypi response")
	}

	url, filename := pickWheelURL(payload.URLs)
	if url == "" || filename == "" {
		return Wheel{}, fmt.Errorf("no suitable wordfreq wheel found")
	}

	destPath := filepath.Join(cacheDir, filename)
	if _, err := os.Stat(destPath); err == nil {
		return Wheel{Version: payload.Info.Version, Path: destPath, Filename: filename, Cached: true}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Wheel{}, fmt.Errorf("failed to stat cached wheel: %w", err)
	}

	tmpFile, err := os.CreateTemp(cacheDir, "wordfreq-*.whl")
	if err != nil {
		return Wheel{}, fmt.Errorf("failed to create temp wheel: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	wheelResp, err := httpRequest(ctx, url)
	if err != nil {
		return Wheel{}, err
	}
	defer func() {
		_ = wheelResp.Body.Close()
	}()
	if wheelResp.StatusCode != http.StatusOK {
		return Wheel{}, fmt.Errorf("unexpected wheel status: %s", wheelResp.Status)
	}

	if _, err := io.Copy(tmpFile, wheelResp.Body); err != nil {
		return Wheel{}, fmt.Errorf("failed to download wheel: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return Wheel{}, fmt.Errorf("failed to close temp wheel: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		return Wheel{}, fmt.Errorf("failed to move wheel into cache: %w", err)
	}

	return Wheel{Version: payload.Info.Version, Path: destPath, Filename: filename, Cached: false}, nil
}

// ExtractWordlist extracts a word list from the wheel for the given language and type.
func ExtractWordlist(wheelPath, lang, listType string, limit int) ([]string, error) {
	if wheelPath == "" {
		return nil, fmt.Errorf("wheel path is required")
	}
	lang = normalizeLang(lang)
	if lang == "" {
		return nil, fmt.Errorf("unsupported language")
	}
	if listType == "" {
		return nil, fmt.Errorf("word list type is required")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}

	entries, err := readWordEntries(wheelPath, lang, listType)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].score > entries[j].score
	})

	words := make([]string, 0, len(entries))
	seen := make(map[string]struct{})
	langFilter := wordlist.FilterForLang(lang)
	for _, entry := range entries {
		if _, ok := seen[entry.word]; ok {
			continue
		}
		if !isAlpha(entry.word) {
			continue
		}
		length := utf8.RuneCountInString(entry.word)
		if length < 2 || length > 20 {
			continue
		}
		if !langFilter(entry.word) {
			continue
		}
		seen[entry.word] = struct{}{}
		words = append(words, entry.word)
		if len(words) >= limit {
			break
		}
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("no words found for %s/%s", lang, listType)
	}
	return words, nil
}

// WriteAttribution writes attribution and license files based on the wheel.
func WriteAttribution(wheelPath, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	attrPath := filepath.Join(outDir, "ATTRIBUTION.txt")
	attrText := strings.Join([]string{
		"Word lists generated from the wordfreq dataset.",
		"Source: https://github.com/rspeer/wordfreq",
		"Data license: Creative Commons Attribution-ShareAlike 4.0 International (CC BY-SA 4.0).",
		"This word list is licensed CC BY-SA 4.0: https://creativecommons.org/licenses/by-sa/4.0/",
		"Changes were made: filtered to alphabetic words and truncated to the requested size.",
		"Includes data from Google Books Ngrams (acknowledgement requested by wordfreq): https://books.google.com/ngrams",
		"Includes data from the Leeds Internet Corpus: https://corpus.leeds.ac.uk/",
		"For other upstream sources, see the wordfreq project documentation.",
		"Please attribute wordfreq when redistributing derived word lists.",
		"",
	}, "\n")
	if err := os.WriteFile(attrPath, []byte(attrText), 0o644); err != nil {
		return fmt.Errorf("failed to write attribution: %w", err)
	}

	licenseText, err := readWheelLicense(wheelPath)
	if err != nil {
		return err
	}
	licensePath := filepath.Join(outDir, "LICENSE.txt")
	if err := os.WriteFile(licensePath, licenseText, 0o644); err != nil {
		return fmt.Errorf("failed to write license: %w", err)
	}
	dataLicensePath := filepath.Join(outDir, "DATA_LICENSE.txt")
	dataLicenseText := strings.Join([]string{
		"This word list is licensed under CC BY-SA 4.0.",
		"https://creativecommons.org/licenses/by-sa/4.0/",
		"",
	}, "\n")
	if err := os.WriteFile(dataLicensePath, []byte(dataLicenseText), 0o644); err != nil {
		return fmt.Errorf("failed to write data license: %w", err)
	}
	return nil
}

func httpRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

func pickWheelURL(urls []struct {
	URL          string `json:"url"`
	Filename     string `json:"filename"`
	Packagetype  string `json:"packagetype"`
	PythonTarget string `json:"python_version"`
}) (string, string) {
	for _, u := range urls {
		if u.Packagetype != "bdist_wheel" {
			continue
		}
		if strings.HasSuffix(u.Filename, "py3-none-any.whl") {
			return u.URL, u.Filename
		}
	}
	for _, u := range urls {
		if u.Packagetype == "bdist_wheel" {
			return u.URL, u.Filename
		}
	}
	return "", ""
}

func normalizeLang(lang string) string {
	return strings.ToLower(lang)
}

func langAliases(lang string) []string {
	return []string{lang}
}

func readWordEntries(wheelPath, lang, listType string) ([]wordEntry, error) {
	reader, err := zip.OpenReader(wheelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wheel: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	dataFile := selectDataFile(reader.File, lang, listType)
	if dataFile == nil {
		return nil, fmt.Errorf("no data file found for %s/%s", lang, listType)
	}

	rc, err := dataFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	decoded, err := decodeMsgpackStream(dataFile.Name, rc)
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("wordfreq data contained no entries")
	}
	return decoded, nil
}

func selectDataFile(files []*zip.File, lang, listType string) *zip.File {
	aliases := langAliases(lang)
	listType = strings.ToLower(listType)

	type candidate struct {
		file  *zip.File
		score int
	}
	candidates := make([]candidate, 0, len(files))
	listCandidates := make([]candidate, 0, len(files))

	for _, file := range files {
		name := file.Name
		if !strings.HasPrefix(name, "wordfreq/data/") {
			continue
		}
		lower := strings.ToLower(name)
		if !strings.Contains(lower, ".msgpack") {
			continue
		}
		langScore := 0
		for _, alias := range aliases {
			if hasToken(lower, alias) {
				langScore = 3
				break
			}
		}
		if langScore == 0 {
			continue
		}
		score := langScore
		listMatch := false
		listScore := 0
		if hasToken(lower, listType) {
			listMatch = true
			listScore = 2
		}
		if listMatch {
			score += listScore
		}
		if strings.HasSuffix(lower, ".msgpack") {
			score++
		}
		entry := candidate{file: file, score: score}
		candidates = append(candidates, entry)
		if listMatch {
			listCandidates = append(listCandidates, entry)
		}
	}

	if len(listCandidates) > 0 {
		candidates = listCandidates
	}

	if len(candidates) == 0 {
		return nil
	}
	if len(listCandidates) == 0 && len(candidates) > 1 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].file
}

func hasToken(name, token string) bool {
	if token == "" {
		return false
	}
	name = strings.ToLower(name)
	token = strings.ToLower(token)
	for i := 0; i+len(token) <= len(name); i++ {
		if name[i:i+len(token)] != token {
			continue
		}
		var before byte
		if i > 0 {
			before = name[i-1]
		}
		var after byte
		if i+len(token) < len(name) {
			after = name[i+len(token)]
		}
		if !isAlphaNum(before) && !isAlphaNum(after) {
			return true
		}
	}
	return false
}

// LanguageTypes maps language codes to available list types.
type LanguageTypes map[string]map[string]struct{}

// ListLanguageTypes returns available languages and list types in the wheel.
func ListLanguageTypes(wheelPath string) (LanguageTypes, error) {
	if wheelPath == "" {
		return nil, fmt.Errorf("wheel path is required")
	}
	reader, err := zip.OpenReader(wheelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wheel: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	langs := make(LanguageTypes)
	for _, file := range reader.File {
		lang, listType := parseLanguageAndType(file.Name)
		if lang == "" || listType == "" {
			continue
		}
		if _, ok := langs[lang]; !ok {
			langs[lang] = make(map[string]struct{})
		}
		langs[lang][listType] = struct{}{}
	}
	if len(langs) == 0 {
		return nil, fmt.Errorf("no languages found in wordfreq wheel")
	}
	return langs, nil
}

// LanguagesFromTypes returns sorted language codes from the map.
func LanguagesFromTypes(types LanguageTypes) []string {
	out := make([]string, 0, len(types))
	for lang := range types {
		out = append(out, lang)
	}
	sort.Strings(out)
	return out
}

func parseLanguageAndType(name string) (string, string) {
	name = strings.ToLower(name)
	if !strings.HasPrefix(name, "wordfreq/data/") {
		return "", ""
	}
	base := strings.TrimPrefix(name, "wordfreq/data/")
	base = trimWordfreqSuffixes(base)
	if base == "" {
		return "", ""
	}
	if strings.HasPrefix(base, "large_") {
		return strings.TrimPrefix(base, "large_"), "large"
	}
	if strings.HasPrefix(base, "small_") {
		return strings.TrimPrefix(base, "small_"), "small"
	}
	if strings.HasPrefix(base, "wordfreq-") {
		base = strings.TrimPrefix(base, "wordfreq-")
		if strings.HasSuffix(base, "-large") {
			return strings.TrimSuffix(base, "-large"), "large"
		}
		if strings.HasSuffix(base, "-small") {
			return strings.TrimSuffix(base, "-small"), "small"
		}
	}
	return "", ""
}

func trimWordfreqSuffixes(name string) string {
	switch {
	case strings.HasSuffix(name, ".msgpack.gz"):
		return strings.TrimSuffix(name, ".msgpack.gz")
	case strings.HasSuffix(name, ".msgpack"):
		return strings.TrimSuffix(name, ".msgpack")
	case strings.HasSuffix(name, ".gz"):
		return strings.TrimSuffix(name, ".gz")
	default:
		return name
	}
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

func decodeMsgpackStream(name string, r io.Reader) ([]wordEntry, error) {
	reader := r
	if strings.HasSuffix(name, ".msgpack.gz") || strings.HasSuffix(name, ".gz") {
		gzReader, err := gzipReader(r)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = gzReader.Close()
		}()
		reader = gzReader
	}

	payload, err := decodeMsgpack(reader)
	if err != nil {
		return nil, err
	}
	entries, err := entriesFromData(payload)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

type gzipReadCloser struct {
	reader io.Reader
	close  func() error
}

func (g gzipReadCloser) Read(p []byte) (int, error) {
	return g.reader.Read(p)
}

func (g gzipReadCloser) Close() error {
	if g.close != nil {
		return g.close()
	}
	return nil
}

func gzipReader(r io.Reader) (gzipReadCloser, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return gzipReadCloser{}, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	return gzipReadCloser{reader: gr, close: gr.Close}, nil
}

func entriesFromData(data interface{}) ([]wordEntry, error) {
	switch v := data.(type) {
	case []interface{}:
		return entriesFromSlice(v)
	case map[interface{}]interface{}:
		return entriesFromMap(v)
	case map[string]interface{}:
		return entriesFromStringMap(v)
	default:
		return nil, fmt.Errorf("unsupported msgpack root type %T", data)
	}
}

func entriesFromSlice(items []interface{}) ([]wordEntry, error) {
	var entries []wordEntry
	for i, item := range items {
		switch typed := item.(type) {
		case map[interface{}]interface{}:
			if mapEntries, err := entriesFromMap(typed); err == nil {
				entries = append(entries, mapEntries...)
				continue
			}
		case map[string]interface{}:
			if mapEntries, err := entriesFromStringMap(typed); err == nil {
				entries = append(entries, mapEntries...)
				continue
			}
		}
		if binEntries, ok := entriesFromBin(item); ok {
			entries = append(entries, binEntries...)
			continue
		}
		if words, ok := toStringSlice(item); ok {
			score := float64(len(items) - i)
			for _, word := range words {
				entries = append(entries, wordEntry{word: word, score: score})
			}
			continue
		}
		return nil, fmt.Errorf("unsupported msgpack slice entry %T", item)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no word entries parsed from slice")
	}
	return entries, nil
}

func entriesFromBin(item interface{}) ([]wordEntry, bool) {
	switch v := item.(type) {
	case []interface{}:
		if len(v) != 2 {
			return nil, false
		}
		score, ok := toFloat64(v[0])
		if !ok {
			return nil, false
		}
		words, ok := toStringSlice(v[1])
		if !ok {
			return nil, false
		}
		entries := make([]wordEntry, 0, len(words))
		for _, word := range words {
			entries = append(entries, wordEntry{word: word, score: score})
		}
		return entries, true
	case map[string]interface{}:
		score, ok := toFloat64(v["zipf"])
		if !ok {
			score, ok = toFloat64(v["score"])
		}
		words, okWords := toStringSlice(v["words"])
		if ok && okWords {
			entries := make([]wordEntry, 0, len(words))
			for _, word := range words {
				entries = append(entries, wordEntry{word: word, score: score})
			}
			return entries, true
		}
	case map[interface{}]interface{}:
		var score float64
		scoreSet := false
		var words []string
		for k, val := range v {
			if key, ok := k.(string); ok {
				if key == "zipf" || key == "score" {
					if s, ok := toFloat64(val); ok {
						score = s
						scoreSet = true
					}
				}
				if key == "words" {
					if ws, ok := toStringSlice(val); ok {
						words = ws
					}
				}
			}
		}
		if scoreSet && len(words) > 0 {
			entries := make([]wordEntry, 0, len(words))
			for _, word := range words {
				entries = append(entries, wordEntry{word: word, score: score})
			}
			return entries, true
		}
	}
	return nil, false
}

func entriesFromMap(items map[interface{}]interface{}) ([]wordEntry, error) {
	entries := make([]wordEntry, 0, len(items))
	for key, value := range items {
		if words, ok := toStringSlice(value); ok {
			score, okScore := toFloat64(key)
			if !okScore {
				continue
			}
			for _, word := range words {
				entries = append(entries, wordEntry{word: word, score: score})
			}
			continue
		}
		word, okWord := toString(key)
		score, okScore := toFloat64(value)
		if okWord && okScore {
			entries = append(entries, wordEntry{word: word, score: score})
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no word entries parsed from map")
	}
	return entries, nil
}

func entriesFromStringMap(items map[string]interface{}) ([]wordEntry, error) {
	entries := make([]wordEntry, 0, len(items))
	for key, value := range items {
		if words, ok := toStringSlice(value); ok {
			score, okScore := toFloat64(key)
			if !okScore {
				continue
			}
			for _, word := range words {
				entries = append(entries, wordEntry{word: word, score: score})
			}
			continue
		}
		score, okScore := toFloat64(value)
		if okScore {
			entries = append(entries, wordEntry{word: key, score: score})
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no word entries parsed from map")
	}
	return entries, nil
}

func toFloat64(v interface{}) (float64, bool) {
	switch num := v.(type) {
	case float64:
		return num, true
	case float32:
		return float64(num), true
	case int:
		return float64(num), true
	case int8:
		return float64(num), true
	case int16:
		return float64(num), true
	case int32:
		return float64(num), true
	case int64:
		return float64(num), true
	case uint:
		return float64(num), true
	case uint8:
		return float64(num), true
	case uint16:
		return float64(num), true
	case uint32:
		return float64(num), true
	case uint64:
		return float64(num), true
	case string:
		if num == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func toString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		if utf8.Valid(val) {
			return string(val), true
		}
		return "", false
	default:
		return "", false
	}
}

func toStringSlice(v interface{}) ([]string, bool) {
	switch val := v.(type) {
	case []string:
		return val, true
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			str, ok := toString(item)
			if !ok {
				return nil, false
			}
			out = append(out, str)
		}
		return out, true
	default:
		return nil, false
	}
}

func isAlpha(word string) bool {
	for _, r := range word {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return word != ""
}

func readWheelLicense(wheelPath string) ([]byte, error) {
	reader, err := zip.OpenReader(wheelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wheel for license: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	for _, file := range reader.File {
		name := strings.ToLower(file.Name)
		if !strings.Contains(name, "license") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open license: %w", err)
		}
		defer func() {
			_ = rc.Close()
		}()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("failed to read license: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("license file not found in wheel")
}
