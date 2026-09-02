package plugin

import (
	"errors"
	"github.com/mahalaxmiginnam/Asecra_Fabric/internal/router"
	"net/http"
	"reflect"
	"testing"
)

type testPlugin struct {
	name  string
	order *[]string
}

func (p testPlugin) Name() string {
	return p.name
}

type testPolicy struct {
	name string
}
type errorPlugin struct {
	name  string
	order *[]string
}

func (p errorPlugin) Name() string {
	return p.name
}

func (p errorPlugin) Execute(ctx *Context) (Result, error) {
	if p.order != nil {
		*p.order = append(*p.order, p.name)
	}

	return Result{}, errors.New("plugin execution failed")
}
func (p testPolicy) Name() string {
	return p.name
}

func (p testPolicy) Evaluate(ctx *Context) (Result, error) {
	return Result{
		Outcome: Continue,
	}, nil
}

func (p testPlugin) Execute(ctx *Context) (Result, error) {
	if ctx == nil || ctx.Request == nil {
		return Result{}, nil
	}

	if p.order != nil {
		*p.order = append(*p.order, p.name)
	}

	return Result{
		Outcome: Continue,
	}, nil
}

func newTestContext(t *testing.T) *Context {
	t.Helper()

	req, err := http.NewRequest(
		http.MethodGet,
		"/api/orders",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	return &Context{
		Request: req,
		Route: router.Route{
			Name:     "orders",
			Prefix:   "/api/orders",
			Upstream: "orders",
		},
	}
}

func TestPluginContract(t *testing.T) {
	ctx := newTestContext(t)

	p := testPlugin{name: "test"}

	if p.Name() != "test" {
		t.Fatalf(
			"expected plugin name %q, got %q",
			"test",
			p.Name(),
		)
	}

	result, err := p.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected plugin error: %v", err)
	}

	if result.Outcome != Continue {
		t.Fatalf(
			"expected Continue, got %v",
			result.Outcome,
		)
	}

	if ctx.Route.Name != "orders" {
		t.Fatalf(
			"expected route %q, got %q",
			"orders",
			ctx.Route.Name,
		)
	}
}

func TestPipelinePreservesPluginOrder(t *testing.T) {
	ctx := newTestContext(t)

	var order []string

	pipeline := NewPipeline(
		PluginComponent(testPlugin{
			name:  "first",
			order: &order,
		}),
		PluginComponent(testPlugin{
			name:  "second",
			order: &order,
		}),
		PluginComponent(testPlugin{
			name:  "third",
			order: &order,
		}),
	)

	if _, err := pipeline.Execute(ctx); err != nil {
		t.Fatalf(
			"unexpected pipeline error: %v",
			err,
		)
	}

	expected := []string{
		"first",
		"second",
		"third",
	}

	if !reflect.DeepEqual(order, expected) {
		t.Fatalf(
			"expected plugin order %v, got %v",
			expected,
			order,
		)
	}
}

func TestPolicyContract(t *testing.T) {
	ctx := newTestContext(t)

	p := testPolicy{name: "test-policy"}

	if p.Name() != "test-policy" {
		t.Fatalf(
			"expected policy name %q, got %q",
			"test-policy",
			p.Name(),
		)
	}

	result, err := p.Evaluate(ctx)
	if err != nil {
		t.Fatalf(
			"unexpected policy error: %v",
			err,
		)
	}

	if result.Outcome != Continue {
		t.Fatalf(
			"expected Continue, got %v",
			result.Outcome,
		)
	}
}

func TestMethodPolicyAllowsConfiguredMethod(t *testing.T) {
	ctx := newTestContext(t)

	policy := MethodPolicy{
		AllowedMethods: map[string]bool{
			http.MethodGet: true,
		},
	}

	result, err := policy.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected policy error: %v", err)
	}

	if result.Outcome != Continue {
		t.Fatalf(
			"expected Continue, got %v",
			result.Outcome,
		)
	}
}

func TestMethodPolicyRejectsUnconfiguredMethod(t *testing.T) {
	ctx := newTestContext(t)
	ctx.Request.Method = http.MethodPost

	policy := MethodPolicy{
		AllowedMethods: map[string]bool{
			http.MethodGet: true,
		},
	}

	result, err := policy.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected policy error: %v", err)
	}

	if result.Outcome != Reject {
		t.Fatalf(
			"expected Reject, got %v",
			result.Outcome,
		)
	}

	if result.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			result.StatusCode,
		)
	}

	if result.Message != "method not allowed" {
		t.Fatalf(
			"expected message %q, got %q",
			"method not allowed",
			result.Message,
		)
	}
}

type rejectingPolicy struct {
	name string
}

func (p rejectingPolicy) Name() string {
	return p.name
}

func (p rejectingPolicy) Evaluate(ctx *Context) (Result, error) {
	return Result{
		Outcome:    Reject,
		StatusCode: http.StatusForbidden,
		Message:    "request rejected by policy",
	}, nil
}

func TestPipelineShortCircuitsOnPolicyReject(t *testing.T) {
	ctx := newTestContext(t)

	var order []string

	pipeline := NewPipeline(
		PluginComponent(testPlugin{
			name:  "before-policy",
			order: &order,
		}),
		PolicyComponent(rejectingPolicy{
			name: "reject-policy",
		}),
		PluginComponent(testPlugin{
			name:  "after-policy",
			order: &order,
		}),
	)

	result, err := pipeline.Execute(ctx)
	if err != nil {
		t.Fatalf(
			"unexpected pipeline error: %v",
			err,
		)
	}

	if result.Outcome != Reject {
		t.Fatalf(
			"expected Reject, got %v",
			result.Outcome,
		)
	}

	if result.StatusCode != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusForbidden,
			result.StatusCode,
		)
	}

	expected := []string{
		"before-policy",
	}

	if !reflect.DeepEqual(order, expected) {
		t.Fatalf(
			"expected executed plugins %v, got %v",
			expected,
			order,
		)
	}
}

func TestPipelineStopsOnPluginError(t *testing.T) {
	ctx := newTestContext(t)

	var order []string

	pipeline := NewPipeline(
		PluginComponent(errorPlugin{
			name:  "failing-plugin",
			order: &order,
		}),
		PluginComponent(testPlugin{
			name:  "after-error",
			order: &order,
		}),
	)

	_, err := pipeline.Execute(ctx)
	if err == nil {
		t.Fatal("expected pipeline error, got nil")
	}

	expected := []string{
		"failing-plugin",
	}

	if !reflect.DeepEqual(order, expected) {
		t.Fatalf(
			"expected executed plugins %v, got %v",
			expected,
			order,
		)
	}
}

func TestPipelineRejectsNilComponent(t *testing.T) {
	ctx := newTestContext(t)

	pipeline := NewPipeline(
		PluginComponent(testPlugin{
			name: "before-nil",
		}),
		nil,
	)

	_, err := pipeline.Execute(ctx)
	if !errors.Is(err, ErrNilComponent) {
		t.Fatalf(
			"expected ErrNilComponent, got %v",
			err,
		)
	}
}
