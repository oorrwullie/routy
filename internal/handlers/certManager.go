package handlers

import (
	"github.com/oorrwullie/routy/internal/models"
	"golang.org/x/crypto/acme/autocert"
)

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
