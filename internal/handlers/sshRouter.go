package handlers

import (
	"fmt"
	"io"
	"net"

	"github.com/oorrwullie/routy/internal/logging"
	"github.com/oorrwullie/routy/internal/models"
	"golang.org/x/crypto/ssh"
)

func (r *Routy) sshRouter(port int, configs []models.SshConfig) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	defer lis.Close()

	for {
		conn, err := lis.Accept()
		if err != nil {
			msg := fmt.Sprintf("Failed to accept incoming connection: %v", err)
			r.eventLog <- logging.EventLogMessage{
				Level:   "ERROR",
				Caller:  "sshListener()->lis.Accept()",
				Message: msg,
			}
			continue
		}

		defer conn.Close()

		go func() {
			for _, config := range configs {
				go r.handleSSHConnection(conn, config.Host, config.Port)
			}
		}()
	}
}

func (r *Routy) handleSSHConnection(conn net.Conn, host string, port int) {
	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		msg := fmt.Sprintf("SSH handshake failed: %v", err)
		r.eventLog <- logging.EventLogMessage{
			Level:   "ERROR",
			Caller:  "handleSSHConnection()->ssh.NewServerConn()",
			Message: msg,
		}
		return
	}
	defer sshConn.Close()

	msg := fmt.Sprintf("SSH connection established from %s", sshConn.RemoteAddr())
	r.eventLog <- logging.EventLogMessage{
		Level:   "INFO",
		Caller:  "handleSSHConnection()",
		Message: msg,
	}

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, _, err := newChannel.Accept()
		if err != nil {
			msg := fmt.Sprintf("Failed to accept channel: %v", err)
			r.eventLog <- logging.EventLogMessage{
				Level:   "ERROR",
				Caller:  "handleSSHConnection()->newChannel.Accept()",
				Message: msg,
			}
			return
		}

		go func() {
			defer channel.Close()

			targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
			if err != nil {
				msg := fmt.Sprintf("Failed to connect to target server: %v", err)
				r.eventLog <- logging.EventLogMessage{
					Level:   "ERROR",
					Caller:  "handleSSHConnection()->net.Dial()",
					Message: msg,
				}
				return
			}
			defer targetConn.Close()

			go io.Copy(channel, targetConn)
			go io.Copy(targetConn, channel)
		}()
	}
}
