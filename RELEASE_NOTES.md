# Kamaji v0.3.0

Kamaji v0.3.0 adds a lightweight, language-neutral build graph with verified
local artifact caching and structured interfaces for coding agents. It also
adds Homebrew installation. Existing language-rule workspaces remain supported.

## What's new

- A single `kamaji.toml` project document, with equivalent graph-schema YAML.
  Explicit targets declare argv commands, dependencies, inputs, outputs,
  environment, tools, timeouts, effects, and scheduling slots.
- Dependency validation, generated tools and inputs, parallel scheduling,
  process-group cancellation, and preserved child exit statuses.
- Opt-in content-based artifact caching. Cache hits recheck payload digests;
  missing or changed outputs are restored and corrupt entries are rebuilt.
- Read-only plans, plan-bound execution with `--expect-plan`, affected-target
  selection, saved build history, and cleanup previews.
- Versioned JSON capabilities, plans, events, results, and errors. Machine output
  remains separate from child diagnostics. Declared external-effect actions
  require `--allow-effects` and cannot be cached.
- A complete Go compilation/generation/verification example, an agent workflow
  wrapper, and guides for cache assumptions, JSON contracts, and resource limits.
- A checksummed binary Homebrew formula for macOS/Linux on ARM64/AMD64, with a
  local build/cache/history smoke test.

## Compatibility and migration

`kamaji init` now creates a graph project. Use `kamaji init --template minimal`
for the previous YAML language-rule scaffold. Existing `run` commands and
language schemas continue to work; `targets`, `validate`, `doctor`, and `explain`
recognize both project models. Graph commands accept `--file`; legacy commands
retain `--build`. The graph engine does not interpret legacy rule schemas or
expand their `@@` dependency references.

Go compilation is an explicit build target, not an inferred plugin operation.
Graph builds use declared inputs and tools, not automatic import discovery.
Commands keep user permissions: effect declarations, a restricted inherited
environment, and working-copy isolation are not an OS sandbox. Undeclared input,
SDK, library, network, or clock dependencies can invalidate cache assumptions.
There is no remote execution/cache service or embedded model provider.

Project-local `.kamaji/cache` and `.kamaji/runs` persist until explicitly cleaned.
The older `cache` and `runs` commands manage legacy storage separately. Graph
builds have no automatic total disk quota, and failures do not roll back earlier
outputs. Legacy download/archive limits do not constrain arbitrary child commands.

## Installation

```sh
brew install ozgurcd/tap/kamaji
kamaji version
brew test ozgurcd/tap/kamaji
```

The expected version is `v0.3.0`. Alternatively, download your platform archive
and `SHA256SUMS` from this release, verify its digest, and extract it. Archives
contain `kamaji` and `LICENSE`; example sources are in the tagged repository.
Go 1.27.1 is required to build from source, but not to run the released binary.
This release adds `github.com/pelletier/go-toml/v2 v2.4.3`; one YAML parser remains.

Binaries are not signed or notarized. Native validation is on macOS ARM64;
Linux and macOS AMD64 assets are cross-built, not runtime-certified. Infrastructure
extensions remain outside this release's fixes and tests.

## Documentation

- [Build configuration](https://github.com/ozgurcd/kamaji/blob/v0.3.0/docs/HOW_TO_USE.md)
- [Agent workflows and JSON contracts](https://github.com/ozgurcd/kamaji/blob/v0.3.0/docs/AGENT_WORKFLOWS.md)
- [Go build walkthrough](https://github.com/ozgurcd/kamaji/blob/v0.3.0/examples/build-project/README.md)
- [Homebrew installation and maintenance](https://github.com/ozgurcd/kamaji/blob/v0.3.0/homebrew/README.md)
- [Verification evidence](https://github.com/ozgurcd/kamaji/blob/v0.3.0/docs/TESTING.md)
- [Previous v0.2.0 release notes](https://github.com/ozgurcd/kamaji/blob/v0.3.0/docs/RELEASE-v0.2.0.md)
