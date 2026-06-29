package handlers

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"net"
	"testing"
	"time"
)

func TestParseClientHelloSNI(t *testing.T) {
	t.Parallel()

	serverName := "vault.example.com"
	clientHello, err := tlsClientHelloBytes(serverName)
	if err != nil {
		t.Fatalf("build ClientHello: %v", err)
	}

	got, err := peekClientHelloSNI(bufio.NewReader(bytes.NewReader(clientHello)))
	if err != nil {
		t.Fatalf("peekClientHelloSNI returned error: %v", err)
	}
	if got != serverName {
		t.Fatalf("SNI = %q, want %q", got, serverName)
	}
}

func TestNormalizeTCPAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "10.0.0.20", want: "10.0.0.20:443"},
		{in: "10.0.0.20:8443", want: "10.0.0.20:8443"},
		{in: "tls://vault.internal:443", want: "vault.internal:443"},
		{in: "tcp://vault.internal:8443", want: "vault.internal:8443"},
	}

	for _, tt := range tests {
		if got := normalizeTCPAddress(tt.in, "443"); got != tt.want {
			t.Fatalf("normalizeTCPAddress(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func tlsClientHelloBytes(serverName string) ([]byte, error) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	errCh := make(chan error, 1)
	go func() {
		conn := tls.Client(clientConn, &tls.Config{ServerName: serverName, InsecureSkipVerify: true})
		errCh <- conn.Handshake()
	}()

	buf := make([]byte, maxTLSClientHelloSize)
	_ = serverConn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := serverConn.Read(buf)
	if err != nil {
		return nil, err
	}

	_ = serverConn.Close()
	<-errCh
	return buf[:n], nil
}
