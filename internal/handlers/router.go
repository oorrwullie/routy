package handlers

import (
	"net/http"

	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"
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
	routes, err := models.GetDomainRoutes()
	if err != nil {
		return err
	}

	if routes.Ssh.Enabled {
		go r.sshRouter(routes.Ssh.ListenPort, routes.Ssh.Configs)
	}

	return r.HttpRouter(routes.Http.Domains)
}
