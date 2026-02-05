GO := go
GOLANGCI_LINT := golangci-lint

deps:
	$(GO) mod download

build: deps
	mkdir -p bin
	$(GO) build -o bin/tuipe ./cmd/tuipe

lint: deps
	$(GOLANGCI_LINT) run ./...

test: deps
	$(GO) test ./...
