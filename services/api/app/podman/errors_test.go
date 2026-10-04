package podman

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"404", 404, `{"cause":"no such container","message":"no container with name or ID \"x\" found: no such container","response":404}`, true},
		{"removed while read", 500, `{"cause":"no such container","message":"no container with ID abc found in database: no such container","response":500}`, true},
		{"other 500", 500, `{"cause":"something else","message":"boom","response":500}`, false},
		{"409", 409, `{"cause":"container state improper","message":"x","response":409}`, false},
	}
	for _, c := range cases {
		err := readError(&http.Response{StatusCode: c.status, Body: io.NopCloser(strings.NewReader(c.body))})
		if got := IsNotFound(fmt.Errorf("wrapped: %w", err)); got != c.want {
			t.Errorf("%s: IsNotFound = %v, want %v", c.name, got, c.want)
		}
	}
}
