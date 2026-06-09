//go:build darwin

package ghimg

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type browser struct {
	supportDir string
	account    string
}

var browsers = []browser{
	{"Arc/User Data", "Arc"},
	{"Google/Chrome", "Chrome"},
	{"BraveSoftware/Brave-Browser", "Brave"},
	{"Microsoft Edge", "Microsoft Edge"},
	{"Chromium", "Chromium"},
}

func keychainPassword(account string) ([]byte, error) {
	service := account + " Safe Storage"
	out, err := exec.Command(
		"/usr/bin/security", "find-generic-password",
		"-s", service,
		"-wa", account,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("keychain lookup for %q: %w", account, err)
	}
	return []byte(strings.TrimRight(string(out), "\n")), nil
}

func cookieFiles(supportDir string) []string {
	base := filepath.Join(os.Getenv("HOME"), "Library", "Application Support", supportDir)
	var files []string
	for _, profile := range []string{"Default"} {
		for _, sub := range []string{"Network/Cookies", "Cookies"} {
			p := filepath.Join(base, profile, sub)
			if _, err := os.Stat(p); err == nil {
				files = append(files, p)
			}
		}
	}
	// glob Profile * directories
	globbed, _ := filepath.Glob(filepath.Join(base, "Profile *"))
	for _, dir := range globbed {
		for _, sub := range []string{"Network/Cookies", "Cookies"} {
			p := filepath.Join(dir, sub)
			if _, err := os.Stat(p); err == nil {
				files = append(files, p)
			}
		}
	}
	return files
}

// copyCookieDB copies path and its -wal/-shm siblings into a temp dir.
// Caller must remove the returned dir.
func copyCookieDB(src string) (string, string, error) {
	tmp, err := os.MkdirTemp("", "ghimg-cookies-*")
	if err != nil {
		return "", "", err
	}
	dst := filepath.Join(tmp, "Cookies")
	if err := copyFile(src, dst); err != nil {
		os.RemoveAll(tmp)
		return "", "", err
	}
	for _, ext := range []string{"-wal", "-shm"} {
		_ = copyFile(src+ext, dst+ext) // ok if absent
	}
	return tmp, dst, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

func sqliteQuery(dbPath, query string) ([]string, error) {
	out, err := exec.Command("/usr/bin/sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return nil, fmt.Errorf("sqlite3: %w", err)
	}
	var rows []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line != "" {
			rows = append(rows, line)
		}
	}
	return rows, nil
}

func dbVersion(dbPath string) (int, error) {
	rows, err := sqliteQuery(dbPath, "SELECT value FROM meta WHERE key='version';")
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errors.New("no version row in meta table")
	}
	v, err := strconv.Atoi(rows[0])
	if err != nil {
		return 0, fmt.Errorf("parse db version %q: %w", rows[0], err)
	}
	return v, nil
}

func encryptedUserSessions(dbPath string) ([][]byte, error) {
	rows, err := sqliteQuery(dbPath,
		"SELECT hex(encrypted_value) FROM cookies WHERE name='user_session' AND host_key IN ('github.com','.github.com');")
	if err != nil {
		return nil, err
	}
	var out [][]byte
	for _, row := range rows {
		b, err := hex.DecodeString(row)
		if err != nil {
			continue
		}
		if len(b) > 0 {
			out = append(out, b)
		}
	}
	return out, nil
}

func browserCandidates() ([]string, error) {
	var candidates []string
	var errs []string

	for _, b := range browsers {
		files := cookieFiles(b.supportDir)
		if len(files) == 0 {
			continue
		}
		pw, err := keychainPassword(b.account)
		if err != nil {
			// browser installed but keychain lookup failed; skip silently
			continue
		}
		for _, cookieFile := range files {
			tmp, dbCopy, err := copyCookieDB(cookieFile)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: copy db: %v", b.account, err))
				continue
			}
			ver, err := dbVersion(dbCopy)
			if err != nil {
				os.RemoveAll(tmp)
				errs = append(errs, fmt.Sprintf("%s: db version: %v", b.account, err))
				continue
			}
			blobs, err := encryptedUserSessions(dbCopy)
			os.RemoveAll(tmp)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: query cookies: %v", b.account, err))
				continue
			}
			for _, blob := range blobs {
				plain, err := decryptCookie(blob, pw, ver)
				if err != nil {
					continue
				}
				if v := string(plain); v != "" {
					candidates = append(candidates, v)
				}
			}
		}
	}
	if len(candidates) == 0 {
		msg := "no valid GitHub session found in Arc/Chrome/Brave/Edge/Chromium — are you logged in?"
		if len(errs) > 0 {
			msg += "\n" + strings.Join(errs, "\n")
		}
		return nil, errors.New(msg)
	}
	return candidates, nil
}
