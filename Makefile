BINARY_NAME=seintech
VERSION=1.0.0
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

.PHONY: all build build-linux clean test dev

all: build

build:
	@echo "Building Seintech Panel binary..."
	go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY_NAME) ./cmd/seintech

build-linux:
	@echo "Cross-compiling for Linux AMD64 (Static Single Binary)..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/seintech

build-linux-arm64:
	@echo "Cross-compiling for Linux ARM64 (Static Single Binary)..."
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/seintech

dev:
	@echo "Starting Seintech Panel in Development Mode..."
	go run ./cmd/seintech -dev -port 8443

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning up build artifacts..."
	rm -rf bin/
