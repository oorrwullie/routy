package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/sync/errgroup"
)

type Routy struct {
	hostnames []string
	eventLog  chan logging.EventLogMessage
}

func NewRouty(
	eventLog chan logging.EventLogMessage,
) *Routy {
	return &Routy{
		eventLog: eventLog,
	}
}

func (r *Routy) Route() error {
	denyList, err := models.GetDenyList()
	if err != nil {
		return err
	}

	// listens for any traffic on ws and redirects it to wss
	go func() {
		httpServer := &http.Server{
			Addr: ":ws",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if denyList.IsDenied(logging.GetRequestRemoteAddress(req)) {
					return
				}

				targetURL := "wss://" + req.Host + req.URL.Path
				http.Redirect(w, req, targetURL, http.StatusPermanentRedirect)
			}),
		}

		httpServer.ListenAndServe()

	}()

	accessLog := make(chan *http.Request)
	go func() {
		err := logging.StartAccessLogger(accessLog)
		if err != nil {
			log.Fatal(err)
		}
	}()

	router := mux.NewRouter()

	g, _ := errgroup.WithContext(context.Background())

	routes, err := models.GetDomainRoutes()
	if err != nil {
		return err
	}

	for _, domain := range routes.Domains {
		certManager, err := r.getCertManager(domain)
		if err != nil {
			return err
		}

		if domain.Target != "" {
			sd := models.Subdomain{
				Name:   domain.Name,
				Target: domain.Target,
			}

			domain.Subdomains = append(domain.Subdomains, sd)
		}

		for _, sd := range domain.Subdomains {

			targetURL, err := url.Parse(sd.Target)
			if err != nil {
				msg := fmt.Sprintf("failed to parse target URL for subdomain %s: %v\n", sd.Name, err)
				r.eventLog <- logging.EventLogMessage{
					Level:   "ERROR",
					Caller:  "Route()->url.Parse()",
					Message: msg,
				}

				continue
			}

			proxy := httputil.NewSingleHostReverseProxy(targetURL)

			handler := func(w http.ResponseWriter, req *http.Request) {
				if denyList.IsDenied(logging.GetRequestRemoteAddress(req)) {
					return
				}

				accessLog <- req

				req.Host = req.URL.Host
				proxy.ServeHTTP(w, req)
			}

			var host string
			if sd.Name == domain.Name {
				host = domain.Name
			} else {
				host = fmt.Sprintf("%s.%s", sd.Name, domain.Name)
			}

			subdomainRouter := router.Host(host).Subrouter()
			subdomainRouter.PathPrefix("/").Handler(http.HandlerFunc(handler))
		}

		// listens fo any traffic on http and redirects it to https
		go func() {
			httpServer := &http.Server{
				Addr:    ":http",
				Handler: certManager.HTTPHandler(nil),
			}

			httpServer.ListenAndServe()

		}()

		server := &http.Server{
			Addr:      ":https",
			Handler:   router,
			TLSConfig: certManager.TLSConfig(),
		}

		g.Go(func() error {
			return server.ListenAndServeTLS("", "")
		})

	}

	return g.Wait()
}

func (r *Routy) getCertManager(domain models.Domain) (*autocert.Manager, error) {
	var l []string

	if domain.Target != "" {
		l = append(l, domain.Name)
	}

	for _, sd := range domain.Subdomains {
		l = append(l, fmt.Sprintf("%s.%s", sd.Name, domain.Name))
	}

	list := strings.Join(l[:], ",")

	model, err := models.NewModel()
	if err != nil {
		return nil, err
	}

	certDir, err := model.GetFilepath("certs")
	if err != nil {
		return nil, err
	}

	manager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(list),
		Cache:      autocert.DirCache(certDir),
	}

	return manager, nil
}
