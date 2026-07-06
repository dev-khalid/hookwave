package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/dev-khalid/hookwave/internal/config"
	"github.com/dev-khalid/hookwave/internal/observability"
	"github.com/dev-khalid/hookwave/internal/processor"
)

const serviceName = "processor"
const MaxNumberOfMessagePerBatch = 10
const MessagePollingWorkers = 50

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
					processor.ProcessEvents(ctx, client, messages, *logger)
				}
			}
		}()
	}

	<-ctx.Done()
	logger.Info("Received termination signal. Handing in-flight jobs gracefully...")
	wg.Wait()
}
