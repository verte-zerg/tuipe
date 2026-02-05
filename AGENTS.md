# AGENTS

Developer guide for tuipe.

## Repo Layout
- `cmd/tuipe`: CLI entrypoint
- `internal/tui`: Bubble Tea UI (rendering, input handling)
- `internal/stats`: session stats, tables, and reports
- `internal/store`: SQLite schema and queries
- `internal/generator`: text/word generation logic
- `internal/wordfreq`: wordfreq wheel download + wordlist extraction
- `internal/config`: XDG paths

## Core Commands
Build:
```bash
go build -o tuipe ./cmd/tuipe
```

Install (no clone):
```bash
go install github.com/verte-zerg/tuipe/cmd/tuipe@latest
```

Run:
```bash
go run cmd/tuipe/main.go
```

Config:
```bash
tuipe config
```

Lint:
```bash
make lint
```

Tests:
```bash
go test ./...
```

## Data Paths (XDG)
- DB: `$XDG_DATA_HOME/tuipe/tuipe.db`
- Wordlists: `$XDG_CONFIG_HOME/tuipe/wordlists` (practice always reads from here)
- Wordfreq cache: `$XDG_DATA_HOME/tuipe/wordfreq`
- Config: `$XDG_CONFIG_HOME/tuipe/config.toml` (practice only)

## Wordlist Generation
- Uses `tuipe wordlist` to download the latest wordfreq wheel from PyPI.
- Requires network access to `https://pypi.org`.
- No Python is used; msgpack is decoded in Go.
- Generated outputs include `ATTRIBUTION.txt`, `LICENSE.txt` (code), and `DATA_LICENSE.txt` (data license).
- Wordlists support `--lang all` and use short language codes (e.g., `en`, `ru`).
- Use `tuipe langs` to list downloaded wordlists.
- English wordlists are filtered to ASCII `[a-z]` words only; add filters in `internal/wordlist/filter.go`.

## Conventions
- Go formatting is required (`gofmt`).
- Prefer explicit error handling and early returns.
- Avoid non-ASCII unless already present in the file.

## Notes
- The TUI uses Bubble Tea in full-screen mode and supports space input.
- Weak-character focus relies on session stats stored in SQLite.
