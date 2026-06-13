package config

import (
	"fmt"
	"os"
)

type SQSConfig struct {
	QueueName          string
	Endpoint           string // empty = real AWS; non-empty = override (ElasticMQ, etc.)
	Region             string
	AWSAccessKeyID     string // populated only when env var is set; empty = use default credential chain
	AWSSecretAccessKey string
}

const defaultQueueName = "webhook-events"
const defaultRegion = "us-east-1"

func LoadSQSConfig() (SQSConfig, error) {
	queueName := getEnv("SQS_QUEUE_NAME", defaultQueueName)
	region := getEnv("AWS_REGION", defaultRegion)

	if queueName == "" {
		return SQSConfig{}, fmt.Errorf("SQS_QUEUE_NAME is required")
	}
	if region == "" {
		return SQSConfig{}, fmt.Errorf("AWS_REGION is required")
	}

	cfg := SQSConfig{
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
