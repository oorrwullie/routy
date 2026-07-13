package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"
	"golang.org/x/crypto/ssh"
)

const sshHostKeyFilename = "ssh_host_ed25519_key"

func (r *Routy) sshRouter(port int, configs []models.SshConfig) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	defer func() {
		_ = lis.Close()
	}()

	signer, err := loadOrCreateSSHSigner()
	if err != nil {
		return err
	}

	for {
		conn, err := lis.Accept()
		if err != nil {
			msg := fmt.Sprintf("Failed to accept incoming connection: %v", err)
			r.EventLog <- logging.EventLogMessage{
				Level:   "ERROR",
				Caller:  "sshListener()->lis.Accept()",
				Message: msg,
			}
			continue
		}

		go r.handleSSHConnection(conn, configs, signer)
	}
}

func loadOrCreateSSHSigner() (ssh.Signer, error) {
	m, err := models.NewModel()
	if err != nil {
		return nil, err
	}

	keyPath, err := m.GetFilepath(sshHostKeyFilename)
	if err != nil {
		return nil, err
	}

	keyData, err := os.ReadFile(keyPath)
	if err == nil {
		return ssh.ParsePrivateKey(keyData)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})

	if err := os.WriteFile(keyPath, pemData, 0600); err != nil {
		return nil, err
	}

	return ssh.NewSignerFromKey(privateKey)
}

func (r *Routy) handleSSHConnection(conn net.Conn, configs []models.SshConfig, signer ssh.Signer) {
	defer func() {
		_ = conn.Close()
	}()

	if len(configs) == 0 {
		return
	}

	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	config.AddHostKey(signer)

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		msg := fmt.Sprintf("SSH handshake failed: %v", err)
		r.EventLog <- logging.EventLogMessage{
			Level:   "ERROR",
			Caller:  "handleSSHConnection()->ssh.NewServerConn()",
			Message: msg,
		}
		return
	}
	defer func() {
		_ = sshConn.Close()
	}()

	msg := fmt.Sprintf("SSH connection established from %s", sshConn.RemoteAddr())
	r.EventLog <- logging.EventLogMessage{
		Level:   "INFO",
		Caller:  "handleSSHConnection()",
		Message: msg,
	}

	target, ok := findSSHRoute(configs, sshConn.User())
	if !ok {
		r.EventLog <- logging.EventLogMessage{
			Level:   "ERROR",
			Caller:  "handleSSHConnection()->findSSHRoute()",
			Message: fmt.Sprintf("No SSH route configured for user %q", sshConn.User()),
		}
		return
	}

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			if err := newChannel.Reject(ssh.UnknownChannelType, "unknown channel type"); err != nil {
				r.EventLog <- logging.EventLogMessage{
					Level:   "ERROR",
					Caller:  "handleSSHConnection()->newChannel.Reject()",
					Message: fmt.Sprintf("Failed to reject channel: %v", err),
				}
			}
			continue
		}

		channel, _, err := newChannel.Accept()
		if err != nil {
			msg := fmt.Sprintf("Failed to accept channel: %v", err)
			r.EventLog <- logging.EventLogMessage{
				Level:   "ERROR",
				Caller:  "handleSSHConnection()->newChannel.Accept()",
				Message: msg,
			}
			return
		}

		go func() {
			defer func() {
				_ = channel.Close()
			}()

			targetConn, err := net.Dial("tcp", net.JoinHostPort(target.Host, strconv.Itoa(target.Port)))
			if err != nil {
				msg := fmt.Sprintf("Failed to connect to target server: %v", err)
				r.EventLog <- logging.EventLogMessage{
					Level:   "ERROR",
					Caller:  "handleSSHConnection()->net.Dial()",
					Message: msg,
				}
				return
			}
			defer func() {
				_ = targetConn.Close()
			}()

			done := make(chan struct{}, 2)
			go r.copySSHStream(channel, targetConn, done)
			go r.copySSHStream(targetConn, channel, done)
			<-done
		}()
	}
}

func findSSHRoute(configs []models.SshConfig, user string) (models.SshConfig, bool) {
	user = strings.TrimSpace(strings.ToLower(user))
	for _, config := range configs {
		if strings.TrimSpace(strings.ToLower(config.Domain)) == user {
			return config, true
		}
	}

	return models.SshConfig{}, false
}

func (r *Routy) copySSHStream(dst io.Writer, src io.Reader, done chan<- struct{}) {
	defer func() {
		done <- struct{}{}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		r.EventLog <- logging.EventLogMessage{
			Level:   "ERROR",
			Caller:  "copySSHStream()->io.Copy()",
			Message: fmt.Sprintf("SSH stream copy failed: %v", err),
		}
	}
}
