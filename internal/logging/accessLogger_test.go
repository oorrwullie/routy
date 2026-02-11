package logging

import (
	"net/http"
	"testing"
)

func TestGetRequestRemoteAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		realIP     string
		forwarded  string
		want       string
	}{
		{
			name:       "remote addr used",
			remoteAddr: "192.168.1.10:1234",
			want:       "192.168.1.10",
		},
		{
			name:       "x-forwarded-for wins",
			remoteAddr: "192.168.1.10:1234",
			forwarded:  "203.0.113.1, 198.51.100.2",
			want:       "203.0.113.1",
		},
		{
			name:       "x-real-ip used",
			remoteAddr: "192.168.1.10:1234",
			realIP:     "203.0.113.5",
			want:       "203.0.113.5",
		},
		{
			name:       "x-forwarded-for preferred over x-real-ip",
			remoteAddr: "192.168.1.10:1234",
			forwarded:  "203.0.113.9",
			realIP:     "203.0.113.5",
			want:       "203.0.113.9",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{RemoteAddr: tt.remoteAddr, Header: http.Header{}}
			if tt.realIP != "" {
				req.Header.Set("X-Real-Ip", tt.realIP)
			}
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}

			if got := GetRequestRemoteAddress(req); got != tt.want {
				t.Fatalf("GetRequestRemoteAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIPAddrFromRemoteAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "with port",
			addr: "203.0.113.10:443",
			want: "203.0.113.10",
		},
		{
			name: "without port",
			addr: "203.0.113.10",
			want: "203.0.113.10",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ipAddrFromRemoteAddr(tt.addr); got != tt.want {
				t.Fatalf("ipAddrFromRemoteAddr(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}
