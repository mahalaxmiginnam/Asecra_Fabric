package gateway

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/circuitbreaker"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/config"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/metrics"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/observability"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/plugin"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/router"
)

type Gateway struct {
	Config    config.Config
	Router    *router.Router
	Pipeline  *plugin.Pipeline
	Upstreams map[string]*UpstreamRuntime
}

func New(
	cfg config.Config,
	requestRouter *router.Router,
	pluginPipeline *plugin.Pipeline,
	metricsCollector *metrics.Metrics,
) (*Gateway, error) {
	upstreams := make(map[string]*UpstreamRuntime)

	for name, rawURL := range cfg.Upstreams {
		runtime, err := NewUpstreamRuntime(
			name,
			rawURL,
			cfg,
			metricsCollector,
		)
		if err != nil {
			return nil, err
		}

		upstreams[name] = runtime
	}

	return &Gateway{
		Config:    cfg,
		Router:    requestRouter,
		Pipeline:  pluginPipeline,
		Upstreams: upstreams,
	}, nil
}

func (g *Gateway) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	if r.URL.Path == "/metrics" {
		upstream := g.Upstreams["metrics"]

		if upstream != nil && upstream.Executor.Metrics != nil {
			metrics.Handler(
				upstream.Executor.Metrics,
			).ServeHTTP(w, r)
			return
		}

		http.Error(
			w,
			"metrics unavailable",
			http.StatusInternalServerError,
		)
		return
	}

	route, matched := g.Router.Match(r.URL.Path)
	if !matched {
		http.Error(
			w,
			"route not found",
			http.StatusNotFound,
		)
		return
	}

	pluginContext := &plugin.Context{
		Request: r,
		Route:   route,
	}

	pipelineResult, err := g.Pipeline.Execute(pluginContext)
	if err != nil {
		http.Error(
			w,
			"plugin pipeline failed",
			http.StatusInternalServerError,
		)
		return
	}

	if pipelineResult.Outcome == plugin.Reject {
		statusCode := pipelineResult.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusForbidden
		}

		message := pipelineResult.Message
		if message == "" {
			message = http.StatusText(statusCode)
		}

		http.Error(
			w,
			message,
			statusCode,
		)
		return
	}

	upstream, ok := g.Upstreams[route.Upstream]
	if !ok {
		http.Error(
			w,
			"upstream configuration not found",
			http.StatusBadGateway,
		)
		return
	}

	originalPath := r.URL.Path

	r.URL.Path = strings.TrimPrefix(
		r.URL.Path,
		route.Prefix,
	)

	if r.URL.Path == "" {
		r.URL.Path = "/"
	}

	result, err := upstream.Executor.Execute(
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

	if result != nil {
		_ = result.WriteTo(w)
	}
}
