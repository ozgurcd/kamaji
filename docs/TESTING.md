# Go tests

Run the complete offline suite from the repository root:

```sh
go test ./... -count=1 -race -coverprofile=coverage.out
```

Other validation commands are `go build ./...`, `go vet ./...`,
`staticcheck ./...`, `go mod verify`, and `git diff --check`.
The module requires Go 1.27.1. There is no repository Makefile verification
target or configured Legattus project.

The latest recorded full check is the
[v0.1.0 release verification](#v010-release-verification--2026-09-28)
below: 82.7% statement coverage. The preceding implementation's source census contains 114 top-level tests
(113 in the default suite and one tagged terminal integration test) and 505
static assertion/guard sites (504 default, one integration), under the predicates
stated there. Earlier sections retain measurements of earlier trees; their
percentages and test populations are not current totals.

The current executor review and its trust boundary are documented in
[Executor security](EXECUTOR_SECURITY.md). Its tests launch the Go test binary
as a stand-in interpreter to check the real process boundary offline.

## What the tests exercise

The general-review implementation adds strict YAML/error-redaction checks,
independent command/executor instances, read-only CLI commands, maintenance
leases across processes, bounded retention, transactional Python selection,
installation coordination, and destination-copy confinement. Current command
and lifecycle behavior is documented in [Runtime lifecycle](RUNTIME_LIFECYCLE.md).

- `main_test.go`: the CLI process exit status and help/error output through
  the Go test executable.
- `cmd/root_test.go`: target execution using real YAML parsing and validation,
  flag propagation, Python precedence, rules overrides, defaults, cleanup,
  administrative command errors, and help without initialization.
- `config/UserConfig_test.go`: flag/environment/config/default precedence,
  missing and malformed user configuration.
- `rt/rt_test.go`: workspace discovery, rules resolution, runtime directories,
  and Python environment setup success and failures.
- `target/target_test.go`: build selection, duplicate/missing targets, schemas,
  required/default values, HTTP errors and body closure, hash validation,
  partial cache recovery, and metadata regeneration.
- `execroot/execroot_test.go`: unique execution directories, ZIP/tar/raw
  executables, portable archive layouts, path traversal rejection, malformed
  metadata, aliases, and missing/duplicate artifacts.
- `runner/runner_test.go`: deterministic argument vectors preserving spaces,
  quotes and shell syntax as data; nested JSON values; rule paths; actual
  working directories; child errors; and independent isolated working copies.
- `tools/tools_test.go`: checksums, recursive normalization, archive confinement,
  file copying and permissions, same-file protection, executable lookup,
  metadata, and bounded stream copying.
- `utils/utils_test.go`: write probes preserving existing files, file/directory
  copies, executable modes, and preservation of the installation when staging
  replacement rules fails.

`obj` contains data declarations without executable statements. Serialization
of those declarations is exercised through configuration and target tests.

HTTP tests inject an in-memory `RoundTripper`; they do not open sockets or
perform DNS. Python and administrative operations use test boundaries rather
than real Python, pip, global rules directories, or infrastructure. Fixtures
use `t.TempDir`; fixture tests do not read the user's home configuration.
Tests mutating package configuration are deliberately not parallel.

## Initial test-expansion measurement on 2026-09-28

The baseline command was:

```sh
go test ./... -skip '^TestSetupPythonEnv$' -count=1 -coverprofile=.audit/tests-before.cover
```

It measured 9.7% module statement coverage. The skipped old test would run
real Python/pip against the global installation; it has been replaced by
offline tests. The final full command, with no skipped tests, was:

```sh
go test ./... -count=1 -race -coverprofile=.audit/tests-after.cover
```

`go tool cover -func=.audit/tests-after.cover` measured 85.5% of module
statements. Per-package coverage: main 100.0%, cmd 89.0%, config 100.0%,
execroot 79.6%, rt 81.8%, runner 91.7%, target 90.2%, tools 86.3%, utils 71.8%.
The aggregate is statement-weighted, not an average of those percentages.

The original three test files had five top-level `Test*` functions. The nine
test files listed above now have 57, measured from ast-grep Go
`function_declaration` nodes whose text starts with `func Test...(`.
Static assertion/guard call sites increased from 24 to 250: ast-grep
`call_expression` nodes starting with `t.Fatal(`, `t.Fatalf(`, `t.Error(`, or
`t.Errorf(`. These counts include fixture guards, count each table-driven
assertion once, and exclude `.audit`, dependencies, and Python environments.
They are not counts of distinct behaviors or runtime assertions.

All final validation commands above passed. Expected failing regression runs
preceded fixes; they are distinct from the final green checks.

## Limits

These tests do not certify real Terraform, kubeseal, 1Password, or Python rule
behavior, network services, global installation permissions, or crash
durability. Cache concurrency now has the independent-process test described
below. Rare operating-system I/O failures and
other-platform branches are not fully covered. An isolated execution copies
the working directory and rejects symlinks/special files; it is not an OS
sandbox. Its destination must be outside the directory being copied.

The original findings and current Go-fix status are in the locally ignored
`PROBLEMS.md`. `wiki/audit-2026-09-28.md` and `.audit/probes/probe_test.go`
preserve pre-fix observations; the latter intentionally tests old failures
and is not part of this suite.

## Core-problem follow-up on 2026-09-28

The full race suite now also covers configurable download/archive limits,
overflow rejection, exact byte boundaries, cached-file limits, tar/ZIP entry
counts, corrupt gzip trailers, trailing compressed expansion, and bounded
stream copies. See [Download and archive limits](RESOURCE_LIMITS.md).

`TestCacheConcurrentProcesses` runs four separate copies of the Go test binary,
each performing twenty cache-publication and private-extraction cycles against
the same hash. Two aliases have distinct file contents, so the test detects
both incomplete cache reads and alias confusion. Every HTTP response comes
from an in-memory transport; no network service or extension is required.
Other added tests cover debug-value suppression, ambiguous basenames,
comma-bearing metadata names, preservation after a failed cache replacement,
reinstallation from the installed directory, and full CLI core dispatch to
the harmless system `true` executable.

The final command was `go test ./... -count=1 -race
-coverprofile=.audit/problems-after.cover`; it passed without skips.
`go tool cover -func=.audit/problems-after.cover` measured 86.2% of module
statements. The normal build, Linux AMD64 cross-build, vet, staticcheck, module
verification, and whitespace check also passed. Extension-source fixes remain
deferred at the owner's request.

At completion of that follow-up, the same AST predicates measured 71 top-level
`Test*` functions (previously 57) and 305 static assertion/fixture-guard call
sites (previously 250) across the nine test files.

## Obsolete-code cleanup on 2026-09-28

Production-caller queries identified `RandStringRunes`,
`MirrorDirectoryWithSymLinks`, `mac_binary`, and `doesThirdPartyExist` as used
only by tests. They were removed with their obsolete test-only expectations.
The active paths retain tests for unique execution directories, independent
working copies, both executable formats, and validated/recoverable caches.
The cache-rejection test now calls the real cache validator directly.

File-copy I/O is shared through `tools.CopyFile`; the installation wrapper
adds parent-directory creation. Archive dispatch no longer carries an unused
filename argument or a one-line ZIP wrapper. The raw-binary handler is named
`handleExecutableFile` because it serves both Mach-O and ELF.

The full race suite passes with 69 top-level tests and 294 static
assertion/fixture-guard call sites, using the same nine-file AST census. The
count decrease removes tests for retired code, not coverage of active behavior.
`go tool cover -func=.audit/cleanup-after.cover` reports 86.0% statement
coverage for this tree. Build, Linux AMD64 cross-build, vet, and staticcheck
also pass.

## General-review implementation validation — 2026-09-28

The final command `go test ./... -count=1 -race
-coverprofile=.audit/improvements-final.cover` passes. `go tool cover -func`
reports 84.3% module statement coverage for the expanded implementation. Native
and Linux AMD64 builds, vet, staticcheck, module verification, and whitespace
checks pass. The Go vulnerability scan reports `No vulnerabilities found.`
These measurements establish the checked behavior, not exhaustive correctness.

The module now has 95 top-level Test functions excluding TestMain, compared
with 74 at the preceding review. Static t.Fatal/Fatalf/Error/Errorf sites number
419, compared with 330. The AST census includes all module test files, including
internal/fsutil and fixture guards, excludes `.audit` and dependencies, and
counts each table-driven assertion site once.

New behavioral test files are `config/yaml_test.go`, `rt/strict_test.go`,
`rt/lifecycle_test.go`, `target/improvements_test.go`, `cmd/improvements_test.go`,
`runner/instance_test.go`, `utils/copy_security_test.go`,
`utils/installation_test.go`, and `internal/fsutil/lock_test.go`. Existing tests
were adapted to instance-owned runtime state. Test-only compatibility helpers
reside in the package `test_support_test.go` files and are excluded from builds.

During refactoring, a legacy public-entrypoint test bypassed its old global
mock and launched its synthetic fixture with local Python through default
runtime wiring. It failed on the fixture text. The test now uses help-only
public execution, and target execution tests use explicit dependency injection.
That intermediate run could access normal user configuration/default runtime
storage; the final suite uses repository-local fixtures and offline boundaries.
Staticcheck also identified unused temporary test adapters, which were removed
before the final passing check.

## Usability implementation validation — 2026-09-28

The final command was `go test ./... -count=1 -race
-coverprofile=.audit/usability-final.cover`; every package passed. The aggregate
reported by `go tool cover -func=.audit/usability-final.cover` is 82.7% of
statements. This covers the expanded tree and is not directly comparable as a
quality score with the smaller prior tree's 84.3%. The separate tagged terminal
test is not included in this coverage profile.

Native build, Linux AMD64 cross-build, `go vet ./...`, `staticcheck ./...`,
`go mod verify`, and `git diff --check` passed. `govulncheck ./...` reported
`No vulnerabilities found.` No module dependency was added. These checks do
not certify extension code or every operating-system failure.

The behavioral evidence maps to the requested improvements:

- `cmd/usability_test.go`: shared run/validate prerequisites, all-target results,
  relevant help flags, and rejection of ignored execution flags on validation.
- `cmd/workflow_test.go`: non-overwriting scaffolding, interpreter failures,
  all-target diagnostics, user installation/removal, explicit requirements,
  description-aware completion, redacted human/JSON explain output and default
  provenance, isolation mode, verified cache reporting, retained-path output,
  run inspection, cleanup previews, and CLI cache pruning.
- `target/schema_test.go`: schema descriptions, enums, inclusive bounds, invalid
  bounds/defaults, structured values, opt-in unknown-option rejection, and
  compatibility with undeclared options when that restriction is omitted.
- `target/preflight_test.go`: current-platform HTTP(S) URL validation without
  network requests or URL disclosure.
- `target/cancellation_test.go`: a canceled in-memory HTTP request preserves
  deadline identity without exposing URL data.
- `runner/cancellation_test.go`: a real harmless Go subprocess reaches its
  deadline and its execution directory is removed.
- `rt/storage_test.go`: byte-budget selection, dry-run preservation, active-run
  lease refusal, zero-budget empty-entry removal, and read-only/symlink checks.
- `internal/process/run_test.go`: child exit status, deadline/cancellation
  classification, graceful descendant signaling, and forced termination.
- `internal/process/terminal_integration_test.go`: actual terminal input through
  a managed foreground child. It uses the Go test executable as the child.

The terminal test was compiled with `go test -tags=integration -c
-o .audit/process-tests ./internal/process`, then run in a PTY using
`.audit/process-tests -test.run '^TestTerminalInteraction$' -test.v` with the
synthetic input line `hello`. It passed on the final process implementation.
Run this test explicitly in a terminal; do not add the integration tag to an
unattended full suite that cannot provide that input.

The AST census scans `main_test.go` and every `*_test.go` under cmd, config,
execroot, rt, runner, target, tools, utils, and internal. It counts top-level
`Test*` declarations excluding `TestMain`, including subprocess helper test
entrypoints, and calls to `t.Fatal`, `t.Fatalf`, `t.Error`, or `t.Errorf`.
Fixture guards count; table-driven assertion sites count once; `.audit`, external
dependencies, and Python environments are excluded. The previous source census
was 95 tests and 419 sites; the new totals are 114 tests and 505 sites, with
113/504 in the default build and 1/1 in the tagged integration file. An obsolete
type-classifier-only test was removed with its retired production helper.

Focused red regressions preceded the schema/preflight and zero-cache-budget
fixes. A cache-test fixture initially had the wrong directory permissions; it
was corrected before reproducing the actual zero-budget defect. A subprocess
fixture initially triggered Go's deadlock exit instead of waiting; it now uses
a live timer and exercises cancellation. During editing, a gograph source query
used stale line ranges after a preceding edit; the malformed replacement was
repaired, the graph rebuilt, and the affected package tests passed. No finding
or completion claim relies on those intermediate harness/tool failures.

## v0.1.0 release verification — 2026-09-28

Release preparation added version metadata, packaging and documentation without
changing Go source or tests. `make build` and `make release-assets` succeeded.
The native build and packaged macOS ARM64 binary both printed `v0.1.0` for
`version` and `kamaji version v0.1.0` for `--version`; a plain development build
printed `dev` before release packaging.

The full command was:

```sh
go test ./... -count=1 -race -coverprofile=.audit/release-v0.1.0.cover
```

Its output was:

```text
ok  kamaji                   2.336s  coverage: 100.0% of statements
ok  kamaji/cmd               1.496s  coverage: 85.1% of statements
ok  kamaji/config            1.261s  coverage: 94.1% of statements
ok  kamaji/execroot          1.515s  coverage: 82.2% of statements
ok  kamaji/internal/fsutil   2.239s  coverage: 48.3% of statements
ok  kamaji/internal/process  1.690s  coverage: 62.3% of statements
?   kamaji/obj               [no test files]
ok  kamaji/rt                1.293s  coverage: 78.8% of statements
ok  kamaji/runner            4.484s  coverage: 87.2% of statements
ok  kamaji/target            2.568s  coverage: 87.7% of statements
ok  kamaji/tools             1.282s  coverage: 87.8% of statements
ok  kamaji/utils             1.279s  coverage: 74.4% of statements
```

`go tool cover -func=.audit/release-v0.1.0.cover` reported 82.7% total statement
coverage. `go vet ./...`, `staticcheck ./...`, `go mod verify` and
`git diff --check` passed. `govulncheck ./...` reported
`No vulnerabilities found.` The tagged terminal test was compiled to
`.audit/release-terminal.test` and passed in a PTY with synthetic `hello` input:

```text
=== RUN   TestTerminalInteraction
terminal ready
hello
--- PASS: TestTerminalInteraction (9.27s)
PASS
```

All four release archives passed `shasum -a 256 -c SHA256SUMS`, each contained
only `kamaji` and `LICENSE`, and executable-format inspection confirmed the
intended OS/architecture. Runtime execution was tested on macOS ARM64 only;
the other platforms were cross-built. No extension or infrastructure operation
was exercised.

The test-file inventory is unchanged from the implementation:

```text
main_test.go
cmd/improvements_test.go
cmd/private_dir_test.go
cmd/root_test.go
cmd/test_support_test.go
cmd/usability_test.go
cmd/workflow_test.go
config/UserConfig_test.go
config/yaml_test.go
execroot/execroot_test.go
execroot/private_dir_test.go
execroot/test_support_test.go
internal/fsutil/lock_test.go
internal/process/run_test.go
internal/process/terminal_integration_test.go
rt/lifecycle_test.go
rt/private_dir_test.go
rt/rt_test.go
rt/storage_test.go
rt/strict_test.go
rt/test_support_test.go
runner/cancellation_test.go
runner/executor_test.go
runner/instance_test.go
runner/runner_test.go
runner/test_support_test.go
target/cancellation_test.go
target/improvements_test.go
target/preflight_test.go
target/schema_test.go
target/target_test.go
target/test_support_test.go
tools/test_support_test.go
tools/tools_test.go
utils/copy_security_test.go
utils/installation_test.go
utils/test_support_test.go
utils/utils_test.go
```

The prior source census of 114 tests and 505 static assertion/guard sites is a
historical measurement under the predicate above, not a new release census.
An attempted Python-wrapped AST census was blocked by the gograph-first hook;
the agent stopped and the owner explicitly authorized continuation. No tests
or assertions were added or removed during release preparation.

Gograph v1.7.5 precise indexing and uncommitted review passed. The release
session had no Go edits and therefore no pre-edit plan; its compliance grade
is not evidence of a production failure. The preceding implementation session
recorded both plan and review. Review summaries still under-detect environment
reads and error returns, as recorded in the locally ignored gograph report.
No Legattus project or wiki postcheck target exists in this repository; these
checks do not certify extension behavior, other-platform runtime behavior or
binary signing/notarization.
