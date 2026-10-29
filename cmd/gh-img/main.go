package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

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
	if len(os.Args) > 1 && os.Args[1] == "get" {
		os.Exit(runGet(os.Args[2:]))
	}

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

// flagsFirst reorders flags ahead of the positional URL so `get <url> -o f` parses.
func flagsFirst(args []string) []string {
	valueFlag := map[string]bool{"-o": true, "--o": true, "-token": true, "--token": true}
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if valueFlag[a] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return append(flags, positional...)
}

func runGet(args []string) int {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	var (
		token string
		out   string
	)
	fs.StringVar(&token, "token", "", "GitHub user_session token (overrides env and browser)")
	fs.StringVar(&out, "o", "", "write to file instead of stdout")
	_ = fs.Parse(flagsFirst(args))

	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "usage: gh-img get [--token <v>] [-o <file>] <url>")
		return 1
	}

	tok, _, err := ghimg.SessionToken(token)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	w := io.Writer(os.Stdout)
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	if err := ghimg.Download(rest[0], tok, w); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
