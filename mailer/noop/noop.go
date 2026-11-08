// Package noop provides a mailer that discards messages without delivering them.
package noop

import (
	"context"

	"github.com/tavocg/go-email/mailer"
)

type Mailer struct{}

var _ mailer.Mailer = Mailer{}

func (Mailer) Send(ctx context.Context, message *mailer.Message) error {
	if ctx == nil {
		return mailer.ErrNilContext
	}
	if message == nil {
		return mailer.ErrNilMessage
	}
	return ctx.Err()
}
