.PHONY: all lint fmt vet test build tidy clean

all: lint test build

# Format check. Fails (non-empty list) if any file is not gofmt clean.
fmt:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "gofmt: files need formatting:"; \
		echo "$$out"; \
		exit 1; \
	fi

vet:
	go vet ./...

# golangci-lint encompasses vet, but we keep the explicit vet target above so
# `make lint` still runs something useful before golangci-lint is installed.
lint: fmt vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed; skipping. install: brew install golangci-lint"; \
		exit 1; \
	fi

test:
	go test ./...

# Compile-check every package without producing artifacts. Binaries are
# emitted only by the explicit `bin/ocp` target below.
build:
	go build -o /dev/null ./...

bin/ocp: $(shell find cmd internal -name '*.go' 2>/dev/null) go.mod
	@mkdir -p bin
	go build -o bin/ocp ./cmd/ocp

tidy:
	go mod tidy

clean:
	rm -rf bin dist
	go clean ./...
