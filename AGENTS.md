# Agent notes

`gh-img` exists so an agent can put **inline images on a GitHub PR** without a human drag-drop and without trusting a third-party cookie tool.

## Use it

- `gh img <image>...` — uploads to the repo inferred from the git remote; prints one `![name](url)` line per image. Paste those lines straight into a PR description or comment.
- `gh img --repo owner/repo <image>...` — explicit repo.
- `gh img --token "$GH_SESSION_TOKEN" --repo owner/repo <image>` — CI / non-macOS / Firefox / Safari: pass the `user_session` value directly; no browser is read.

## When to reach for it

Any PR where a screenshot beats prose — especially **frontend / desktop UI changes**. Capture the changed states (drive the app headless via the project's own run/verify path), then `gh img` them and edit the markdown into the PR body. Show before/after for visual changes.

## Constraints

- Browser cookie reading is **macOS + Chromium-family only** (Arc, Chrome, Brave, Edge, Chromium). Firefox, Safari, Linux, Windows → use `--token` / `GH_SESSION_TOKEN`.
- Needs write access to the target repo.
- The `user_session` cookie is unscoped, full-account access. Never echo it into logs, commits, or PR text; prefer `GH_SESSION_TOKEN` on shared machines.
