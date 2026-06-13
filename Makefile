.PHONY: build up down run-producer run-processor run-subscriber test lint fmt tidy

BIN_DIR := bin

build:
	go build -o $(BIN_DIR)/producer   ./cmd/producer
	go build -o $(BIN_DIR)/processor  ./cmd/processor
	go build -o $(BIN_DIR)/subscriber ./cmd/subscriber

up:
	docker compose -f deploy/compose/docker-compose.yaml up -d

down:
	docker compose -f deploy/compose/docker-compose.yaml down

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
