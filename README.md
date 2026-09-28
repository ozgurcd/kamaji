# Kamaji

Kamaji is a Go CLI that runs YAML-defined targets through Python rules. It
validates configuration, downloads checksum-pinned dependencies, prepares
private execution files, and passes options and trailing arguments to a rule.
Rules implement integrations with other tools; the Go core does not provide
authentication or certify those integrations.

## Build and start

The module requires Go 1.27.1. Runtime storage supports macOS and Linux.
Python is needed to execute rules, but not to display help or list targets.

```sh
make build
./kamaji --help
./kamaji version
```

The current version is v0.1.0. `make build` embeds the version from `VERSION`;
plain `go build` produces a development build reporting `dev`.
Download platform binaries and checksums from the
[v0.1.0 release](https://github.com/ozgurcd/kamaji/releases/tag/v0.1.0).
See [Release builds](docs/RELEASING.md) for packaging and verification.

From a new project directory, scaffold a minimal workspace and inspect it:

```sh
kamaji init
kamaji targets
kamaji validate hello
kamaji run hello -- extra-argument
```

`kamaji hello` is also supported. Use `--build` for another build file and `--`
to separate Kamaji flags from arguments forwarded to the rule.

## Core behavior

- Strict YAML parsing rejects unknown fixed fields, duplicate keys/names, and
  extra documents. Rule schemas enforce supported types, defaults, and required
  values, with optional enums, numeric bounds, and unknown-option rejection.
- Shared preflight, all-target validation, doctor, and redacted explain commands
  help diagnose setup before execution. Target descriptions appear in listings
  and shell completion.
- Dependencies use platform-specific URLs and SHA256 digests. Cache publication
  is atomic; cached payloads are verified before reuse.
- Downloads default to 512 MiB; archive expansion defaults to 2 GiB of file data
  and 10,000 members per archive. Workspace configuration can override these.
- Execution directories are removed after ordinary completion or failure.
  `--keep-execroot` enables debugging retention bounded by count and bytes.
- `--isolated` runs in an independent working-directory copy. It is not an OS
  sandbox: rules retain the user's permissions and inherited environment.
- Cleanup coordinates with active runs. Python setup selects a new environment
  only after setup succeeds; user-scoped installation and explicit requirements
  are supported. Installation and removal share a lock.
- Execution supports deadlines and graceful process-group cancellation, preserves
  child exit codes, and restores foreground terminal ownership after interactive
  children. Cache and retained-run inspection includes deletion previews.

## Documentation

- [Usage and configuration](docs/HOW_TO_USE.md)
- [CLI commands and flags](docs/CLI_REFERENCE.md)
- [Commands, Python setup, cleanup, retention, and embedding](docs/RUNTIME_LIFECYCLE.md)
- [Download and archive limits](docs/RESOURCE_LIMITS.md)
- [Executor trust boundary](docs/EXECUTOR_SECURITY.md)
- [Dependencies and YAML choice](docs/DEPENDENCIES.md)
- [Tests and recorded validation](docs/TESTING.md)
- [Review findings and implementation follow-up](docs/GENERAL_REVIEW.md)
- [Repository-local wiki](wiki/index.md)

Extension fixes remain outside the completed core work. Historical audits
describe earlier trees and are labeled separately from current guides.

## Contributing and license

Run the checks in [Testing](docs/TESTING.md) when changing the core. Issues and
pull requests are welcome. Kamaji is distributed under the [MIT License](LICENSE).
