package gateway

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/circuitbreaker"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/config"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/metrics"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/proxy"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/retry"
)

type UpstreamRuntime struct {
	Name     string
	URL      *url.URL
	Executor *proxy.Executor
}

func NewUpstreamRuntime(
	name string,
	rawURL string,
	cfg config.Config,
	metricsCollector *metrics.Metrics,
) (*UpstreamRuntime, error) {
	upstreamURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if upstreamURL.Scheme != "http" &&
		upstreamURL.Scheme != "https" {
		return nil, fmt.Errorf(
			"upstream %q must use http or https scheme",
			name,
		)
	}

	if upstreamURL.Host == "" {
		return nil, fmt.Errorf(
			"upstream %q must have a host",
			name,
		)
	}

	breaker := circuitbreaker.New(
		cfg.CircuitBreaker.FailureThreshold,
		cfg.CircuitBreaker.ResetTimeout,
	)

	executor := &proxy.Executor{
		Client: &http.Client{
			Timeout: 0,
		},
		Upstream: upstreamURL,
		Retry: retry.Controller{
			MaxAttempts: cfg.Retry.MaxAttempts,
			BaseDelay:   cfg.Retry.BaseDelay,
		},
		Breaker:        breaker,
		Metrics:        metricsCollector,
		RequestTimeout: cfg.Gateway.RequestTimeout,
	}

	return &UpstreamRuntime{
		Name:     name,
		URL:      upstreamURL,
		Executor: executor,
	}, nil
}
