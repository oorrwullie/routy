package handlers

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
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
	accessLog chan *http.Request
	denyList  *models.DenyList
}

func NewRouty(eventLog chan logging.EventLogMessage, accessLog chan *http.Request, denyList *models.DenyList) *Routy {
	return &Routy{
		eventLog:  eventLog,
		accessLog: accessLog,
		denyList:  denyList,
	}
}

func (r *Routy) Route() error {
	router := mux.NewRouter()

	g, _ := errgroup.WithContext(context.Background())

	routes, err := models.GetDomainRoutes()
	if err != nil {
		return err
	}

	for _, d := range routes.Domains {
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

	for _, domain := range routes.Domains {
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
						r.eventLog <- logging.EventLogMessage{
							Level:   "ERROR",
							Caller:  "Route()->url.Parse()",
							Message: msg,
						}

						continue
					}

					proxy := httputil.NewSingleHostReverseProxy(targetURL)

					websocketHandler := func(w http.ResponseWriter, req *http.Request) {
						backendURL := *targetURL
						backendURL.Scheme = "wss"

						backendConn, err := tls.Dial("tcp", backendURL.Host, &tls.Config{InsecureSkipVerify: true})
						if err != nil {
							msg := fmt.Sprintf("failed to connect to WebSocket backend: %v\n", err)
							r.eventLog <- logging.EventLogMessage{
								Level:   "ERROR",
								Caller:  "WebsocketHandler()->tls.Dial()",
								Message: msg,
							}

							return
						}
						defer backendConn.Close()

						hj, ok := w.(http.Hijacker)
						if !ok {
							http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
							return
						}

						clientConn, _, err := hj.Hijack()
						if err != nil {
							msg := fmt.Sprintf("failed to hijack connection: %v\n", err)
							r.eventLog <- logging.EventLogMessage{
								Level:   "ERROR",
								Caller:  "WebsocketHandler()->hj.Hijack()",
								Message: msg,
							}

							return
						}
						defer clientConn.Close()

						backendConn.Write([]byte(req.Method + " " + req.URL.RequestURI() + " HTTP/1.1\r\n"))
						req.Header.WriteSubset(backendConn, map[string]bool{"Host": true, "Sec-WebSocket-Key": true, "Sec-WebSocket-Version": true})

						resp, err := http.ReadResponse(bufio.NewReader(backendConn), req)
						if err != nil {
							msg := fmt.Sprintf("failed to read WebSocket handshake response: %v\n", err)
							r.eventLog <- logging.EventLogMessage{
								Level:   "ERROR",
								Caller:  "WebsocketHandler()->http.ReadResponse()",
								Message: msg,
							}

							return
						}
						resp.Header.Del("Sec-WebSocket-Key") // Security measure
						resp.Header.Del("Sec-WebSocket-Accept")
						resp.Write(clientConn)

						go func() {
							copyWebSocket(clientConn, backendConn)
						}()

						copyWebSocket(backendConn, clientConn)
					}

					subdomainHandler := func(w http.ResponseWriter, req *http.Request) {
						if r.denyList.IsDenied(logging.GetRequestRemoteAddress(req)) {
							return
						}

						if isWebSocketRequest(req) {
							websocketHandler(w, req)
							return
						}

						r.accessLog <- req

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
					if path.Upgrade {
						// if the path is an upgrade path, it's a websocket path.
						subdomainRouter.PathPrefix(path.Location).Handler(http.HandlerFunc(websocketHandler))

						if path.ListenPort != 0 {
							server := &http.Server{
								Addr:      fmt.Sprintf(":%d", path.ListenPort),
								Handler:   subdomainRouter,
								TLSConfig: certManager.TLSConfig(),
							}

							g.Go(func() error {
								return server.ListenAndServeTLS("", "")
							})
						}
					} else {
						subdomainRouter.PathPrefix(path.Location).Handler(http.HandlerFunc(subdomainHandler))
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

func (r *Routy) getCertManager() (*autocert.Manager, error) {
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
		HostPolicy: autocert.HostWhitelist(r.hostnames...),
		Cache:      autocert.DirCache(certDir),
	}

	return manager, nil
}

// Check if the request is a WebSocket upgrade request
func isWebSocketRequest(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket" &&
		strings.ToLower(r.Header.Get("Connection")) == "upgrade"
}

// Copy WebSocket messages between connections
func copyWebSocket(dst, src net.Conn) {
	buffer := make([]byte, 1024)
	for {
		n, err := src.Read(buffer)
		if err != nil {
			return
		}
		_, err = dst.Write(buffer[:n])
		if err != nil {
			return
		}
	}
}
