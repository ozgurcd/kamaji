# CLI reference

Commands below describe v0.1.0. Release builds and `make build` report `v0.1.0`;
plain `go build` produces a development build reporting `dev`.

## Start and inspect a project

`kamaji init [directory]` creates a minimal workspace, a `hello` target, and a
self-contained Python rule/schema. The default directory is the working
directory; `--template minimal` names the currently available template.
Existing destination files and symlinked destination directories are rejected.
No Python, package installer, or external service is invoked by scaffolding.

`kamaji targets` lists sorted names and optional descriptions from the selected
build file. Target-taking commands support name completion, including descriptions,
through Cobra's `completion` command. Completion reads only the build file.

`kamaji validate hello` and `kamaji validate --all` share non-mutating preflight
checks with execution. They check user/workspace/build configuration, schema
constraints, the required workspace organization entry, working directory,
interpreter lookup, rule file, option names, and selected dependency paths,
SHA256 syntax, and HTTP(S) URLs. The all-target form reports each target's
result and returns failure if any target fails. Common configuration failures
stop validation before target checks. It makes no network requests and creates
no runtime storage.

`kamaji doctor [target]` runs the same readiness diagnostics; omitting the target
checks the whole build file. Neither command executes/imports a rule, checks
Python package installation, verifies remote availability, or guarantees that
later filesystem/network operations will succeed.

`kamaji explain hello` shows resolved workspace/build/rule paths, interpreter
selection and its source, rule/target descriptions, effective resource limits,
runtime location, option provenance, and referenced dependency cache status.
`--isolated` describes working-copy execution instead of the run default.
`--json` emits structured output. All option values are redacted, including
defaults; URLs and arguments are not printed. Cache checks inspect existing
private directories and verify a referenced payload's checksum without repairing
metadata or downloading anything.

## Run targets

Both `kamaji hello -- extra-argument` and `kamaji run hello -- extra-argument`
execute a target. The explicit `run` form handles target names that collide with
built-in commands. Arguments after `--` retain their boundaries.

Target execution, validation, doctor, and explain accept:

- `--build`, `-b`: build file, default `BUILD.yaml` in the working directory.
- `--python`, `-p`: explicit interpreter; selection precedence is documented in
  [Usage](HOW_TO_USE.md#python-installation-and-debugging).
- `--rules-directory`: override the workspace rules path.
- `--user`: use `~/.local/share/kamaji` for the default rules installation and
  installed requirements. An explicit workspace rules path still takes precedence.
- `--install-root`: an alternative installation root; mutually exclusive with
  `--user`. The compatibility default remains `/usr/local/share/kamaji`.

`targets` accepts `--build`. Execution also accepts:

- `--isolated`, `-i`: independent working-directory copy; off by default.
- `--keep-execroot`: retain completed execution files within the retention bounds;
  the retained path is printed to stderr, including after child failure.
- `--max-retained-execroots`: positive directory count, default 10.
- `--max-retained-bytes`: positive byte count, default 4294967296 (4 GiB).
- `--timeout`: duration such as `5m`; default zero means no execution deadline.
- `--kill-after`: positive termination grace duration; default `2s`.
- `--debug`, `-d`: core debug setting; does not enable argument/value dumps.

Flags are scoped to applicable commands. For example, validation rejects
`--cleanup` and `--keep-execroot` instead of silently ignoring them. Do not pass
`--python` to a rules installation/removal command; those commands do not run it.

The deadline starts at executor entry, including dependency preparation. HTTP
requests carry the execution context. Filesystem copy/extraction is synchronous;
expiration is observed before the child starts, not as a hard real-time bound
on every filesystem operation. Cancellation of Kamaji's context (including
interrupt/TERM signals delivered to Kamaji) sends TERM to the child process
group, then KILL after the grace period. Descendants that deliberately
leave that group are outside this mechanism. Interactive children receive the
foreground terminal, which is restored afterward. Simultaneous interactive
invocations serialize access to that process-wide terminal.
Terminal-generated Ctrl-C goes directly to an interactive foreground child group,
following normal terminal signal behavior.

The CLI preserves child exit codes and maps child signals to `128 + signal`.
Wrapper/configuration errors use 1, execution deadlines 124, and canceled command
contexts 130. A cleanup/retention error can make an otherwise successful run fail.

## Install rules and Python packages

`kamaji rules-directory-create --user` installs the current directory's rules and
optional requirements without needing the global installation directory.
`kamaji rules-directory-delete --user` removes that installation's rules and
requirements. Both accept `--install-root` instead of `--user`, and both use
the same installation lock.

`kamaji setup-python-env --user` creates a new managed environment using the
selected base interpreter and the user installation's requirements if present.
`--requirements requirements.txt` selects an explicit regular requirements file,
resolved relative to the working directory. An explicit missing file is an error;
an absent default installed file skips package installation. Setup may invoke
pip and access the network. The previous environment remains selected if setup
fails or is canceled. See [Runtime lifecycle](RUNTIME_LIFECYCLE.md).

## Inspect and clean storage

`kamaji runs list` reports execution directory names, paths, logical regular-file
sizes, and completion state. `kamaji runs show <name>` selects one entry; it
does not dump file contents. Both accept `--json`. An entry lacking a completion
marker is reported as incomplete or active, not assumed safe to prune.

`kamaji cache status` lists cache directories and sizes; `--json` emits structured
output. This inventory does not certify payload integrity. Execution and explain
perform checksum checks on referenced payloads.

`kamaji cache prune --max-bytes 1073741824 --dry-run` previews oldest-payload
removals needed to reach a 1 GiB cache budget. Omit `--dry-run` to apply them.
The default budget is 4 GiB; zero selects all recognized cache entries for
removal. This is an explicit maintenance budget, not automatic eviction during
downloads. Unrecognized directories are preserved and reported if they prevent
meeting the budget. Byte accounting includes regular files and does not follow
symlinks; selection uses payload modification time, not access frequency.

`kamaji cache clean` removes download storage. Add `--runs` to also remove
execution files, or `--dry-run` to preview directory paths and sizes.
`kamaji --cleanup` remains the compatibility alias for `cache clean --runs` and
rejects unrelated flags. Python environments are preserved.

Actual cache deletion/pruning holds an exclusive runtime lease and refuses while
a run or Python setup uses that root. Read-only listings/previews are snapshots;
concurrent work can change them before a later deletion command.

`kamaji --help`, command-specific `--help`, `version`, and `--version` do not
execute targets. Use command-specific help for the exact supported flags.
