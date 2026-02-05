// Package main provides the CLI entrypoint for tuipe.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/verte-zerg/tuipe/internal/config"
	"github.com/verte-zerg/tuipe/internal/generator"
	"github.com/verte-zerg/tuipe/internal/model"
	"github.com/verte-zerg/tuipe/internal/stats"
	"github.com/verte-zerg/tuipe/internal/statsui"
	"github.com/verte-zerg/tuipe/internal/store"
	"github.com/verte-zerg/tuipe/internal/tui"
	"github.com/verte-zerg/tuipe/internal/wordfreq"
	"github.com/verte-zerg/tuipe/internal/wordlist"
)

const (
	defaultLang        = "en"
	defaultWords       = 25
	defaultCaps        = 0.5
	defaultPunct       = 0.5
	defaultWeakTop     = 8
	defaultWeakFactor  = 2.0
	defaultWeakWindow  = 20
	defaultCurveWindow = 20
	defaultWordlistSz  = 10000
)

const defaultPunctSet = ".,!?;:\"'{}()[]-=/<>`"

var (
	practiceLang       string
	practiceWords      int
	practiceCaps       float64
	practicePunct      float64
	practicePunctSet   string
	practiceFocusWeak  bool
	practiceWeakTop    int
	practiceWeakFactor float64
	practiceWeakWindow int

	statsLang        string
	statsSince       string
	statsLast        int
	statsCurveWindow int
	statsChars       string

	wordlistLang  string
	wordlistSize  int
	wordlistForce bool
)

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "tuipe",
		Short:         "TUI typing trainer",
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE:          runPracticeCmd,
	}

	rootCmd.Flags().StringVar(&practiceLang, "lang", defaultLang, "language code (default: en)")
	rootCmd.Flags().IntVar(&practiceWords, "words", defaultWords, "words per text")
	rootCmd.Flags().Float64Var(&practiceCaps, "caps", defaultCaps, "probability of capitalized first letter (0-1)")
	rootCmd.Flags().Float64Var(&practicePunct, "punct", defaultPunct, "punctuation probability per word (0-1)")
	rootCmd.Flags().StringVar(&practicePunctSet, "punct-set", defaultPunctSet, "punctuation set")
	rootCmd.Flags().BoolVar(&practiceFocusWeak, "focus-weak", false, "bias practice toward weak characters")
	rootCmd.Flags().IntVar(&practiceWeakTop, "weak-top", defaultWeakTop, "number of weak characters to focus on")
	rootCmd.Flags().Float64Var(&practiceWeakFactor, "weak-factor", defaultWeakFactor, "weight factor for weak characters")
	rootCmd.Flags().IntVar(&practiceWeakWindow, "weak-window", defaultWeakWindow, "number of recent sessions to compute weak chars")

	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newLangsCmd())
	rootCmd.AddCommand(newStatsCmd())
	rootCmd.AddCommand(newWordlistCmd())

	return rootCmd
}

