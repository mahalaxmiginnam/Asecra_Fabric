package router

import "testing"

func TestRouterMatch(t *testing.T) {
	router := NewRouter([]Route{
		{
			Name:     "api",
			Prefix:   "/api",
			Upstream: "default",
		},
		{
			Name:     "orders",
			Prefix:   "/api/orders",
			Upstream: "orders",
		},
	})

	tests := []struct {
		name             string
		path             string
		matched          bool
		expectedRoute    string
		expectedUpstream string
	}{
		{
			name:             "api root",
			path:             "/api",
			matched:          true,
			expectedRoute:    "api",
			expectedUpstream: "default",
		},
		{
			name:             "api resource",
			path:             "/api/customers",
			matched:          true,
			expectedRoute:    "api",
			expectedUpstream: "default",
		},
		{
			name:             "orders resource",
			path:             "/api/orders",
			matched:          true,
			expectedRoute:    "orders",
			expectedUpstream: "orders",
		},
		{
			name:             "orders nested resource",
			path:             "/api/orders/123",
			matched:          true,
			expectedRoute:    "orders",
			expectedUpstream: "orders",
		},
		{
			name:    "similar prefix does not match",
			path:    "/apiary",
			matched: false,
		},
		{
			name:    "unrelated path",
			path:    "/health",
			matched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, matched := router.Match(tt.path)

			if matched != tt.matched {
				t.Fatalf("expected matched=%v, got %v", tt.matched, matched)
			}

			if !matched {
				return
			}

			if route.Name != tt.expectedRoute {
				t.Fatalf(
					"expected route %q, got %q",
					tt.expectedRoute,
					route.Name,
				)
			}
			if route.Upstream != tt.expectedUpstream {
				t.Fatalf(
					"expected upstream %q, got %q",
					tt.expectedUpstream,
					route.Upstream,
				)
			}
		})
	}
}
