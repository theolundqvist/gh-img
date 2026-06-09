package ghimg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

var reUploadToken = regexp.MustCompile(`"uploadToken":"([^"]+)"`)

type Result struct {
	URL      string
	Name     string
	Markdown string
}

var defaultClient = &http.Client{Timeout: 30 * time.Second}

func newGitHubRequest(method, url string, body io.Reader, token string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "user_session", Value: token})
		// GitHub's upload endpoints reject the request for CSRF unless the
		// same-site twin is present; the browser gives it the same value.
		req.AddCookie(&http.Cookie{Name: "__Host-user_session_same_site", Value: token})
	}
	return req, nil
}

func readBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// Uploader uploads images to a single GitHub repo. NewUploader fetches the
// repo page once to resolve the uploadToken and numeric repo ID, which are
// then reused for every image.
type Uploader struct {
	token       string
	repoURL     string
	uploadToken string
	repoID      int
}

func NewUploader(token, owner, repo string) (*Uploader, error) {
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	req, err := newGitHubRequest("GET", repoURL, nil, token)
	if err != nil {
		return nil, err
	}
	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch repo page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch repo page: HTTP %d for %s", resp.StatusCode, repoURL)
	}
	pageBody, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	repoID, err := scrapeRepoID(pageBody, owner, repo)
	if err != nil {
		return nil, err
	}

	m := reUploadToken.FindSubmatch(pageBody)
	if m == nil {
		return nil, fmt.Errorf(
			"uploadToken not found on %s — no write access, or org SAML SSO not authorized (authorize at https://github.com/orgs/%s/sso)",
			repoURL, owner,
		)
	}

	return &Uploader{token: token, repoURL: repoURL, uploadToken: string(m[1]), repoID: repoID}, nil
}

func (u *Uploader) Upload(imagePath string) (*Result, error) {
	info, err := os.Stat(imagePath)
	if err != nil {
		return nil, err
	}
	baseName := filepath.Base(imagePath)
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(baseName)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var policyBuf bytes.Buffer
	pw := multipart.NewWriter(&policyBuf)
	for _, kv := range [][2]string{
		{"name", baseName},
		{"size", strconv.FormatInt(info.Size(), 10)},
		{"content_type", contentType},
		{"authenticity_token", u.uploadToken},
		{"repository_id", strconv.Itoa(u.repoID)},
	} {
		if err := pw.WriteField(kv[0], kv[1]); err != nil {
			return nil, err
		}
	}
	pw.Close()

	policyReq, err := newGitHubRequest("POST", "https://github.com/upload/policies/assets", &policyBuf, u.token)
	if err != nil {
		return nil, err
	}
	policyReq.Header.Set("Content-Type", pw.FormDataContentType())
	policyReq.Header.Set("Accept", "application/json")
	policyReq.Header.Set("Origin", "https://github.com")
	policyReq.Header.Set("Referer", u.repoURL)
	policyReq.Header.Set("X-Requested-With", "XMLHttpRequest")

	policyResp, err := defaultClient.Do(policyReq)
	if err != nil {
		return nil, fmt.Errorf("policies/assets: %w", err)
	}
	defer policyResp.Body.Close()
	if policyResp.StatusCode != 201 {
		body, _ := readBody(policyResp)
		return nil, fmt.Errorf("policies/assets: HTTP %d: %s", policyResp.StatusCode, body)
	}

	var policy policyResponse
	if err := json.NewDecoder(policyResp.Body).Decode(&policy); err != nil {
		return nil, fmt.Errorf("parse policies/assets response: %w", err)
	}
	if policy.UploadURL == "" || policy.Asset.ID == 0 || policy.AssetUploadAuthenticityToken == "" {
		return nil, fmt.Errorf("incomplete policies/assets response (missing upload_url, asset.id, or token)")
	}

	if err := uploadToS3(&policy, imagePath, baseName, contentType); err != nil {
		return nil, err
	}

	var confirmBuf bytes.Buffer
	cw := multipart.NewWriter(&confirmBuf)
	if err := cw.WriteField("authenticity_token", policy.AssetUploadAuthenticityToken); err != nil {
		return nil, err
	}
	cw.Close()

	confirmURL := fmt.Sprintf("https://github.com/upload/assets/%d", policy.Asset.ID)
	confirmReq, err := newGitHubRequest("PUT", confirmURL, &confirmBuf, u.token)
	if err != nil {
		return nil, err
	}
	confirmReq.Header.Set("Content-Type", cw.FormDataContentType())
	confirmReq.Header.Set("Accept", "application/json")
	confirmReq.Header.Set("Origin", "https://github.com")
	confirmReq.Header.Set("Referer", u.repoURL)
	confirmReq.Header.Set("X-Requested-With", "XMLHttpRequest")

	confirmResp, err := defaultClient.Do(confirmReq)
	if err != nil {
		return nil, fmt.Errorf("confirm upload: %w", err)
	}
	defer confirmResp.Body.Close()
	if confirmResp.StatusCode != 200 {
		body, _ := readBody(confirmResp)
		return nil, fmt.Errorf("confirm upload: HTTP %d: %s", confirmResp.StatusCode, body)
	}

	var asset struct {
		Href string `json:"href"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(confirmResp.Body).Decode(&asset); err != nil {
		return nil, fmt.Errorf("parse confirm response: %w", err)
	}

	return &Result{
		URL:      asset.Href,
		Name:     asset.Name,
		Markdown: fmt.Sprintf("![%s](%s)", asset.Name, asset.Href),
	}, nil
}

type policyResponse struct {
	UploadURL string `json:"upload_url"`
	Asset     struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Href string `json:"href"`
	} `json:"asset"`
	Form                         map[string]string `json:"form"`
	AssetUploadAuthenticityToken string            `json:"asset_upload_authenticity_token"`
}

var s3FieldOrder = []string{
	"key", "acl", "policy", "X-Amz-Algorithm", "X-Amz-Credential",
	"X-Amz-Date", "X-Amz-Signature", "Content-Type", "Cache-Control",
	"x-amz-meta-Surrogate-Control",
}

func uploadToS3(policy *policyResponse, imagePath, baseName, contentType string) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	seen := make(map[string]bool)
	for _, k := range s3FieldOrder {
		if v, ok := policy.Form[k]; ok {
			if err := w.WriteField(k, v); err != nil {
				return err
			}
			seen[k] = true
		}
	}
	for k, v := range policy.Form {
		if !seen[k] {
			if err := w.WriteField(k, v); err != nil {
				return err
			}
		}
	}

	// file field must be last
	fh := make(textproto.MIMEHeader)
	fh.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, baseName))
	fh.Set("Content-Type", contentType)
	part, err := w.CreatePart(fh)
	if err != nil {
		return err
	}
	f, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		f.Close()
		return err
	}
	f.Close()
	w.Close()

	req, err := http.NewRequest("POST", policy.UploadURL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Origin", "https://github.com")
	req.Header.Set("User-Agent", userAgent)

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("S3 upload: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
		return fmt.Errorf("S3 upload: HTTP %d", resp.StatusCode)
	}
	return nil
}
