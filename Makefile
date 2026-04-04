.PHONY: build run clean

BINARY := websz
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/warriorguo/websz/internal/updater.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/websz

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
