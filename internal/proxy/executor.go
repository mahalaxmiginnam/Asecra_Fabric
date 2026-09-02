package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/circuitbreaker"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/metrics"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/retry"
)

type Executor struct {
	Client         *http.Client
	Upstream       *url.URL
	Retry          retry.Controller
	Breaker        *circuitbreaker.Breaker
	Metrics        *metrics.Metrics
	RequestTimeout time.Duration
}

type Result struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Attempts   int
}

func (e *Executor) Execute(
	ctx context.Context,
	req *http.Request,
) (*Result, error) {

	if e.Breaker != nil {
		if err := e.Breaker.Allow(); err != nil {
			if e.Metrics != nil {
				e.Metrics.RecordCircuitBreakerRejection()
			}

			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(
		ctx,
		e.RequestTimeout,
	)
	defer cancel()

	// Read the incoming request body once.
	// The same byte slice is reused for every retry.
	var requestBody []byte

	if req.Body != nil {
		var err error

		requestBody, err = io.ReadAll(req.Body)
		if err != nil {
			if e.Breaker != nil {
				e.Breaker.Failure()
			}

			return nil, err
		}

		_ = req.Body.Close()
	}

	var lastErr error

	for attempt := 1; attempt <= e.Retry.MaxAttempts; attempt++ {

		result, err := e.executeOnce(
			ctx,
			req,
			requestBody,
		)

		if err != nil {
			lastErr = err

			if e.Metrics != nil {
				e.Metrics.RecordUpstreamFailure()
			}

			if !e.shouldRetry(req, attempt) {
				if e.Breaker != nil {
					e.Breaker.Failure()
				}

				return nil, err
			}

			if e.Metrics != nil {
				e.Metrics.RecordRetry()
			}

			if waitErr := e.Retry.Wait(
				ctx,
				attempt,
			); waitErr != nil {

				if e.Breaker != nil {
					e.Breaker.Failure()
				}

				return nil, waitErr
			}

			continue
		}

		if circuitbreaker.ShouldCountAsFailure(
			result.StatusCode,
		) {

			if !e.shouldRetryResponse(
				req,
				result,
				attempt,
			) {

				if e.Breaker != nil {
					e.Breaker.Failure()
				}

				result.Attempts = attempt

				return result, nil
			}

			if e.Metrics != nil {
				e.Metrics.RecordUpstreamFailure()
				e.Metrics.RecordRetry()
			}

			if waitErr := e.Retry.Wait(
				ctx,
				attempt,
			); waitErr != nil {

				if e.Breaker != nil {
					e.Breaker.Failure()
				}

				return nil, waitErr
			}

			continue
		}

		if e.Breaker != nil {
			e.Breaker.Success()
		}

		result.Attempts = attempt

		return result, nil
	}

	if e.Breaker != nil {
		e.Breaker.Failure()
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, context.DeadlineExceeded
}

func (e *Executor) shouldRetry(
	req *http.Request,
	attempt int,
) bool {

	if !e.Retry.ShouldAttempt(attempt + 1) {
		return false
	}

	decision := retry.IsRetryable(
		retry.RequestPolicy{
			Method: req.Method,
		},
		retry.ResponsePolicy{
			StatusCode: http.StatusServiceUnavailable,
		},
	)

	return decision == retry.Retry
}

func (e *Executor) shouldRetryResponse(
	req *http.Request,
	result *Result,
	attempt int,
) bool {

	if !e.Retry.ShouldAttempt(attempt + 1) {
		return false
	}

	decision := retry.IsRetryable(
		retry.RequestPolicy{
			Method: req.Method,
		},
		retry.ResponsePolicy{
			StatusCode: result.StatusCode,
		},
	)

	return decision == retry.Retry
}

func (e *Executor) executeOnce(
	ctx context.Context,
	req *http.Request,
	body []byte,
) (*Result, error) {

	if e.Metrics != nil {
		e.Metrics.RecordUpstreamRequest()
	}

	upstreamURL := *e.Upstream

	upstreamURL.Path = joinPath(
		e.Upstream.Path,
		req.URL.Path,
	)

	upstreamURL.RawQuery = req.URL.RawQuery

	upstreamReq, err := http.NewRequestWithContext(
		ctx,
		req.Method,
		upstreamURL.String(),
		bytes.NewReader(body),
	)

	if err != nil {
		return nil, err
	}

	upstreamReq.Header = req.Header.Clone()
	removeHopByHopHeaders(upstreamReq.Header)

	resp, err := e.Client.Do(upstreamReq)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &Result{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       responseBody,
	}, nil
}

func (r *Result) WriteTo(
	w http.ResponseWriter,
) error {

	for key, values := range r.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Avoid forwarding an upstream Content-Length that may no longer
	// match the response after gateway processing.
	w.Header().Del("Content-Length")

	w.WriteHeader(r.StatusCode)

	_, err := io.Copy(
		w,
		bytes.NewReader(r.Body),
	)

	return err
}

func joinPath(
	base string,
	path string,
) string {

	if base == "" {
		return path
	}

	if path == "" {
		return base
	}

	if base[len(base)-1] == '/' &&
		path[0] == '/' {

		return base + path[1:]
	}

	if base[len(base)-1] != '/' &&
		path[0] != '/' {

		return base + "/" + path
	}

	return base + path
}

func removeHopByHopHeaders(header http.Header) {
	header.Del("Connection")
	header.Del("Keep-Alive")
	header.Del("Proxy-Authenticate")
	header.Del("Proxy-Authorization")
	header.Del("TE")
	header.Del("Trailer")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")
}
