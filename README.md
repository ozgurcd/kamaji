# Kamaji

Kamaji is a lightweight Go build and automation CLI. A single `kamaji.toml`
declares commands, dependencies, inputs, and outputs. Kamaji schedules independent
work, reuses verified local artifacts, and exposes structured plans and results
for developers and coding agents. Existing YAML-defined language rules remain
available through `run`.

The graph build system is available starting with **v0.3.0**. The v0.2.0
binaries provide the earlier language-independent rule runner.

## Install with Homebrew

```sh
brew install ozgurcd/tap/kamaji
kamaji version
```

The formula installs a checksummed native binary for macOS or Linux, on ARM64
or AMD64. It does not install language runtimes or extension tools. See
[Homebrew installation and maintenance](homebrew/README.md).

## Build and start

The module requires Go 1.27.1. Runtime storage supports macOS and Linux.
Python is needed only for Python rules. Compiled Go rules run directly;
other languages declare their interpreter or executable mode in the rule schema.

```sh
make build
./kamaji --help
./kamaji version
```

The current version is v0.3.0. `make build` embeds the version from `VERSION`;
plain `go build` produces a development build reporting `dev`.
Download platform binaries and checksums from the
[v0.3.0 release](https://github.com/ozgurcd/kamaji/releases/tag/v0.3.0).
See [Release builds](docs/RELEASING.md) for packaging and verification.

Generic language execution and Go rule support are available starting with
v0.2.0. See [release notes](RELEASE_NOTES.md) for changes and compatibility details.
The older v0.1.0 binaries support Python rules only.

## Examples

### Start a build project

With Kamaji v0.3.0 or newer on PATH:

```sh
kamaji init
kamaji plan
kamaji build
```

This creates a runnable `kamaji.toml`. Commands are argument lists; no shell is
inserted. For a real compiled Go pipeline, from the repository root:

```sh
make build
cd examples/build-project
../../kamaji plan --json
../../kamaji build --jobs 2
../../kamaji build --json
```

The [example walkthrough](examples/build-project/README.md) compiles a Go tool, uses it to
generate a greeting file, and verifies the result. Subsequent builds reuse the
compiled tool and generated file while rerunning the verification. Missing
outputs are restored from a verified cache; changed source or options invalidate
the relevant action. Declared outputs live at their configured paths; cached
artifacts and execution evidence live under `.kamaji/`.

For an agent workflow, `capabilities` describes the JSON interface, `plan --json`
returns a content-bound plan ID, and `build --expect-plan` accepts that ID to
reject stale plans. `build --events` streams JSON status events and a final
result; child output goes to stderr. `affected` reports reverse dependencies of
changed paths, and `history` reads a recorded build result. Targets declared
`effect = "external"` require `--allow-effects` and cannot be cached.

See [build configuration](docs/HOW_TO_USE.md) for cache limitations and cleanup,
and [agent workflows](docs/AGENT_WORKFLOWS.md) for JSON contracts and a complete
plan/build/history wrapper. No LLM service,
daemon, or language SDK is required.

### Language rule examples

Start with the [runnable examples guide](docs/EXAMPLES.md) for Go and a
multilingual release workflow. The [example catalog](examples/README.md) links
to the complete source, schemas, and build files.

The Python examples assume `kamaji` is on your PATH and `python3` is installed.
They use local files and Python's standard library; no cloud account, package
installation, or bundled extension is needed.

### Run a compiled Go rule

The repository includes a complete [Go example](examples/go-rule/BUILD.yaml).
From the repository root, build Kamaji and the rule, then run it:

```sh
make build
go build -o examples/go-rule/rules/hello/hello ./examples/go-rule/rules/hello
cd examples/go-rule
../../kamaji validate hello-go
../../kamaji run hello-go -- Ada "Grace Hopper"
```

Output:

```text
Hello from Go, Ada!
Hello from Go, Grace Hopper!
```

The rule schema declares `language: go`; the target points to the compiled
`hello/hello` executable. Kamaji passes config values as `--name=value` options
and trailing arguments unchanged. Go is needed to build the rule, but neither
Go nor Python is needed to run a prebuilt rule. Build for the machine where it
will run. See [Rule languages](docs/RULE_LANGUAGES.md) for the complete contract.

### Verify a release with Go and other languages

The [release workflow](examples/release-workflow/BUILD.yaml) uses Python to
inventory files, a compiled Go rule to verify their hashes and metadata, Ruby
to enforce a size budget, and JavaScript to write a Markdown report. It uses
local sample files and standard libraries. Install `python3`, `ruby`, and
`node`; from the repository root:

```sh
make build
go build -o examples/release-workflow/rules/verify/verify ./examples/release-workflow/rules/verify
cd examples/release-workflow
../../kamaji validate --all
(
  set -e
  ../../kamaji run inventory --timeout 30s
  ../../kamaji run verify-go --timeout 30s
  ../../kamaji run policy --timeout 30s
  ../../kamaji run report --timeout 30s
)
```

The workflow creates `out/inventory.json` and `out/report.md`. The
[Go verifier](examples/release-workflow/rules/verify/main.go) demonstrates
`flag`, `encoding/json`, streaming SHA256 checks, and nonzero failure exits.
It accepts an expected metadata map as a JSON flag value and detects changes
to the inventoried files.

These legacy `BUILD.yaml` commands run in explicit order. New `kamaji.toml`
build projects declare graph dependencies through `deps`. Each legacy stage
has its own schema and can also run independently when its
inputs exist. The [full walkthrough](docs/EXAMPLES.md#a-multilingual-release-workflow)
shows a deliberate policy failure using `BUILD.strict.yaml`, structured
options across languages, and retained isolated output.

### Use another language without changing Kamaji

Set execution details in the rule's `rule_definition.yaml`. For example, a
JavaScript rule can declare:

```yaml
language: javascript
execution:
  mode: interpreter
  command: [node, --enable-source-maps]
variables: {}
```

The target's `rule` points to its JavaScript file. Kamaji runs `node` with the
fixed flag, rule path, configured options, and trailing arguments as separate
arguments. Install the selected runtime yourself. For Rust, C, or another
compiled language, declare `execution: {mode: executable}` and point `rule`
to its binary. Language names with explicit execution settings are open-ended.
No shell expansion, compilation, or package installation is implicit.

### Start a workspace and pass arguments

Create a new project with a working greeting rule:

```sh
mkdir kamaji-demo
cd kamaji-demo
kamaji init --template minimal
kamaji targets
kamaji validate hello
kamaji run hello -- "release candidate" "ready for review"
```

The rule prints:

```text
Hello from Kamaji
release candidate
ready for review
```

`init --template minimal` creates `kamaji.workspace.yaml`, `BUILD.yaml`, and the rule and schema
under `rules/hello/`. It refuses to overwrite existing files. Quotes preserve
each argument containing spaces, and `--` separates Kamaji flags from arguments
for the rule. The shorthand `kamaji hello -- "release candidate"` also works.

### Generate checksums for files you distribute

Turn a repeatable Python operation into a named task. In the workspace above,
create `rules/checksums/` and save this as `rules/checksums/rule.py`:

```python
import argparse
import hashlib
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("--output", required=True)
parser.add_argument("files", nargs="+")
args = parser.parse_args()

lines = []
for name in args.files:
    digest = hashlib.sha256()
    with Path(name).open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    lines.append(f"{digest.hexdigest()}  {name}")

output = Path(args.output)
output.parent.mkdir(parents=True, exist_ok=True)
output.write_text("\n".join(lines) + "\n", encoding="utf-8")
print(f"Wrote {len(lines)} checksums to {output}")
```

Save the rule's option schema as `rules/checksums/rule_definition.yaml`:

```yaml
language: python
description: Write SHA256 checksums for the supplied files
allow_unknown: false
variables:
  output:
    type: string
    description: Destination checksum manifest
    default: dist/SHA256SUMS
```

Replace the generated `BUILD.yaml` with the following to keep `hello` and add
the new task:

```yaml
targets:
  - name: hello
    description: Print a greeting
    rule: hello/rule.py
    config:
      greeting: Hello from Kamaji
  - name: checksums
    description: Generate a checksum manifest
    rule: checksums/rule.py
    config:
      output: dist/SHA256SUMS
```

Generate a manifest for the workspace configuration, then verify it:

```sh
kamaji validate checksums
kamaji run checksums -- BUILD.yaml kamaji.workspace.yaml
cat dist/SHA256SUMS
shasum -a 256 -c dist/SHA256SUMS
```

The rule reports `Wrote 2 checksums to dist/SHA256SUMS`; verification reports
`OK` for each input. Replace the input paths with your own release files when
needed. Kamaji turns `config.output` into the Python argument
`--output=dist/SHA256SUMS`, and forwards the filenames after `--` unchanged.
The rule creates or replaces the manifest in the project's `dist` directory.
Removing `output` from the target's config uses the schema default instead.

### Check a project before running it in CI

From the same workspace, check every target and inspect how the checksum task
will be configured before executing it:

```sh
kamaji validate --all
kamaji doctor checksums
kamaji explain checksums --json
kamaji run checksums --timeout 2m --kill-after 5s -- BUILD.yaml kamaji.workspace.yaml
```

Validation and doctor check configuration, rules, interpreter availability, and
declared dependency settings without downloading or executing a rule. The JSON
explanation shows resolved paths and option provenance with values redacted.
These checks do not inspect arbitrary filenames forwarded to Python: a missing
input file is reported by this rule when it runs.

The execution deadline is two minutes, with five seconds for a child to exit
after cancellation before it is killed. Kamaji preserves a failed child's exit
code, so a CI step fails when the rule fails; a deadline uses exit code 124.
Synchronous file preparation is not preempted by the deadline. See the
[CLI reference](docs/CLI_REFERENCE.md#run-targets) for cancellation details.

### Keep a working copy for debugging, then preview cleanup

Run the same task in an independent copy of the project and retain it for
inspection:

```sh
kamaji run checksums --isolated --keep-execroot -- BUILD.yaml kamaji.workspace.yaml
kamaji runs list
kamaji runs list --json
```

The generated `dist/SHA256SUMS` is in the retained working copy; this invocation
does not replace the manifest in the original project. Kamaji prints the
retained path to stderr. `runs list` shows its name and path; pass the reported
name to `kamaji runs show` to inspect its size and completion state.
Retention is bounded, so this is debugging storage, not a permanent artifact
directory. Isolation copies files; it does not sandbox the Python rule.

Preview removal of downloaded dependencies and execution files:

```sh
kamaji cache status
kamaji cache clean --runs --dry-run
```

The checksum example downloads nothing, so an otherwise unused cache is empty.
When you want to discard cached downloads and retained runs, execute
`kamaji cache clean --runs` without `--dry-run`. It leaves managed Python
environments in place and refuses cleanup while an active run holds the lease.

## Legacy rule-runner behavior

The following applies to `run` and YAML language-rule workspaces. Graph build
behavior is covered in [Usage](docs/HOW_TO_USE.md).

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

- [Runnable Go and multilingual examples](docs/EXAMPLES.md)
- [Usage and configuration](docs/HOW_TO_USE.md)
- [Agent workflows and JSON contracts](docs/AGENT_WORKFLOWS.md)
- [Go build, cache, and restoration walkthrough](examples/build-project/README.md)
- [Rule languages, interpreter configuration, and Go plugins](docs/RULE_LANGUAGES.md)
- [CLI commands and flags](docs/CLI_REFERENCE.md)
- [Commands, Python setup, cleanup, retention, and embedding](docs/RUNTIME_LIFECYCLE.md)
- [Build, download, and archive limits](docs/RESOURCE_LIMITS.md)
- [Executor trust boundary](docs/EXECUTOR_SECURITY.md)
- [Dependencies and TOML/YAML parsers](docs/DEPENDENCIES.md)
- [Tests and recorded validation](docs/TESTING.md)
- [Review findings and implementation follow-up](docs/GENERAL_REVIEW.md)
- [Repository-local wiki](wiki/index.md)

Extension fixes remain outside the completed core work. Historical audits
describe earlier trees and are labeled separately from current guides.

## Contributing and license

Run the checks in [Testing](docs/TESTING.md) when changing the core. Issues and
pull requests are welcome. Kamaji is distributed under the [MIT License](LICENSE).
