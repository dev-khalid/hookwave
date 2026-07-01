package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dev-khalid/hookwave/internal/config"
	"github.com/dev-khalid/hookwave/internal/events/order"
	"github.com/dev-khalid/hookwave/internal/observability"
)

const serviceName = "processor"
const maxNumberOfMessagePerBatch = 10
const MessagePollingWorkers = 50
const ConcurrentProcessor = 500

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

	for {
		select {
		case <-ctx.Done():
			// do something;
			logger.Info("Received termination signal. Handing in-flight jobs gracefully...")
			wg.Wait()
			return
		default:
			//TODO: Create worker pool of 50 workers. And pool 100 message at a time.

			// pull data
			messages, err := client.ReceiveMessages(ctx, maxNumberOfMessagePerBatch)

			if err != nil {
				logger.Error("receive messages", "error", err)
				continue
			}

			for _, message := range messages {
				var base order.BaseOrderEvent
				if err := json.Unmarshal(message.Body, &base); err != nil {
					logger.Error("failed to parse event envelope", "error", err)
					continue
				}

				switch base.Type {
				case order.OrderCreatedEventType:
					var e order.OrderCreatedEvent
					if err := json.Unmarshal(message.Body, &e); err != nil {
						logger.Error("failed to parse order.created", "error", err)
						continue
					}
					// TODO: handle e
					logData(e.Type)
				case order.OrderUpdatedEventType:
					var e order.OrderUpdatedEvent
					if err := json.Unmarshal(message.Body, &e); err != nil {
						logger.Error("failed to parse order.updated", "error", err)
						continue
					}
					// TODO: handle e
				case order.OrderShippedEventType:
					var e order.OrderShippedEvent
					if err := json.Unmarshal(message.Body, &e); err != nil {
						logger.Error("failed to parse order.shipped", "error", err)
						continue
					}
					// TODO: handle e
				default:
					logger.Warn("unknown event type", "type", base.Type)
				}
			}
			// TODO: Push message to processor-worker-pool (500 processor at a time). It's super realistic for golang with very low cpu / ram. Because the goroutine is 2-8kb in stack size and let's assume the data itself is 10kb. So 500 processor will take 10mb + 4mb = 14mb of ram. And the cpu will be very low because the processor is just doing some simple processing and not doing any heavy computation. So it's super realistic to have 500 processor at a time.
			// TODO: Process and delete data from SQS.

			wg.Add(1)
			go fakeDeliverJob(wg)
		}
	}
}

func logData(data any) {
	fmt.Printf("Order event: %+v\n", data)
}

func fakeDeliverJob(wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Println("Sleeping before graceful shutdown")
	time.Sleep(15 * time.Second)
	fmt.Println("Done sleeping before graceful shutdown")
}
