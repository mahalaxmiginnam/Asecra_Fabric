package main

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/circuitbreaker"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/idempotency"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/proxy"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/retry"
)

const (
	gatewayAddr = ":8080"
	backendURL  = "http://localhost:9000"

	requestTimeout = 2 * time.Second

	maxAttempts = 3
	baseDelay   = 100 * time.Millisecond

	apiPrefix = "/api"

	circuitFailureThreshold = 3
	circuitResetTimeout     = 10 * time.Second

	idempotencyTTL = 10 * time.Minute
)

func main() {

	// ------------------------------------------------------------
	// Upstream
	// ------------------------------------------------------------

	upstream, err := url.Parse(backendURL)
	if err != nil {
		log.Fatal(err)
	}

	// ------------------------------------------------------------
	// Circuit Breaker
	// ------------------------------------------------------------

	breaker := circuitbreaker.New(
		circuitFailureThreshold,
		circuitResetTimeout,
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
			MaxAttempts: maxAttempts,
			BaseDelay:   baseDelay,
		},

		Breaker: breaker,

		RequestTimeout: requestTimeout,
	}

	// ------------------------------------------------------------
	// Idempotency
	// ------------------------------------------------------------

	idempotencyStore := idempotency.NewStore(
		idempotencyTTL,
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

				_, _ = w.Write(
					[]byte("ok"),
				)

				return
			}

			// ----------------------------------------------------
			// API Routing
			// ----------------------------------------------------

			if !strings.HasPrefix(
				r.URL.Path,
				apiPrefix+"/",
			) {
				http.NotFound(
					w,
					r,
				)

				return
			}

			// Preserve client-facing path.
			originalPath := r.URL.Path

			// ----------------------------------------------------
			// Strip /api before forwarding.
			//
			// /api/hello
			//      ↓
			// /hello
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
			// Upstream Proxy
			// ----------------------------------------------------

			result, err := executor.Execute(
				r.Context(),
				r,
			)

			// Restore client-facing path.
			r.URL.Path = originalPath

			if err != nil {

				log.Printf(
					"gateway: request failed path=%s error=%v",
					originalPath,
					err,
				)

				// Circuit breaker rejection.
				if err == circuitbreaker.ErrOpen ||
					err == circuitbreaker.ErrBusy {

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

			// ----------------------------------------------------
			// Logging
			// ----------------------------------------------------

			log.Printf(
				"gateway: path=%s status=%d attempts=%d",
				originalPath,
				result.StatusCode,
				result.Attempts,
			)

			// ----------------------------------------------------
			// Write upstream response.
			// ----------------------------------------------------

			if err := result.WriteTo(w); err != nil {

				log.Printf(
					"gateway: response write error=%v",
					err,
				)
			}
		},
	)

	// ------------------------------------------------------------
	// Middleware
	// ------------------------------------------------------------

	handler = idempotencyMiddleware.Handler(
		handler,
	)

	// ------------------------------------------------------------
	// Start Gateway
	// ------------------------------------------------------------

	log.Printf(
		"Asecra Fabric gateway listening on %s",
		gatewayAddr,
	)

	if err := http.ListenAndServe(
		gatewayAddr,
		handler,
	); err != nil {
		log.Fatal(err)
	}
}
