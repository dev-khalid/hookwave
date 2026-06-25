package config

import (
	"context"
	"sync"

	"github.com/dev-khalid/hookwave/internal/queue"
)

var (
	once      sync.Once
	sqsClient *queue.Client
	clientErr error
)

// SQSClient returns the shared SQS client, initialising it on first call.
func SQSClient(ctx context.Context) (*queue.Client, error) {
	once.Do(func() {
		cfg, err := loadSQSConfig()
		if err != nil {
			clientErr = err
			return
		}
		sqsClient, clientErr = queue.NewClient(ctx, cfg)
	})
	return sqsClient, clientErr
}
