GO ?= go
BIN_DIR := bin
APP := spike-server

.PHONY: build run test lint tidy clean

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(APP) ./cmd/$(APP)

run:
	$(GO) run ./cmd/$(APP)

test:
	$(GO) test ./... -race -count=1

lint:
	golangci-lint run

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)


