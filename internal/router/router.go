package router

import "strings"

type Route struct {
	Name             string
	Prefix           string
	Upstream         string
	ExpectedUpstream string
}

type Router struct {
	routes []Route
}

func NewRouter(routes []Route) *Router {
	return &Router{routes: routes}
}

func (r *Router) Match(path string) (Route, bool) {
	var matched Route

	for _, route := range r.routes {
		if route.Prefix == "" {
			continue
		}

		if path != route.Prefix &&
			!strings.HasPrefix(path, route.Prefix+"/") {
			continue
		}

		if matched.Prefix == "" ||
			len(route.Prefix) > len(matched.Prefix) {
			matched = route
		}
	}

	if matched.Prefix == "" {
		return Route{}, false
	}

	return matched, true
}
