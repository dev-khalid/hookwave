package processor

import (
	"context"
	"encoding/json"
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
	// Decode the envelope once. `Data` stays raw until we know which concrete
	// type it is — this is the only difference from a plain decode-then-switch:
	// it avoids re-parsing id/type/occurred_at a second time per branch.
	var envelope struct {
		order.BaseOrderEvent
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(message.Body, &envelope); err != nil {
		return fmt.Errorf("decode envelope for message %s: %w", message.MessageID, err)
	}

	var err error
	switch envelope.Type {
	case order.OrderCreatedEventType:
		err = handleEvent(envelope.BaseOrderEvent, envelope.Data, "order.created", message.MessageID,
			func(base order.BaseOrderEvent, data order.OrderCreatedData) *order.OrderCreatedEvent {
				return &order.OrderCreatedEvent{BaseOrderEvent: base, Data: data}
			})

	case order.OrderUpdatedEventType:
		err = handleEvent(envelope.BaseOrderEvent, envelope.Data, "order.updated", message.MessageID,
			func(base order.BaseOrderEvent, data order.OrderUpdatedData) *order.OrderUpdatedEvent {
				return &order.OrderUpdatedEvent{BaseOrderEvent: base, Data: data}
			})

	case order.OrderShippedEventType:
		err = handleEvent(envelope.BaseOrderEvent, envelope.Data, "order.shipped", message.MessageID,
			func(base order.BaseOrderEvent, data order.OrderShippedData) *order.OrderShippedEvent {
				return &order.OrderShippedEvent{BaseOrderEvent: base, Data: data}
			})

	default:
		return fmt.Errorf("unknown event type %q for message %s", envelope.Type, message.MessageID)
	}

	if err != nil {
		return err
	}

	fmt.Printf("Message %s (%s) processed successfully\n", message.MessageID, envelope.Type)

	if err := client.DeleteMessage(ctx, message.ReceiptHandle); err != nil {
		return fmt.Errorf("delete message %s: %w", message.MessageID, err)
	}

	return nil
}

// handleEvent decodes raw into D, builds the concrete event via build, and runs it
// through the fake API call and DB update. D and E are inferred from build, so each
// call site keeps its own concrete types (order.OrderCreatedData/*order.OrderCreatedEvent,
// etc.) instead of widening to `any`.
func handleEvent[D any, E events.ListedEventTypes](base order.BaseOrderEvent, raw json.RawMessage, eventName, messageID string, build func(order.BaseOrderEvent, D) E) error {
	var data D
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("decode %s data for message %s: %w", eventName, messageID, err)
	}

	event := build(base, data)

	if err := fakeApiCall(base.ID); err != nil {
		return fmt.Errorf("api call for message %s: %w", messageID, err)
	}
	if err := fakeDbUpdate(event); err != nil {
		return fmt.Errorf("db update for message %s: %w", messageID, err)
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

// fakeDbUpdate simulates a DB write for event. T is a type parameter, not an interface,
// so it can't be type-switched on directly — boxing it into `any` first is what lets us
// identify which concrete member of events.ListedEventTypes was actually passed in.
func fakeDbUpdate[T events.ListedEventTypes](event T) error {
	switch e := any(event).(type) {
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
