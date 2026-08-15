---
name: release
description: Cut a new huego release - bump version, tag, and push, triggering the GoReleaser GitHub Actions workflow that builds cross-platform binaries and publishes them to GitHub Releases. Use when the user asks to "release", "cut a release", "ship a new version", or similar.
---

# Release huego

huego releases are driven by GoReleaser (`.goreleaser.yaml`) via
`.github/workflows/release.yml`, which runs on any pushed tag matching `v*`.
Pushing the tag is the entire release step — GitHub Actions does the rest
(cross-compiling Linux/macOS/Windows for amd64+arm64, archiving, checksums,
and publishing the GitHub Release).

## Steps

1. **Make sure `main` is clean and pushed.**
   ```sh
   git status
   git push origin main
   ```
   If there are uncommitted changes intended for this release, commit them
   first (ask the user if unsure what belongs in the release).

2. **Determine the next version.** List existing tags and follow semver:
   ```sh
   git tag -l --sort=-v:refname
   ```
   - Bug fixes / no breaking changes / no new features → patch (`v0.1.0` → `v0.1.1`)
   - New backwards-compatible features → minor (`v0.1.0` → `v0.2.0`)
   - Breaking changes → major (`v0.1.0` → `v1.0.0`)

   If genuinely ambiguous, ask the user which bump they want rather than guessing.

3. **Tag and push:**
   ```sh
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

4. **Confirm the workflow started:**
   ```sh
   gh run list --workflow=release.yml --limit=1
   ```
   Report the run URL back to the user. Do not wait around polling unless
   asked — the workflow takes about a minute.

## Notes

- Never delete or move a tag that's already been pushed (`git tag -d` +
  re-push) without explicit confirmation — GoReleaser has likely already
  published a Release off it, and floating tags are confusing for anyone
  who already pulled it.
- If the workflow fails, check `gh run view --log-failed` before trying
  to fix `.goreleaser.yaml` blind.
- `cmd/root.go` wires `--version` via `-X github.com/mathisarends/huego/cmd.version=...`
  ldflags set in `.goreleaser.yaml` — don't rename that var without updating both.
