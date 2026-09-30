package domain

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// TOTP parameters (RFC 6238), the ones every authenticator app defaults to.
const (
	totpPeriod = 30
	totpDigits = 6
	// RecoveryCodeCount is how many Recovery codes are handed out at once.
	RecoveryCodeCount = 10
)

var (
	ErrWrongCode = errors.New("the code is wrong")
	ErrCodeUsed  = errors.New("the code was already used; wait for the next one")
)

// TwoFactorState is whether a Member's Two-factor authentication counts.
type TwoFactorState string

const (
	TwoFactorOff TwoFactorState = "off"
	// TwoFactorPending has a secret that was never confirmed with a code;
	// it does not count for sign-in.
	TwoFactorPending TwoFactorState = "pending"
	TwoFactorOn      TwoFactorState = "on"
)

// TwoFactor is a Member's Two-factor authentication.
type TwoFactor struct {
	State  TwoFactorState
	Secret []byte
	// LastStep is the time step of the last accepted Authenticator code;
	// only a later step is accepted.
	LastStep int64
}

// On reports whether signing in needs a second step.
func (t TwoFactor) On() bool { return t.State == TwoFactorOn }

var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret reads a 160-bit secret, the size RFC 4226 recommends.
func NewTOTPSecret(random io.Reader) ([]byte, error) {
	b := make([]byte, 20)
	if _, err := io.ReadFull(random, b); err != nil {
		return nil, err
	}
	return b, nil
}

// EncodeTOTPSecret is the secret as authenticator apps take it by hand.
func EncodeTOTPSecret(secret []byte) string { return base32NoPad.EncodeToString(secret) }

// TOTPStep is the time step t falls in.
func TOTPStep(t time.Time) int64 { return t.Unix() / totpPeriod }

// TOTPCode is the Authenticator code for one time step.
func TOTPCode(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	// Dynamic truncation, RFC 4226 section 5.3.
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, n%1_000_000)
}

// VerifyTOTP checks code against the step of now and one step either side,
// for clocks that drift, and returns the step it matched. A match at or
// before lastStep is ErrCodeUsed.
func VerifyTOTP(secret []byte, code string, now time.Time, lastStep int64) (int64, error) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return 0, ErrWrongCode
	}
	current := TOTPStep(now)
	for _, step := range []int64{current - 1, current, current + 1} {
		if subtle.ConstantTimeCompare([]byte(TOTPCode(secret, step)), []byte(code)) == 1 {
			if step <= lastStep {
				return 0, ErrCodeUsed
			}
			return step, nil
		}
	}
	return 0, ErrWrongCode
}

// OTPAuthURI is what the QR code holds for an authenticator app to add.
func OTPAuthURI(account string, secret []byte) string {
	q := url.Values{}
	q.Set("secret", EncodeTOTPSecret(secret))
	q.Set("issuer", "Bakery")
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	label := url.PathEscape("Bakery:" + account)
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// NewRecoveryCodes reads RecoveryCodeCount codes of 10 base32 characters
// (50 bits each), written as xxxxx-xxxxx.
func NewRecoveryCodes(random io.Reader) ([]string, error) {
	codes := make([]string, RecoveryCodeCount)
	for i := range codes {
		b := make([]byte, 7)
		if _, err := io.ReadFull(random, b); err != nil {
			return nil, err
		}
		s := strings.ToLower(base32NoPad.EncodeToString(b))[:10]
		codes[i] = s[:5] + "-" + s[5:]
	}
	return codes, nil
}

// NormalizeRecoveryCode is how a typed Recovery code is compared: without
// dashes or spaces, lower case.
func NormalizeRecoveryCode(code string) string {
	return strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
}
