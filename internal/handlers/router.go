package handlers

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"log"
	"net"
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
						Director: func(req *http.Request) {
							// Resolve the hostname using the custom DNS resolver
							rts, err := models.GetDomainRoutes()
							if err != nil {
								fmt.Println("Error resolving hostname:", err)
								return
							}
							ip, err := resolve(targetURL.Host, dns.TypeA, rts)
							if err != nil {
								fmt.Println("Error resolving hostname:", err)
								return
							}

							// Update the request's Host field to the resolved IP
							req.Host = ip[0].String()
							req.URL.Host = ip[0].String()
						},
					}

					// proxy = httputil.NewSingleHostReverseProxy(targetURL)
					// proxy.Transport = &preserveHeadersTransport{targetURL: targetURL}

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

						subdomainHandler := func(w http.ResponseWriter, req *http.Request) {
							if r.denyList.IsDenied(logging.GetRequestRemoteAddress(req)) {
								return
							}

							r.accessLog <- req

							req.Host = req.URL.Host
							req.Header.Set("X-Forwarded", "true")
							req.Header.Set("X-Forwarded-For", req.RemoteAddr)
							req.Header.Set("X-Forwarded-Host", req.Host)
							req.Header.Set("X-Forwarded-Proto", req.URL.Scheme)

							proxy.ServeHTTP(w, req)
						}

						var host string
						if sd.Name == domain.Name {
							host = domain.Name
						} else {
							host = fmt.Sprintf("%s.%s", sd.Name, domain.Name)
						}

						subdomainRouter := router.Host(host).Subrouter()
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

type preserveHeadersTransport struct {
	http.Transport
	targetURL *url.URL
}

func (t *preserveHeadersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	reqCopy := req.Clone(req.Context())

	// Set the Host header of the cloned request to match the target server
	reqCopy.Host = t.targetURL.Host

	// Make the actual request
	resp, err := t.Transport.RoundTrip(reqCopy)
	if err != nil {
		return nil, err
	}

	// Copy the headers from the response to the original request
	for key, values := range resp.Header {
		req.Header[key] = values
	}

	return resp, nil
}

func resolve(domain string, qtype uint16, routes *models.Routes) ([]dns.RR, error) {
	answers := make([]dns.RR, 0)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = true

	for _, dm := range routes.Domains {
		name := dm.Name
		d := dns.A{}

		for _, s := range dm.Subdomains {
			subName := fmt.Sprintf("%s.%s", s.Name, name)
			d.Hdr = dns.RR_Header{
				Name:     fmt.Sprintf("%s.", subName),
				Rrtype:   dns.TypeA,
				Class:    dns.ClassINET,
				Ttl:      4,
				Rdlength: 4,
			}
			for _, p := range s.Paths {
				url, _ := url.Parse(p.Target)
				fmt.Println(url)
				parsedIP := net.ParseIP(url.Hostname())
				d.A = parsedIP
				answers = append(answers, &d)
			}
		}
	}

	if len(answers) > 0 {
		return answers, nil
	}

	c := new(dns.Client)
	in, _, err := c.Exchange(m, "8.8.8.8:53")
	if err != nil {
		return nil, err
	}

	for _, ans := range in.Answer {
		answers = append(answers, ans)
	}

	return answers, nil
}
