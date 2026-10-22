.PHONY: all build test clean run run-demo demo stream-demo lint

BINARY=bin/otel-tui

all: build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY) ./cmd/otel-tui

test:
	go test -v -race ./...

lint:
	go vet ./...

run: build
	./$(BINARY)

# Start TUI with built-in continuous background trace generator
run-demo: build
	./$(BINARY) --demo

# Send sample batches
demo: build
	./$(BINARY) demo --count 4

# Continuously stream synthetic events over HTTP OTLP
stream-demo: build
	./$(BINARY) demo --stream --interval 1s

clean:
	rm -rf bin/ trace_*.json
