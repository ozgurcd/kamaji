# Release builds

The release version is v0.3.0, which introduces graph builds and Homebrew
installation. The v0.2.0 release remains the earlier language-rule runner.
Historical notes are preserved in [RELEASE-v0.2.0.md](RELEASE-v0.2.0.md).
A development checkout can differ from its embedded version; a published tag
and its checksummed assets identify the released source and bytes.

`VERSION` is the version used by `make build` and `make release-assets`.
Both pass `-X kamaji/cmd.Version=<version>` to the Go linker. Plain `go build`
keeps the development value `dev`. Builds require the Go version in `go.mod`.

```sh
make build
./kamaji version
./kamaji --version
make release-assets
```

The release target cross-compiles macOS and Linux for AMD64 and ARM64 with
CGO disabled. Each archive contains `kamaji` and `LICENSE`; packaging disables
macOS copyfile metadata so AppleDouble sidecar files are not included. Outputs and
`SHA256SUMS` are under `.audit/release/<version>/`, which is ignored by Git.
Archive timestamps are not normalized; checksums identify the uploaded bytes,
not a promise of byte-identical archive reproduction. Build paths are trimmed
and automatic VCS stamping is disabled, so local unrelated files cannot stamp
a misleading dirty revision into an otherwise identical binary. The release
tag identifies the source revision.

Download the archive for your operating system and architecture and its
`SHA256SUMS` from the same release. In that download directory, verify the
selected archive (example for macOS ARM64):

```sh
shasum -a 256 kamaji_v0.3.0_darwin_arm64.tar.gz
```

Compare the digest with that archive's entry in `SHA256SUMS`, then extract it:

```sh
tar -xzf kamaji_v0.3.0_darwin_arm64.tar.gz
./kamaji version
```

The expected version is `v0.3.0`. Place the executable on your PATH if desired.
Checksums detect changed bytes; they are not independent publisher signatures.
The binaries are not signed or notarized.

## Publication scope

- Include core Go source/tests, module pins, user documentation, local wiki,
  version metadata and packaging recipes: these define and verify the release.
- Include the Go and multilingual workflow examples, their schemas, and their
  integration test: they demonstrate and verify the language-rule contract.
- For a graph release, also include `buildsys`, graph CLI source/tests, the
  `examples/build-project` source/configuration/walkthrough, and the agent JSON
  guide: these define and demonstrate the new build interface. Exclude generated
  example outputs, caches, run history, and compiled example programs.
- Retain existing tracked extension sources and dependency pins unchanged;
  extension behavior is not certified by the core or example tests.
- Include the root `RELEASE_NOTES.md` file and use its exact contents as the GitHub
  release body after preparing it for that numbered release. Preserve the prior
  contents in a versioned documentation file before writing the next version.
- Exclude untracked `rules/kubeseal` work: extension development is outside the
  authorized core scope. It remains on disk.
- Exclude local findings (`PROBLEMS.md`, `gograph-report.md`), `.audit`,
  `.gograph`, binaries, environment files, caches and local planning notes.
  Release binary archives are uploaded assets, not committed files.

Before publication, run the checks in [Testing](TESTING.md), inspect the exact
staged manifest, verify both version commands, and inspect archive members and
checksums. A cross-build does not establish runtime behavior on another platform.
The terminal integration test requires a real PTY and explicit synthetic input.

Commit only the reviewed source manifest, tag that commit, push the named branch
and tag, and publish using the prepared notes and assets. Verify the remote tag
resolves to the intended commit and read back release metadata and asset digests.
Do not move an existing published version tag to another commit.

## Homebrew publication

Kamaji follows the binary formula pattern used by rulefloor. The maintained
configuration is [homebrew/kamaji.rb.in](../homebrew/kamaji.rb.in); the published
formula belongs to `ozgurcd/homebrew-tap`, at `Formula/kamaji.rb`. The gograph
cask and rulefloor formula were inspected as references and are not modified.

After `make release-assets`, run `make homebrew-formula`. It verifies the existing
archive checksums and fills all platform URLs/digests from those exact archives.
Review `.audit/release/v0.3.0/homebrew/Formula/kamaji.rb`, publish and verify the
GitHub assets, then publish that generated formula to the tap. Do not rebuild
archives between generating the formula and uploading them: tar timestamps are
not normalized. The formula must identify the uploaded bytes.

Validate formula syntax/style, install the published formula, and run
`brew test ozgurcd/tap/kamaji`. Its test checks version/capabilities, read-only
planning, plan-bound execution, cache restoration, and saved history. Verify
`kamaji version` through the Homebrew-linked executable after installation.
See [the Homebrew guide](../homebrew/README.md) for end-user commands and scope.
