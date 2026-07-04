package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brianvoe/gofakeit/v6"

	"github.com/dev-khalid/hookwave/internal/events/order"
)

const defaultSubscriptionsFile = "configs/subscriptions.json"

var listedEventTypes = []string{
	string(order.OrderCreatedEventType),
	string(order.OrderUpdatedEventType),
	string(order.OrderShippedEventType),
}

// subscription is the seed shape for configs/subscriptions.json - the processor
// will load this file to match events to subscriber URLs (see plans/sprint-2-processor.md).
type subscription struct {
	Events    []string `json:"events"`
	URL       string   `json:"url"`
	CompanyID int      `json:"company_id"`
}

// generateSubscriptionsFile writes count random subscriptions to path, creating
// parent directories as needed.
func generateSubscriptionsFile(path string, count int) error {
	subs := make([]subscription, count)
	for i := range subs {
		subs[i] = subscription{
			Events:    randomEventSubset(),
			URL:       fmt.Sprintf("http://localhost:3000/%s", gofakeit.UUID()),
			CompanyID: gofakeit.IntRange(1, 100),
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
func randomEventSubset() []string {
	pool := make([]string, len(listedEventTypes))
	copy(pool, listedEventTypes)
	gofakeit.ShuffleStrings(pool)

	n := gofakeit.IntRange(1, len(pool))
	return pool[:n]
}
