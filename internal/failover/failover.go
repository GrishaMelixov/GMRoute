package failover

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/GrishaMelixov/GMRoute/internal/router"
)

type Failover struct {
	router   *router.Router
	fallback router.Route
	cache    sync.Map
}

func New(r *router.Router, fallback router.Route) *Failover {
	return &Failover{router: r, fallback: fallback}
}

func (f *Failover) ResolvedRoute(host string) router.Route {
	if cached, ok := f.cache.Load(host); ok {
		return cached.(router.Route)
	}
	return f.router.Resolve(host)
}

func (f *Failover) Dial(host, target string) (net.Conn, error) {
	if cached, ok := f.cache.Load(host); ok {
		return dial(cached.(router.Route), target)
	}

	route := f.router.Resolve(host)

	conn, err := dial(route, target)
	if err == nil {
		return conn, nil
	}

	if route.Type == router.Direct && f.fallback.Type == router.Upstream {
		conn, err = dial(f.fallback, target)
		if err == nil {
			f.cache.Store(host, f.fallback)
			return conn, nil
		}
	}

	return nil, err
}

func dial(route router.Route, target string) (net.Conn, error) {
	switch route.Type {
	case router.Direct:
		return net.Dial("tcp", target)
	case router.Upstream:
		return dialViaUpstream(route.ProxyAddr, target)
	}
	return nil, fmt.Errorf("unknown route type: %v", route.Type)
}

func dialViaUpstream(proxyAddr, target string) (net.Conn, error) {
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		conn.Close()
		return nil, err
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		conn.Close()
		return nil, err
	}
	if resp[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("upstream proxy requires auth")
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		conn.Close()
		return nil, err
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port&0xff))

	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}

	buf := make([]byte, 10)
	if _, err := io.ReadFull(conn, buf); err != nil {
		conn.Close()
		return nil, err
	}
	if buf[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("upstream connect failed: %d", buf[1])
	}

	return conn, nil
}
