package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/dev-khalid/hookwave/internal/observability"
)

const serviceName = "subscriber"

// GENERATE_SUBSCRIPTIONS=N regenerates configs/subscriptions.json with N random
// entries and exits, instead of starting the service. See plans/sprint-2-processor.md
// and plans/sprint-3-subscriber-storage.md: the processor loads subscriber routing
// data from this generated JSON file, not from a hand-written subscriptions.yaml.
func main() {
	logger, err := observability.NewLogger(serviceName)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	if count := intEnv("GENERATE_SUBSCRIPTIONS", 0); count > 0 {
		path := stringEnv("SUBSCRIPTIONS_FILE", defaultSubscriptionsFile)
		if err := generateSubscriptionsFile(path, count); err != nil {
			logger.Error("generate subscriptions", "error", err)
			os.Exit(1)
		}
		logger.Info("generated subscriptions", "count", count, "path", path)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting")

	<-ctx.Done()

	logger.Info("shutting down")
}

func intEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func stringEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
