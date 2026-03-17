package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/GrishaMelixov/GMRoute/internal/failover"
	"github.com/GrishaMelixov/GMRoute/internal/metrics"
	"github.com/GrishaMelixov/GMRoute/internal/proxy"
	"github.com/GrishaMelixov/GMRoute/internal/router"
)

func main() {
	r := router.NewRouter(router.RouteDirectly)
	f := failover.New(r, router.RouteDirectly)

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

	server := proxy.NewServer(":1080", f)
	if err := server.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
