# tuipe

A fast, minimal TUI typing trainer with weak-character focus and detailed stats.

## Features
- Full-screen centered typing UI with cursor indicator
- Caps and punctuation controls
- Weak-character focus mode (`--focus-weak`)
- SQLite-backed stats and stats TUI
- Wordlist generator powered by wordfreq (no Python required)

## Install
Go install (no clone):
```bash
go install github.com/verte-zerg/tuipe/cmd/tuipe@latest
```

Local build:
```bash
go build -o tuipe ./cmd/tuipe
```

Optional:
```bash
go install ./cmd/tuipe
```

## Usage
Quick start:
```bash
tuipe wordlist --lang en
tuipe
```

Commands:
- `tuipe` — start practice
- `tuipe wordlist` — generate wordlists
- `tuipe stats` — stats TUI
- `tuipe langs` — list downloaded wordlists
- `tuipe config` — create/open config

Practice:
```bash
tuipe
```

Common flags:
```bash
tuipe --lang en --words 25 --caps 0.2 --punct 0.3 --punct-set ".,?!"
tuipe --focus-weak --weak-top 8 --weak-window 20 --weak-factor 2.0
```

Practice flags (defaults):
- `--lang en` — language code
- `--words 25` — number of words per session
- `--caps 0.0` — probability of capitalized first letter
- `--punct 0.0` — punctuation probability per word
- `--punct-set ".,?!"` — punctuation characters
- `--focus-weak` — bias toward weak characters
- `--weak-top 8` — number of weak characters to focus on
- `--weak-factor 2.0` — weight factor for weak characters
- `--weak-window 20` — number of recent sessions to compute weak chars

Stats:
```bash
tuipe stats
```

Stats UI:
- Full-screen TUI with sections: Overview, Char Table, Char Curves.
- Navigation: `left/right` to change sections, `up/down`/`pgup`/`pgdn` to scroll, `q` to quit.
- Settings: press `/` to edit settings (lang/since/last/curve window), `enter` to apply, `esc` to cancel.
- Char curves: press `enter` in Char Curves to edit the character set (defaults to top 5 by frequency).
- Char input: type characters (no commas). Spaces are ignored.
- Curves are colorized (disable with `NO_COLOR=1`).

Generate wordlists:
```bash
tuipe wordlist --force
tuipe wordlist --size 10000 --force
tuipe wordlist --lang en --force
tuipe wordlist --lang ru --force
tuipe wordlist --lang all
```
Generated wordlists include `ATTRIBUTION.txt`, `LICENSE.txt` (code), and `DATA_LICENSE.txt` (data).
Use `tuipe wordlist --lang all` to generate every available language.
English wordlists are filtered to ASCII `[a-z]` words only. To add another language filter,
extend `internal/wordlist/filter.go`.

List downloaded wordlists:
```bash
tuipe langs
```

Create or edit config:
```bash
tuipe config
```

## Configuration
Config is read from `$XDG_CONFIG_HOME/tuipe/config.toml`. CLI flags override config values.

Example:
```toml
[practice]
lang = "en"
words = 25
caps = 0.2
punct = 0.3
punct-set = ".,?!"
focus-weak = true
weak-top = 8
weak-factor = 2.0
weak-window = 20
```

Config reference (`[practice]`):
- `lang` (default `en`) — language code used for practice
- `words` (default `25`) — number of words per session
- `caps` (default `0.0`) — probability of capitalized first letter
- `punct` (default `0.0`) — punctuation probability per word
- `punct-set` (default `.,?!`) — punctuation characters
- `focus-weak` (default `false`) — bias toward weak characters
- `weak-top` (default `8`) — number of weak characters to focus on
- `weak-factor` (default `2.0`) — weight factor for weak characters
- `weak-window` (default `20`) — recent sessions used for weak-char stats

Status bar:
- Shows progress, last-session WPM/accuracy, and all-time WPM/accuracy (current language).

## Data Paths
- Database: `$XDG_DATA_HOME/tuipe/tuipe.db`
- Wordlists: `$XDG_CONFIG_HOME/tuipe/wordlists` (practice always reads from here)

## Troubleshooting
- No wordlists found: run `tuipe wordlist --lang en` or list available ones with `tuipe langs`.
- Wordlist download requires network access to `https://pypi.org`.

## Development
Lint:
```bash
make lint
```

Tests:
```bash
go test ./...
```

## Attribution
Generated wordlists are derived from the wordfreq dataset. The `wordlist` command writes
`ATTRIBUTION.txt`, `LICENSE.txt` (code), and `DATA_LICENSE.txt` (data) alongside the output.
