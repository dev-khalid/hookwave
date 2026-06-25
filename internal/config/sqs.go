package config

import (
	"fmt"
	"os"

	"github.com/dev-khalid/hookwave/internal/queue"
)

const defaultQueueName = "webhook-events"
const defaultRegion = "us-east-1"

func loadSQSConfig() (queue.Config, error) {
	queueName := getEnv("SQS_QUEUE_NAME", defaultQueueName)
	region := getEnv("AWS_REGION", defaultRegion)

	if queueName == "" {
		return queue.Config{}, fmt.Errorf("SQS_QUEUE_NAME is required")
	}
	if region == "" {
		return queue.Config{}, fmt.Errorf("AWS_REGION is required")
	}

	cfg := queue.Config{
		QueueName: queueName,
		Endpoint:  os.Getenv("SQS_ENDPOINT"),
		Region:    region,
	}

	if id := os.Getenv("AWS_ACCESS_KEY_ID"); id != "" {
		cfg.AWSAccessKeyID = id
		cfg.AWSSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	return cfg, nil
}

func getEnv(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
