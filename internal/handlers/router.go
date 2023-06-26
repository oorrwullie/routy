package handlers

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"
)

type Router struct {
	port     string
	hostname string
	eventLog chan logging.EventLogMessage
}

func NewRouter(
	port string,
	hostname string,
	eventLog chan logging.EventLogMessage,
) *Router {
	return &Router{
		port:     port,
		hostname: hostname,
		eventLog: eventLog,
	}
}

func (r *Router) Route() error {
	denyList, err := models.GetDenyList()

	accessLog := make(chan *http.Request)
	go logging.AccessLogger(accessLog)

	routes, err := models.GetRoutes()
	if err != nil {
		return err
	}

	router := mux.NewRouter()

	for _, route := range routes {
		targetURL, err := url.Parse(route.Target)
		if err != nil {
			msg := fmt.Sprintf("Failed to parse target URL for subdomain %s: %v\n", route.Subdomain, err)
			r.eventLog <- logging.EventLogMessage{
				Level:   "ERROR",
				Caller:  "Route()->url.Parse()",
				Message: msg,
			}

			continue
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		handler := func(w http.ResponseWriter, r *http.Request) {
			if denyList.IsDenied(logging.GetRequestRemoteAddress(r)) {
				return
			}

			accessLog <- r

			r.Host = r.URL.Host
			proxy.ServeHTTP(w, r)
		}

		host := fmt.Sprintf("%s.%s", route.Subdomain, r.hostname)
		subdomainRouter := router.Host(host).Subrouter()
		subdomainRouter.PathPrefix("/").Handler(http.HandlerFunc(handler))
	}

	server := &http.Server{
		Addr:    ":" + r.port,
		Handler: router,
	}

	return server.ListenAndServe()
}
