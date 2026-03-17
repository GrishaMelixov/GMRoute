package router

import "testing"

func TestResolve_DefaultRoute(t *testing.T) {
	r := NewRouter(RouteDirectly)

	route := r.Resolve("google.com")
	if route.Type != Direct {
		t.Errorf("expected Direct default route, got %v", route.Type)
	}
}

func TestResolve_ExactRule(t *testing.T) {
	r := NewRouter(RouteDirectly)
	r.AddRule("youtube.com", NewUpstreamRoute("127.0.0.1:7890"))

	route := r.Resolve("youtube.com")
	if route.Type != Upstream {
		t.Errorf("expected Upstream, got %v", route.Type)
	}
	if route.ProxyAddr != "127.0.0.1:7890" {
		t.Errorf("expected proxy addr 127.0.0.1:7890, got %q", route.ProxyAddr)
	}
}

func TestResolve_SubdomainMatchesParentRule(t *testing.T) {
	r := NewRouter(RouteDirectly)
	r.AddRule("youtube.com", NewUpstreamRoute("127.0.0.1:7890"))

	tests := []string{"www.youtube.com", "cdn.youtube.com", "s.cdn.youtube.com"}
	for _, domain := range tests {
		route := r.Resolve(domain)
		if route.Type != Upstream {
			t.Errorf("domain %q: expected Upstream, got %v", domain, route.Type)
		}
	}
}

func TestResolve_UnknownDomainFallsToDefault(t *testing.T) {
	r := NewRouter(NewUpstreamRoute("127.0.0.1:7890"))
	r.AddRule("direct.com", RouteDirectly)

	route := r.Resolve("unknown.com")
	if route.Type != Upstream {
		t.Errorf("expected Upstream default, got %v", route.Type)
	}
}

func TestResolve_MultipleRules(t *testing.T) {
	r := NewRouter(RouteDirectly)
	r.AddRule("youtube.com", NewUpstreamRoute("proxy:1080"))
	r.AddRule("blocked.com", NewUpstreamRoute("proxy:1080"))
	r.AddRule("local.dev", RouteDirectly)

	cases := []struct {
		domain   string
		wantType RouteType
	}{
		{"youtube.com", Upstream},
		{"www.youtube.com", Upstream},
		{"blocked.com", Upstream},
		{"local.dev", Direct},
		{"google.com", Direct},
	}

	for _, tc := range cases {
		route := r.Resolve(tc.domain)
		if route.Type != tc.wantType {
			t.Errorf("domain %q: expected %v, got %v", tc.domain, tc.wantType, route.Type)
		}
	}
}
