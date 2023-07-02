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
	hostname string
	eventLog chan logging.EventLogMessage
}

func NewRouty(
	hostname string,
	eventLog chan logging.EventLogMessage,
) *Routy {
	return &Routy{
		hostname: hostname,
		eventLog: eventLog,
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

	certManager, err := r.getCertManager(subs)
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

	for _, s := range subs {
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

		host := fmt.Sprintf("%s.%s", s.Subdomain, r.hostname)
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

	return server.ListenAndServeTLS("", "")
}

func (r *Routy) getCertManager(subdomains []models.SubdomainRoute) (*autocert.Manager, error) {
	var l []string
	for _, s := range subdomains {
		l = append(l, fmt.Sprintf("%s.%s", s.Subdomain, r.hostname))
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
