package processor

import (
	"context"
	"fmt"
	"sync"

	"github.com/dev-khalid/hookwave/internal/observability"
	"github.com/dev-khalid/hookwave/internal/queue"
)

func ProcessEvents(ctx context.Context, client *queue.Client, messages []queue.Message, logger observability.Logger) {
	fmt.Printf("Processing %d messages...\n", len(messages))
	// The processes should be concurrent. And the parent should wait for these processes to be complete.
	var wg sync.WaitGroup
	for _, message := range messages {
		wg.Add(1)
		go func(message *queue.Message) {
			defer wg.Done()
			// Process event
			if err := processEvent(ctx, client, message); err != nil {
				logger.Error("failed to process event", "error", err, "message_id", message.MessageID)
			}
		}(&message)
	}

	wg.Wait()
}
