# Using Kamaji

This guide describes the Go core. The example rule below is self-contained;
bundled Terraform, kubeseal, and other extensions have separate behavior and
were excluded from the recent core fixes.

For complete checked-in Go examples and a workflow combining Python, Go, Ruby,
and JavaScript, see [Runnable rule examples](EXAMPLES.md). It covers structured
options, file verification, policy failures, report generation, and isolation.

## A minimal workspace

Run `kamaji init` in a new project directory to generate a complete minimal
workspace. It refuses to overwrite existing files. The following manual example
shows the same core layout. See [CLI reference](CLI_REFERENCE.md) for all commands.

Create this layout in your project:

```text
kamaji.workspace.yaml
BUILD.yaml
rules/
  hello/
    rule.py
    rule_definition.yaml
```

`kamaji.workspace.yaml`:

```yaml
rules_directory: "//rules"
workspace_vars:
  - org_domain: "example.com"
```

`BUILD.yaml`:

```yaml
targets:
  - name: hello
    description: Print a greeting
    rule: hello/rule.py
    config:
      greeting: "Hello from Kamaji"
```

`rules/hello/rule_definition.yaml`:

```yaml
language: python
description: Print a greeting and extra arguments
allow_unknown: false
variables:
  greeting:
    type: string
    description: Greeting to print
    default: Hello
```

`rules/hello/rule.py`:

```python
import argparse

parser = argparse.ArgumentParser()
parser.add_argument("--greeting", required=True)
parser.add_argument("extra", nargs="*")
args = parser.parse_args()
print(args.greeting)
for value in args.extra:
    print(value)
```

From that project directory, with Kamaji on PATH:

```sh
kamaji targets
kamaji validate hello
kamaji run hello -- extra-argument
```

`targets` prints sorted names and descriptions. `validate` shares non-mutating
execution preflight, including interpreter availability, workspace prerequisites,
rule/schema checks, and dependency references. It applies defaults in memory;
it does not download, execute Python, or create runtime storage. It cannot check
remote availability or certify rule behavior. Use `validate --all` for the whole
build, `doctor` for readiness diagnostics, and `explain hello` for redacted
resolved configuration. `run` executes the rule with the selected interpreter
or launches its compiled executable directly.
The shorthand `kamaji hello` also works; use `run` when a target name matches
a built-in command such as `version`.

## Configuration and paths

Kamaji searches the working directory and its parents for
`kamaji.workspace.yaml`. The build file defaults to `BUILD.yaml` in the working
directory, independently of where the workspace marker is found. Select another
build file with `--build` (`-b`).

`rules_directory` is resolved relative to the detected workspace. `//rules`
also means the workspace's `rules` directory; absolute paths are accepted.
`--rules-directory` overrides the workspace value. If omitted, the rules path
is `/usr/local/share/kamaji/rules`.

A target's relative `rule` path is relative to the rules directory.
`//rules/hello/rule.py` resolves directly from the detected workspace; an
absolute rule path is also accepted. Every rule requires a sibling
`rule_definition.yaml`, even when its `variables` mapping is empty.

`rules_common_directory` defaults to `common`, relative to the rules directory,
or can be absolute. For rules labeled Python, Kamaji sets PYTHONPATH to that location; it does not
automatically import or invoke helper modules. Rules must do their own imports.

Runs require at least one `workspace_vars` entry. The first entry's `org_domain`
becomes KAMAJI_ORGANIZATION_DOMAIN. The recognized `base_dir` field does not
change the child's working directory. `workspace_root` is recognized but is
replaced by the detected workspace location; normally omit it.

YAML files contain one document. Unknown fixed fields, duplicate mapping keys,
duplicate target/dependency names, and unexpected types are errors. Omit a
second `---` at the end of examples: it begins another document. Empty or missing
optional user configuration is allowed. Use `true`/`false` for booleans; in
dynamic option maps, `yes`, `no`, `on`, and `off` are strings.

## Rule options and arguments

The core checks each declared schema variable. Supported types are `string`,
`int`, `bool`, `number`, `map`, and `list`, either as a type string or a mapping.
Mappings accept `type`, `description`, `mandatory`, `default`, `enum`, `minimum`,
and `maximum`; unknown schema-definition fields fail. Bounds apply only to
numeric types and are inclusive. Enums must be nonempty lists of correctly typed
values. Defaults must also satisfy the declared constraints.

