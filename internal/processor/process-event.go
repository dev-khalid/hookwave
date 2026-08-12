package processor

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/dev-khalid/hookwave/internal/events"
	"github.com/dev-khalid/hookwave/internal/events/order"
	"github.com/dev-khalid/hookwave/internal/queue"
)

const defaultTimeout = 10 * time.Second

// fakeAPITimeoutRate simulates roughly 1-in-N calls hanging past defaultTimeout,
// so the timeout/retry/DLQ path gets exercised deliberately instead of by accident.
const fakeAPITimeoutRate = 5

func processEvent(ctx context.Context, client *queue.Client, message *queue.Message) error {
	base, event, err := events.Decode(message.Body)
	if err != nil {
		return fmt.Errorf("decode message %s: %w", message.MessageID, err)
	}

	if err := fakeApiCall(base.ID); err != nil {
		return fmt.Errorf("api call for message %s: %w", message.MessageID, err)
	}
	if err := fakeDbUpdate(event); err != nil {
		return fmt.Errorf("db update for message %s: %w", message.MessageID, err)
	}

	fmt.Printf("Message %s (%s) processed successfully\n", message.MessageID, base.Type)

	if err := client.DeleteMessage(ctx, message.ReceiptHandle); err != nil {
		return fmt.Errorf("delete message %s: %w", message.MessageID, err)
	}

	return nil
}

// fakeApiCall simulates an outbound call keyed by event ID.
func fakeApiCall(id string) error {
	var sleepTime time.Duration
	if rand.Intn(fakeAPITimeoutRate) == 0 {
		// Deliberately hang past defaultTimeout.
		sleepTime = defaultTimeout + time.Duration(rand.Intn(5000)+100)*time.Millisecond
	} else {
		// Normal latency, comfortably under defaultTimeout.
		sleepTime = time.Duration(rand.Intn(2000)+100) * time.Millisecond
	}
	// Buffered so the goroutine's send never blocks if ctx.Done() wins the
	// select below — otherwise a timed-out call leaks this goroutine forever.
	resultCh := make(chan bool, 1)
	go func() {
		time.Sleep(sleepTime)
		resultCh <- true
	}()
	// I need to race between timeout time vs sleepTime

	ctx, clearResources := context.WithTimeout(context.Background(), defaultTimeout)
	defer clearResources()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-resultCh:
		fmt.Printf("ID: %s Done\n", id)
		close(resultCh)

	}
	return nil
}

// fakeDbUpdate simulates a DB write for event. event is `any` (rather than an
// events.ListedEventTypes type parameter) because events.Decode already erases
// the concrete type — the switch below is what recovers it.
func fakeDbUpdate(event any) error {
	switch e := event.(type) {
	case *order.OrderCreatedEvent:
		fmt.Printf("DB: inserting order %s (status=%s)\n", e.Data.OrderID, e.Data.Status)
	case *order.OrderUpdatedEvent:
		fmt.Printf("DB: updating order %s (status %s -> %s)\n", e.Data.OrderID, e.Data.PreviousStatus, e.Data.Changes.Status.To)
	case *order.OrderShippedEvent:
		fmt.Printf("DB: marking order %s shipped via %s\n", e.Data.OrderID, e.Data.Shipment.Carrier)
	default:
		return fmt.Errorf("fakeDbUpdate: unhandled event type %T", e)
	}

	sleepTime := rand.Intn(300) + 100
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	return nil
}
