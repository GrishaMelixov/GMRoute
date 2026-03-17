package proxy

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/GrishaMelixov/GMRoute/internal/failover"
)

const dialTimeout = 10 * time.Second

type Server struct {
	addr     string
	listener net.Listener
	failover *failover.Failover
	wg       sync.WaitGroup
}

func NewServer(addr string, f *failover.Failover) *Server {
	return &Server{addr: addr, failover: f}
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	log.Printf("proxy listening on %s", s.addr)

	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				s.wg.Wait()
				return nil
			default:
				return err
			}
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			handleConn(conn, s.failover)
		}()
	}
}
