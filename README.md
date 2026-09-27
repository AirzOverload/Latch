# Latch

Latch is an experimental C-family language centered on explicit mutation authority through `latch(...)` contracts.

## Automated releases

The repository contains `.github/workflows/release.yml`. A version tag such as `v1.0.0-alpha.2.5` builds and tests the Windows compiler, packages the VS Code extension and SDK, creates/updates the GitHub Release, uploads the assets, and updates `release-manifest.json` on `main`.

The installed VS Code extension reads the public manifest and can install a newer VSIX after user confirmation.
