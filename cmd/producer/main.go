package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/dev-khalid/hookwave/internal/config"
	"github.com/dev-khalid/hookwave/internal/observability"
)

const serviceName = "producer"

func main() {
	_ = godotenv.Load()

	logger, err := observability.NewLogger(serviceName)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := config.SQSClient(ctx)
	if err != nil {
		logger.Error("init SQS client", "error", err)
		os.Exit(1)
	}

	Run(ctx, client, logger)
	logger.Info("shutdown")
}
