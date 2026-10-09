package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"testing"
	"time"
)

func hmacHex(secret, msg string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

func TestVerifyDelivery(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"repo":"shop"}`)
	trigger := func(m SigningMode) RoutineTrigger {
		return RoutineTrigger{ID: 9, Kind: WebhookTrigger, Secret: "s3cret", SigningMode: m, ReplayWindowSec: 300}
	}
	sec := strconv.FormatInt(now.Unix(), 10)
	ms := strconv.FormatInt(now.UnixMilli(), 10)
	old := strconv.FormatInt(now.Add(-time.Hour).Unix(), 10)
	ahead := strconv.FormatInt(now.Add(6*time.Minute).Unix(), 10)
	ts := func(stamp string) string { return hmacHex("s3cret", stamp+"."+string(body)) }
	gh := hmacHex("s3cret", string(body))

	for _, c := range []struct {
		name string
		mode SigningMode
		h    DeliveryHeaders
		err  error
	}{
		{"none", NoSigning, DeliveryHeaders{}, nil},
		{"bearer good", BearerSigning, DeliveryHeaders{Authorization: "Bearer s3cret"}, nil},
		{"bearer wrong", BearerSigning, DeliveryHeaders{Authorization: "Bearer nope"}, ErrBadCredentials},
		{"bearer missing", BearerSigning, DeliveryHeaders{}, ErrBadCredentials},
		{"bearer without scheme", BearerSigning, DeliveryHeaders{Authorization: "s3cret"}, ErrBadCredentials},
		{"github prefixed", GitHubHMACSigning, DeliveryHeaders{HubSignature256: "sha256=" + gh}, nil},
		{"github bare", GitHubHMACSigning, DeliveryHeaders{HubSignature256: gh}, nil},
		{"github in X-Bakery-Signature", GitHubHMACSigning, DeliveryHeaders{Signature: "sha256=" + gh}, nil},
		{"github wrong", GitHubHMACSigning, DeliveryHeaders{HubSignature256: "sha256=" + hmacHex("other", string(body))}, ErrBadCredentials},
		{"github missing", GitHubHMACSigning, DeliveryHeaders{}, ErrBadCredentials},
		{"hmac seconds", HMACSHA256Signing, DeliveryHeaders{Timestamp: sec, Signature: "sha256=" + ts(sec)}, nil},
		{"hmac milliseconds", HMACSHA256Signing, DeliveryHeaders{Timestamp: ms, Signature: ts(ms)}, nil},
		{"hmac wrong", HMACSHA256Signing, DeliveryHeaders{Timestamp: sec, Signature: ts(old)}, ErrBadCredentials},
		{"hmac no timestamp", HMACSHA256Signing, DeliveryHeaders{Signature: ts(sec)}, ErrBadCredentials},
		{"hmac no signature", HMACSHA256Signing, DeliveryHeaders{Timestamp: sec}, ErrBadCredentials},
		{"hmac bad timestamp", HMACSHA256Signing, DeliveryHeaders{Timestamp: "soon", Signature: ts("soon")}, ErrBadCredentials},
		{"hmac an hour old", HMACSHA256Signing, DeliveryHeaders{Timestamp: old, Signature: ts(old)}, ErrOutsideReplayWindow},
		{"hmac ahead", HMACSHA256Signing, DeliveryHeaders{Timestamp: ahead, Signature: ts(ahead)}, ErrOutsideReplayWindow},
	} {
		if _, err := VerifyDelivery(trigger(c.mode), c.h, body, now); !errors.Is(err, c.err) || (c.err == nil) != (err == nil) {
			t.Errorf("%s: %v, want %v", c.name, err, c.err)
		}
	}
}

func TestDeliveryIdempotencyKey(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	body := []byte(`{}`)
	key, err := VerifyDelivery(RoutineTrigger{SigningMode: NoSigning}, DeliveryHeaders{IdempotencyKey: "abc"}, body, now)
	if err != nil || key != "abc" {
		t.Fatalf("the header's key = %q, %v", key, err)
	}
	if key, _ := VerifyDelivery(RoutineTrigger{SigningMode: NoSigning}, DeliveryHeaders{}, body, now); key != "" {
		t.Fatalf("no key = %q", key)
	}
	// hmac_sha256 keys by the signed delivery, whatever the header says.
	tr := RoutineTrigger{ID: 9, Secret: "k", SigningMode: HMACSHA256Signing, ReplayWindowSec: 300}
	stamp := "1800000000"
	h := DeliveryHeaders{Timestamp: stamp, Signature: hmacHex("k", stamp+".{}"), IdempotencyKey: "abc"}
	a, err := VerifyDelivery(tr, h, body, now)
	b, _ := VerifyDelivery(tr, h, body, now.Add(time.Second))
	if err != nil || a == "" || a == "abc" || a != b {
		t.Fatalf("hmac keys %q, %q, %v", a, b, err)
	}
	tr.ID = 10
	if c, _ := VerifyDelivery(tr, h, body, now); c == a {
		t.Fatal("another trigger's delivery has the same key")
	}
}
