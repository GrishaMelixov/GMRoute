package proxy

import (
	"log"
	"net"

	"github.com/GrishaMelixov/GMRoute/internal/router"
)

type Server struct {
	addr     string
	listener net.Listener
	router   *router.Router
}

func NewServer(addr string, r *router.Router) *Server {
	return &Server{addr: addr, router: r}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	log.Printf("proxy listening on %s", s.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleConn(conn, s.router)
	}
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}
