package logging

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/oorrwullie/routy/internal/models"
)

func StartAccessLogger(requestChan <-chan *http.Request) error {
	m, err := models.NewModel()
	if err != nil {
		return err
	}

	for request := range requestChan {
		t := time.Now()

		entry := fmt.Sprintf(
			`{"Timestamp": "%s", "IPAddress": "%s", "URL": "%s", "User-Agent": "%s"}
`,
			t.Format("15:04:05 MST 10-02-2006"),
			GetRequestRemoteAddress(request),
			request.URL.String(),
			request.Header.Get("User-Agent"),
		)

		m.WriteToAccessLog(entry)
	}

	return nil
}

func GetRequestRemoteAddress(r *http.Request) string {
	hdr := r.Header
	hdrRealIP := hdr.Get("X-Real-Ip")
	hdrForwardedFor := hdr.Get("X-Forwarded-For")
	if hdrRealIP == "" && hdrForwardedFor == "" {
		return ipAddrFromRemoteAddr(r.RemoteAddr)
	}
	if hdrForwardedFor != "" {
		parts := strings.Split(hdrForwardedFor, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		// TODO: should return first non-local address
		return parts[0]
	}
	return hdrRealIP
}

func ipAddrFromRemoteAddr(s string) string {
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s
	}
	return s[:idx]
}
