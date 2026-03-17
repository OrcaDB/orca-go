---
name: create-release
description: >-
  Create a new Go SDK release with auto-incremented semver tag and generated
  release notes. Use when the user says "create release", "new release",
  "cut a release", "tag a release", or invokes /create_release.
---

# Create Release

## Workflow

### Step 1: Gather state

Run these shell commands in parallel:

```bash
# Latest tag
git describe --tags --abbrev=0

# All tags (to verify uniqueness later)
git tag -l

# Commits since last tag
git log $(git describe --tags --abbrev=0)..HEAD --oneline

# Uncommitted changes
git status --porcelain
```

If there are uncommitted changes, **stop** and ask the user to commit first.
If there are no commits since the last tag, **stop** and tell the user there is nothing to release.

### Step 2: Determine next version

Parse the latest tag (e.g. `v0.1.2`) and auto-increment the **patch** number (e.g. `v0.1.3`).

If the user explicitly requested a specific version or bump level (major/minor), use that instead.

Confirm the new version tag is not already in the tag list from Step 1.

### Step 3: Generate release notes

From the commit log in Step 1, produce a bulleted summary of changes. Each bullet should be a concise, user-facing description — not a raw commit message. Group related commits into a single bullet when appropriate.

Format:

```
- Fixed/Added/Updated <what> — <brief why>
```

### Step 4: Confirm with user

Present the tag and release notes, then ask the user to confirm before proceeding.

### Step 5: Create tag and release

Run sequentially:

```bash
git tag -a <version> -m "<one-line summary>"
git push origin <version>
gh release create <version> --title "<version>" --notes "<release notes>"
```

### Step 6: Report

Print the GitHub release URL returned by `gh release create`.
