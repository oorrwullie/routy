package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/oorrwullie/routy/internal/logging"
)

type resolver struct {
	mu sync.Mutex
	m  map[string]string
}

func (res *resolver) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	res.mu.Lock()
	defer res.mu.Unlock()

	host, _, _ := net.SplitHostPort(address)

	if resolvedAddr, ok := res.m[host]; ok {
		return net.Dial(network, resolvedAddr)
	}
	return net.Dial(network, address)
}

func (r *Routy) getDnsResolver() *http.Transport {
	m := make(map[string]string)

	for _, domain := range r.routes.Domains {
		if len(domain.Paths) != 0 {
			for _, path := range domain.Paths {
				targetURL, err := url.Parse(path.Target)
				if err != nil {
					msg := fmt.Sprintf("failed to parse target URL for domain %s path %s: %v\n", domain.Name, path.Location, err)
					r.EventLog <- logging.EventLogMessage{
						Level:   "ERROR",
						Caller:  "Route()->url.Parse()",
						Message: msg,
					}

					continue
				}

				m[domain.Name] = targetURL.Host

				break
			}
		}

		for _, sd := range domain.Subdomains {
			if len(sd.Paths) != 0 {
				for _, path := range sd.Paths {
					targetURL, err := url.Parse(path.Target)
					if err != nil {
						msg := fmt.Sprintf("failed to parse target URL for subdomain %s path %s: %v\n", sd.Name, path.Location, err)
						r.EventLog <- logging.EventLogMessage{
							Level:   "ERROR",
							Caller:  "Route()->url.Parse()",
							Message: msg,
						}

						continue
					}

					dName := fmt.Sprintf("%s.%s", sd.Name, domain.Name)
					m[dName] = targetURL.Host

					break
				}
			}
		}
	}

	t := &http.Transport{
		DialContext: (&resolver{m: m}).DialContext,
	}

	return t
}
