package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dev-khalid/hookwave/internal/config"
	"github.com/dev-khalid/hookwave/internal/events"
	"github.com/dev-khalid/hookwave/internal/events/order"
	"github.com/dev-khalid/hookwave/internal/observability"
	"github.com/dev-khalid/hookwave/internal/queue"
)

const serviceName = "processor"
const MaxNumberOfMessagePerBatch = 10
const MessagePollingWorkers = 50
const DefaultTimeout = 10 * time.Second

func main() {
	logger, err := observability.NewLogger(serviceName)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("Processor running...")

	client, err := config.SQSClient(ctx)
	if err != nil {
		logger.Error("init SQS client", "error", err)
		os.Exit(1)
	}

	/**
	1. We will continiously pull data until SIGTERM.
	2. Before termination, we need to process in-flight messages.
	*/
	wg := &sync.WaitGroup{}

	for worker := 0; worker < MessagePollingWorkers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Get the messages
					messages, err := client.ReceiveMessages(ctx, MaxNumberOfMessagePerBatch)
					if err != nil {
						logger.Error("Error polling messages from Queue", "error", err)
						continue
					}

					logger.Info(fmt.Sprintf("Wokrer %d found %d messages to process.", worker, len(messages)))
					// Start processing the messages...
					// This messages should be concurrent as well. And we need to wait for the messages to be complete as well.
					processEvents(ctx, client, messages, *logger)
				}
			}
		}()
	}

	<-ctx.Done()
	logger.Info("Received termination signal. Handing in-flight jobs gracefully...")
	wg.Wait()
}

func processEvents(ctx context.Context, client *queue.Client, messages []queue.Message, logger observability.Logger) {
	// The processes should be concurrent. And the parent should wait for these processes to be complete.
	var wg sync.WaitGroup
	for _, message := range messages {
		wg.Add(1)
		go func(message queue.Message) {
			defer wg.Done()
			// Process event
			if err := processEvent(ctx, client, message); err != nil {
				logger.Error("failed to process event", "error", err, "message_id", message.MessageID)
			}
		}(message)
	}

	wg.Wait()
}

func processEvent(ctx context.Context, client *queue.Client, message queue.Message) error {
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

	var apiErr error
	switch envelope.Type {
	case order.OrderCreatedEventType:
		var data order.OrderCreatedData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return fmt.Errorf("decode order.created data for message %s: %w", message.MessageID, err)
		}
		// e is concretely *order.OrderCreatedEvent here — matches events.ListedEventTypes.
		e := &order.OrderCreatedEvent{BaseOrderEvent: envelope.BaseOrderEvent, Data: data}
		if apiErr = fakeApiCall(e.ID); apiErr == nil {
			apiErr = fakeDbUpdate(e)
		}

	case order.OrderUpdatedEventType:
		var data order.OrderUpdatedData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return fmt.Errorf("decode order.updated data for message %s: %w", message.MessageID, err)
		}
		e := &order.OrderUpdatedEvent{BaseOrderEvent: envelope.BaseOrderEvent, Data: data}
		if apiErr = fakeApiCall(e.ID); apiErr == nil {
			apiErr = fakeDbUpdate(e)
		}

	case order.OrderShippedEventType:
		var data order.OrderShippedData
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return fmt.Errorf("decode order.shipped data for message %s: %w", message.MessageID, err)
		}
		e := &order.OrderShippedEvent{BaseOrderEvent: envelope.BaseOrderEvent, Data: data}
		if apiErr = fakeApiCall(e.ID); apiErr == nil {
			apiErr = fakeDbUpdate(e)
		}

	default:
		return fmt.Errorf("unknown event type %q for message %s", envelope.Type, message.MessageID)
	}

	if apiErr != nil {
		return fmt.Errorf("api call for message %s: %w", message.MessageID, apiErr)
	}

	fmt.Printf("Message %s (%s) processed successfully\n", message.MessageID, envelope.Type)

	if err := client.DeleteMessage(ctx, message.ReceiptHandle); err != nil {
		return fmt.Errorf("delete message %s: %w", message.MessageID, err)
	}

	return nil
}

// fakeApiCall is generic so each call site keeps its concrete event type (order.OrderCreatedEvent,
// etc.) instead of widening to `any` — T is inferred from the argument at compile time.
func fakeApiCall(id string) error {
	sleepTime := rand.Intn(20000) + 100
	// Buffered so the goroutine's send never blocks if ctx.Done() wins the
	// select below — otherwise a timed-out call leaks this goroutine forever.
	resultCh := make(chan bool, 1)
	go func() {
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		resultCh <- true
	}()
	// I need to race between timeout time vs sleepTime

	ctx, clearResources := context.WithTimeout(context.Background(), DefaultTimeout)
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
