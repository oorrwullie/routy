package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"

	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

// Routy is the main struct for the router
type Routy struct {
	accessLog chan *http.Request
	denyList  *models.DenyList
	EventLog  chan logging.EventLogMessage
	hostnames []string
	routes    *models.Routes
}

// NewRouty creates a new instance of the Routy struct
func NewRouty() (*Routy, error) {
	routes, err := models.GetDomainRoutes()
	if err != nil {
		return nil, err
	}

	denyList, err := models.GetDenyList()
	if err != nil {
		return nil, err
	}

	accessLog := make(chan *http.Request)

	// start the access logger
	go func() {
		err := logging.StartAccessLogger(accessLog)
		if err != nil {
			return
		}
	}()

	eventLog := make(chan logging.EventLogMessage)

	// start the event logger
	go func() {
		err := logging.StartEventLogger(eventLog)
		if err != nil {
			return
		}
	}()

	return &Routy{
		EventLog:  eventLog,
		accessLog: accessLog,
		denyList:  denyList,
		routes:    routes,
	}, nil
}

// Route starts the routing process
func (r *Routy) Route() error {
	router := mux.NewRouter()

	g, _ := errgroup.WithContext(context.Background())

	for _, d := range r.routes.Domains {
		if len(d.Paths) != 0 {
			r.hostnames = append(r.hostnames, d.Name)
		}
		for _, sd := range d.Subdomains {
			r.hostnames = append(r.hostnames, fmt.Sprintf("%s.%s", sd.Name, d.Name))
		}
	}

	certManager, err := r.getCertManager()
	if err != nil {
		return err
	}

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
					if path.Upgrade {
						go r.handleWebSocket(path)
					} else {
						go r.handleHttp(router, domain, sd, path)
					}
				}
			}
		}

		// listens fo any traffic on http and redirects it to https
		go func() {
			httpServer := &http.Server{
				Addr:    ":http",
				Handler: certManager.HTTPHandler(nil),
			}

			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				r.EventLog <- logging.EventLogMessage{
					Level:   "ERROR",
					Caller:  "Route()->httpServer.ListenAndServe()",
					Message: fmt.Sprintf("failed to start http server: %v", err),
				}
			}
		}()

		server := &http.Server{
			Addr:      ":https",
			Handler:   router,
			TLSConfig: certManager.TLSConfig(),
		}

		// start the https server
		g.Go(func() error {
			return server.ListenAndServeTLS("", "")
		})

	}

	return g.Wait()
}
