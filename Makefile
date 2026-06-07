.PHONY: build run-producer run-processor run-subscriber test lint fmt tidy

BIN_DIR := bin

build:
	go build -o $(BIN_DIR)/producer   ./cmd/producer
	go build -o $(BIN_DIR)/processor  ./cmd/processor
	go build -o $(BIN_DIR)/subscriber ./cmd/subscriber

run-producer:
	go run ./cmd/producer

run-processor:
	go run ./cmd/processor

run-subscriber:
	go run ./cmd/subscriber

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy
