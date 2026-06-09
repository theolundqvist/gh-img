package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	ghimg "github.com/theolundqvist/gh-img"
)

var version = "dev"

func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	var (
		repoSpec string
		token    string
		showVer  bool
	)
	flag.StringVar(&repoSpec, "repo", "", "owner/repo (default: inferred from git remote)")
	flag.StringVar(&token, "token", "", "GitHub user_session token (overrides env and browser)")
	flag.BoolVar(&showVer, "version", false, "print version and exit")
	flag.Parse()

	if showVer {
		fmt.Println(resolveVersion())
		return
	}

	images := flag.Args()
	if len(images) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gh-img [--repo owner/repo] [--token <v>] <image>...")
		os.Exit(1)
	}

	tok, _, err := ghimg.SessionToken(token)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	owner, name, err := ghimg.Resolve(repoSpec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error resolving repo:", err)
		os.Exit(1)
	}

	uploader, err := ghimg.NewUploader(tok, owner, name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	failed := false
	for _, img := range images {
		result, err := uploader.Upload(img)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error uploading %s: %v\n", img, err)
			failed = true
			continue
		}
		fmt.Println(result.Markdown)
	}

	if failed {
		os.Exit(1)
	}
}
