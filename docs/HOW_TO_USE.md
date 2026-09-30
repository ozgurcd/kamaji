# Using Kamaji

This guide describes Kamaji v0.3.0. Graph builds require v0.3.0 or newer;
v0.2.0 supports the legacy rule runner. Existing Terraform, kubeseal, and other extensions
have separate behavior and were excluded from the core changes.

## A build project in one file

`kamaji init` creates a runnable `kamaji.toml` in the current directory and
refuses to overwrite an existing build document. Use `kamaji plan` to inspect
it and `kamaji build` to run it. The [compiled Go walkthrough](../examples/build-project/README.md)
shows a complete compiler → generator → verification graph.

```toml
version = 1
default = ["check"]

[[targets]]
name = "compile"
command = ["go", "build", "-o", "out/app", "main.go"]
inputs = ["main.go"]
outputs = ["out/app"]
cache = true
timeout = "2m"

[[targets]]
name = "check"
deps = ["compile"]
command = ["out/app", "--self-test"]
inputs = ["out/app"]
```

This configuration shape assumes your `main.go` implements `--self-test`; the
checked-in example contains an actual runnable program. Commands can invoke any
interpreter or executable. Declare script files as inputs when using a script.
Kamaji does not insert a shell, interpolate variables, or compile a rule unless
you declare a compilation target.

`targets`, `validate --all`, `doctor`, and `explain` also recognize graph
projects. Graph explanation uses the same plan contract as `plan`; validation
checks generated inputs symbolically and does not build their producers.
Explicit legacy-only interpreter, installation, and isolation flags are rejected
when inspecting a graph project. To inspect a legacy file alongside a graph,
select it explicitly with `--build BUILD.yaml`.

The document is discovered from the working directory upwards. Paths and child
working directories are relative to the document's directory. `--file` selects
a document explicitly. `kamaji.yaml` and `kamaji.yml` can express the same graph
schema; discovering multiple documents in one directory is an error. Legacy
`BUILD.yaml` and `kamaji.workspace.yaml` remain a separate rule-runner format.
There is no forced migration or silent interpretation of one schema as another.

| Target field | Meaning |
| --- | --- |
| `name`, `description` | Unique target identity and optional human-readable explanation |
| `command` | Exact executable and arguments; omit only for a dependency-only aggregate |
| `deps` | Targets that must finish first; cycles and unknown names are rejected |
| `inputs` | Project-relative files, recursive directories, or globs; `**` matches directory levels |
| `outputs` | Exact project-relative artifact files or directories; no wildcards |
| `cache` | Opt in to local content-based reuse; defaults to false and requires outputs |
| `env` | Explicit environment map; values participate in the fingerprint but are not printed in plans |
| `pass_env` | Additional inherited environment variable names |
| `tools` | Additional executables to fingerprint, beyond `command[0]` |
| `timeout` | Per-action positive duration, such as `30s` or `2m` |
| `slots` | Parallel execution slots consumed by the target, default 1 |
| `effect` | `build` by default; `external` requires explicit permission and disables caching |

`version = 1` is required. Without target arguments, `default` selects roots;
without `default`, all targets are selected. Dependencies execute once in
topological order; independent targets may run concurrently under `--jobs`.
On failure, new work is blocked and running children are canceled through their
process groups. Children have no interactive stdin. Nonzero child exit codes
are preserved; deadlines use exit code 124. A whole-build `--timeout` also
limits execution; synchronous filesystem work is not preempted mid-operation.

Paths must stay within the project and cannot use `..`, symlinks, or special
files. `.git`, `.kamaji`, `.audit`, and `.gograph` are reserved and excluded
from wildcard traversal. Outputs cannot overlap each other or the producer's
inputs. A target consuming another target's declared output must depend on it.
Missing source inputs fail; generated inputs and executables may be absent
during planning if a dependency will produce them.

Dependency validation considers each output a possible directory tree, even
before it exists. For example, `inputs = ["**/*.txt"]` can consume files below
an output named `generated`, so the consumer must depend on its producer.
The same overlap is rejected between a target's own inputs and outputs. Prefer
narrow patterns such as `src/**/*.txt` when generated files are not inputs.
Generated parent directories do not change a reviewed plan's identity; source
files alongside the generated outputs still do.

Use explicit paths for project executables: `./tool`, `tools/compiler`, or an
absolute path inside the project. These spellings share dependency and affected
path handling. Bare names such as `go` are PATH lookups; declare project files
they consume separately. Absolute project-local executable paths are subject to
the same symlink and path checks as relative ones.

## Incremental builds and cache correctness

An action key includes declared input content and modes, its command and
configuration, effective environment, platform, executable contents, and
dependency results. The baseline environment contains PATH, HOME, TMPDIR when
present, PWD, and a C locale. Other inherited variables must be named in
`pass_env`. Every effective value participates in the key; values and command
arguments are omitted from plan/result JSON.

