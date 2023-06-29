package handlers

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"

	"golang.org/x/crypto/acme/autocert"
)

type Routy struct {
	port     string
	hostname string
	eventLog chan logging.EventLogMessage
}

func NewRouty(
	port string,
	hostname string,
	eventLog chan logging.EventLogMessage,
) *Routy {
	return &Routy{
		port:     port,
		hostname: hostname,
		eventLog: eventLog,
	}
}

func (r *Routy) Route() error {
	denyList, err := models.GetDenyList()

	accessLog := make(chan *http.Request)
	go logging.AccessLogger(accessLog)

	subs, err := models.GetSubdomainRoutes()
	if err != nil {
		return err
	}

	list := r.buildAllowList(subs)

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(list),
		Cache:      autocert.DirCache("./certs"),
	}

	go func() {
		httpServer := &http.Server{
			Addr: ":http",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetURL := "https://" + r.Host + r.URL.Path
				http.Redirect(w, r, targetURL, http.StatusPermanentRedirect)
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
			GetCertificate: m.GetCertificate,
		},
	}

	return server.ListenAndServeTLS("", "")
}

func (r *Routy) buildAllowList(subdomains []models.SubdomainRoute) string {
	var l []string
	for _, s := range subdomains {
		l = append(l, fmt.Sprintf("%s.%s", s.Subdomain, r.hostname))
	}

	return strings.Join(l[:], ",")
}
