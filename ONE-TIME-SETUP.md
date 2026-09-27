# One-time automated release setup

1. Copy **everything inside this folder** into the root of your local `Latch` repository. Keep the `.github` folder; Windows Explorer may hide folders beginning with a dot.
2. In GitHub Desktop, confirm the changed files include `.github/workflows/release.yml`, `scripts/build-release.sh`, `compiler-src`, `vscode-extension`, `lib`, `VERSION`, and this documentation.
3. Commit with a message such as `Add automated Latch release pipeline`, then **Push origin**.
4. On GitHub, open **Settings → Actions → General**. Under **Workflow permissions**, select **Read and write permissions**, then Save. This is required so the workflow can create releases and update `release-manifest.json`.
5. For the first automated release, create/push the tag `v1.0.0-alpha.2.5` from GitHub Desktop: **Repository → Create Tag…** (or the Branch/Repository tag command shown by your Desktop version), enter `v1.0.0-alpha.2.5`, then push the tag. If Desktop does not expose tag creation, create the release tag on GitHub once; the workflow will run when the tag is pushed.
6. Open the repository's **Actions** tab and select **Build and publish Latch**. The run must be green before treating the release as published.

## Future releases

Change the source and VERSION, commit/push, then push the matching `v<version>` tag. GitHub Actions handles building, tests, release assets, and the public updater manifest. You no longer edit `release-manifest.json` manually or assemble releases in the browser.

Important: the updater remains confirmation-based. Installed Latch checks the manifest and offers **Install Update**; it does not silently replace itself.
