.PHONY: build build-linux build-all run clean test coverage deps install dev fmt lint

BINARY_NAME=pgpipe
BUILD_DIR=.
DIST_DIR=./dist
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/pgpipe

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux ./cmd/pgpipe

build-all:
	mkdir -p $(DIST_DIR)
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64   ./cmd/pgpipe
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64   ./cmd/pgpipe
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64  ./cmd/pgpipe
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64  ./cmd/pgpipe

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	rm -rf $(DIST_DIR)
	rm -rf .pgpipe/

test:
	go test -v ./...

coverage:
	go test -cover ./...

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
