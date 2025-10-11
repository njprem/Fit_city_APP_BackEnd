package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

type PasswordResetMailer struct {
	host     string
	port     string
	username string
	password string
	from     string
	useTLS   bool
}

func NewPasswordResetMailer(host, port, username, password, from string, useTLS bool) *PasswordResetMailer {
	return &PasswordResetMailer{
		host:     strings.TrimSpace(host),
		port:     strings.TrimSpace(port),
		username: username,
		password: password,
		from:     strings.TrimSpace(from),
		useTLS:   useTLS,
	}
}

func (m *PasswordResetMailer) SendPasswordReset(ctx context.Context, email, otp string) error {
	if m == nil {
		return errors.New("mailer not configured")
	}
	if m.host == "" || m.port == "" || m.from == "" {
		return errors.New("mailer missing configuration")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	subject := "Your FitCity password reset code"
	body := fmt.Sprintf("Use the following code to reset your password: %s\n\nIf you did not request this, ignore this email.", otp)

	message := strings.Builder{}
	message.WriteString(fmt.Sprintf("From: %s\r\n", m.from))
	message.WriteString(fmt.Sprintf("To: %s\r\n", email))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	message.WriteString(body)
	message.WriteString("\r\n")

	addr := net.JoinHostPort(m.host, m.port)
	var auth smtp.Auth
	if m.username != "" || m.password != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	if err := smtp.SendMail(addr, auth, m.from, []string{email}, []byte(message.String())); err != nil {
		return err
	}

	return nil
}
