package proxy

import (
	"log"
	"net"

	"github.com/GrishaMelixov/GMRoute/internal/failover"
)

type Server struct {
	addr     string
	listener net.Listener
	failover *failover.Failover
}

func NewServer(addr string, f *failover.Failover) *Server {
	return &Server{addr: addr, failover: f}
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
		go handleConn(conn, s.failover)
	}
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}
