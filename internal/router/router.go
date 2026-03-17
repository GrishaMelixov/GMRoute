package router

import "strings"

type Router struct {
	rules        map[string]Route
	defaultRoute Route
}

func NewRouter(defaultRoute Route) *Router {
	return &Router{
		rules:        make(map[string]Route),
		defaultRoute: defaultRoute,
	}
}

func (r *Router) AddRule(domain string, route Route) {
	r.rules[strings.ToLower(domain)] = route
}

func (r *Router) Resolve(domain string) Route {
	domain = strings.ToLower(domain)

	if route, ok := r.rules[domain]; ok {
		return route
	}

	parts := strings.SplitN(domain, ".", 2)
	for len(parts) == 2 {
		parent := parts[1]
		if route, ok := r.rules[parent]; ok {
			return route
		}
		parts = strings.SplitN(parent, ".", 2)
	}

	return r.defaultRoute
}
