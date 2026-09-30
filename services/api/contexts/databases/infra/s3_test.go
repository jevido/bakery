package infra

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// The examples of AWS's "Signature Calculations for the Authorization
// Header" documentation for Amazon S3.
func TestSignAWSExamples(t *testing.T) {
	const (
		access = "AKIAIOSFODNN7EXAMPLE"
		secret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	)
	when := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, url, range_, signed, signature string
	}{
		{
			"GET object", "https://examplebucket.s3.amazonaws.com/test.txt", "bytes=0-9",
			"host;range;x-amz-content-sha256;x-amz-date",
			"f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41",
		},
		{
			"GET bucket list", "https://examplebucket.s3.amazonaws.com/?max-keys=2&prefix=J", "",
			"host;x-amz-content-sha256;x-amz-date",
			"34b48302e7b5fa45bde8084f4b7868a86f0a534bc59db6670ed5711ef69dc6f7",
		},
	} {
		req, _ := http.NewRequest(http.MethodGet, tc.url, nil)
		if tc.range_ != "" {
			req.Header.Set("Range", tc.range_)
		}
		sign(req, access, secret, "us-east-1", emptySHA256, when)
		want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=" +
			tc.signed + ", Signature=" + tc.signature
		if got := req.Header.Get("Authorization"); got != want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, want)
		}
	}
}

func TestURIEncode(t *testing.T) {
	if got := uriEncode("/b/nightly/main-1/2026 01.dump", false); got != "/b/nightly/main-1/2026%2001.dump" {
		t.Fatal(got)
	}
	if got := uriEncode("a/b=c", true); got != "a%2Fb%3Dc" {
		t.Fatal(got)
	}
	if !strings.Contains(canonicalQuery(map[string][]string{"prefix": {"a/"}, "list-type": {"2"}}), "list-type=2&prefix=a%2F") {
		t.Fatal(canonicalQuery(map[string][]string{"prefix": {"a/"}, "list-type": {"2"}}))
	}
}