Successful cacheable actions must produce every declared output. Cache payloads
are hashed again before reuse, including when existing workspace outputs appear
unchanged. Missing or modified outputs are restored; corrupt cache entries are
rebuilt. Input or tool changes during execution prevent cache publication.
`--no-cache` bypasses reads and writes. Non-cacheable checks run every time;
an always-running dependency without outputs also prevents downstream reuse.

This is a declared-input cache, not a hermetic build guarantee. A command can
read undeclared files, compiler support files, HOME configuration, external
services, or the clock. Executable fingerprints do not capture an entire SDK or
its dynamically loaded libraries. Declare relevant project inputs and tools,
pin your toolchain, and leave `cache = false` when inputs or effects cannot be
described reliably. Prefer narrow source directories or file patterns over
patterns that also include generated outputs. Isolation and operating-system
permissions are not enforced by the graph runner; `effect` is a declaration,
not a sandbox or network firewall.

Add `.kamaji/` to your project's `.gitignore`. Each project serializes concurrent
build/cleanup processes with a lease, while actions within a build can run in
parallel. Cache entries and private JSON evidence records persist until cleaned:

```sh
kamaji clean --dry-run
kamaji clean
kamaji clean --cache --dry-run
```

`clean` removes declared outputs for the selected dependency closure. `--cache`
also removes the whole project's action cache. Run records are preserved unless
`--history` is explicitly requested. The existing `cache`/`runs` commands manage
legacy runner storage and do not manage `.kamaji/`.

## Choosing an execution model

| Need | Graph project | Legacy rule workspace |
| --- | --- | --- |
| Configuration | `kamaji.toml`, or equivalent graph-schema YAML | `kamaji.workspace.yaml`, `BUILD.yaml`, and per-rule schemas |
| Execution | `build` follows explicit `deps` | `run` launches one rule; caller sequences rules |
| Language interface | Explicit argv in `command` | Schema selects interpreter or native executable; config becomes flags |
| Reuse | Opt-in declared-output action cache | Verified downloaded dependencies; rule execution itself is not action-cached |
| Evidence | Plan IDs, JSON events/results, `history` | Redacted `explain`, child output, optional retained execution directory |
| Storage commands | `clean`, optionally `--cache` or `--history` | `cache` and `runs` |

Existing language rules do not need migration. To model one as a graph target,
declare its actual argv, inputs, outputs, environment, and dependencies yourself;
graph commands do not interpret `rule_definition.yaml`, expand legacy `@@`
references, or select managed Python environments. Pin an explicit interpreter
or executable as needed. Leave caching off until the action's inputs and outputs
are accurately declared. A schema label such as `language: go` belongs to the
legacy interface; a graph target compiles Go only when its command says to.

## Agent workflows

For a copyable plan/build/history wrapper, JSON field reference, and streaming
error-handling guidance, see [Agent workflows](AGENT_WORKFLOWS.md).

The engine works without an LLM. An agent can discover contracts with
`kamaji capabilities`, inspect `kamaji plan --json`, and pass the returned ID
to `kamaji build --expect-plan`. The build refuses changed plans before starting
actions and rechecks source state as each action becomes ready. Cached/uncached
availability does not change a plan's content identity. Dependent cache decisions
are deferred until upstream artifacts exist; the plan labels those targets pending.

`build --json` writes a `kamaji.result.v1` document to stdout; child diagnostics
go to stderr. `build --events` instead emits newline-delimited
`kamaji.event.v1` status records followed by the final result. Preflight failures
use `kamaji.error.v1`. Results identify the reviewed plan, target statuses and
exit codes, input/tool digests, and verified output digests. The same result is
stored in `.kamaji/runs` and can be retrieved with `kamaji history` and its run ID.
Child stdout/stderr are streamed, not persisted in those JSON records; capture
stderr externally when an agent needs the detailed tool log later.

`kamaji affected --json` accepts project-relative changed paths, including
deleted paths, and returns matching targets plus their reverse dependencies.
It relies on declared inputs; it does not infer imports or discover undeclared
dependencies. Explicit local executable and `tools` paths are included, along
with reverse dependencies. Changed directories are matched against the input
patterns beneath them. A deleted or unreadable path is conservatively treated as
a possible directory, so the result can include extra targets. An agent can run
the returned targets, inspect failures, propose
source edits, then request a fresh plan. Kamaji does not autonomously rewrite
source, weaken tests, contact a model provider, or publish anything.

Targets such as deployments and publication should declare `effect = "external"`.
They cannot be cached and require `build --allow-effects`; combine that flag with
`--expect-plan` to bind execution to a reviewed plan. Configuration is trusted
project code: these declarations do not make arbitrary commands safe.

For complete checked-in Go examples and a workflow combining Python, Go, Ruby,
and JavaScript, see [Runnable rule examples](EXAMPLES.md). It covers structured
options, file verification, policy failures, report generation, and isolation.

## Legacy language-rule workspaces

Run `kamaji init --template minimal` in a new project directory to generate a complete minimal
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
