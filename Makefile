BINARY_NAME := api-load-tester
BUILD_DIR   := bin
CMD_DIR     := ./cmd
VERSION     ?= dev
LDFLAGS     := -ldflags "-X main.version=$(VERSION)"

.PHONY: all build test test-race test-cover lint fmt vet run clean install help

all: build

## build: Compile the binary into ./bin
build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

## install: Install the binary into $GOPATH/bin
install:
	go install $(LDFLAGS) $(CMD_DIR)

## test: Run the full test suite
test:
	go test ./...

## test-race: Run the full test suite with the race detector
test-race:
	go test -race ./...

## test-cover: Run tests and print a coverage summary
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## fmt: Format all Go source files
fmt:
	go fmt ./...

## vet: Run go vet across the module
vet:
	go vet ./...

## lint: Run fmt + vet as a lightweight lint pass
lint: fmt vet

## run: Build and run against a target, e.g. make run ARGS="-url https://example.com -n 100 -c 10"
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

## clean: Remove build artifacts and coverage output
clean:
	rm -rf $(BUILD_DIR) coverage.out

## help: Show this help message
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
