package handlers

import (
	"crypto/tls"
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
)

type Routy struct {
	hostnames []string
	eventLog  chan logging.EventLogMessage
}

func NewRouty(
	hostnames []string,
	eventLog chan logging.EventLogMessage,
) *Routy {
	return &Routy{
		hostnames: hostnames,
		eventLog:  eventLog,
	}
}

func (r *Routy) Route() error {
	denyList, err := models.GetDenyList()
	if err != nil {
		return err
	}

	accessLog := make(chan *http.Request)
	go func() {
		err := logging.StartAccessLogger(accessLog)
		if err != nil {
			log.Fatal(err)
		}
	}()

	subs, err := models.GetSubdomainRoutes()
	if err != nil {
		return err
	}

	certManager, err := r.getCertManager(subs.Domains)
	if err != nil {
		return err
	}

	go func() {
		httpServer := &http.Server{
			Addr: ":http",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if denyList.IsDenied(logging.GetRequestRemoteAddress(req)) {
					return
				}

				targetURL := "https://" + req.Host + req.URL.Path
				http.Redirect(w, req, targetURL, http.StatusPermanentRedirect)
			}),
		}

		httpServer.ListenAndServe()

	}()

	router := mux.NewRouter()

	for d, sd := range subs.Domains {
		for _, s := range sd {

			targetURL, err := url.Parse(s.Target)
			if err != nil {
				msg := fmt.Sprintf("failed to parse target URL for subdomain %s: %v\n", s.Subdomain, err)
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

			host := fmt.Sprintf("%s.%s", s.Subdomain, d)
			subdomainRouter := router.Host(host).Subrouter()
			subdomainRouter.PathPrefix("/").Handler(http.HandlerFunc(handler))
		}

		server := &http.Server{
			Addr:    ":https",
			Handler: router,
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
			},
		}

		err := server.ListenAndServeTLS("", "")

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Routy) getCertManager(subdomains map[string][]models.SubdomainRoute) (*autocert.Manager, error) {
	var l []string
	for d, sd := range subdomains {
		for _, s := range sd {
			l = append(l, fmt.Sprintf("%s.%s", s.Subdomain, d))
		}
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
