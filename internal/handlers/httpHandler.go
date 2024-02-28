package handlers

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"
)

func (r *Routy) handleHttp(router *mux.Router, domain models.Domain, sd models.Subdomain, path models.Path) {
	targetURL, err := url.Parse(path.Target)
	if err != nil {
		msg := fmt.Sprintf("failed to parse target URL for subdomain %s path %s: %v\n", sd.Name, path.Location, err)
		r.EventLog <- logging.EventLogMessage{
			Level:   "ERROR",
			Caller:  "Route()->url.Parse()",
			Message: msg,
		}

		return
	}

	h, port, _ := net.SplitHostPort(targetURL.Host)

	if sd.Name == domain.Name {
		h = domain.Name
	} else {
		h = fmt.Sprintf("%s.%s", sd.Name, domain.Name)
	}

	targetURL.Host = net.JoinHostPort(h, port)

	proxy := &httputil.ReverseProxy{
		Rewrite: func(req *httputil.ProxyRequest) {
			req.Out.Header["X-Forwarded"] = req.In.Header["X-Forwarded"]
			req.Out.Header["X-Forwarded-For"] = req.In.Header["X-Forwarded-For"]
			req.Out.Header["X-Forwarded-Host"] = req.In.Header["X-Forwarded-Host"]
			req.Out.Header["X-Forwarded-Proto"] = req.In.Header["X-Forwarded-Proto"]
			req.SetXForwarded()
			req.SetURL(targetURL)
			host, _, _ := net.SplitHostPort(targetURL.Host)
			req.Out.Host = host
		},
		Transport: r.getDnsResolver(),
	}

	var host string

	if sd.Name == domain.Name {
		host = domain.Name
	} else {
		host = fmt.Sprintf("%s.%s", sd.Name, domain.Name)
	}

	subdomainRouter := router.Host(host).Subrouter()
	subdomainRouter.PathPrefix(path.Location).HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			if r.denyList.IsDenied(logging.GetRequestRemoteAddress(req)) {
				return
			}

			r.accessLog <- req
			proxy.ServeHTTP(w, req)
		},
	)
}
