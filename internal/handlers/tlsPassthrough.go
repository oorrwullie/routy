package handlers

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/oorrwullie/routy/internal/logging"
)

const maxTLSClientHelloSize = 64 * 1024

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	if c.reader != nil && c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}
	return c.Conn.Read(p)
}

type hybridTLSListener struct {
	net.Listener
	routy             *Routy
	passthroughRoutes map[string]string
}

func (l *hybridTLSListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		reader := bufio.NewReader(conn)
		sni, err := peekClientHelloSNI(reader)
		if err != nil {
			l.routy.EventLog <- logging.EventLogMessage{
				Level:   "WARN",
				Caller:  "hybridTLSListener.Accept()->peekClientHelloSNI()",
				Message: fmt.Sprintf("could not read TLS SNI from %s: %v", conn.RemoteAddr(), err),
			}
			return &bufferedConn{Conn: conn, reader: reader}, nil
		}

		if target, ok := l.passthroughRoutes[strings.ToLower(sni)]; ok {
			go l.routy.proxyTLS(&bufferedConn{Conn: conn, reader: reader}, sni, target)
			continue
		}

		return &bufferedConn{Conn: conn, reader: reader}, nil
	}
}

func (r *Routy) getPassthroughRouteMap() map[string]string {
	routes := make(map[string]string)
	if r.routes == nil {
		return routes
	}

	for _, route := range r.routes.TLSRoutes {
		host := strings.ToLower(strings.TrimSpace(route.Host))
		target := strings.TrimSpace(route.Target)
		if host == "" || target == "" {
			continue
		}
		routes[host] = normalizeTCPAddress(target, "443")
	}

	return routes
}

func (r *Routy) proxyTLS(client net.Conn, sni, target string) {
	defer func() {
		_ = client.Close()
	}()

	if r.denyList.IsDenied(ipAddrFromNetAddr(client.RemoteAddr())) {
		return
	}

	backend, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		r.EventLog <- logging.EventLogMessage{
			Level:   "ERROR",
			Caller:  "proxyTLS()->net.DialTimeout()",
			Message: fmt.Sprintf("failed to connect TLS passthrough route %s -> %s: %v", sni, target, err),
		}
		return
	}
	defer func() {
		_ = backend.Close()
	}()

	r.EventLog <- logging.EventLogMessage{
		Level:   "INFO",
		Caller:  "proxyTLS()",
		Message: fmt.Sprintf("TLS passthrough %s -> %s", sni, target),
	}

	done := make(chan struct{}, 2)
	go copyAndClose(backend, client, done)
	go copyAndClose(client, backend, done)
	<-done
}

func copyAndClose(dst net.Conn, src net.Conn, done chan<- struct{}) {
	_, _ = io.Copy(dst, src)
	_ = dst.Close()
	_ = src.Close()
	done <- struct{}{}
}

func ipAddrFromNetAddr(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func normalizeTCPAddress(address, defaultPort string) string {
	address = strings.TrimSpace(address)
	address = strings.TrimPrefix(address, "tls://")
	address = strings.TrimPrefix(address, "tcp://")

	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}
	return net.JoinHostPort(address, defaultPort)
}

func peekClientHelloSNI(reader *bufio.Reader) (string, error) {
	header, err := reader.Peek(5)
	if err != nil {
		return "", err
	}
	if header[0] != 0x16 {
		return "", fmt.Errorf("not a TLS handshake record")
	}

	recordLen := int(binary.BigEndian.Uint16(header[3:5]))
	if recordLen <= 0 || recordLen > maxTLSClientHelloSize {
		return "", fmt.Errorf("invalid TLS record length %d", recordLen)
	}

	data, err := reader.Peek(5 + recordLen)
	if err != nil {
		return "", err
	}

	return parseClientHelloSNI(data[5:])
}

func parseClientHelloSNI(handshake []byte) (string, error) {
	if len(handshake) < 4 || handshake[0] != 0x01 {
		return "", fmt.Errorf("not a ClientHello")
	}

	handshakeLen := int(handshake[1])<<16 | int(handshake[2])<<8 | int(handshake[3])
	if handshakeLen+4 > len(handshake) {
		return "", fmt.Errorf("incomplete ClientHello")
	}

	body := handshake[4 : 4+handshakeLen]
	if len(body) < 34 {
		return "", fmt.Errorf("ClientHello too short")
	}

	pos := 34 // protocol version + random
	if pos >= len(body) {
		return "", fmt.Errorf("missing session id")
	}
	sessionIDLen := int(body[pos])
	pos++
	pos += sessionIDLen
	if pos+2 > len(body) {
		return "", fmt.Errorf("missing cipher suites")
	}

	cipherSuitesLen := int(binary.BigEndian.Uint16(body[pos : pos+2]))
	pos += 2 + cipherSuitesLen
	if pos >= len(body) {
		return "", fmt.Errorf("missing compression methods")
	}

	compressionMethodsLen := int(body[pos])
	pos++
	pos += compressionMethodsLen
	if pos+2 > len(body) {
		return "", fmt.Errorf("missing extensions")
	}

	extensionsLen := int(binary.BigEndian.Uint16(body[pos : pos+2]))
	pos += 2
	if pos+extensionsLen > len(body) {
		return "", fmt.Errorf("incomplete extensions")
	}

	extensions := body[pos : pos+extensionsLen]
	for len(extensions) >= 4 {
		extensionType := binary.BigEndian.Uint16(extensions[0:2])
		extensionLen := int(binary.BigEndian.Uint16(extensions[2:4]))
		extensions = extensions[4:]
		if extensionLen > len(extensions) {
			return "", fmt.Errorf("incomplete extension")
		}

		if extensionType == 0x0000 {
			return parseServerNameExtension(extensions[:extensionLen])
		}

		extensions = extensions[extensionLen:]
	}

	return "", fmt.Errorf("SNI not found")
}

func parseServerNameExtension(data []byte) (string, error) {
	if len(data) < 2 {
		return "", fmt.Errorf("server name extension too short")
	}

	listLen := int(binary.BigEndian.Uint16(data[0:2]))
	data = data[2:]
	if listLen > len(data) {
		return "", fmt.Errorf("server name list incomplete")
	}
	data = data[:listLen]

	for len(data) >= 3 {
		nameType := data[0]
		nameLen := int(binary.BigEndian.Uint16(data[1:3]))
		data = data[3:]
		if nameLen > len(data) {
			return "", fmt.Errorf("server name incomplete")
		}
		if nameType == 0 {
			serverName := strings.ToLower(string(data[:nameLen]))
			if serverName == "" {
				return "", fmt.Errorf("empty server name")
			}
			return serverName, nil
		}
		data = data[nameLen:]
	}

	return "", fmt.Errorf("host_name SNI not found")
}

func newHybridTLSListener(listener net.Listener, routy *Routy) net.Listener {
	return &hybridTLSListener{
		Listener:          listener,
		routy:             routy,
		passthroughRoutes: routy.getPassthroughRouteMap(),
	}
}
