// Package sendg provides a simple interface to send emails using Sendgrid with dynamic templates.
package sendg

import (
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type Template[T any] struct {
	id     string
	client *sendgrid.Client
}

func NewTemplate[T any](apiKey string, templateID string) *Template[T] {
	return &Template[T]{
		templateID, sendgrid.NewSendClient(apiKey),
	}
}

// SendMail sends an email via Sendgrid, using the template.
func (t *Template[T]) SendMail(fromEmail string, data T, toEmail string, extraToEmails ...string) error {
	personalization := mail.NewPersonalization()
	personalization.To = []*mail.Email{}
	toEmails := append([]string{toEmail}, extraToEmails...)
	for _, to := range toEmails {
		personalization.To = append(personalization.To, mail.NewEmail("", to))
	}
	personalization.SetDynamicTemplateData("data", data)

	// Create email
	message := mail.NewV3Mail()
	message.SetFrom(mail.NewEmail("", fromEmail))
	message.AddPersonalizations(personalization)
	message.SetTemplateID(t.id)

	// Send email
	_, err := t.client.Send(message)
	if err != nil {
		return err
	}
	return nil
}
