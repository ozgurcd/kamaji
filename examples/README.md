# Runnable examples

Use Kamaji v0.3.0 or newer for the artifact graph example. The Go greeting and
multilingual release workflow also run with published Kamaji v0.2.0. Generic
language execution is not available in the older v0.1.0 binaries.

| Example | Languages | What it demonstrates |
| --- | --- | --- |
| [Artifact build graph](build-project/README.md) | Go | TOML graph, generated executable, dependency scheduling, local cache restoration, and verification |
| [Go greeting](go-rule/BUILD.yaml) | Go | A compiled rule, scalar flags, and positional arguments containing spaces |
| [Release workflow](release-workflow/BUILD.yaml) | Python, Go, Ruby, JavaScript | JSON maps/lists, file hashing, metadata verification, policy failures, Markdown output, and isolation |

The [examples guide](../docs/EXAMPLES.md) contains build/run commands, expected
results, and a walkthrough of the more involved Go verifier. All example programs
use their language's standard library. No bundled infrastructure extension,
cloud account, or package installation is needed; install the language runtimes
you intend to use.

Outputs and compiled rule binaries are ignored by the examples' `.gitignore`
files. These examples operate on trusted local inputs; hashes detect changes
relative to the inventory, not publisher authenticity.
