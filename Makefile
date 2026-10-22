.PHONY: all build test clean run demo lint

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

demo: build
	./$(BINARY) demo

clean:
	rm -rf bin/ trace_*.json
