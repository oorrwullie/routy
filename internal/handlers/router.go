package handlers

import (
	"context"
	"fmt"
	"net"
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

	r.collectHostnames()

	certManager, err := r.getCertManager()
	if err != nil {
		return err
	}

	r.registerHTTPRoutes(router)

	// Listen for plain HTTP traffic and redirect/challenge through autocert.
	g.Go(func() error {
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
			return err
		}
		return nil
	})

	tcpListener, err := net.Listen("tcp", ":https")
	if err != nil {
		return err
	}

	tlsListener := newHybridTLSListener(tcpListener, r)

	server := &http.Server{
		Handler:   router,
		TLSConfig: certManager.TLSConfig(),
	}

	// ServeTLS receives only the connections that are not selected for raw TLS
	// passthrough by the hybrid listener.
	g.Go(func() error {
		return server.ServeTLS(tlsListener, "", "")
	})

	return g.Wait()
}

func (r *Routy) collectHostnames() {
	r.hostnames = nil
	seen := make(map[string]struct{})

	for _, d := range r.routes.Domains {
		if len(d.Paths) != 0 {
			appendHostname(&r.hostnames, seen, d.Name)
		}
		for _, sd := range d.Subdomains {
			appendHostname(&r.hostnames, seen, fmt.Sprintf("%s.%s", sd.Name, d.Name))
		}
	}
}

func appendHostname(hostnames *[]string, seen map[string]struct{}, hostname string) {
	if hostname == "" {
		return
	}
	if _, ok := seen[hostname]; ok {
		return
	}
	seen[hostname] = struct{}{}
	*hostnames = append(*hostnames, hostname)
}

func (r *Routy) registerHTTPRoutes(router *mux.Router) {
	for _, domain := range r.routes.Domains {
		if len(domain.Paths) != 0 {
			sd := models.Subdomain{
				Name:  domain.Name,
				Paths: domain.Paths,
			}

			domain.Subdomains = append(domain.Subdomains, sd)
		}

		for _, sd := range domain.Subdomains {
			for _, path := range sd.Paths {
				if path.Upgrade {
					go r.handleWebSocket(path)
				} else {
					r.handleHttp(router, domain, sd, path)
				}
			}
		}
	}
}
