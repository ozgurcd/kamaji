# Kamaji v0.2.0

Kamaji v0.2.0 adds generic rule execution and native Go rule support. Rules can
use a configured interpreter or run as compiled executables while sharing
Kamaji's configuration validation, execution deadlines, exit-code handling,
working-directory isolation, and cleanup.

## What's new

- Open-ended language support through the rule schema's `execution` block.
  Interpreter mode accepts an argument list such as `[node]`, `[ruby]`, or
  `[python3, -I]`; executable mode launches the configured rule file directly.
- Compiled Go rules with `language: go` or the `golang` alias. No Python runtime
  or Kamaji-specific Go SDK is required to execute a prebuilt Go rule.
- Language-aware `validate`, `doctor`, and `explain`, including mixed-language
  workspaces. Explain reports `language`, `execution_mode`, and `executable`.
- Runnable Go greeting and release-verification examples. The larger workflow
  combines Python file inventory, Go SHA256 and metadata verification, Ruby
  policy checks, and JavaScript Markdown reporting using standard libraries.
- Documentation covering structured JSON options, interpreter configuration,
  explicit target sequencing, policy failure codes, and retained isolated output.
- Regression tests for language dispatch and schemas, plus an integration test
  that runs the multilingual workflow with real installed language runtimes.

## Compatibility and migration

- Existing Python schemas retain their interpreter selection when `execution`
  is omitted. Missing `language` still defaults to Python.
- An explicit `execution` block takes precedence over language defaults. Explicit
  Python commands do not load Kamaji's user Python configuration or select its
  managed environment; Python selection flags apply to implicit Python mode.
- Every rule must have a sibling `rule_definition.yaml`, including direct
  executor embedding. Use `variables: {}` when the rule has no configuration.
- Other language labels require explicit execution settings. Interpreter commands
  are argv lists, not shell command strings. Invalid execution modes and unknown
  schema fields are rejected.
- Config values remain `--name=value` arguments: strings are literal; maps,
  lists, numbers, and booleans are JSON encoded. Rules parse their own arguments.
- Kamaji does not compile plugins, install arbitrary language runtimes, load Go
  shared-object plugins, infer target dependencies, or automatically promote
  outputs from isolated working copies.
- Rules retain the user's permissions. Working-copy isolation is not an OS sandbox.

## Installation

Download the archive matching your OS and architecture and the accompanying
`SHA256SUMS`. Available platforms are macOS and Linux, each for AMD64 and ARM64.
Each archive contains `kamaji` and `LICENSE`; interpreters, example sources, and
compiled example rules are not bundled. Example sources are in the tagged repository.

Verify the archive digest against `SHA256SUMS`, extract it, and run
`kamaji version`; the expected output is `v0.2.0`. Build-from-source instructions
require Go 1.27.1. Go dependency pins are unchanged from v0.1.0.

The binaries are not signed or notarized. Runtime verification was performed on
macOS ARM64; the other platform binaries are cross-built, not runtime-certified.
Bundled infrastructure extensions remain outside this release's fixes and tests.

## Documentation

- [Rule language contract](https://github.com/ozgurcd/kamaji/blob/v0.2.0/docs/RULE_LANGUAGES.md)
- [Go and multilingual examples](https://github.com/ozgurcd/kamaji/blob/v0.2.0/docs/EXAMPLES.md)
- [Build and checksum verification](https://github.com/ozgurcd/kamaji/blob/v0.2.0/docs/RELEASING.md)
- [Test evidence](https://github.com/ozgurcd/kamaji/blob/v0.2.0/docs/TESTING.md)
