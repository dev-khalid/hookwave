.PHONY: build up down run-producer run-processor run-subscriber latest-message ack-latest-message test lint fmt tidy

BIN_DIR := bin

build:
	go build -o $(BIN_DIR)/producer   ./cmd/producer
	go build -o $(BIN_DIR)/processor  ./cmd/processor
	go build -o $(BIN_DIR)/subscriber ./cmd/subscriber

up:
	docker-compose -f deploy/compose/docker-compose.yaml up -d

down:
	docker-compose -f deploy/compose/docker-compose.yaml down

run-producer:
	@set -a; [ ! -f .env ] || . ./.env; set +a; go run ./cmd/producer

run-processor:
	@set -a; [ ! -f .env ] || . ./.env; set +a; go run ./cmd/processor

run-subscriber:
	@set -a; [ ! -f .env ] || . ./.env; set +a; go run ./cmd/subscriber

latest-message:
	@set -a; [ ! -f .env ] || . ./.env; set +a; \
	ENDPOINT=$${SQS_ENDPOINT:-http://localhost:9324}; \
	QUEUE=$${SQS_QUEUE_NAME:-webhook-events}; \
	AWS_ACCESS_KEY_ID=$${AWS_ACCESS_KEY_ID:-x} \
	AWS_SECRET_ACCESS_KEY=$${AWS_SECRET_ACCESS_KEY:-x} \
	AWS_DEFAULT_REGION=$${AWS_REGION:-us-east-1} \
	aws --endpoint-url=$$ENDPOINT sqs receive-message \
		--queue-url "$$ENDPOINT/000000000000/$$QUEUE" \
		--message-attribute-names All \
		--attribute-names All \
		--max-number-of-messages 1 \
		--visibility-timeout 0 \
		--output json

ack-latest-message:
	@set -a; [ ! -f .env ] || . ./.env; set +a; \
	ENDPOINT=$${SQS_ENDPOINT:-http://localhost:9324}; \
	QUEUE=$${SQS_QUEUE_NAME:-webhook-events}; \
	RECEIPT_HANDLE=$$(AWS_ACCESS_KEY_ID=$${AWS_ACCESS_KEY_ID:-x} \
		AWS_SECRET_ACCESS_KEY=$${AWS_SECRET_ACCESS_KEY:-x} \
		AWS_DEFAULT_REGION=$${AWS_REGION:-us-east-1} \
		aws --endpoint-url=$$ENDPOINT sqs receive-message \
			--queue-url "$$ENDPOINT/000000000000/$$QUEUE" \
			--max-number-of-messages 1 \
			--visibility-timeout 30 \
			--query 'Messages[0].ReceiptHandle' \
			--output text); \
	if [ "$$RECEIPT_HANDLE" = "None" ] || [ -z "$$RECEIPT_HANDLE" ]; then \
		echo "No available message to acknowledge."; \
		exit 0; \
	fi; \
	AWS_ACCESS_KEY_ID=$${AWS_ACCESS_KEY_ID:-x} \
	AWS_SECRET_ACCESS_KEY=$${AWS_SECRET_ACCESS_KEY:-x} \
	AWS_DEFAULT_REGION=$${AWS_REGION:-us-east-1} \
	aws --endpoint-url=$$ENDPOINT sqs delete-message \
		--queue-url "$$ENDPOINT/000000000000/$$QUEUE" \
		--receipt-handle "$$RECEIPT_HANDLE"; \
	echo "Acknowledged one message from $$QUEUE."

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy
