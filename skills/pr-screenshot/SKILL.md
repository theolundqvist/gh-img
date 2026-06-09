---
name: pr-screenshot
description: Attach inline screenshots to a GitHub PR using gh-img. Uploads local image(s) and embeds them in the PR description (or as a comment). For UI changes with no images yet, capture them first via the project's run/verify path, then attach. Use when a PR — especially a frontend/desktop UI change — should carry a screenshot.
allowed-tools: Bash, Read, Glob
---

**Arguments:** `$ARGUMENTS` — image path(s), optional PR number, optional `--comment` (post as a comment instead of editing the description).

Put inline screenshots on a PR with [`gh-img`](https://github.com/theolundqvist/gh-img), which uploads via GitHub's `user-attachments` flow so images render inline yet inherit repo visibility (private repo → private image).

## 1. Get the images

- If image paths are given in the arguments, use them.
- If the task is "screenshot the UI" and no images exist yet, **capture first** — drive the app headless via the project's own run/verify skill (e.g. `/verify` or `/run`) or its documented screenshot path, writing PNGs to a gitignored scratch dir. Capture before/after for visual changes. Do not reimplement capture here.
- If neither applies, ask which images to attach.

## 2. Resolve the PR and repo

```bash
gh pr view ${PR:-} --json number,url -q '.number'   # current branch's PR if no number given
gh repo view --json nameWithOwner -q '.nameWithOwner'
```

## 3. Upload each image

```bash
gh img --repo <owner/repo> <image>...   # prints one ![name](url) markdown line per image
```

- If `gh img` is missing: `gh extension install theolundqvist/gh-img`.
- First run per session may need a one-time macOS Keychain "Allow" and a permission OK — that's expected.
- On CI / Linux / Windows / Firefox / Safari, browser reading is unavailable: pass `gh img --token "$GH_SESSION_TOKEN" ...`.

Collect the printed `![…](…)` lines.

## 4. Attach to the PR

Default — append a screenshots section to the description, preserving the existing body:

```bash
body=$(gh pr view <number> --json body -q '.body')
printf '%s\n\n## Screenshots\n\n%s\n' "$body" "$markdown" | gh pr edit <number> --body-file -
```

With `--comment`, post instead of editing:

```bash
gh pr comment <number> --body "$markdown"
```

## 5. Confirm

Report the PR URL. The image renders inline for anyone with repo access; on a private repo it 404s for everyone else. Never paste the `user_session` value or token into the PR text or logs.
