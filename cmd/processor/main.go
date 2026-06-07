package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/dev-khalid/hookwave/internal/observability"
)

const serviceName = "processor"

func main() {

	
	logger, err := observability.NewLogger(serviceName)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("Processor running...")

	<-ctx.Done()
 
	logger.Info("Processor shutting down...")
}
