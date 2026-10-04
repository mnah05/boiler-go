// Package mailer isolates the provider behind a small application-facing API.
package mailer

import (
	"context"
	"errors"

	"boiler-go/internal/config"

	"github.com/resend/resend-go/v2"
)

var ErrNotConfigured = errors.New("email provider is not configured")

type Message struct {
	To                  []string
	Subject, HTML, Text string
}
type Sender interface {
	Send(context.Context, Message) (string, error)
}

type ResendSender struct {
	client *resend.Client
	from   string
}

func New(cfg *config.Config) Sender {
	if cfg.ResendAPIKey == "" {
		return disabled{}
	}
	return &ResendSender{client: resend.NewClient(cfg.ResendAPIKey), from: cfg.ResendFrom}
}

func (s *ResendSender) Send(ctx context.Context, message Message) (string, error) {
	result, err := s.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{From: s.from, To: message.To, Subject: message.Subject, Html: message.HTML, Text: message.Text})
	if err != nil {
		return "", err
	}
	return result.Id, nil
}

type disabled struct{}

func (disabled) Send(context.Context, Message) (string, error) { return "", ErrNotConfigured }
