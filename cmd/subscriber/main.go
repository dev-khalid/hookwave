package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dev-khalid/hookwave/internal/observability"
	"github.com/dev-khalid/hookwave/internal/subscriber/routers"
	"github.com/dev-khalid/hookwave/internal/utility"
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

	if count := utility.IntEnv("GENERATE_SUBSCRIPTIONS", 0); count > 0 {
		path := utility.StringEnv("SUBSCRIPTIONS_FILE", defaultSubscriptionsFile)
		if err := generateSubscriptionsFile(path, count); err != nil {
			logger.Error("generate subscriptions", "error", err)
			os.Exit(1)
		}
		logger.Info("generated subscriptions", "count", count, "path", path)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)

	logger.Info("starting")

	server := &http.Server{
		Addr:    ":8080",
		Handler: routers.SubscriberRouter(),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	<-ctx.Done() // Wait for interrupt signal
	stop()       // Stop receiving signals

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Warn("Server shutdown forcefully 😥")
	} else {
		logger.Info("Server stopped gracefully 👍")
	}

	cancel()

}
