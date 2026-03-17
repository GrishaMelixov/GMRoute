package main

import (
	"log"

	"github.com/GrishaMelixov/GMRoute/internal/proxy"
)

func main() {
	server := proxy.NewServer(":1080")
	log.Fatal(server.Start())
}
