package handlers

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"
)

func (r *Routy) getDnsResolver() *net.Resolver {
	var resolverMap map[string]string

	for _, domain := range r.routes.Domains {
		if len(domain.Paths) != 0 {
			sd := models.Subdomain{
				Name:  domain.Name,
				Paths: domain.Paths,
			}

			domain.Subdomains = append(domain.Subdomains, sd)
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

					resolverMap[targetURL.Host] = targetURL.Host
				}
			}
		}
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, ok := resolverMap[address]
			if !ok {
				return net.Dial(network, address)
			}

			return net.Dial(network, host)
		},
	}

	return resolver
}
