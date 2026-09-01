package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/circuitbreaker"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/retry"
)

type Executor struct {
	Client         *http.Client
	Upstream       *url.URL
	Retry          retry.Controller
	Breaker        *circuitbreaker.Breaker
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

	// 1. Circuit breaker decides whether this client request
	// is allowed to enter the retry/upstream layer.
	if e.Breaker != nil {
		if err := e.Breaker.Allow(); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(
		ctx,
		e.RequestTimeout,
	)
	defer cancel()

	var lastErr error

	for attempt := 1; attempt <= e.Retry.MaxAttempts; attempt++ {

		result, err := e.executeOnce(
			ctx,
			req,
		)

		// Network / transport failure.
		if err != nil {
			lastErr = err

			// Retry if budget remains.
			if e.Retry.ShouldAttempt(attempt + 1) {

				if waitErr := e.Retry.Wait(
					ctx,
					attempt,
				); waitErr != nil {
					// The request itself failed because the
					// context expired/cancelled. Treat this
					// client execution as one failure.
					if e.Breaker != nil {
						e.Breaker.Failure()
					}

					return nil, waitErr
				}

				continue
			}

			// Exhausted retry budget.
			if e.Breaker != nil {
				e.Breaker.Failure()
			}

			return nil, lastErr
		}

		// Dependency-level failure.
		if circuitbreaker.ShouldCountAsFailure(
			result.StatusCode,
		) {

			// Retry while budget remains.
			if e.Retry.ShouldAttempt(attempt + 1) {

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

			// Final dependency failure.
			if e.Breaker != nil {
				e.Breaker.Failure()
			}

			result.Attempts = attempt

			return result, nil
		}

		// Any non-dependency-failure response is considered
		// successful from the circuit breaker's perspective.
		if e.Breaker != nil {
			e.Breaker.Success()
		}

		result.Attempts = attempt

		return result, nil
	}

	// Defensive fallback.
	if e.Breaker != nil {
		e.Breaker.Failure()
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, context.DeadlineExceeded
}

func (e *Executor) executeOnce(
	ctx context.Context,
	req *http.Request,
) (*Result, error) {

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
		bytes.NewReader(nil),
	)

	if err != nil {
		return nil, err
	}

	upstreamReq.Header = req.Header.Clone()

	resp, err := e.Client.Do(upstreamReq)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &Result{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       body,
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