Defaults are inserted before execution. Missing mandatory values or mismatched
types fail. Undeclared config options remain allowed by default; a rule can set
top-level `allow_unknown: false` to reject them. The optional top-level
`description` documents the rule. `language: go` selects a compiled executable;
omitted language keeps Python behavior. An optional `execution` block selects
an interpreter command or direct executable mode for any language. See
[Rule languages](RULE_LANGUAGES.md). Map/list types validate their outer type and JSON serializability,
not a recursive item/property schema.

For example, a variable can declare `type: int`, `minimum: 1`, `maximum: 8`,
and `default: 2`; a string variable can use `enum: [fast, safe]`. Use descriptions
for public help text, since they appear in explain output.

Options are sorted and passed as individual `--name=value` arguments. Strings
remain literal; other values, including maps and lists, are JSON encoded.
Kamaji does not pass a Python dictionary or build a shell command. The rule
must parse the arguments. Arguments after the CLI `--` separator are appended
unchanged; Kamaji's separator itself is not forwarded.

The child inherits the environment, with PWD and
KAMAJI_ORGANIZATION_DOMAIN set for the run; rules labeled Python also receive
the configured PYTHONPATH. Values in process arguments can be
visible to process inspection. See [Executor security](EXECUTOR_SECURITY.md).

## Third-party dependencies

Add entries to the workspace's `third_party` list. Each entry has a unique
`name`, a local `file_path`, and `url`/`sha256` mappings keyed by Go platform,
such as `darwin_arm64` or `linux_amd64`.

The following is a shape example, not a downloadable artifact. Replace the URL
and digest with a trusted artifact and its actual SHA256 before use:

```yaml
third_party:
  - name: example_tool
    file_path: bin/example-tool
    url:
      linux_amd64: https://example.invalid/example-tool.tar.gz
    sha256:
      linux_amd64: "REPLACE_WITH_64_HEXADECIMAL_SHA256_CHARACTERS"
```

Reference the name as a top-level target config string, for example
`tool: "@@example_tool"`. Kamaji substitutes the prepared artifact's absolute
path. It does not search PATH for an `@@` dependency. Nested map/list references
are not expanded. Dependencies must declare an HTTP(S) URL and valid digest for
the current platform, even when a verified payload is already cached;
the checksum establishes agreement with your configuration, not publisher
authenticity or program safety.

ZIP, gzip-compressed tar, Mach-O, and ELF payloads are supported. Downloads and
archive extraction are bounded; see [Resource limits](RESOURCE_LIMITS.md).
Repeated references within an invocation are initialized once. Archive
extraction remains private to each execution.

## Python, installation, and debugging

For implicit Python execution, select Python with `--python` (`-p`), KAMAJI_PYTHON, or `python` in the optional
`~/.kamaji/config.yaml`, in that order. Otherwise Kamaji selects its managed
environment, a legacy executable `venv/bin/python`, or `python3`. An unreadable
or malformed user file is an error, even with an explicit interpreter override.

`rules-directory-create` installs the current directory's `rules` and optional
`requirements.txt` under `/usr/local/share/kamaji`; it needs write permission
there. `rules-directory-delete` removes the installed rules and requirements.
Add `--user` to use `~/.local/share/kamaji`, or `--install-root` to choose another
installation. Use the same scope for setup and for target commands that rely on
the default installed rules path.
`setup-python-env` creates a managed environment and installs the installed
requirements file when present; this command can invoke pip/network access.
It does not install the current workspace's requirements automatically.
Pass `--requirements requirements.txt` to setup to select that file explicitly.

`--isolated` (`-i`) copies the working directory for the child and rejects
source symlinks/special files. By default, the child runs in the original working
directory. Rules retain user permissions in either mode.

Normal execution files are removed afterward. Use `--keep-execroot` for bounded
debugging retention and `--cleanup` without a target to remove caches and
execution files. See [Runtime lifecycle](RUNTIME_LIFECYCLE.md) for storage paths,
limits, locks, and Python promotion behavior.

Use `--timeout 5m` to set an execution deadline and `--kill-after 2s` to control
the graceful termination interval. Child exit codes are preserved. Inspect
retained files with `runs list`/`runs show`, and downloads with `cache status`.
`cache prune` and `cache clean` support `--dry-run`; see
[CLI reference](CLI_REFERENCE.md#inspect-and-clean-storage).