func runPracticeCmd(cmd *cobra.Command, _ []string) error {
	fileCfg, err := config.LoadConfig(config.DefaultConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	applyStringConfig(cmd, "lang", &practiceLang, fileCfg.Practice.Lang)
	applyIntConfig(cmd, "words", &practiceWords, fileCfg.Practice.Words)
	applyFloatConfig(cmd, "caps", &practiceCaps, fileCfg.Practice.CapsPct)
	applyFloatConfig(cmd, "punct", &practicePunct, fileCfg.Practice.PunctPct)
	applyStringConfig(cmd, "punct-set", &practicePunctSet, fileCfg.Practice.PunctSet)
	applyBoolConfig(cmd, "focus-weak", &practiceFocusWeak, fileCfg.Practice.FocusWeak)
	applyIntConfig(cmd, "weak-top", &practiceWeakTop, fileCfg.Practice.WeakTop)
	applyFloatConfig(cmd, "weak-factor", &practiceWeakFactor, fileCfg.Practice.WeakFactor)
	applyIntConfig(cmd, "weak-window", &practiceWeakWindow, fileCfg.Practice.WeakWindow)

	cfg := model.Config{
		Lang:       practiceLang,
		Words:      practiceWords,
		CapsPct:    practiceCaps,
		PunctPct:   practicePunct,
		PunctSet:   practicePunctSet,
		FocusWeak:  practiceFocusWeak,
		WeakTop:    practiceWeakTop,
		WeakFactor: practiceWeakFactor,
		WeakWindow: practiceWeakWindow,
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}

	wordPath := resolveWordListPath(cfg)
	wordsList, err := wordlist.LoadWords(wordPath)
	if err != nil {
		return wordListLoadError(cfg.Lang, wordPath, err)
	}

	storePath := config.DefaultDBPath()
	st, err := store.Open(storePath)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	defer func() {
		if cerr := st.Close(); cerr != nil {
			logErrf("failed to close db: %v\n", cerr)
		}
	}()

	punctRunes := []rune(cfg.PunctSet)

	weakSet := map[rune]struct{}{}
	weakNoticePrinted := false
	if cfg.FocusWeak {
		aggs, err := st.GetWeakChars(context.Background(), cfg.WeakWindow, cfg.Lang)
		if err != nil {
			logErrf("failed to load weak chars: %v\n", err)
		} else {
			weakSet = stats.SelectWeakChars(aggs, cfg.WeakTop)
			if len(weakSet) == 0 {
				logErrln("no stats available for weak-char focus yet; using normal generator")
				weakNoticePrinted = true
			}
		}
	}

	gen := generator.New()
	model := tui.NewModel(cfg, st, gen, wordsList, wordPath, punctRunes, weakSet, weakNoticePrinted)
	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	return nil
}

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Create/open config file",
		Args:  cobra.NoArgs,
		RunE:  runConfigCmd,
	}
}

func runConfigCmd(_ *cobra.Command, _ []string) error {
	path := config.DefaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat config: %w", err)
		}
		if err := os.WriteFile(path, []byte(defaultConfigTemplate()), 0o644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("editor command is empty")
	}
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}
	return nil
}

func newLangsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "langs",
		Short: "List downloaded wordlist languages",
		Args:  cobra.NoArgs,
		RunE:  runLangsCmd,
	}
}

func runLangsCmd(cmd *cobra.Command, _ []string) error {
	wordlistDir := config.DefaultWordListDir()
	entries, err := os.ReadDir(wordlistDir)
	if err != nil {
		if os.IsNotExist(err) {
			logErrf("No wordlists found. Download with: tuipe wordlist --lang <code>\n")
			return fmt.Errorf("wordlist directory does not exist")
		}
		return fmt.Errorf("failed to read wordlist directory: %w", err)
	}
	langs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".txt") {
			continue
		}
		if name == "ATTRIBUTION.txt" || name == "LICENSE.txt" || name == "DATA_LICENSE.txt" {
			continue
		}
		langs = append(langs, strings.TrimSuffix(name, ".txt"))
	}
	if len(langs) == 0 {
		logErrf("No wordlists found. Download with: tuipe wordlist --lang <code>\n")
		return fmt.Errorf("no wordlists found")
	}
	sort.Strings(langs)
	for _, lang := range langs {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), lang); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}
	return nil
}

func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show stats",
		RunE:  runStatsCmd,
	}
	cmd.Flags().StringVar(&statsLang, "lang", "", "language filter")
	cmd.Flags().StringVar(&statsSince, "since", "", "start date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&statsLast, "last", 0, "limit to last N sessions")
	cmd.Flags().IntVar(&statsCurveWindow, "curve-window", defaultCurveWindow, "moving average window")
	cmd.Flags().StringVar(&statsChars, "char", "", "characters for per-char curves")
	return cmd
}

func runStatsCmd(_ *cobra.Command, _ []string) error {
	var sinceTime *time.Time
	if statsSince != "" {
		parsed, err := time.ParseInLocation("2006-01-02", statsSince, time.Local)
		if err != nil {
			return fmt.Errorf("invalid --since value: %w", err)
		}
		sinceTime = &parsed
	}

	cfg := model.StatsConfig{
		Lang:        statsLang,
		Since:       sinceTime,
		Last:        statsLast,
		CurveWindow: statsCurveWindow,
		Chars:       statsChars,
	}

	storePath := config.DefaultDBPath()
	st, err := store.Open(storePath)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	defer func() {
		if cerr := st.Close(); cerr != nil {
			logErrf("failed to close db: %v\n", cerr)
		}
	}()

	model := statsui.NewModel(st, cfg)
	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run stats TUI: %w", err)
	}
	return nil
}

func newWordlistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wordlist",
		Short: "Generate wordlists",
		RunE:  runWordlistCmd,
	}
	cmd.Flags().StringVar(&wordlistLang, "lang", "", "language code or 'all' (default: en)")
	cmd.Flags().IntVar(&wordlistSize, "size", defaultWordlistSz, "number of words")
	cmd.Flags().BoolVar(&wordlistForce, "force", false, "overwrite existing files")
	return cmd
}

func runWordlistCmd(_ *cobra.Command, _ []string) error {
	if _, err := config.LoadConfig(config.DefaultConfigPath()); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	listTypeNormalized := "large"
	wordlistOutDir := config.DefaultWordListDir()
	if wordlistSize <= 0 {
		return fmt.Errorf("--size must be greater than 0")
	}

	cacheDir := config.DefaultWordfreqCacheDir()
	logErrln("Fetching wordfreq metadata...")
	wheel, err := wordfreq.DownloadLatestWheel(context.Background(), cacheDir)
	if err != nil {
		return fmt.Errorf("failed to download wordfreq wheel: %w", err)
	}
	if wheel.Cached {
		logErrf("Using cached wheel %s\n", wheel.Filename)
	} else {
		logErrf("Downloaded wheel %s\n", wheel.Filename)
	}
	langTypes, err := wordfreq.ListLanguageTypes(wheel.Path)
	if err != nil {
		return fmt.Errorf("failed to list languages: %w", err)
	}
	availableLangs := wordfreq.LanguagesFromTypes(langTypes)
	langs, allRequested, err := resolveWordlistLangs(wordlistLang, availableLangs)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(wordlistOutDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, langCode := range langs {
		outPath := filepath.Join(wordlistOutDir, langCode+".txt")
		if !wordlistForce {
			if _, err := os.Stat(outPath); err == nil {
				return fmt.Errorf("word list already exists: %s (use --force to overwrite)", outPath)
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("failed to stat word list: %w", err)
			}
		}

		logErrf("Extracting %s word list...\n", langCode)
		selectedType, ok := selectWordlistType(langTypes[langCode], listTypeNormalized)
		if !ok {
			if allRequested {
				logErrf("Skipping %s (no %s word list)\n", langCode, listTypeNormalized)
				continue
			}
			return fmt.Errorf("no %s word list available for %s", listTypeNormalized, langCode)
		}
		if selectedType != listTypeNormalized {
			logErrf("Using %s for %s (no %s word list)\n", selectedType, langCode, listTypeNormalized)
		}
		words, err := wordfreq.ExtractWordlist(wheel.Path, langCode, selectedType, wordlistSize)
		if err != nil {
			if allRequested {
				logErrf("Skipping %s (no word list): %v\n", langCode, err)
				continue
			}
			return fmt.Errorf("failed to extract %s word list: %w", langCode, err)
		}
		if err := writeWordList(outPath, words); err != nil {
			return fmt.Errorf("failed to write %s: %w", outPath, err)
		}
		logErrf("Wrote %s\n", outPath)
	}

	if err := wordfreq.WriteAttribution(wheel.Path, wordlistOutDir); err != nil {
		return fmt.Errorf("failed to write attribution: %w", err)
	}
	logErrln("Wrote ATTRIBUTION.txt, LICENSE.txt, and DATA_LICENSE.txt")
	return nil
}

func resolveWordlistLangs(lang string, available []string) ([]string, bool, error) {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" {
		return []string{"en"}, false, nil
	}
	if lang == "all" {
		return append([]string(nil), available...), true, nil
	}
	parts := strings.Split(lang, ",")
	requested := make([]string, 0, len(parts))
	availableSet := make(map[string]struct{}, len(available))
	for _, a := range available {
		availableSet[a] = struct{}{}
	}
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		if _, ok := availableSet[part]; !ok {
			return nil, false, fmt.Errorf("unknown language %q (available: %s)", part, strings.Join(available, ", "))
		}
		requested = append(requested, part)
	}
	if len(requested) == 0 {
		return nil, false, fmt.Errorf("--lang must not be empty")
	}
	return requested, false, nil
}

