package ghimg

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

type assetTransport func(*http.Request) (*http.Response, error)

func (f assetTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadRejectsLoginPageWithoutWritingIt(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, test := range []struct {
		contentType string
		body        string
		wantError   bool
	}{
		{"text/html; charset=utf-8", "<!DOCTYPE html><title>Sign in</title>", true},
		{"application/xhtml+xml", "<html>SSO required</html>", true},
		{"image/png", "\x89PNG\r\n\x1a\n", false},
	} {
		t.Run(test.contentType, func(t *testing.T) {
			http.DefaultTransport = assetTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {test.contentType}}, Body: io.NopCloser(strings.NewReader(test.body)), Request: r}, nil
			})
			var output bytes.Buffer
			err := Download("https://github.com/user-attachments/assets/example", "synthetic-session", &output)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v; wantError = %v", err, test.wantError)
			}
			want := test.body
			if test.wantError {
				want = ""
			}
			if output.String() != want {
				t.Fatalf("download wrote %q, want %q", output.String(), want)
			}
		})
	}
}
