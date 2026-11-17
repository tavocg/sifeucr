package cmd

import (
	"github.com/tavocg/go-email/mailer"
	"github.com/tavocg/go-email/mailer/backends/smtp"
	"sifeucr/mailer/noop"
)

func newMailer(cfg serverConfig) (mailer.Mailer, error) {
	if cfg.SMTPAddress == "" {
		return noop.Mailer{}, nil
	}
	var options []smtp.Option
	if cfg.SMTPStartTLS {
		options = append(options, smtp.WithStartTLS())
	}
	return smtp.NewClient(cfg.SMTPAddress, cfg.SMTPUser, cfg.SMTPPassword, options...)
}
