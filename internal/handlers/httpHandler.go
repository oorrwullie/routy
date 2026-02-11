package handlers

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

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
		ModifyResponse: func(resp *http.Response) error {
			if sd.CORS != nil && resp != nil && resp.Request != nil {
				enforceCORSHeaders(resp.Header, resp.Request, sd.CORS)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			if sd.CORS != nil {
				enforceCORSHeaders(w.Header(), req, sd.CORS)
			}
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
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

			if sd.CORS != nil {
				if req.Method == http.MethodOptions {
					enforceCORSHeaders(w.Header(), req, sd.CORS)
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			proxy.ServeHTTP(w, req)
		},
	)
}

func enforceCORSHeaders(header http.Header, req *http.Request, cfg *models.CORSConfig) {
	if cfg == nil {
		return
	}

	clearCORSHeaders(header)

	origin := req.Header.Get("Origin")
	allowOrigin, vary := corsAllowedOrigin(origin, cfg.AllowOrigins, cfg.AllowCredentials)
	if allowOrigin != "" {
		header.Set("Access-Control-Allow-Origin", allowOrigin)
		if vary {
			header.Add("Vary", "Origin")
		}
	}

	if cfg.AllowCredentials {
		header.Set("Access-Control-Allow-Credentials", "true")
	}

	if len(cfg.AllowMethods) > 0 {
		header.Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ", "))
	}

	if len(cfg.AllowHeaders) > 0 {
		header.Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
	}

	if len(cfg.ExposeHeaders) > 0 {
		header.Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
	}

	if cfg.MaxAge > 0 {
		header.Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
	}
}

func corsAllowedOrigin(origin string, allowOrigins []string, allowCredentials bool) (string, bool) {
	if origin == "" || len(allowOrigins) == 0 {
		return "", false
	}

	for _, o := range allowOrigins {
		if o == "*" {
			if allowCredentials {
				return origin, true
			}
			return "*", false
		}
		if o == origin {
			return origin, true
		}
	}

	return "", false
}

func clearCORSHeaders(header http.Header) {
	header.Del("Access-Control-Allow-Origin")
	header.Del("Access-Control-Allow-Credentials")
	header.Del("Access-Control-Allow-Methods")
	header.Del("Access-Control-Allow-Headers")
	header.Del("Access-Control-Expose-Headers")
	header.Del("Access-Control-Max-Age")
}
