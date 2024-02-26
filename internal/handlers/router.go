package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/sync/errgroup"
)

type Routy struct {
	hostnames []string
	eventLog  chan logging.EventLogMessage
	accessLog chan *http.Request
	denyList  *models.DenyList
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
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

					proxy := &httputil.ReverseProxy{
						Rewrite: func(r *httputil.ProxyRequest) {
							r.SetURL(targetURL)
							r.Out.Host = r.In.Host
						},
					}

					if path.Upgrade {
						http.HandleFunc(path.Location, func(w http.ResponseWriter, req *http.Request) {
							if r.denyList.IsDenied(logging.GetRequestRemoteAddress(req)) {
								return
							}

							r.accessLog <- req

							conn, err := upgrader.Upgrade(w, req, nil)
							if err != nil {
								log.Printf("Error upgrading connection to WebSocket: %v", err)
								return
							}
							defer conn.Close()

							targetWs, _, err := websocket.DefaultDialer.Dial(path.Target, req.Header)
							if err != nil {
								log.Printf("Error connecting to target server: %v", err)
								return
							}
							defer targetWs.Close()

							// Bidirectional proxy
							go func() {
								defer targetWs.Close()
								defer conn.Close()

								for {
									_, message, err := conn.ReadMessage()
									if err != nil {
										msg := fmt.Sprintf("Error receiving message from client: %v", err)
										r.eventLog <- logging.EventLogMessage{
											Level:   "ERROR",
											Caller:  "handleWebSocket()->conn.ReadMessage()",
											Message: msg,
										}

										return
									}

									err = targetWs.WriteMessage(websocket.TextMessage, message)
									if err != nil {
										msg := fmt.Sprintf("Error sending message to target server: %v", err)
										r.eventLog <- logging.EventLogMessage{
											Level:   "ERROR",
											Caller:  "handleWebSocket()->targetWs.WriteMessage()",
											Message: msg,
										}

										return
									}
								}
							}()

							for {
								_, message, err := targetWs.ReadMessage()
								if err != nil {
									msg := fmt.Sprintf("Error receiving message from target server: %v", err)
									r.eventLog <- logging.EventLogMessage{
										Level:   "ERROR",
										Caller:  "handleWebSocket()->targetWs.ReadMessage()",
										Message: msg,
									}

									return
								}

								err = conn.WriteMessage(websocket.TextMessage, message)
								if err != nil {
									msg := fmt.Sprintf("Error sending message to client: %v", err)
									r.eventLog <- logging.EventLogMessage{
										Level:   "ERROR",
										Caller:  "handleWebSocket()->conn.WriteMessage()",
										Message: msg,
									}

									return
								}
							}

						})

						go func() {
							http.ListenAndServe(fmt.Sprintf(":%d", path.ListenPort), nil)
						}()
					} else {
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

								req.Host = req.URL.Host
								req.Header.Set("X-Forwarded", "true")
								req.Header.Set("X-Forwarded-For", req.RemoteAddr)
								req.Header.Set("X-Forwarded-Host", host)
								req.Header.Set("X-Forwarded-Proto", targetURL.Scheme)

								proxy.Rewrite = func(r *httputil.ProxyRequest) {
									r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
									r.Out.Header["X-Forwarded-Host"] = r.In.Header["X-Forwarded-Host"]
									r.Out.Header["X-Forwarded-Proto"] = r.In.Header["X-Forwarded-Proto"]
									r.SetXForwarded()
									r.SetURL(targetURL)
									r.Out.Host = r.In.Host
								}

								proxy.ServeHTTP(w, req)
							},
						)
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
