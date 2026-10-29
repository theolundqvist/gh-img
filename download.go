package ghimg

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"
)

var allowedGetHosts = map[string]bool{
	"github.com": true,
	"private-user-images.githubusercontent.com": true,
}

// validateGetURL rejects any host the session cookie must not be sent to.
func validateGetURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("refusing non-https URL %q", rawURL)
	}
	if !allowedGetHosts[u.Host] {
		return nil, fmt.Errorf("refusing host %q — only github.com and private-user-images.githubusercontent.com are allowed", u.Host)
	}
	return u, nil
}

// stripCookieLeavingGitHub drops the session cookie from redirects off github.com.
func stripCookieLeavingGitHub(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if req.URL.Host != "github.com" {
		req.Header.Del("Cookie")
	}
	return nil
}

// Download streams a GitHub asset URL to w, authenticated by the session token.
func Download(rawURL, token string, w io.Writer) error {
	u, err := validateGetURL(rawURL)
	if err != nil {
		return err
	}
	reqToken := ""
	if u.Host == "github.com" {
		reqToken = token
	}
	req, err := newGitHubRequest("GET", rawURL, nil, reqToken)
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout:       120 * time.Second,
		CheckRedirect: stripCookieLeavingGitHub,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("GET %s: HTTP %d", rawURL, resp.StatusCode)
	}
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return fmt.Errorf("GET %s returned an HTML page instead of an asset; authorize the selected browser's GitHub SSO session", rawURL)
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("write asset: %w", err)
	}
	return nil
}