func selectWordlistType(available map[string]struct{}, desired string) (string, bool) {
	if len(available) == 0 {
		return "", false
	}
	switch desired {
	case "large":
		if _, ok := available["large"]; ok {
			return "large", true
		}
		if _, ok := available["small"]; ok {
			return "small", true
		}
	case "small":
		if _, ok := available["small"]; ok {
			return "small", true
		}
	}
	return "", false
}

func writeWordList(path string, words []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create word list dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(path), "wordlist-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp word list: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	writer := bufio.NewWriter(tmpFile)
	for _, word := range words {
		if _, err := fmt.Fprintln(writer, word); err != nil {
			return fmt.Errorf("failed to write word list: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush word list: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close word list: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to write word list: %w", err)
	}
	return nil
}

func applyStringConfig(cmd *cobra.Command, name string, target, value *string) {
	if value == nil {
		return
	}
	if cmd.Flags().Changed(name) {
		return
	}
	*target = *value
}

func applyIntConfig(cmd *cobra.Command, name string, target, value *int) {
	if value == nil {
		return
	}
	if cmd.Flags().Changed(name) {
		return
	}
	*target = *value
}

func applyFloatConfig(cmd *cobra.Command, name string, target, value *float64) {
	if value == nil {
		return
	}
	if cmd.Flags().Changed(name) {
		return
	}
	*target = *value
}

func applyBoolConfig(cmd *cobra.Command, name string, target, value *bool) {
	if value == nil {
		return
	}
	if cmd.Flags().Changed(name) {
		return
	}
	*target = *value
}

func defaultConfigTemplate() string {
	return fmt.Sprintf(`# tuipe configuration
# Uncomment a value to enable it. CLI flags override config values.

[practice]
# lang = "en"             # Language code (default %q)
# words = %d              # Words per text
# caps = %.2f             # Probability of capitalized first letter (0-1)
# punct = %.2f            # Punctuation probability per word (0-1)
# punct-set = %q          # Punctuation set
# focus-weak = false      # Bias practice toward weak characters
# weak-top = %d           # Number of weak characters to focus on
# weak-factor = %.1f      # Weight factor for weak characters
# weak-window = %d        # Number of recent sessions to compute weak chars
`,
		defaultLang,
		defaultWords,
		defaultCaps,
		defaultPunct,
		defaultPunctSet,
		defaultWeakTop,
		defaultWeakFactor,
		defaultWeakWindow,
	)
}

func validateConfig(cfg model.Config) error {
	if cfg.Words <= 0 {
		return fmt.Errorf("--words must be > 0")
	}
	if cfg.CapsPct < 0 || cfg.CapsPct > 1 {
		return fmt.Errorf("--caps must be between 0 and 1")
	}
	if cfg.PunctPct < 0 || cfg.PunctPct > 1 {
		return fmt.Errorf("--punct must be between 0 and 1")
	}
	if cfg.PunctSet == "" {
		return fmt.Errorf("--punct-set must not be empty")
	}
	if cfg.WeakTop < 0 {
		return fmt.Errorf("--weak-top must be >= 0")
	}
	if cfg.WeakFactor < 0 {
		return fmt.Errorf("--weak-factor must be >= 0")
	}
	if cfg.WeakWindow < 0 {
		return fmt.Errorf("--weak-window must be >= 0")
	}
	return nil
}

func resolveWordListPath(cfg model.Config) string {
	return config.DefaultWordListPath(cfg.Lang)
}

func wordListLoadError(lang, path string, err error) error {
	lines := []string{
		fmt.Sprintf("failed to load word list: %v", err),
		fmt.Sprintf("expected word list at: %s", path),
		fmt.Sprintf("language %q not found", lang),
		"Run: tuipe langs",
		fmt.Sprintf("Download: tuipe wordlist --lang %s", lang),
		"Download all: tuipe wordlist --lang all",
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

func logErrf(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, format, args...); err != nil {
		// Best-effort logging to stderr.
		_ = err
	}
}

func logErrln(args ...any) {
	if _, err := fmt.Fprintln(os.Stderr, args...); err != nil {
		// Best-effort logging to stderr.
		_ = err
	}
}
