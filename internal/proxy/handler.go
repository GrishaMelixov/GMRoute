package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/GrishaMelixov/GMRoute/internal/router"
	"github.com/GrishaMelixov/GMRoute/internal/sniffer"
)

const (
	socks5Version = 0x05
	noAuth        = 0x00
	noAcceptable  = 0xFF
	cmdConnect    = 0x01

	atypIPv4   = 0x01
	atypDomain = 0x03
	atypIPv6   = 0x04
)

func handleConn(conn net.Conn, r *router.Router) {
	defer conn.Close()

	if err := handshake(conn); err != nil {
		return
	}

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

	route := r.Resolve(routingHost)

	var targetConn net.Conn
	switch route.Type {
	case router.Direct:
		targetConn, err = net.Dial("tcp", target)
	case router.Upstream:
		targetConn, err = dialViaUpstream(route.ProxyAddr, target)
	}

	if err != nil {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer targetConn.Close()

	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	tunnel(clientConn, targetConn)
}

func dialViaUpstream(proxyAddr, target string) (net.Conn, error) {
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Write([]byte{0x05, 0x01, noAuth}); err != nil {
		conn.Close()
		return nil, err
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		conn.Close()
		return nil, err
	}
	if resp[1] != noAuth {
		conn.Close()
		return nil, fmt.Errorf("upstream proxy requires auth")
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		conn.Close()
		return nil, err
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	req := []byte{0x05, cmdConnect, 0x00, atypDomain, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port&0xff))

	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}

	buf := make([]byte, 10)
	if _, err := io.ReadFull(conn, buf); err != nil {
		conn.Close()
		return nil, err
	}
	if buf[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("upstream proxy connect failed: %d", buf[1])
	}

	return conn, nil
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

	go func() {
		io.Copy(target, client)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(client, target)
		done <- struct{}{}
	}()

	<-done
}
