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
)

const apiPrefix = "/api"

func newHandler() (http.Handler, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	logger := observability.NewLogger()
	metricsCollector := metrics.New()

	// ------------------------------------------------------------
	// Upstream
	// ------------------------------------------------------------

	upstream, err := url.Parse(cfg.Upstream.URL)
	if err != nil {
		return nil, err
	}

	// ------------------------------------------------------------
	// Circuit Breaker
	// ------------------------------------------------------------

	breaker := circuitbreaker.New(
		cfg.CircuitBreaker.FailureThreshold,
		cfg.CircuitBreaker.ResetTimeout,
	)

	// ------------------------------------------------------------
	// Proxy Executor
	// ------------------------------------------------------------

	executor := &proxy.Executor{
		Client: &http.Client{
			Timeout: 0,
		},

		Upstream: upstream,

		Retry: retry.Controller{
			MaxAttempts: cfg.Retry.MaxAttempts,
			BaseDelay:   cfg.Retry.BaseDelay,
		},

		Breaker: breaker,

		Metrics: metricsCollector,

		RequestTimeout: cfg.Gateway.RequestTimeout,
	}

	// ------------------------------------------------------------
	// Idempotency
	// ------------------------------------------------------------

	idempotencyStore := idempotency.NewStore(
		cfg.Idempotency.TTL,
	)

	idempotencyController := idempotency.NewController(
		idempotencyStore,
	)

	idempotencyMiddleware := idempotency.NewMiddleware(
		idempotencyController,
	)

	// ------------------------------------------------------------
	// Gateway Handler
	// ------------------------------------------------------------

	var handler http.Handler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {

			// ----------------------------------------------------
			// Health
			// ----------------------------------------------------

			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)

				_, _ = w.Write([]byte("ok"))

				return
			}

			// ----------------------------------------------------
			// Metrics
			// ----------------------------------------------------

			if r.URL.Path == "/metrics" {
				metrics.Handler(
					metricsCollector,
				).ServeHTTP(w, r)

				return
			}

			// ----------------------------------------------------
			// API Routing
			// ----------------------------------------------------

			if !strings.HasPrefix(
				r.URL.Path,
				apiPrefix+"/",
			) {
				http.NotFound(w, r)

				return
			}

			originalPath := r.URL.Path

			// ----------------------------------------------------
			// Strip /api before forwarding.
			//
			// /api/orders
			//       ↓
			// /orders
			// ----------------------------------------------------

			r.URL.Path = strings.TrimPrefix(
				r.URL.Path,
				apiPrefix,
			)

			if r.URL.Path == "" {
				r.URL.Path = "/"
			}

			// ----------------------------------------------------
			// Execute through:
			//
			// Retry
			// Circuit Breaker
			// Proxy
			// Metrics
			// ----------------------------------------------------

			result, err := executor.Execute(
				r.Context(),
				r,
			)

			if result != nil {
				r.Header.Set(
					observability.UpstreamAttemptsHeader,
					strconv.Itoa(result.Attempts),
				)
			}

			// Restore client-facing path.
			r.URL.Path = originalPath

			if err != nil {

				// ------------------------------------------------
				// Circuit breaker rejection.
				// ------------------------------------------------

				if err == circuitbreaker.ErrOpen ||
					err == circuitbreaker.ErrBusy {

					http.Error(
						w,
						"upstream temporarily unavailable",
						http.StatusServiceUnavailable,
					)

					return
				}

				// ------------------------------------------------
				// Generic upstream failure.
				// ------------------------------------------------

				http.Error(
					w,
					"upstream request failed",
					http.StatusBadGateway,
				)

				return
			}

			// ----------------------------------------------------
			// Write upstream response.
			// ----------------------------------------------------

			if err := result.WriteTo(w); err != nil {
				log.Printf(
					"gateway: response write error request_id=%s error=%v",
					r.Header.Get(requestid.Header),
					err,
				)
			}
		},
	)

	// ------------------------------------------------------------
	// Middleware
	//
	// Execution order:
	//
	// Request ID
	//      ↓
	// Observability
	//      ↓
	// Idempotency
	//      ↓
	// Gateway Handler
	//      ↓
	// Proxy
	//      ↓
	// Upstream
	//
	// Request ID is outermost so every request receives a
	// correlation ID, including middleware-generated errors.
	// ------------------------------------------------------------

	handler = idempotencyMiddleware.Handler(handler)

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
		log.Fatal(err)
	}

	handler, err := newHandler()
	if err != nil {
		log.Fatal(err)
	}

	// ------------------------------------------------------------
	// HTTP Server
	// ------------------------------------------------------------

	server := &http.Server{
		Addr:    cfg.Gateway.Address,
		Handler: handler,

		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// ------------------------------------------------------------
	// Start Server
	// ------------------------------------------------------------

	go func() {
		log.Printf(
			"Asecra Fabric gateway listening on %s",
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

	// ------------------------------------------------------------
	// Graceful Shutdown
	// ------------------------------------------------------------

	stop := make(chan os.Signal, 1)

	signal.Notify(
		stop,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	<-stop

	log.Println(
		"gateway shutdown initiated",
	)

	// Give active requests time to finish.
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf(
			"gateway graceful shutdown failed: %v",
			err,
		)

		return
	}

	log.Println(
		"gateway shutdown complete",
	)
}
