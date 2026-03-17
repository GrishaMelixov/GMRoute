package router

type RouteType int

const (
	Direct   RouteType = iota
	Upstream
)

type Route struct {
	Type      RouteType
	ProxyAddr string
}

var RouteDirectly = Route{Type: Direct}

func NewUpstreamRoute(addr string) Route {
	return Route{Type: Upstream, ProxyAddr: addr}
}
