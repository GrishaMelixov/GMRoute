package main

import (
	"log"

	"github.com/GrishaMelixov/GMRoute/internal/failover"
	"github.com/GrishaMelixov/GMRoute/internal/proxy"
	"github.com/GrishaMelixov/GMRoute/internal/router"
)

func main() {
	r := router.NewRouter(router.RouteDirectly)

	// r.AddRule("youtube.com", router.NewUpstreamRoute("127.0.0.1:7890"))

	f := failover.New(r, router.RouteDirectly)

	server := proxy.NewServer(":1080", f)
	log.Fatal(server.Start())
}
