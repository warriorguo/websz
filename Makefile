.PHONY: build run clean

BINARY := websz

build:
	go build -o $(BINARY) ./cmd/websz

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
