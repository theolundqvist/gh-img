package ghimg

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"time"
)

var reUsername = regexp.MustCompile(`<meta name="user-login" content="([^"]+)"`)

// CheckSession reports whether token is a live GitHub session, returning the
// login when the profile page exposes it (empty string otherwise).
func CheckSession(token string) (string, error) {
	req, err := newGitHubRequest("GET", "https://github.com/settings/profile", nil, token)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
	case 301, 302, 303:
		return "", fmt.Errorf("session invalid or expired (redirect to %s)", resp.Header.Get("Location"))
	default:
		return "", fmt.Errorf("unexpected status %d from GitHub", resp.StatusCode)
	}

	body, err := readBody(resp)
	if err != nil {
		return "", err
	}
	if m := reUsername.FindSubmatch(body); m != nil {
		return string(m[1]), nil
	}
	return "", nil
}

// SessionToken resolves the user_session token to use.
// Priority: tokenFlag > GH_SESSION_TOKEN env > browser cookie.
// Returns (token, source, err) where source is one of "flag", "env", "browser".
func SessionToken(tokenFlag string) (string, string, error) {
	if tokenFlag != "" {
		return tokenFlag, "flag", nil
	}
	if v := os.Getenv("GH_SESSION_TOKEN"); v != "" {
		return v, "env", nil
	}
	candidates, err := browserCandidates()
	if err != nil {
		return "", "", err
	}
	for _, tok := range candidates {
		if _, err := CheckSession(tok); err == nil {
			return tok, "browser", nil
		}
	}
	return "", "", fmt.Errorf("no valid GitHub session found in Arc/Chrome/Brave/Edge/Chromium — are you logged in?")
}
