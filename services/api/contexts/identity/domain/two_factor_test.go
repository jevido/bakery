package domain

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

var rfcSecret = []byte("12345678901234567890")

// The SHA-1 vectors of RFC 6238 appendix B; the RFC gives 8 digits, the
// last 6 are ours.
func TestTOTPCodeRFC6238(t *testing.T) {
	for _, tc := range []struct {
		unix int64
		want string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	} {
		got := TOTPCode(rfcSecret, TOTPStep(time.Unix(tc.unix, 0)))
		if got != tc.want[2:] {
			t.Errorf("at %d: got %s, want %s", tc.unix, got, tc.want[2:])
		}
	}
}

func TestVerifyTOTP(t *testing.T) {
	now := time.Unix(1234567890, 0)
	step := TOTPStep(now)
	for _, d := range []int64{-1, 0, 1} {
		got, err := VerifyTOTP(rfcSecret, TOTPCode(rfcSecret, step+d), now, 0)
		if err != nil || got != step+d {
			t.Errorf("drift %d: step %d, err %v", d, got, err)
		}
	}
	for _, d := range []int64{-2, 2} {
		if _, err := VerifyTOTP(rfcSecret, TOTPCode(rfcSecret, step+d), now, 0); !errors.Is(err, ErrWrongCode) {
			t.Errorf("drift %d: err %v, want ErrWrongCode", d, err)
		}
	}
	code := TOTPCode(rfcSecret, step)
	if _, err := VerifyTOTP(rfcSecret, code, now, step); !errors.Is(err, ErrCodeUsed) {
		t.Errorf("reused: err %v, want ErrCodeUsed", err)
	}
	if _, err := VerifyTOTP(rfcSecret, TOTPCode(rfcSecret, step-1), now, step); !errors.Is(err, ErrCodeUsed) {
		t.Errorf("earlier step: err %v, want ErrCodeUsed", err)
	}
	if _, err := VerifyTOTP(rfcSecret, " "+code[:3]+" "+code[3:]+" ", now, 0); err != nil {
		t.Errorf("spaces: %v", err)
	}
	if _, err := VerifyTOTP(rfcSecret, "12345", now, 0); !errors.Is(err, ErrWrongCode) {
		t.Errorf("short: %v", err)
	}
}

func TestOTPAuthURI(t *testing.T) {
	got := OTPAuthURI("a+b@example.com", rfcSecret)
	want := "otpauth://totp/Bakery:a+b@example.com?algorithm=SHA1&digits=6&issuer=Bakery&period=30&secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, err := NewRecoveryCodes(bytes.NewReader(bytes.Repeat([]byte{0xab, 0x01, 0x77}, 100)))
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("%d codes", len(codes))
	}
	for _, c := range codes {
		if len(c) != 11 || c[5] != '-' || strings.ToLower(c) != c {
			t.Errorf("code %q", c)
		}
	}
	if NormalizeRecoveryCode(" ABCDE-fghij ") != "abcdefghij" {
		t.Error("normalize")
	}
}

func TestNewTOTPSecret(t *testing.T) {
	s, err := NewTOTPSecret(bytes.NewReader(make([]byte, 20)))
	if err != nil || len(s) != 20 {
		t.Fatalf("%v %d", err, len(s))
	}
	if _, err := NewTOTPSecret(bytes.NewReader(make([]byte, 3))); err == nil {
		t.Fatal("short random accepted")
	}
}
