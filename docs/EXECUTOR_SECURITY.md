# Executor review — 2026-09-28

This review uses the current Go source and fresh offline test results. Existing
project descriptions are not evidence of security guarantees. Extension rule
source is excluded at the owner's request.

## Verified behavior

Current execution supports both configured interpreters and compiled rule
executables; the generic mode was added after the original review below.
`runner.Executor.Run` starts the selected program with an argument vector through
`exec.Command`. It does not construct a shell command. Configured options are
sorted, structured values are JSON encoded, and trailing arguments retain their
boundaries, including empty strings. A real Go test subprocess stands in for
Python to verify spaces, quotes, newlines, and shell syntax remain literal data.

The child runs in the runtime's explicit working directory (or actual process
working directory when omitted), or an independent copy when
isolation is selected. Relative interpreter paths containing a directory
component are resolved before changing the child's directory. Tests cover a
relative interpreter outside the copied tree, which failed before this fix.
The environment preserves ordinary inherited variables while overriding PWD
and KAMAJI_ORGANIZATION_DOMAIN for the run. Rules labeled Python additionally
receive the configured PYTHONPATH; other labels preserve its inherited value. Child errors remain
wrapped errors that callers can inspect with `errors.As`; the real-process test
checks a child exit code of 23. The CLI now preserves child exit codes, uses 1
for wrapper errors, 124 for deadlines, and 130 for canceled command contexts.
Child termination by a signal maps to `128 + signal`.

Runtime directory creation checks the root and Kamaji-controlled cache,
execution, and Python-environment directories before writing descendants.
Accepted directories must be real directories, owned by the effective user,
with no group/other permission bits. New directories use 0700. Unsafe existing
directories are rejected, not chmodded or deleted. Synthetic symlink and
permission tests reproduced acceptance and writes through runtime symlinks
before the fix. Tests also verify that unsafe cleanup and Python setup fail
before their destructive operation or subprocess call.

Fresh execution roots are unique and private. Existing tests verify independent
working copies and rejection of symlinks/special files in copied inputs.
Dependencies are checksum-validated, privately extracted, and subject to the
download/archive bounds described in `RESOURCE_LIMITS.md`. Current source uses
rooted archive writes; traversal and symlink-escape regression tests pass.
Concurrent cache publication is tested using independent offline processes.

Execution and Python setup use a private process group. Cancellation sends TERM
then KILL after the grace interval; the process tests exercise graceful descendant
termination and forced termination. Execution deadlines also cancel HTTP requests.
Terminal foreground ownership is transferred for interactive children and restored
afterward; a tagged PTY test exercises actual input. See
[CLI reference](CLI_REFERENCE.md#run-targets) for cancellation limits.

## Trust boundary and limits

- Rules, the selected interpreter, workspace configuration, and dependency
  hashes are trusted code/configuration supplied by the user. A matching hash
  establishes agreement with that configuration, not the safety of a program.
- Isolated mode is a working-copy feature, not an OS sandbox. Rules can access
  absolute paths, the network, inherited environment variables, and other
  resources available to the same user. Rule files themselves are not copied
  into a sandbox. Do not use this mode to contain hostile extensions.
- Options are passed as process arguments. The Go runner does not log their
  values, but process inspection and the child program can expose them.
- Directory checks protect against pre-existing symlink/permission hazards and
  other users at the default sticky system-temporary parent. A custom temporary
  directory's ancestors must be trusted. These checks do not defend against
  malicious processes running under the same effective user or against root.
- Execution deadlines are optional; the default remains unlimited. Cancellation
  is not an OS sandbox or a hard bound on synchronous filesystem operations, and
  descendants that deliberately leave their process group are not contained.
  Normal execution directories
  are removed after the run; optional debugging retention has count/byte bounds.
  Cleanup refuses active runs through an exclusive lease. Per-download/archive
  limits and retention bounds are not a total quota on active working copies or
  the download cache. See `RUNTIME_LIFECYCLE.md` for current policy.
- Tests do not execute Python, Terraform, kubeseal, 1Password, network services,
  or production infrastructure. They do not certify extension correctness,
  privilege separation, every operating-system failure, or power-loss behavior.

## Recorded executor-review validation

The measurements below describe the earlier executor-review tree. The expanded
implementation's later race run reports 84.3% statement coverage; its test census
and validation evidence are in [Testing](TESTING.md#general-review-implementation-validation--2026-09-28).
Neither coverage measurement is a security certification.

The subsequent usability implementation has its own validation entry in
[Testing](TESTING.md#usability-implementation-validation--2026-09-28); older
measurements below remain historical.

The final `go test ./... -count=1 -race
-coverprofile=.audit/executors-after.cover` passed. Native build, Linux AMD64
cross-build, vet, and staticcheck also passed. `go tool cover` measured 86.9%
module statement coverage; coverage is not a security certification.

New checks reside in `runner/executor_test.go`, `rt/private_dir_test.go`,
`execroot/private_dir_test.go`, and `cmd/private_dir_test.go`. The full source
census contains 74 top-level Test functions excluding TestMain, and 330 static
t.Fatal/Fatalf/Error/Errorf sites, including fixture guards. The previous tree
had 69 tests and 294 such sites. Counts cover module test files only, excluding
`.audit` and dependencies; table-driven assertions count once.
