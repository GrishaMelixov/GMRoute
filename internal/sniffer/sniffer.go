package sniffer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
)

const minTLSRecordSize = 5

type PeekConn struct {
	net.Conn
	buf io.Reader
}

func (c *PeekConn) Read(b []byte) (int, error) {
	return c.buf.Read(b)
}

func SniffSNI(conn net.Conn) (sni string, peeked net.Conn, err error) {
	header := make([]byte, minTLSRecordSize)
	if _, err = io.ReadFull(conn, header); err != nil {
		return
	}

	if header[0] != 0x16 {
		err = errors.New("not a TLS handshake")
		peeked = newPeekConn(conn, header)
		return
	}

	recordLen := int(binary.BigEndian.Uint16(header[3:5]))

	body := make([]byte, recordLen)
	if _, err = io.ReadFull(conn, body); err != nil {
		return
	}

	all := append(header, body...)
	peeked = newPeekConn(conn, all)
	sni = extractSNI(body)
	return
}

func newPeekConn(conn net.Conn, buf []byte) *PeekConn {
	return &PeekConn{
		Conn: conn,
		buf:  io.MultiReader(bytes.NewReader(buf), conn),
	}
}

func extractSNI(data []byte) string {
	if len(data) < 38 {
		return ""
	}
	pos := 38

	if pos+1 > len(data) {
		return ""
	}
	pos += 1 + int(data[pos])

	if pos+2 > len(data) {
		return ""
	}
	pos += 2 + int(binary.BigEndian.Uint16(data[pos:pos+2]))

	if pos+1 > len(data) {
		return ""
	}
	pos += 1 + int(data[pos])

	if pos+2 > len(data) {
		return ""
	}
	extEnd := pos + 2 + int(binary.BigEndian.Uint16(data[pos:pos+2]))
	pos += 2

	if extEnd > len(data) {
		return ""
	}

	for pos+4 <= extEnd {
		extType := binary.BigEndian.Uint16(data[pos : pos+2])
		extLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4

		if pos+extLen > extEnd {
			break
		}

		if extType == 0x0000 {
			if extLen < 5 {
				break
			}
			nameLen := int(binary.BigEndian.Uint16(data[pos+3 : pos+5]))
			if pos+5+nameLen > extEnd {
				break
			}
			return string(data[pos+5 : pos+5+nameLen])
		}

		pos += extLen
	}

	return ""
}
