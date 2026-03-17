package router

import (
	"github.com/GrishaMelixov/GMRoute/internal/trie"
)

type Router struct {
	rules        *trie.Trie[Route]
	defaultRoute Route
}

func NewRouter(defaultRoute Route) *Router {
	return &Router{
		rules:        trie.New[Route](),
		defaultRoute: defaultRoute,
	}
}

func (r *Router) AddRule(domain string, route Route) {
	r.rules.Add(domain, route)
}

func (r *Router) Resolve(domain string) Route {
	if route, ok := r.rules.Lookup(domain); ok {
		return route
	}
	return r.defaultRoute
}
