package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrBadCredentials is a Webhook delivery whose credentials do not
	// match its Webhook trigger's Signing mode and secret.
	ErrBadCredentials = errors.New("the webhook delivery's credentials do not match")
	// ErrOutsideReplayWindow is an hmac_sha256 Webhook delivery whose
	// timestamp is further from now than the trigger's Replay window.
	ErrOutsideReplayWindow = errors.New("the webhook delivery's timestamp is outside the replay window")
)

// DeliveryHeaders are the headers of a Webhook delivery that prove it or
// name it. IdempotencyKey is Idempotency-Key, else X-GitHub-Delivery.
type DeliveryHeaders struct {
	Authorization   string
	Signature       string
	HubSignature256 string
	Timestamp       string
	IdempotencyKey  string
}

// VerifyDelivery checks a Webhook delivery against the Webhook trigger's
// Signing mode and secret, as Paperclip's firePublicTrigger, and answers
// its Idempotency key ("" for none). Every comparison of a secret value is
// constant-time.
func VerifyDelivery(t RoutineTrigger, h DeliveryHeaders, body []byte, now time.Time) (string, error) {
	switch t.SigningMode {
	case NoSigning:
	case BearerSigning:
		if !equal(h.Authorization, "Bearer "+t.Secret) {
			return "", ErrBadCredentials
		}
	case GitHubHMACSigning:
		sig := h.HubSignature256
		if sig == "" {
			sig = h.Signature
		}
		if sig == "" || !equal(strings.TrimPrefix(sig, "sha256="), sign(t.Secret, body)) {
			return "", ErrBadCredentials
		}
	case HMACSHA256Signing:
		if h.Signature == "" || h.Timestamp == "" {
			return "", ErrBadCredentials
		}
		ms, err := timestampMs(h.Timestamp)
		if err != nil {
			return "", ErrBadCredentials
		}
		if d := now.UnixMilli() - ms; d > int64(t.ReplayWindowSec)*1000 || -d > int64(t.ReplayWindowSec)*1000 {
			return "", ErrOutsideReplayWindow
		}
		sig := strings.TrimPrefix(h.Signature, "sha256=")
		if !equal(sig, sign(t.Secret, append([]byte(h.Timestamp+"."), body...))) {
			return "", ErrBadCredentials
		}
		// The signature already makes each delivery unique, so a retry
		// of the same signed delivery is the same Routine run.
		key := sha256.Sum256([]byte(strconv.FormatUint(t.ID, 10) + ":" + h.Timestamp + ":" + sig))
		return hex.EncodeToString(key[:]), nil
	default:
		return "", ErrBadCredentials
	}
	return h.IdempotencyKey, nil
}

// sign is the hex HMAC-SHA256 of message under the secret.
func sign(secret string, message []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(message)
	return hex.EncodeToString(m.Sum(nil))
}

// equal compares two credentials in constant time; a different length
// answers false at once, which tells nothing about the secret.
func equal(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// timestampMs reads a Webhook delivery's timestamp, in seconds or
// milliseconds since the epoch, as milliseconds: Paperclip's
// normalizeWebhookTimestampMs counts a value below 1e12 as seconds.
func timestampMs(s string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	if n < 1e12 {
		return n * 1000, nil
	}
	return n, nil
}
