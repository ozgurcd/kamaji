# Commands, configuration, and runtime storage

## Commands

`kamaji targets` lists target names and descriptions from the selected build file. Use `--build`
to select a different file. A missing target error also lists available names.

`kamaji validate <target>` and `validate --all` share execution preflight,
including interpreter lookup, workspace prerequisites, rule/schema constraints,
and dependency URL/digest/path checks. They do not download, execute a rule, or
create runtime storage. `doctor [target]` provides the same diagnostics, checking
all targets when omitted. These checks do not establish remote availability or
certify what a rule will do. `explain <target>` shows resolved paths, provenance,
limits, and cache status with option values redacted.

`kamaji version` prints the build version; `kamaji --version` prints Cobra's
version line. Release builds and `make build` use `VERSION` (currently v0.1.0).
Plain `go build` produces a development build reporting `dev`.

`kamaji <target> -- <rule arguments>` remains supported. The explicit form
`kamaji run <target> -- <rule arguments>` also supports target names that match
built-in commands, such as `targets` or `version`.

Use `kamaji --help` or command-specific `--help` for scoped flags. The complete
[CLI reference](CLI_REFERENCE.md) covers scaffolding, completions, maintenance,
execution deadlines, exit statuses, and examples. Flags irrelevant to a command
are rejected rather than accepted and ignored.

## Configuration errors

A missing optional user configuration file is allowed. An unreadable or
malformed file now returns an error instead of silently choosing another
interpreter. Explicit Python selection still takes precedence over automatic
managed-environment selection, including when a managed selection is invalid.

Fixed YAML fields are checked, and configuration must contain one document
(the optional user file may also be empty).
Unknown fields, duplicate keys, and unexpected value types fail before target
execution. Dynamic target option maps remain supported, with opt-in
`allow_unknown: false` per rule. Schemas support descriptions, enums, inclusive
numeric bounds, and string/int/bool/number/map/list types. See
[Usage](HOW_TO_USE.md#rule-options-and-arguments). Diagnostics
identify a field/location when available and do not echo scalar values.

Duplicate target and dependency names are rejected. Dependency initialization
validates references before network work and deduplicates repeated references.
Cache hits still verify checksums, while unchanged metadata is left in place.
Fresh downloads are not hashed a second time after their verified publication.
Download transport/body errors do not expose URL queries or nested error text.

## Execution directories and cleanup

The default runtime root is `/var/tmp/_kamaji_<username>` on macOS and
`/tmp/_kamaji_<username>` on Linux. It contains `cache`, `execroot`, and managed
Python state. Runtime directories must be real directories owned by the user,
with no group/other permissions. Unsafe existing directories are rejected.

Normal runs remove their private execution directory after completion, including
child failures. Setup failures also remove any execution directory they created.
Rule and interpreter checks happen before dependency preparation. Archives are
still extracted privately for each run, preserving independent writable files.

`--keep-execroot` retains files after the child runs, for debugging. Completed
retained runs are bounded by both of these defaults:

- `--max-retained-execroots=10`
- `--max-retained-bytes=4294967296` (4 GiB)

The newest completed runs fitting both limits are kept. The byte accounting
sums regular-file sizes without following symlinks. A run larger than the
retention budget is removed and reports that it could not be retained. Active
and incomplete runs are not pruning candidates. Retention is optional; these
limits do not impose a quota on active working copies or the download cache.
Abrupt process termination and operating-system cleanup failures can leave
scratch data for explicit cleanup.

Retained paths are printed to stderr. `runs list` and `runs show <name>` report
paths, sizes, and completion state without reading file contents. `cache status`
lists download storage. `cache prune --max-bytes <bytes>` applies an explicit
cache budget by removing oldest recognized payloads; it is not automatic
eviction during downloads. These commands are described in the CLI reference.

`--cleanup` removes downloaded cache and execution directories while preserving
Python environments. It holds an exclusive runtime lease throughout deletion
and refuses while a target or Python setup is using that runtime root. Target
runs hold shared leases; Python setup holds an exclusive lease. Busy maintenance
operations report an error rather than waiting for a long-running target.
The persistent lock files remain in place to keep their inode identity stable.

The equivalent maintenance command is `cache clean --runs`; `cache clean` alone
removes only downloads. Clean and prune support `--dry-run`. Actual deletion
uses the exclusive runtime lease; read-only previews can change under concurrent
work and do not acquire a lease or create storage.

The retention flags accept positive values; zero is rejected by the CLI.
The embedding API treats zero-valued retention fields as defaults. These are
separate from the workspace download/archive limits, where zero selects defaults.

## Python setup and rule installation

`kamaji setup-python-env` uses the explicit Python selection (`--python`, then
KAMAJI_PYTHON, then the optional user config) or `python3` to create the new
environment. Normal execution additionally considers managed and legacy
environments before falling back to `python3`. Setup installs
`/usr/local/share/kamaji/requirements.txt` by default if present and may access
the network. `--user` selects `~/.local/share/kamaji`; `--install-root` chooses
another root. `--requirements` explicitly selects a requirements file for setup.

Python setup creates a new private directory under `venvs/env-*`. Only after
environment creation and dependency installation succeed does it atomically
update the `python-current` selection file. Failure preserves the prior
selection and removes incomplete staging data during ordinary error handling.
Cancellation also terminates the Python/pip process group before that cleanup.

The environment itself is never renamed, so absolute interpreter paths in
installed scripts remain valid. The replaced managed environment is removed
after successful promotion. An existing legacy `venv/bin/python` remains a
fallback when there is no managed selection. This is atomic selection during
normal operation, not a power-loss durability guarantee.

Rule installation stages replacement data and uses rollback on publication
failure. A new source without `requirements.txt` removes stale installed
requirements. Installation and removal share a filesystem lock so simultaneous
mutations cannot interleave. Copying requires a fresh destination and rejects
source symlinks/special files and aliases that place the destination inside the
source. Requirements files must be regular files.

## Embedding and tests

Production runtime state belongs to `rt.Runtime` instances. The target manager,
executor, archive helpers, and command use explicit runtime references instead
of a process-wide configuration. `cmd.NewCommand(Options)` supports owned
dependencies; a zero-value Options selects ordinary CLI behavior. Set
`Runtime.Config.WorkingDir` for an explicit working directory without changing
the process directory. Use a separate Runtime for each concurrent invocation.

`Runtime.Context`, `Timeout`, and `KillGrace` control execution cancellation;
command callers can use `ExecuteContext`. Lookup and user-home resolution have
owned hooks in `cmd.Options` for tests. Injected execution hooks remain responsible
for their own cancellation. Interactive terminal ownership is necessarily a
process-wide resource and is serialized separately from per-invocation runtime
configuration. The foreground helper temporarily ignores SIGTTOU during the
terminal handoff and restores its prior ignored/default state afterward;
embedding applications with custom SIGTTOU handling must coordinate that use.

Parallel tests exercise independent command/executor instances. Legacy serial
tests use adapters confined to `_test.go`; those adapters are not shipped in
the executable. Cross-process tests verify shared/exclusive runtime lock
semantics. Python setup tests simulate subprocess results and verify both
rollback and stable interpreter paths.

These features do not turn Python rules into sandboxed code. Rules retain the
user's permissions and inherited environment; argument values may be visible
to process inspection. Extension source and real infrastructure were excluded
from this implementation scope.
