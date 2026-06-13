package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/dev-khalid/hookwave/internal/config"
	"github.com/dev-khalid/hookwave/internal/observability"
	"github.com/dev-khalid/hookwave/internal/queue"
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

	cfg, err := config.LoadSQSConfig()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	_, err = queue.NewClient(ctx, cfg)
	if err != nil {
		logger.Error("Create SQS client error", "error", err)
		os.Exit(1)
	}

	logger.Info("ready", "queue", cfg.QueueName)

	<-ctx.Done()
	logger.Info("shutting down")
}
