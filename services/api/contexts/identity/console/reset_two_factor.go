// Package console holds identity's artisan commands, for whoever has a
// shell on the server.
package console

import (
	"context"
	"errors"
	"fmt"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"

	"github.com/jevido/bakery/services/api/contexts/identity/app"
)

// ResetTwoFactor switches a Member's two-factor off, the Instance admin's
// included: nobody outranks the Instance admin in the dashboard.
type ResetTwoFactor struct {
	Service *app.Service
}

func (ResetTwoFactor) Signature() string { return "identity:reset-two-factor" }

func (ResetTwoFactor) Description() string {
	return "Switch two-factor authentication off for the Member with this email"
}

func (ResetTwoFactor) Extend() command.Extend {
	return command.Extend{ArgsUsage: "<email>"}
}

func (r ResetTwoFactor) Handle(ctx console.Context) error {
	email := ctx.Argument(0)
	if email == "" {
		return errors.New("usage: identity:reset-two-factor <email>")
	}
	err := r.Service.ResetTwoFactorByEmail(context.Background(), email)
	switch {
	case errors.Is(err, app.ErrMemberNotFound):
		return fmt.Errorf("no member has the email %s", email)
	case errors.Is(err, app.ErrTwoFactorOff):
		return fmt.Errorf("two-factor is already off for %s", email)
	case err != nil:
		return err
	}
	ctx.Info("two-factor switched off for " + email + "; their sessions are signed out")
	return nil
}
