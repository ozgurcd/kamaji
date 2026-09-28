# Release builds

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
shasum -a 256 kamaji_v0.2.0_darwin_arm64.tar.gz
```

Compare the digest with that archive's entry in `SHA256SUMS`, then extract it:

```sh
tar -xzf kamaji_v0.2.0_darwin_arm64.tar.gz
./kamaji version
```

The expected version is `v0.2.0`. Place the executable on your PATH if desired.
Checksums detect changed bytes; they are not independent publisher signatures.
The binaries are not signed or notarized.

## Publication scope

- Include core Go source/tests, module pins, user documentation, local wiki,
  version metadata and packaging recipes: these define and verify the release.
- Include the Go and multilingual workflow examples, their schemas, and their
  integration test: they demonstrate and verify the new execution contract.
- Retain existing tracked extension sources and dependency pins unchanged;
  extension behavior is not certified by the core or example tests.
- Include the root `RELEASE_NOTES.md` file and use its exact contents as the GitHub
  release body. It describes the current release; versioned historical notes
  and release records remain unchanged.
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
