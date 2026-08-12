package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"

	"github.com/dev-khalid/hookwave/internal/events/order"
	"github.com/dev-khalid/hookwave/internal/subscriber"
)

const defaultSubscriptionsFile = "configs/subscriptions.json"

// Subscription is the seed shape for configs/subscriptions.json - the processor
// will load this file to match events to subscriber URLs (see plans/sprint-2-processor.md).

// generateSubscriptionsFile writes count random subscriptions to path, creating
// parent directories as needed.
func generateSubscriptionsFile(path string, count int) error {
	subs := make([]subscriber.Subscription, count)
	for i := range subs {
		subs[i] = subscriber.Subscription{
			ID:        uuid.New(),
			Events:    randomEventSubset(),
			URL:       fmt.Sprintf("http://localhost:3000/api/v1/webhook-listener/%s", gofakeit.UUID()),
			Method:    gofakeit.RandomString([]string{"POST", "PUT"}),
			CompanyID: gofakeit.IntRange(1, 10),
		}
	}

	body, err := json.MarshalIndent(subs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal subscriptions: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create subscriptions dir: %w", err)
	}

	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write subscriptions file %s: %w", path, err)
	}

	return nil
}

// randomEventSubset picks 1-3 unique event types from the listed event types.
func randomEventSubset() []order.EventType {
	pool := make([]order.EventType, len(order.OrderEventTypes))
	copy(pool, order.OrderEventTypes)

	// Convert to strings for shuffling
	strPool := make([]string, len(pool))
	for i, et := range pool {
		strPool[i] = string(et)
	}
	gofakeit.ShuffleStrings(strPool)

	// Convert back to EventType
	result := make([]order.EventType, len(strPool))
	for i, s := range strPool {
		result[i] = order.EventType(s)
	}

	n := gofakeit.IntRange(1, len(result))
	return result[:n]
}
