package http

import (
	"encoding/json"
	"errors"
	"time"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/contexts/identity/app"
	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

// LoginChallengeCookie carries the Login challenge between a correct
// password and the second step.
const LoginChallengeCookie = "bakery_login"

const (
	loginChallengeTTL = 5 * time.Minute
	// maxCodeAttempts is how many wrong codes end a Login challenge.
	maxCodeAttempts = 5
)

// loginChallenge is the cookie's value, encrypted with the app key so the
// browser can neither read nor forge it.
type loginChallenge struct {
	MemberID  uint64    `json:"member_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
}

func setLoginChallenge(ctx contractshttp.Context, ch loginChallenge) error {
	b, err := json.Marshal(ch)
	if err != nil {
		return err
	}
	value, err := facades.Crypt().EncryptString(string(b))
	if err != nil {
		return err
	}
	ctx.Response().Cookie(loginChallengeCookie(value, int(time.Until(ch.ExpiresAt).Seconds())))
	return nil
}

func readLoginChallenge(ctx contractshttp.Context) (loginChallenge, bool) {
	value := ctx.Request().Cookie(LoginChallengeCookie)
	if value == "" {
		return loginChallenge{}, false
	}
	plain, err := facades.Crypt().DecryptString(value)
	if err != nil {
		return loginChallenge{}, false
	}
	var ch loginChallenge
	if json.Unmarshal([]byte(plain), &ch) != nil || ch.MemberID == 0 || time.Now().After(ch.ExpiresAt) {
		return loginChallenge{}, false
	}
	return ch, true
}

// loginChallengeCookie is scoped to the login routes and, like the
// Session cookie, HttpOnly and SameSite=Strict.
func loginChallengeCookie(value string, maxAge int) contractshttp.Cookie {
	return contractshttp.Cookie{
		Name:     LoginChallengeCookie,
		Value:    value,
		Path:     "/api/login",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   facades.Config().GetString("app.env") == "production",
		SameSite: "strict",
	}
}

type loginTwoFactorRequest struct {
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

// LoginTwoFactor is the second sign-in step. A wrong code counts against
// the challenge; the fifth ends it.
func (c *Controller) LoginTwoFactor(ctx contractshttp.Context) contractshttp.Response {
	var req loginTwoFactorRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return respond.BadBody(ctx)
	}
	ch, ok := readLoginChallenge(ctx)
	if !ok {
		ctx.Response().Cookie(loginChallengeCookie("", -1))
		return respond.Error(ctx, contractshttp.StatusUnauthorized, "sign in again")
	}
	m, err := c.service.LoginTwoFactor(ctx.Context(), ch.MemberID, req.Code, req.RecoveryCode)
	switch {
	case errors.Is(err, domain.ErrWrongCode), errors.Is(err, domain.ErrCodeUsed):
		ch.Attempts++
		if ch.Attempts >= maxCodeAttempts {
			ctx.Response().Cookie(loginChallengeCookie("", -1))
			return respond.Error(ctx, contractshttp.StatusUnauthorized, "too many wrong codes, sign in again")
		}
		if err := setLoginChallenge(ctx, ch); err != nil {
			return respond.ServerError(ctx, err)
		}
		return ctx.Response().Json(contractshttp.StatusUnprocessableEntity, contractshttp.Json{
			"message":       err.Error(),
			"errors":        map[string]string{"code": err.Error()},
			"attempts_left": maxCodeAttempts - ch.Attempts,
		})
	case errors.Is(err, app.ErrBadCredentials):
		ctx.Response().Cookie(loginChallengeCookie("", -1))
		return respond.Error(ctx, contractshttp.StatusUnauthorized, "sign in again")
	case err != nil:
		return respond.ServerError(ctx, err)
	}
	ctx.Response().Cookie(loginChallengeCookie("", -1))
	return c.withSession(ctx, contractshttp.StatusOK, m)
}
