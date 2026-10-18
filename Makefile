GO ?= go
BIN ?= meno

.PHONY: build install run test race vet fmt size check term bench clean

build:
	$(GO) build -o $(BIN) ./cmd/meno

install:
	$(GO) install ./cmd/meno

run: build
	./$(BIN)

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
