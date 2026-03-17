package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/GrishaMelixov/GMRoute/internal/failover"
	"github.com/GrishaMelixov/GMRoute/internal/metrics"
	"github.com/GrishaMelixov/GMRoute/internal/router"
	"github.com/GrishaMelixov/GMRoute/internal/sniffer"
)

var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 32*1024)
		return &buf
	},
}

const (
	socks5Version = 0x05
	noAuth        = 0x00
	noAcceptable  = 0xFF
	cmdConnect    = 0x01

	atypIPv4   = 0x01
	atypDomain = 0x03
	atypIPv6   = 0x04
)

func handleConn(conn net.Conn, f *failover.Failover) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(dialTimeout))

	if err := handshake(conn); err != nil {
		return
	}
	conn.SetDeadline(time.Time{})

	target, host, err := readRequest(conn)
	if err != nil {
		return
	}

	routingHost := host
	clientConn := net.Conn(conn)
	if net.ParseIP(host) != nil {
		sniHost, peeked, _ := sniffer.SniffSNI(conn)
		clientConn = peeked
		if sniHost != "" {
			routingHost = sniHost
		}
	}

	route := f.ResolvedRoute(routingHost)
	upstream := route.Type == router.Upstream
	metrics.Global.ConnOpened(upstream)
	defer metrics.Global.ConnClosed()

	targetConn, err := f.Dial(routingHost, target)
	if err != nil {
		metrics.Global.Error()
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer targetConn.Close()

	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	tunnel(clientConn, targetConn)
}

func handshake(conn net.Conn) error {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}

	if buf[0] != socks5Version {
		return fmt.Errorf("unsupported socks version: %d", buf[0])
	}

	methods := make([]byte, buf[1])
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}

	for _, m := range methods {
		if m == noAuth {
			conn.Write([]byte{socks5Version, noAuth})
			return nil
		}
	}

	conn.Write([]byte{socks5Version, noAcceptable})
	return fmt.Errorf("no acceptable auth method")
}

func readRequest(conn net.Conn) (target, host string, err error) {
	buf := make([]byte, 4)
	if _, err = io.ReadFull(conn, buf); err != nil {
		return
	}

	if buf[0] != socks5Version {
		err = fmt.Errorf("unsupported socks version: %d", buf[0])
		return
	}

	if buf[1] != cmdConnect {
		err = fmt.Errorf("unsupported command: %d", buf[1])
		return
	}

	switch buf[3] {
	case atypIPv4:
		addr := make([]byte, 4)
		if _, err = io.ReadFull(conn, addr); err != nil {
			return
		}
		host = net.IP(addr).String()

	case atypDomain:
		lenBuf := make([]byte, 1)
		if _, err = io.ReadFull(conn, lenBuf); err != nil {
			return
		}
		domain := make([]byte, lenBuf[0])
		if _, err = io.ReadFull(conn, domain); err != nil {
			return
		}
		host = string(domain)

	case atypIPv6:
		addr := make([]byte, 16)
		if _, err = io.ReadFull(conn, addr); err != nil {
			return
		}
		host = net.IP(addr).String()

	default:
		err = fmt.Errorf("unsupported address type: %d", buf[3])
		return
	}

	portBuf := make([]byte, 2)
	if _, err = io.ReadFull(conn, portBuf); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(portBuf)

	target = fmt.Sprintf("%s:%d", host, port)
	return
}

func tunnel(client, target net.Conn) {
	done := make(chan struct{}, 2)

	copy := func(dst, src net.Conn) {
		bufp := bufPool.Get().(*[]byte)
		defer bufPool.Put(bufp)
		io.CopyBuffer(dst, src, *bufp)
		done <- struct{}{}
	}

	go copy(target, client)
	go copy(client, target)

	<-done
}
