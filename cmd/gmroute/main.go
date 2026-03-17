package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/GrishaMelixov/GMRoute/internal/config"
	"github.com/GrishaMelixov/GMRoute/internal/dashboard"
	"github.com/GrishaMelixov/GMRoute/internal/failover"
	"github.com/GrishaMelixov/GMRoute/internal/geo"
	"github.com/GrishaMelixov/GMRoute/internal/proxy"
	"github.com/GrishaMelixov/GMRoute/internal/router"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	r := router.NewRouter(router.RouteDirectly)
	for _, rule := range cfg.Rules {
		switch rule.Route {
		case "upstream":
			if cfg.Upstream == "" {
				log.Printf("warning: rule for %s uses upstream but upstream is not set", rule.Domain)
				continue
			}
			r.AddRule(rule.Domain, router.NewUpstreamRoute(cfg.Upstream))
		case "direct":
			r.AddRule(rule.Domain, router.RouteDirectly)
		}
	}

	var fallbackRoute router.Route
	if cfg.Upstream != "" {
		fallbackRoute = router.NewUpstreamRoute(cfg.Upstream)
	} else {
		fallbackRoute = router.RouteDirectly
	}

	f := failover.New(r, fallbackRoute)

	// look up our own location for globe arcs
	if loc, err := geo.LookupSelf(); err == nil {
		proxy.SetSrcLocation(loc.Lat, loc.Lng)
		log.Printf("source location: %.2f, %.2f (%s)", loc.Lat, loc.Lng, loc.Country)
	}

	mux := http.NewServeMux()
	dash := dashboard.New(r, f, cfg, *configPath)
	dash.Register(mux)
	go func() {
		log.Printf("dashboard: http://localhost:9090")
		http.ListenAndServe(":9090", mux)
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	addr := fmt.Sprintf(":%d", cfg.Port)
	server := proxy.NewServer(addr, f)
	log.Printf("proxy listening on %s | upstream=%q | rules=%d", addr, cfg.Upstream, len(cfg.Rules))
	if err := server.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
