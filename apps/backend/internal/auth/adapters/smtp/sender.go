package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/auth/application"
)

type Sender struct {
	Address  string
	From     string
	Username string
	Password string
}

func (s Sender) Send(ctx context.Context, message application.EmailMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.ContainsAny(s.From, "\r\n") || strings.ContainsAny(message.To, "\r\n") || strings.ContainsAny(message.Subject, "\r\n") {
		return errors.New("invalid mail header")
	}
	host, _, err := net.SplitHostPort(s.Address)
	if err != nil {
		return fmt.Errorf("SMTP_ADDRESS must be host:port: %w", err)
	}
	body := "From: " + s.From + "\r\nTo: " + message.To + "\r\nSubject: " + message.Subject + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + message.Body + "\r\n"
	connection, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", s.Address)
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("open SMTP session: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	} else if s.Username != "" {
		return errors.New("SMTP server does not support STARTTLS")
	}
	if s.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.Username, s.Password, host)); err != nil {
			return fmt.Errorf("authenticate SMTP: %w", err)
		}
	}
	if err := client.Mail(s.From); err != nil {
		return fmt.Errorf("SMTP sender: %w", err)
	}
	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP data: %w", err)
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		return fmt.Errorf("write SMTP body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP body: %w", err)
	}
	return client.Quit()
}
