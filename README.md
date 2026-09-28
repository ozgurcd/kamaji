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

## Examples

These examples assume `kamaji` is on your PATH and `python3` is installed.
They use local files and Python's standard library; no cloud account, package
installation, or bundled extension is needed.

### Start a workspace and pass arguments

Create a new project with a working greeting rule:

```sh
mkdir kamaji-demo
cd kamaji-demo
kamaji init
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

`init` creates `kamaji.workspace.yaml`, `BUILD.yaml`, and the rule and schema
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
