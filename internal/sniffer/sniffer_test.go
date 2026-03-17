package sniffer

import (
	"net"
	"testing"
)

// buildClientHello — собирает минимальный TLS ClientHello с указанным SNI.
// Это реальная структура по RFC 5246, используется только в тестах.
func buildClientHello(serverName string) []byte {
	name := []byte(serverName)
	nameLen := len(name)

	// SNI extension data: list_len(2) + type(1) + name_len(2) + name
	sniData := []byte{
		0x00, byte(nameLen + 3), // server name list length
		0x00,                    // name type: host_name
		0x00, byte(nameLen),     // name length
	}
	sniData = append(sniData, name...)

	// Extension entry: type(2) + data_len(2) + data
	ext := []byte{0x00, 0x00} // extension type: SNI
	ext = append(ext, 0x00, byte(len(sniData)))
	ext = append(ext, sniData...)

	// Extensions block: len(2) + extensions
	extBlock := []byte{0x00, byte(len(ext))}
	extBlock = append(extBlock, ext...)

	// ClientHello body (после type+length handshake header):
	// version(2) + random(32) + session_id_len(1) + cipher_suites_len(2) +
	// cipher_suite(2) + compression_len(1) + compression(1) + extensions
	body := []byte{
		0x03, 0x03,             // TLS 1.2
	}
	body = append(body, make([]byte, 32)...) // random (32 нулей)
	body = append(body,
		0x00,       // session ID len = 0
		0x00, 0x02, // cipher suites len = 2
		0x00, 0x2F, // TLS_RSA_WITH_AES_128_CBC_SHA
		0x01,       // compression methods len = 1
		0x00,       // null compression
	)
	body = append(body, extBlock...)

	// Handshake header: type(1) + length(3)
	handshake := []byte{0x01, 0x00, 0x00, byte(len(body))}
	handshake = append(handshake, body...)

	// TLS record header: content_type(1) + version(2) + length(2)
	record := []byte{
		0x16,                        // content type: Handshake
		0x03, 0x01,                  // TLS 1.0
		0x00, byte(len(handshake)),  // record length
	}
	record = append(record, handshake...)
	return record
}

func TestSniffSNI_ExtractsDomain(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		client.Write(buildClientHello("example.com"))
	}()

	sni, peeked, err := SniffSNI(server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sni != "example.com" {
		t.Errorf("expected SNI example.com, got %q", sni)
	}
	if peeked == nil {
		t.Error("expected peeked conn to be non-nil")
	}
}

func TestSniffSNI_PeekConnReplaysBytess(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	original := buildClientHello("test.io")

	go func() {
		client.Write(original)
		client.Close()
	}()

	_, peeked, err := SniffSNI(server)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Читаем всё из peeked — должны получить оригинальные байты целиком
	buf := make([]byte, len(original))
	n, _ := readFull(peeked, buf)
	if n != len(original) {
		t.Errorf("peeked conn: expected %d bytes replayed, got %d", len(original), n)
	}
}

func TestSniffSNI_NotTLS(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		client.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	}()

	sni, peeked, err := SniffSNI(server)
	if err == nil {
		t.Error("expected error for non-TLS traffic")
	}
	if sni != "" {
		t.Errorf("expected empty SNI for non-TLS, got %q", sni)
	}
	if peeked == nil {
		t.Error("expected peeked conn even on error")
	}
}

// readFull — читает из conn пока не заполнит buf или не получит EOF.
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
