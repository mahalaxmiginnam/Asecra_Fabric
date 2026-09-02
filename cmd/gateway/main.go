package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/circuitbreaker"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/config"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/idempotency"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/metrics"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/observability"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/proxy"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/requestid"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/retry"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/router"
)

func newHandler() (http.Handler, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	return newHandlerWithConfig(cfg)
}

func newHandlerWithConfig(cfg config.Config) (http.Handler, error) {
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

	breaker := circuitbreaker.New(
		cfg.CircuitBreaker.FailureThreshold,
		cfg.CircuitBreaker.ResetTimeout,
	)

	idempotencyStore := idempotency.NewStore(
		cfg.Idempotency.TTL,
	)

	idempotencyController := idempotency.NewController(
		idempotencyStore,
	)

	idempotencyMiddleware := idempotency.NewMiddleware(
		idempotencyController,
	)

	var handler http.Handler = http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}

		if r.URL.Path == "/metrics" {
			metrics.Handler(metricsCollector).ServeHTTP(w, r)
			return
		}

		route, matched := requestRouter.Match(r.URL.Path)
		if !matched {
			http.Error(
				w,
				"route not found",
				http.StatusNotFound,
			)
			return
		}

		upstreamURL, ok := cfg.Upstreams[route.Upstream]
		if !ok {
			http.Error(
				w,
				"upstream configuration not found",
				http.StatusBadGateway,
			)
			return
		}

		upstream, err := url.Parse(upstreamURL)
		if err != nil {
			http.Error(
				w,
				"invalid upstream configuration",
				http.StatusBadGateway,
			)
			return
		}

		executor := &proxy.Executor{
			Client: &http.Client{
				Timeout: 0,
			},
			Upstream: upstream,
			Retry: retry.Controller{
				MaxAttempts: cfg.Retry.MaxAttempts,
				BaseDelay:   cfg.Retry.BaseDelay,
			},
			Breaker:        breaker,
			Metrics:        metricsCollector,
			RequestTimeout: cfg.Gateway.RequestTimeout,
		}

		originalPath := r.URL.Path

		r.URL.Path = strings.TrimPrefix(
			r.URL.Path,
			route.Prefix,
		)

		if r.URL.Path == "" {
			r.URL.Path = "/"
		}

		result, err := executor.Execute(
			r.Context(),
			r,
		)

		r.URL.Path = originalPath

		if result != nil {
			r.Header.Set(
				observability.UpstreamAttemptsHeader,
				strconv.Itoa(result.Attempts),
			)
		}

		if err != nil {
			if err == circuitbreaker.ErrOpen {
				http.Error(
					w,
					"upstream temporarily unavailable",
					http.StatusServiceUnavailable,
				)
				return
			}

			http.Error(
				w,
				"upstream request failed",
				http.StatusBadGateway,
			)
			return
		}

		result.WriteTo(w)
	})

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

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf(
			"gateway listening on %s",
			cfg.Gateway.Address,
		)

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(
		shutdownSignal,
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer signal.Stop(shutdownSignal)

	select {
	case err := <-serverErrors:
		log.Fatalf(
			"gateway server failed: %v",
			err,
		)

	case <-shutdownSignal:
		log.Println("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf(
			"gateway shutdown failed: %v",
			err,
		)
		return
	}

	log.Println("gateway stopped")
}
