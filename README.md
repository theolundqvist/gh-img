# gh-img

Upload local images to GitHub and get back `user-attachments` markdown URLs. Because GitHub asset URLs inherit the repository's visibility, images uploaded to a private repo stay private — they 404 for anyone without repo access.

macOS only (cookie decryption requires the macOS Keychain).

## Install

```sh
go build -o gh-img ./cmd/gh-img
# move to somewhere on $PATH, or use as a gh extension
```

## Usage

```sh
# auto-detect repo from git remote, session from browser
gh-img screenshot.png

# explicit repo
gh-img --repo owner/repo screenshot.png

# multiple images
gh-img --repo owner/repo a.png b.png c.png

# explicit session token (skips browser/Keychain)
gh-img --token <user_session value> screenshot.png

# or via env
GH_SESSION_TOKEN=<value> gh-img screenshot.png
```

Each image prints one markdown line to stdout:

```
![screenshot.png](https://github.com/user-attachments/assets/...)
```

Errors go to stderr. When uploading multiple images, `gh-img` continues on failure and exits non-zero if any failed.

## Security

`gh-img` reads your GitHub `user_session` browser cookie. That cookie is an unscoped, full-account credential — the same trust level as your password. It is used only to authenticate requests to `github.com` and GitHub's own S3 upload endpoint; it never leaves your machine for any other destination.

On first use for each browser, macOS will prompt for Keychain access to decrypt the cookie. This is expected.

On shared or CI machines, supply the token explicitly via `--token` or `GH_SESSION_TOKEN` instead of letting the tool read the browser.
