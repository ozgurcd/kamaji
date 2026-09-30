# Homebrew distribution

Install the released binary on macOS or Linux (ARM64 or AMD64):

```sh
brew install ozgurcd/tap/kamaji
kamaji version
brew test ozgurcd/tap/kamaji
```

For this release, `kamaji version` prints `v0.3.0`. Future upgrades use:

```sh
brew update
brew upgrade ozgurcd/tap/kamaji
```

The formula downloads the platform archive from the matching GitHub release
and checks its SHA256 before installing `kamaji`. It does not compile Go or
install interpreters, extension commands, or Python packages. Install the
toolchains required by your own build targets separately. A prebuilt Go rule
does not need the Go compiler at execution time.

The formula has no services or startup hooks. It does not edit shell profiles
or remove project data. `brew uninstall kamaji` removes the Homebrew package;
project outputs, `.kamaji` cache/history, and legacy runner storage remain under
their own lifecycle. The binary is not signed or notarized; a checksum verifies
the release bytes, not an independent publisher signature.

## Maintainer workflow

The canonical template is [kamaji.rb.in](kamaji.rb.in). After preparing `VERSION`
and release notes, run from the repository root:

```sh
make release-assets
make homebrew-formula
```

The formula is generated under `.audit/release/<version>/homebrew/Formula/`.
The second command verifies `SHA256SUMS` and hashes all four platform archives;
it does not rebuild them. Publish those same archives, verify their remote
digests, and copy the generated `kamaji.rb` to `Formula/kamaji.rb` in
`ozgurcd/homebrew-tap`. Review and publish only that formula change. Versioned
release assets must not be replaced to fix a formula mismatch.

The formula uses the prebuilt-binary approach in rulefloor's formula. Gograph's
GoReleaser cask was also inspected; neither reference project is changed.
Kamaji retains its Makefile packaging and needs no release credential embedded
in its configuration. General formula conventions are documented in the
[Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook).

The formula test runs entirely on synthetic local files: version and capability
checks, a plan without storage creation, a plan-bound shell action, output
restoration from cache, and saved-result equality. It requires no Go/Python
runtime, network service, or infrastructure extension.

For release scope and archive verification, see [Releasing](../docs/RELEASING.md).
