package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

type Routy struct {
	accessLog chan *http.Request
	denyList  *models.DenyList
	EventLog  chan logging.EventLogMessage
	hostnames []string
	routes    *models.Routes
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

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

	go func() {
		err := logging.StartAccessLogger(accessLog)
		if err != nil {
			return
		}
	}()

	eventLog := make(chan logging.EventLogMessage)

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

func (r *Routy) Route() error {
	router := mux.NewRouter()

	g, _ := errgroup.WithContext(context.Background())

	for _, d := range r.routes.Domains {
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

	for _, domain := range r.routes.Domains {
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
						r.EventLog <- logging.EventLogMessage{
							Level:   "ERROR",
							Caller:  "Route()->url.Parse()",
							Message: msg,
						}

						continue
					}

					h, port, _ := net.SplitHostPort(targetURL.Host)

					if sd.Name == domain.Name {
						h = domain.Name
					} else {
						h = fmt.Sprintf("%s.%s", sd.Name, domain.Name)
					}

					targetURL.Host = net.JoinHostPort(h, port)

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
								msg := fmt.Sprintf("Error upgrading connection to WebSocket: %v", err)
								r.EventLog <- logging.EventLogMessage{
									Level:   "ERROR",
									Caller:  "handleWebSocket()->upgrader.Upgrade()",
									Message: msg,
								}

								return
							}
							defer conn.Close()

							targetWs, _, err := websocket.DefaultDialer.Dial(path.Target, req.Header)
							if err != nil {
								msg := fmt.Sprintf("Error connecting to target server: %v", err)
								r.EventLog <- logging.EventLogMessage{
									Level:   "ERROR",
									Caller:  "handleWebSocket()->websocket.DefaultDialer.Dial()",
									Message: msg,
								}

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
										r.EventLog <- logging.EventLogMessage{
											Level:   "ERROR",
											Caller:  "handleWebSocket()->conn.ReadMessage()",
											Message: msg,
										}

										return
									}

									err = targetWs.WriteMessage(websocket.TextMessage, message)
									if err != nil {
										msg := fmt.Sprintf("Error sending message to target server: %v", err)
										r.EventLog <- logging.EventLogMessage{
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
									r.EventLog <- logging.EventLogMessage{
										Level:   "ERROR",
										Caller:  "handleWebSocket()->targetWs.ReadMessage()",
										Message: msg,
									}

									return
								}

								err = conn.WriteMessage(websocket.TextMessage, message)
								if err != nil {
									msg := fmt.Sprintf("Error sending message to client: %v", err)
									r.EventLog <- logging.EventLogMessage{
										Level:   "ERROR",
										Caller:  "handleWebSocket()->conn.WriteMessage()",
										Message: msg,
									}

									return
								}
							}

						})

						go func(path models.Path) {
							http.ListenAndServe(fmt.Sprintf(":%d", path.ListenPort), nil)
						}(path)
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

								// proxy.Rewrite = func(r *httputil.ProxyRequest) {
								// 	r.Out.Header["X-Forwarded"] = r.In.Header["X-Forwarded"]
								// 	r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
								// 	r.Out.Header["X-Forwarded-Host"] = r.In.Header["X-Forwarded-Host"]
								// 	r.Out.Header["X-Forwarded-Proto"] = r.In.Header["X-Forwarded-Proto"]
								// 	r.SetXForwarded()
								// 	r.SetURL(targetURL)
								// 	r.Out.Host = r.In.Host
								// }
								proxy.Director = func(req *http.Request) {
									req.Header = r.getHeaders(req)
									req.URL = targetURL
									req.Host = targetURL.Host
								}

								proxy.Transport = r.getDnsResolver()

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

func (r *Routy) getHeaders(req *http.Request) http.Header {
	headers := make(http.Header)

	for key, value := range req.Header {
		headers[key] = value
	}

	return headers
}
