# gh-img

Upload local images to GitHub from the command line and get back a `user-attachments` markdown URL. Because GitHub asset URLs inherit the repository's visibility, an image uploaded against a private repo **stays private** — it renders inline for anyone signed in with repo access and 404s for everyone else.

No third-party dependencies: pure Go standard library plus the macOS `security` and `sqlite3` binaries that already ship with the OS. It reads your existing GitHub browser session — no token to provision for everyday local use.

## Demo

The image below was uploaded to this repo by `gh-img` itself and embedded by its `user-attachments` URL:

![demo.png](https://github.com/user-attachments/assets/987c50cf-05bd-49b8-aa54-2224dd7b74d1)

## Install

As a `gh` extension (recommended):

```sh
gh extension install theolundqvist/gh-img
gh img screenshot.png
```

Or with Go:

```sh
go install github.com/theolundqvist/gh-img/cmd/gh-img@latest
```

Or build from source:

```sh
git clone https://github.com/theolundqvist/gh-img && cd gh-img
go build -o gh-img ./cmd/gh-img
```

## Usage

```sh
# repo auto-detected from the git remote, session read from your browser
gh img screenshot.png

# explicit repo
gh img --repo owner/repo screenshot.png

# multiple images (one markdown line printed per image)
gh img a.png b.png c.png

# explicit session token — works on any OS, never touches the browser/Keychain
gh img --token <user_session value> --repo owner/repo screenshot.png
GH_SESSION_TOKEN=<value> gh img --repo owner/repo screenshot.png
```

Each upload prints one markdown line to stdout:

```
![screenshot.png](https://github.com/user-attachments/assets/...)
```

Errors go to stderr. With multiple images, `gh-img` continues on failure and exits non-zero if any image failed.

## Platform support

| Path | macOS | Linux / Windows |
|---|---|---|
| Read session from browser | ✅ Arc, Chrome, Brave, Edge, Chromium | ❌ |
| `--token` / `GH_SESSION_TOKEN` | ✅ | ✅ |

The upload flow is platform-independent; only reading the cookie out of the browser is macOS-only (it relies on the macOS Keychain). On Linux/Windows, pass the token explicitly.

## How it works

1. Reads the `user_session` cookie for `github.com` from your browser's cookie database and decrypts it with the key stored in the macOS Keychain (PBKDF2-HMAC-SHA1 → AES-128-CBC, the standard Chromium scheme).
2. Validates the session, then drives GitHub's own upload flow: fetch the repo page for an upload token, request an upload policy, `POST` the file to the returned signed storage URL, and confirm the asset.

This is a clean-room reimplementation of what the GitHub web UI does when you drag an image into a comment box — there is no public API for it.

## Security

`gh-img` reads your GitHub `user_session` browser cookie. **That cookie is an unscoped, full-account credential — treat it like your password.** It is used only to authenticate requests to `github.com` and GitHub's own upload storage endpoint; it is never written to disk, never logged, and never sent anywhere else. The temporary copy of the cookie database is created with `0600` permissions and deleted immediately after reading.

On first use per browser, macOS prompts for Keychain access to decrypt the cookie. That prompt is the security boundary — it is expected.

On shared or CI machines, supply the token explicitly via `--token` or `GH_SESSION_TOKEN` rather than letting the tool read the browser.

## Limitations

- Browser cookie reading is macOS + Chromium-family only. Firefox and Safari are not supported (use `--token`).
- Uses an undocumented GitHub upload endpoint; if GitHub changes it, the tool will need updating.
- Requires write access to the target repo (GitHub scopes the upload token to repos you can push to).

## License

MIT — see [LICENSE](LICENSE).
