package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/GrishaMelixov/GMRoute/internal/config"
	"github.com/GrishaMelixov/GMRoute/internal/failover"
	"github.com/GrishaMelixov/GMRoute/internal/metrics"
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

	http.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		m := metrics.Global
		json.NewEncoder(w).Encode(map[string]int64{
			"active_conns":  m.ActiveConns.Load(),
			"total_conns":   m.TotalConns.Load(),
			"direct_conns":  m.DirectConns.Load(),
			"upstream_conn": m.UpstreamConn.Load(),
			"errors":        m.Errors.Load(),
		})
	})
	go http.ListenAndServe(":9090", nil)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	addr := fmt.Sprintf(":%d", cfg.Port)
	server := proxy.NewServer(addr, f)
	log.Printf("config loaded: port=%d upstream=%q rules=%d", cfg.Port, cfg.Upstream, len(cfg.Rules))
	if err := server.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
