package main

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v6"

	"github.com/dev-khalid/hookwave/internal/events/order"
	"github.com/dev-khalid/hookwave/internal/observability"
	"github.com/dev-khalid/hookwave/internal/queue"
)

const (
	defaultInterval = 1 * time.Second
	defaultWorkers  = 10
)

var validEventTypes = []string{
	string(order.OrderCreatedEventType),
	string(order.OrderUpdatedEventType),
	string(order.OrderShippedEventType),
}

// Run starts the producer in either burst or ticker mode based on env vars.
//
//	PRODUCE_BURST=N    send N messages concurrently via a worker pool, then exit
//	PRODUCE_WORKERS=N  number of concurrent workers for burst mode (default 10)
//	PRODUCE_INTERVAL=d ticker interval, e.g. 500ms or 2s (default 1s)
func Run(ctx context.Context, client *queue.Client, logger *observability.Logger) {
	if burst := intEnv("PRODUCE_BURST", 0); burst > 0 {
		workers := intEnv("PRODUCE_WORKERS", defaultWorkers)
		logger.Info("burst mode", "count", burst, "workers", workers)
		runBurst(ctx, client, burst, workers, logger)
		return
	}

	interval := durationEnv("PRODUCE_INTERVAL", defaultInterval)
	logger.Info("ticker mode", "interval", interval)
	runTicker(ctx, client, interval, logger)
}

func runTicker(ctx context.Context, client *queue.Client, interval time.Duration, logger *observability.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			publishOne(ctx, client, logger)
		}
	}
}

func runBurst(ctx context.Context, client *queue.Client, count, workers int, logger *observability.Logger) {
	jobs := make(chan struct{}, workers)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range jobs {
				publishOne(ctx, client, logger)
			}
		})
	}

	sent := 0
	for range count {
		if ctx.Err() != nil {
			break
		}
		jobs <- struct{}{}
		sent++
	}
	close(jobs)
	wg.Wait()

	logger.Info("burst complete", "sent", sent)
}

func publishOne(ctx context.Context, client *queue.Client, logger *observability.Logger) {
	body, eventType, err := randomEvent()
	if err != nil {
		logger.Error("generate event", "error", err)
		return
	}

	if err := client.Publish(ctx, body, map[string]string{"event_type": eventType}); err != nil {
		logger.Error("publish event", "event_type", eventType, "error", err)
		return
	}

	logger.Info("published", "event_type", eventType)
}

// --- event generators ---

func randomEvent() (body []byte, eventType string, err error) {
	eventType = gofakeit.RandomString(validEventTypes)

	var event any
	switch order.EventType(eventType) {
	case order.OrderCreatedEventType:
		event = newOrderCreated()
	case order.OrderUpdatedEventType:
		event = newOrderUpdated()
	case order.OrderShippedEventType:
		event = newOrderShipped()
	}

	body, err = json.Marshal(event)
	return
}

func newOrderCreated() *order.OrderCreatedEvent {
	return &order.OrderCreatedEvent{
		BaseOrderEvent: order.BaseOrderEvent{
			ID:         gofakeit.UUID(),
			Type:       order.OrderCreatedEventType,
			OccurredAt: time.Now().UTC(),
		},
		Data: order.OrderCreatedData{
			BaseOrderData: order.BaseOrderData{
				OrderID:    gofakeit.UUID(),
				CustomerID: gofakeit.UUID(),
				Status:     order.OrderStatusCreated,
				Currency:   "USD",
				Amount:     gofakeit.Float64Range(10, 1000),
			},
			Items:           randomItems(),
			ShippingAddress: randomAddress(),
		},
	}
}

func newOrderUpdated() *order.OrderUpdatedEvent {
	from := gofakeit.RandomString([]string{
		string(order.OrderStatusCreated),
		string(order.OrderStatusUpdated),
	})
	return &order.OrderUpdatedEvent{
		BaseOrderEvent: order.BaseOrderEvent{
			ID:         gofakeit.UUID(),
			Type:       order.OrderUpdatedEventType,
			OccurredAt: time.Now().UTC(),
		},
		Data: order.OrderUpdatedData{
			BaseOrderData: order.BaseOrderData{
				OrderID:    gofakeit.UUID(),
				CustomerID: gofakeit.UUID(),
				Status:     order.OrderStatusUpdated,
				Currency:   "USD",
				Amount:     gofakeit.Float64Range(10, 1000),
			},
			PreviousStatus: from,
			Changes: order.Changes{
				Status: order.StatusChange{
					From: from,
					To:   string(order.OrderStatusUpdated),
				},
				Payment: order.PaymentInfo{
					Method:        order.PaymentMethod(gofakeit.RandomString([]string{"card", "bank_transfer"})),
					TransactionID: gofakeit.UUID(),
					PaidAt:        time.Now().UTC(),
				},
			},
		},
	}
}

func newOrderShipped() *order.OrderShippedEvent {
	shipped := time.Now().UTC()
	return &order.OrderShippedEvent{
		BaseOrderEvent: order.BaseOrderEvent{
			ID:         gofakeit.UUID(),
			Type:       order.OrderShippedEventType,
			OccurredAt: shipped,
		},
		Data: order.OrderShippedData{
			BaseOrderData: order.BaseOrderData{
				OrderID:    gofakeit.UUID(),
				CustomerID: gofakeit.UUID(),
				Status:     order.OrderStatusShipped,
				Currency:   "USD",
				Amount:     gofakeit.Float64Range(10, 1000),
			},
			Shipment: order.Shipment{
				Carrier:           gofakeit.RandomString([]string{"UPS", "FedEx", "DHL", "USPS"}),
				TrackingNumber:    gofakeit.Lexify("????-######"),
				TrackingURL:       "https://tracking.example.com/" + gofakeit.Lexify("??########"),
				ShippedAt:         shipped,
				EstimatedDelivery: shipped.Add(time.Duration(gofakeit.IntRange(2, 7)) * 24 * time.Hour),
				Items:             randomItems(),
			},
		},
	}
}

func randomItems() []order.OrderItem {
	items := make([]order.OrderItem, gofakeit.IntRange(1, 4))
	for i := range items {
		items[i] = order.OrderItem{
			SKU:       gofakeit.Lexify("SKU-????-####"),
			Name:      gofakeit.ProductName(),
			Quantity:  gofakeit.IntRange(1, 5),
			UnitPrice: gofakeit.Float64Range(5, 200),
		}
	}
	return items
}

func randomAddress() order.ShippingAddress {
	return order.ShippingAddress{
		Line1:      gofakeit.StreetName(),
		City:       gofakeit.City(),
		PostalCode: gofakeit.Zip(),
		Country:    gofakeit.RandomString([]string{"US", "CA", "GB", "AU", "DE"}),
	}
}

// --- env helpers ---

func intEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return fallback
}
