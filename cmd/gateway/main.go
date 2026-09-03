package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/config"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/gateway"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/idempotency"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/metrics"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/observability"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/plugin"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/recovery"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/requestid"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/router"
)

func newHandlerWithConfig(cfg config.Config) (http.Handler, error) {
	return newHandlerWithConfigAndPipeline(
		cfg,
		plugin.NewPipeline(),
	)
}

func newHandlerWithConfigAndPipeline(
	cfg config.Config,
	pluginPipeline *plugin.Pipeline,
) (http.Handler, error) {
	logger := observability.NewLogger()
	metricsCollector := metrics.New()

	requestRouter := router.NewRouter([]router.Route{
		{
			Name:     "orders",
			Prefix:   "/api/orders",
			Upstream: "orders",
		},
		{
			Name:     "api",
			Prefix:   "/api",
			Upstream: "default",
		},
	})

	gatewayRuntime, err := gateway.New(
		cfg,
		requestRouter,
		pluginPipeline,
		metricsCollector,
	)
	if err != nil {
		return nil, err
	}

	idempotencyStore := idempotency.NewStore(
		cfg.Idempotency.TTL,
	)

	idempotencyController := idempotency.NewController(
		idempotencyStore,
	)

	idempotencyMiddleware := idempotency.NewMiddleware(
		idempotencyController,
	)

	var handler http.Handler = gatewayRuntime

	handler = idempotencyMiddleware.Handler(handler)

	handler = metrics.Middleware(
		metricsCollector,
		handler,
	)

	handler = observability.Middleware(
		logger,
		handler,
	)

	handler = requestid.Middleware(handler)

	handler = recovery.Middleware(handler)

	return handler, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(
			"failed to load config: %v",
			err,
		)
	}

	handler, err := newHandlerWithConfig(cfg)
	if err != nil {
		log.Fatalf(
			"failed to create gateway handler: %v",
			err,
		)
	}

	server := &http.Server{
		Addr:              cfg.Gateway.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf(
			"gateway listening on %s",
			cfg.Gateway.Address,
		)

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Fatalf(
				"gateway server failed: %v",
				err,
			)
		}
	}()

	signalChannel := make(chan os.Signal, 1)

	signal.Notify(
		signalChannel,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	<-signalChannel

	log.Println("shutting down gateway")

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf(
			"gateway shutdown failed: %v",
			err,
		)
	}

	log.Println("gateway stopped")
}
