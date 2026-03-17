package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/GrishaMelixov/GMRoute/internal/failover"
	"github.com/GrishaMelixov/GMRoute/internal/proxy"
	"github.com/GrishaMelixov/GMRoute/internal/router"
)

func main() {
	r := router.NewRouter(router.RouteDirectly)
	f := failover.New(r, router.RouteDirectly)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := proxy.NewServer(":1080", f)
	if err := server.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
