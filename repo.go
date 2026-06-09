package ghimg

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	reSSH   = regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
	reHTTPS = regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)

	reRepoIDMeta  = regexp.MustCompile(`octolytics-dimension-repository_id" content="(\d+)"`)
	reRepoIDJSON1 = regexp.MustCompile(`"repository_id":(\d+)`)
	reRepoIDJSON2 = regexp.MustCompile(`"repo":\{"id":(\d+)`)
)

// Resolve returns the owner and repo name. If spec is empty they are inferred
// from the git remote.
func Resolve(spec string) (owner, name string, err error) {
	if spec == "" {
		return resolveFromRemote()
	}
	parts := strings.SplitN(spec, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repo spec must be owner/name, got %q", spec)
	}
	return parts[0], parts[1], nil
}

func resolveFromRemote() (owner, name string, err error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	url := strings.TrimSpace(string(out))
	if m := reSSH.FindStringSubmatch(url); m != nil {
		return m[1], m[2], nil
	}
	if m := reHTTPS.FindStringSubmatch(url); m != nil {
		return m[1], m[2], nil
	}
	return "", "", fmt.Errorf("cannot parse GitHub remote URL: %q", url)
}

func scrapeRepoID(body []byte, owner, name string) (int, error) {
	for _, re := range []*regexp.Regexp{reRepoIDMeta, reRepoIDJSON1, reRepoIDJSON2} {
		if m := re.FindSubmatch(body); m != nil {
			v, err := strconv.Atoi(string(m[1]))
			if err == nil {
				return v, nil
			}
		}
	}
	// fallback: use gh CLI
	out, err := exec.Command("gh", "api", "repos/"+owner+"/"+name, "--jq", ".id").Output()
	if err != nil {
		return 0, fmt.Errorf("could not determine repo ID for %s/%s (gh api fallback failed: %w)", owner, name, err)
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse gh api repo id: %w", err)
	}
	return v, nil
}
