package router

import (
	"sync"

	"github.com/GrishaMelixov/GMRoute/internal/trie"
)

type RuleEntry struct {
	Domain string `json:"domain"`
	Route  string `json:"route"`
}

type Router struct {
	mu           sync.RWMutex
	rules        *trie.Trie[Route]
	defaultRoute Route
	ruleList     []RuleEntry
}

func NewRouter(defaultRoute Route) *Router {
	return &Router{
		rules:        trie.New[Route](),
		defaultRoute: defaultRoute,
	}
}

func (r *Router) AddRule(domain string, route Route) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules.Add(domain, route)
	// update or append ruleList entry
	for i, e := range r.ruleList {
		if e.Domain == domain {
			if route.Type == Upstream {
				r.ruleList[i].Route = "upstream"
			} else {
				r.ruleList[i].Route = "direct"
			}
			return
		}
	}
	routeStr := "direct"
	if route.Type == Upstream {
		routeStr = "upstream"
	}
	r.ruleList = append(r.ruleList, RuleEntry{Domain: domain, Route: routeStr})
}

func (r *Router) RemoveRule(domain string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	ok := r.rules.Delete(domain)
	for i, e := range r.ruleList {
		if e.Domain == domain {
			r.ruleList = append(r.ruleList[:i], r.ruleList[i+1:]...)
			break
		}
	}
	return ok
}

func (r *Router) GetRules() []RuleEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]RuleEntry, len(r.ruleList))
	copy(result, r.ruleList)
	return result
}

func (r *Router) Resolve(domain string) Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if route, ok := r.rules.Lookup(domain); ok {
		return route
	}
	return r.defaultRoute
}
