.PHONY: build run clean test deps

BINARY_NAME=pgpipe
BUILD_DIR=.

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/pgpipe

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	rm -rf .pgpipe/

test:
	go test -v ./...

deps:
	go mod tidy
	go mod download

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# Development helpers
dev:
	go run ./cmd/pgpipe

fmt:
	go fmt ./...

lint:
	golangci-lint run
