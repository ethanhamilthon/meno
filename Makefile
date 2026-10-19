GO ?= go
BIN ?= meno
DEV_HOME ?= $(HOME)/.meno-dev

.PHONY: version build install run clean-home test race vet fmt size check term bench clean

build:
	$(GO) build -o $(BIN) ./cmd/meno

# Prints the current version (edit internal/version/VERSION to change it).
version:
	@cat internal/version/VERSION

install:
	$(GO) install ./cmd/meno

# Runs the dev build with its own MENO_HOME, so it never touches the data of an
# installed meno. Override with: make run DEV_HOME=/some/dir
run: build
	MENO_HOME=$(DEV_HOME) ./$(BIN)

# Deletes the dev MENO_HOME (settings, providers, sessions). Refuses unsafe paths.
clean-home:
	@case "$(DEV_HOME)" in ""|"/"|"$(HOME)"|"$(HOME)/") echo "refusing to remove '$(DEV_HOME)'"; exit 1;; esac
	rm -rf "$(DEV_HOME)"

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	@out="$$(gofmt -l cmd internal)"; if [ -n "$$out" ]; then echo "needs gofmt:"; echo "$$out"; exit 1; fi

# Hand-written Go files stay within the limit in AGENTS.md.
size:
	@scripts/check-size.sh 300

check: fmt size vet race

# End-to-end tests: the real binary in a pseudo-terminal against a fake provider.
term:
	$(GO) test -tags termtest -count=1 ./internal/term/...

# Deterministic benchmarks; compare runs with benchstat.
bench:
	$(GO) test -run '^$' -bench . -benchmem ./internal/ui ./internal/agent ./internal/jsvm

clean:
	rm -f $(BIN)
