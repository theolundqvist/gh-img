package ghimg

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"strings"
	"testing"
)

// --- crypto ---

func encryptForTest(plain []byte, password []byte, dbVersion int) []byte {
	key := deriveKey(password)
	block, _ := aes.NewCipher(key)

	// PKCS7 pad (clone first so we never write into the caller's slice)
	padLen := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(append([]byte{}, plain...), bytes.Repeat([]byte{byte(padLen)}, padLen)...)

	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, cbcIV[:]).CryptBlocks(ciphertext, padded)

	// build: "v10" + ciphertext
	var buf []byte
	buf = append(buf, 'v', '1', '0')
	buf = append(buf, ciphertext...)
	return buf
}

func TestDecryptCookie_RoundTrip(t *testing.T) {
	password := []byte("test-password")
	want := "mysecretcookievalue"

	for _, ver := range []int{23, 24} {
		plainInput := []byte(want)
		if ver >= 24 {
			plainInput = bytes.Repeat([]byte{0x42}, 32)
			plainInput = append(plainInput, []byte(want)...)
		}

		blob := encryptForTest(plainInput, password, ver)
		got, err := decryptCookie(blob, password, ver)
		if err != nil {
			t.Fatalf("ver %d: decryptCookie error: %v", ver, err)
		}
		if string(got) != want {
			t.Fatalf("ver %d: got %q, want %q", ver, got, want)
		}
	}
}

func TestDecryptCookie_InvalidPad(t *testing.T) {
	// blob with "v10" prefix and one AES block of all-zeros (pad byte = 0 → invalid)
	blob := append([]byte("v10"), bytes.Repeat([]byte{0}, aes.BlockSize)...)
	_, err := decryptCookie(blob, []byte("pw"), 23)
	if err == nil {
		t.Fatal("expected error for invalid padding, got nil")
	}
}

func TestDecryptCookie_WrongPrefix(t *testing.T) {
	blob := append([]byte("v99"), bytes.Repeat([]byte{0}, aes.BlockSize)...)
	_, err := decryptCookie(blob, []byte("pw"), 23)
	if err == nil {
		t.Fatal("expected error for wrong prefix")
	}
}

func TestDecryptCookie_TooShort(t *testing.T) {
	_, err := decryptCookie([]byte("v10"), []byte("pw"), 23)
	if err == nil {
		t.Fatal("expected error for too-short input")
	}
}

// --- hex decode (used in cookie reading path) ---

func TestHexDecode(t *testing.T) {
	raw := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	encoded := strings.ToUpper(hex.EncodeToString(raw))
	got, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("hex round-trip: got %v, want %v", got, raw)
	}
}

// --- repo URL parsing ---

func TestResolveFromRemote_SSH(t *testing.T) {
	cases := []struct{ url, owner, name string }{
		{"git@github.com:acme/myrepo.git", "acme", "myrepo"},
		{"git@github.com:acme/myrepo", "acme", "myrepo"},
	}
	for _, c := range cases {
		if !reSSH.MatchString(c.url) {
			t.Fatalf("SSH regex did not match %q", c.url)
		}
		m := reSSH.FindStringSubmatch(c.url)
		if m[1] != c.owner || m[2] != c.name {
			t.Fatalf("SSH: got %s/%s, want %s/%s", m[1], m[2], c.owner, c.name)
		}
	}
}

func TestResolveFromRemote_HTTPS(t *testing.T) {
	cases := []struct{ url, owner, name string }{
		{"https://github.com/acme/myrepo.git", "acme", "myrepo"},
		{"https://github.com/acme/myrepo", "acme", "myrepo"},
	}
	for _, c := range cases {
		m := reHTTPS.FindStringSubmatch(c.url)
		if m == nil {
			t.Fatalf("HTTPS regex did not match %q", c.url)
		}
		if m[1] != c.owner || m[2] != c.name {
			t.Fatalf("HTTPS: got %s/%s, want %s/%s", m[1], m[2], c.owner, c.name)
		}
	}
}

// --- repo ID scraping ---

func TestScrapeRepoID_Meta(t *testing.T) {
	html := []byte(`<meta name="octolytics-dimension-repository_id" content="12345678">`)
	id, err := scrapeRepoID(html, "acme", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if id != 12345678 {
		t.Fatalf("got %d, want 12345678", id)
	}
}

func TestScrapeRepoID_JSON1(t *testing.T) {
	html := []byte(`{"repository_id":99887766}`)
	id, err := scrapeRepoID(html, "acme", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if id != 99887766 {
		t.Fatalf("got %d, want 99887766", id)
	}
}

func TestScrapeRepoID_JSON2(t *testing.T) {
	html := []byte(`"repo":{"id":55443322}`)
	id, err := scrapeRepoID(html, "acme", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if id != 55443322 {
		t.Fatalf("got %d, want 55443322", id)
	}
}

// --- uploadToken extraction ---

func TestUploadTokenExtraction(t *testing.T) {
	html := []byte(`<div data-upload-policy-url="/upload/policies/assets" data-upload-authenticity-token="" ` +
		`data-target="upload-button.uploadButton" "uploadToken":"abc123xyz"`)
	m := reUploadToken.FindSubmatch(html)
	if m == nil {
		t.Fatal("uploadToken regex did not match")
	}
	if string(m[1]) != "abc123xyz" {
		t.Fatalf("got %q, want abc123xyz", m[1])
	}
}

// --- S3 form field ordering ---

func TestS3FieldOrdering(t *testing.T) {
	orderedKeys := s3FieldOrder

	// Simulate a form map with all required keys plus an extra.
	form := map[string]string{
		"x-amz-meta-Surrogate-Control": "val10",
		"Cache-Control":                "val9",
		"Content-Type":                 "val8",
		"X-Amz-Signature":              "val7",
		"X-Amz-Date":                   "val6",
		"X-Amz-Credential":             "val5",
		"X-Amz-Algorithm":              "val4",
		"policy":                       "val3",
		"acl":                          "val2",
		"key":                          "val1",
		"extra-field":                  "valX",
	}

	var written []string
	seen := make(map[string]bool)
	for _, k := range orderedKeys {
		if _, ok := form[k]; ok {
			written = append(written, k)
			seen[k] = true
		}
	}
	for k := range form {
		if !seen[k] {
			written = append(written, k)
		}
	}

	// Verify all ordered keys appear before extra-field.
	extraIdx := -1
	for i, k := range written {
		if k == "extra-field" {
			extraIdx = i
			break
		}
	}
	if extraIdx < 0 {
		t.Fatal("extra-field not found in output")
	}
	for _, k := range orderedKeys {
		for i, w := range written {
			if w == k && i > extraIdx {
				t.Fatalf("ordered key %q appeared after extra-field", k)
			}
		}
	}

	// Verify the first 10 entries match orderedKeys exactly.
	for i, k := range orderedKeys {
		if i >= len(written) {
			t.Fatalf("written too short at index %d", i)
		}
		if written[i] != k {
			t.Fatalf("position %d: got %q, want %q", i, written[i], k)
		}
	}
}
