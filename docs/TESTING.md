# Go tests

Run the complete offline suite from the repository root:

```sh
go test ./... -count=1 -race -coverprofile=coverage.out
```

Other validation commands are `go build ./...`, `go vet ./...`,
`staticcheck ./...`, `go mod verify`, and `git diff --check`.
The module requires Go 1.27.1. There is no repository Makefile verification
target or configured Legattus project.

The latest recorded full validation is the
[v0.3.0 release preparation](#v030-release-preparation--2026-09-30).
The latest coverage profile is from the preceding
[build-system gap closure](#build-system-gap-closure--2026-09-30): 80.1% statement
coverage, including example executables that are exercised separately from the
default coverage run. The preceding graph implementation's
136-test/672-site census is historical (last measured 2026-09-30), not a current
total: the gap closure adds `buildsys/path_regression_test.go`. Its attempted
AST census was blocked and not retried. Earlier sections retain measurements
of earlier trees; their percentages and test populations are not current totals.

The current executor review and its trust boundary are documented in
[Executor security](EXECUTOR_SECURITY.md). Its tests launch the Go test binary
as a stand-in interpreter to check the real process boundary offline.

## What the tests exercise

Graph tests and their latest verification are recorded in the dated build-system
sections below. The following package overview describes the legacy runner.

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

## Generic rule execution validation — 2026-09-29

The complete command `go test ./... -count=1 -race
-coverprofile=.audit/generic-rules.cover` passed:

```text
ok  kamaji                          2.342s   coverage: 100.0% of statements
ok  kamaji/cmd                      1.475s   coverage: 85.0% of statements
ok  kamaji/config                   1.214s   coverage: 94.1% of statements
    kamaji/examples/go-rule/rules/hello     coverage: 0.0% of statements
ok  kamaji/execroot                 1.327s   coverage: 82.2% of statements
ok  kamaji/internal/fsutil          2.263s   coverage: 48.3% of statements
ok  kamaji/internal/process         1.692s   coverage: 62.3% of statements
?   kamaji/obj                      [no test files]
ok  kamaji/rt                       1.428s   coverage: 78.8% of statements
ok  kamaji/runner                  11.551s   coverage: 86.0% of statements
ok  kamaji/target                   2.478s   coverage: 88.3% of statements
ok  kamaji/tools                    1.282s   coverage: 87.8% of statements
ok  kamaji/utils                    1.242s   coverage: 74.4% of statements
```

`go tool cover -func=.audit/generic-rules.cover` reported 82.5% aggregate
statement coverage. The example program was built and executed separately;
that manual execution is not included in the coverage profile. The previous
release profile was 82.7% on the smaller tree.

New test files:

- `target/language_test.go`: default/alias languages, open-ended language labels,
  explicit execution modes, command validation and strict nested YAML fields.
- `runner/go_rules_test.go`: compiled Go subprocesses with no Python, scalar and
  structured options, trailing argument boundaries, inherited environment,
  isolation, preserved child failures, deadlines and cleanup.
- `runner/generic_rules_test.go`: custom interpreted and direct executable
  modes, fixed interpreter arguments and relative interpreter resolution before
  changing the child's directory.
- `cmd/go_rules_test.go`: run/validate/doctor/explain without loading Python
  configuration or looking up Python; defaults and native diagnostics.
- `cmd/generic_rules_test.go`: mixed-language validation, Python compatibility,
  custom interpreter diagnostics, redaction and rejection of invalid rules
  before runtime writes.

`runner/runner_test.go` now supplies a default schema alongside Python fixtures;
direct executor callers must provide schemas just like CLI callers. All
previous Python tests passed. Expected failing regressions preceded both the
Go dispatch implementation and its generalization to execution modes.

The census used `ast-grep run --lang go --kind function_declaration` and
`--kind call_expression`, JSON output, `--globs '*_test.go'`, and the roots
`main_test.go cmd config execroot internal rt runner target tools utils`.
It counted top-level declarations starting `func Test` excluding `TestMain`,
and call nodes starting `t.Fatal(`, `t.Fatalf(`, `t.Error(` or `t.Errorf(`.
This includes helper test entrypoints and fixture guards, counts table-driven
sites once, and excludes scratch files and dependencies. It measured 121 tests
and 557 assertion/guard sites, versus the previously recorded 114 and 505.
The unchanged tagged terminal test contributes one test and one guard site.

`make build`, Linux AMD64 cross-build, `go vet ./...`, `staticcheck ./...`,
`go mod verify`, documentation link checks and `git diff --check` passed.
The Go example compiled, printed the documented greetings, and passed CLI
validation/explanation with KAMAJI_PYTHON pointing to a nonexistent executable.
Its binary is ignored. No Go module dependency changed.

Generic interpreter tests use a harmless Go helper process to observe the
process contract; they do not certify installed Node, Ruby, Java or other
language environments. No extension or real infrastructure operation ran.
The existing process implementation and tagged terminal test were unchanged;
the interactive terminal ceremony was not repeated for this dispatch change.

Gograph precise build and final uncommitted review passed. One intermediate
build refused publication because source changed during indexing; it was
rebuilt after edits stopped. Some review summaries still report undetected
environment reads/error returns; source and executable tests establish those
behaviors instead.

## Multilingual examples verification — 2026-09-29

The runnable [examples guide](EXAMPLES.md) adds a release workflow with Python
inventory, compiled Go verification, Ruby policy checks, and JavaScript
reporting. No core implementation or dependency changed in this examples slice.

The new test file is `examples/release-workflow/workflow_test.go`, tagged
`integration`. Its first run failed because `BUILD.yaml` had not been created.
After adding the workflow, the final focused command was:

```sh
go test -tags=integration ./examples/release-workflow -run '^TestReleaseWorkflow$' -count=1 -v
```

```text
=== RUN   TestReleaseWorkflow
Inventoried 2 files -> out/inventory.json
Verified 2 files and expected metadata
Policy passed: 2 files, 78 bytes
Wrote out/report.md
Policy rejected: total bytes exceed budget
Wrote out/report.md
verify: size or SHA256 mismatch for "inputs/app.txt"
--- PASS: TestReleaseWorkflow (0.64s)
PASS
ok  kamaji/examples/release-workflow  1.203s
```

This exercises real local interpreters and a freshly compiled Go executable
through `cmd.NewCommand`, with workspace and runtime storage in separate
temporary directories. It checks validation, native Go explanation, the report,
preservation of policy exit code 3, a same-length content mutation rejected by
Go, and isolated output retained without changing the original report. User
Python configuration is replaced by a callback that fails if accessed. The test
skips with a reason when a required language runtime is unavailable. This run
used all required runtimes and did not skip.

The complete default suite also passed:

```sh
go test ./... -count=1 -race -coverprofile=.audit/multilingual-examples.cover
```

```text
ok  kamaji                          2.384s  coverage: 100.0% of statements
ok  kamaji/cmd                      1.526s  coverage: 85.0% of statements
ok  kamaji/config                   1.300s  coverage: 94.1% of statements
    kamaji/examples/go-rule/rules/hello           coverage: 0.0% of statements
    kamaji/examples/release-workflow/rules/verify coverage: 0.0% of statements
ok  kamaji/execroot                 1.484s  coverage: 82.2% of statements
ok  kamaji/internal/fsutil          2.302s  coverage: 48.3% of statements
ok  kamaji/internal/process         1.738s  coverage: 62.3% of statements
?   kamaji/obj                      [no test files]
ok  kamaji/rt                       1.339s  coverage: 78.8% of statements
ok  kamaji/runner                  11.133s  coverage: 86.0% of statements
ok  kamaji/target                   2.519s  coverage: 88.3% of statements
ok  kamaji/tools                    1.328s  coverage: 87.8% of statements
ok  kamaji/utils                    1.327s  coverage: 74.4% of statements
```

`go tool cover -func=.audit/multilingual-examples.cover` reports 80.6% aggregate
statement coverage, compared with 82.5% on the preceding tree. The default
profile includes the new example executable as uncovered code; its execution
in the separate integration command is not instrumented in this profile.

The AST census uses the preceding section's predicate and adds `examples` to
the scanned roots. It measures 122 `Test*` declarations and 576 static
assertion/guard sites, compared with 121 and 557 before this slice. The new
integration test contributes one declaration and 19 sites. The default-build
subset remains 120/556; tagged tests contribute 2/20 to the combined 122/576.
The test-file inventory is the earlier release inventory plus the five generic
execution test files listed above and `examples/release-workflow/workflow_test.go`.

`go build ./...`, `staticcheck ./...`, and
`staticcheck -tags=integration ./examples/release-workflow` passed. Precise
gograph indexing with integration tags and post-edit review passed. Its static
review does not connect the child executable to the integration test and still
reports `Error returns: none detected` for the verifier despite its explicit
error paths; executable assertions establish the behavior. No bundled extension,
cloud operation, or interactive terminal test was run for this examples slice.

## v0.2.0 release verification — 2026-09-29

Release preparation changed version metadata, documentation, and archive packaging;
it did not change Go source or tests after the example verification above.
The complete release test command passed:

```sh
go test ./... -count=1 -race -coverprofile=.audit/release-v0.2.0.cover
```

```text
ok  kamaji                          2.488s  coverage: 100.0% of statements
ok  kamaji/cmd                      1.680s  coverage: 85.0% of statements
ok  kamaji/config                   1.377s  coverage: 94.1% of statements
    kamaji/examples/go-rule/rules/hello           coverage: 0.0% of statements
    kamaji/examples/release-workflow/rules/verify coverage: 0.0% of statements
ok  kamaji/execroot                 1.456s  coverage: 82.2% of statements
ok  kamaji/internal/fsutil          2.381s  coverage: 48.3% of statements
ok  kamaji/internal/process         1.821s  coverage: 62.3% of statements
?   kamaji/obj                      [no test files]
ok  kamaji/rt                       1.605s  coverage: 78.8% of statements
ok  kamaji/runner                  10.695s  coverage: 86.0% of statements
ok  kamaji/target                   2.646s  coverage: 88.3% of statements
ok  kamaji/tools                    1.537s  coverage: 87.8% of statements
ok  kamaji/utils                    1.475s  coverage: 74.4% of statements
```

The separate command `go test -tags=integration ./examples/release-workflow
-run '^TestReleaseWorkflow$' -count=1 -v` exercised the real language runtimes:

```text
=== RUN   TestReleaseWorkflow
Inventoried 2 files -> out/inventory.json
Verified 2 files and expected metadata
Policy passed: 2 files, 78 bytes
Wrote out/report.md
Policy rejected: total bytes exceed budget
Wrote out/report.md
verify: size or SHA256 mismatch for "inputs/app.txt"
--- PASS: TestReleaseWorkflow (0.95s)
PASS
ok  kamaji/examples/release-workflow  1.433s
```

The terminal test was compiled with `go test -tags=integration -c
-o .audit/release-v0.2.0-terminal.test ./internal/process`, then run in a PTY
with `-test.run '^TestTerminalInteraction$' -test.v` and synthetic input `hello`:

```text
=== RUN   TestTerminalInteraction
terminal ready
hello
--- PASS: TestTerminalInteraction (11.92s)
PASS
```

`make build`, `make release-assets`, `go vet ./...`, `staticcheck ./...`, and
`go mod verify` passed. `govulncheck ./...` reported `No vulnerabilities found.`
Native and packaged macOS ARM64 version commands returned `v0.2.0` and
`kamaji version v0.2.0`. Executable-format inspection confirmed macOS/Linux
AMD64/ARM64 outputs. Other-platform runtime behavior was not exercised.

An initial archive assertion failed because macOS tar included AppleDouble
`._kamaji` and `._LICENSE` metadata members, which its own default listing hid.
After the owner authorized continuation, packaging was corrected with
`COPYFILE_DISABLE=1`. The rebuilt archives all passed SHA256 comparison against
`SHA256SUMS` and Python tarfile inspection requiring exactly `kamaji` and
`LICENSE`, both regular files, with execute bits on `kamaji`. The macOS ARM64
packaged version check passed again. Local Markdown link targets and
`git diff --check` passed.

The test-file inventory and the preceding 122-test/576-site census are unchanged;
no new census is claimed for release preparation. Gograph precise indexing,
uncommitted plan, and review passed. Its broad uncommitted review includes
unchanged symbols and under-detects environment reads/error returns, so Git's
manifest and executable tests establish scope and behavior. No Legattus project
or repository-local wiki postcheck target exists here. The untracked kubeseal
extension and ignored local findings/artifacts remain outside the release commit.

## Graph build system verification — 2026-09-30

The source-tree graph engine adds strict TOML/YAML build documents, dependency
scheduling, content-based artifact caching, JSON plans/results/events, explicit
effect permissions, affected-target queries, evidence records, and cleanup.
The default scaffold is now TOML; legacy scaffold tests explicitly request
`--template minimal` and continue exercising the old runner.

New test files:

- `buildsys/config_test.go`: document discovery, strict parsing, graph cycles,
  unknown dependencies, output ownership, source/output overlap, and generated
  executable dependency requirements.
- `buildsys/plan_test.go`: stable content-bound plans, redacted environment
  values, changed source/environment fingerprints, generated executables,
  affected-target closure, missing inputs, and symlink rejection.
- `buildsys/run_test.go`: subprocess execution, verified cache reuse and
  restoration, corruption recovery, same-process evidence, dependency-aware
  parallel execution, source changes before/during execution, policy rejection
  and explicit permission, child failures, and timeouts.
- `buildsys/clean_test.go`: read-only cleanup preview, source preservation,
  retained evidence, and corrupt history rejection.
- `buildsys/example_integration_test.go`: tagged real Go compilation, generated
  executable consumption, deleted-output restoration, and rerun verification.
- `cmd/build_test.go`: default TOML scaffolding, no-overwrite behavior, familiar
  inspection commands, ignored-flag rejection, JSON plans/results, history, and
  affected-target output.

New APIs were first exercised before implementation. Focused failing regressions
also preceded fixes for source changes after planning, absent cache explanations,
ignored legacy flags, undeclared generated-tool dependencies, dot-slash generated
executables, and corrupt evidence. During implementation an unused import and
two patch-context mismatches were corrected before final validation. These were
development errors, not passing checks or hidden hook bypasses.

The full default suite passed:

```sh
go test ./... -count=1 -race -coverprofile=.audit/build-system.cover
```

```text
ok  kamaji                          2.297s  coverage: 100.0% of statements
ok  kamaji/buildsys                12.004s  coverage: 83.3% of statements
ok  kamaji/cmd                      1.494s  coverage: 79.7% of statements
ok  kamaji/config                   1.291s  coverage: 94.1% of statements
    kamaji/examples/build-project                coverage: 0.0% of statements
    kamaji/examples/go-rule/rules/hello           coverage: 0.0% of statements
    kamaji/examples/release-workflow/rules/verify coverage: 0.0% of statements
ok  kamaji/execroot                 1.359s  coverage: 82.2% of statements
ok  kamaji/internal/fsutil          2.306s  coverage: 48.3% of statements
ok  kamaji/internal/process         1.683s  coverage: 62.3% of statements
?   kamaji/obj                      [no test files]
ok  kamaji/rt                       1.280s  coverage: 78.8% of statements
ok  kamaji/runner                  10.691s  coverage: 86.0% of statements
ok  kamaji/target                   2.485s  coverage: 88.3% of statements
ok  kamaji/tools                    1.273s  coverage: 87.8% of statements
ok  kamaji/utils                    1.286s  coverage: 74.4% of statements
```

`go tool cover -func=.audit/build-system.cover` reports 79.7% aggregate statement
coverage, compared with the earlier release tree's 80.6%. The profile includes
uncovered example executables; their separate integration execution is not
instrumented in this default profile.

The tagged command was scoped to non-interactive packages:

```sh
go test -tags=integration ./buildsys ./examples/release-workflow -count=1 -race -v
```

It passed all selected tests, including:

```text
--- PASS: TestCompiledArtifactGraph (0.66s)
--- PASS: TestBuildParallelDependencies (1.06s)
PASS
ok  kamaji/buildsys 12.620s
=== RUN   TestReleaseWorkflow
Inventoried 2 files -> out/inventory.json
Verified 2 files and expected metadata
Policy passed: 2 files, 78 bytes
Wrote out/report.md
Policy rejected: total bytes exceed budget
Wrote out/report.md
verify: size or SHA256 mismatch for "inputs/app.txt"
--- PASS: TestReleaseWorkflow (0.81s)
PASS
ok  kamaji/examples/release-workflow 2.052s
```

`go build ./...`, `go vet ./...`, `staticcheck ./...`,
`staticcheck -tags=integration ./buildsys ./examples/release-workflow`, and
`go mod verify` passed. `govulncheck ./...` reported `No vulnerabilities found.`
A Linux AMD64 binary was cross-built; its runtime was not exercised.

A separately compiled CLI ran the checked-in Go graph and verified
capabilities, a plan-bound build, cache hits, history identity, affected-target
selection, read-only cleanup, and JSON event framing. Its second build reported
`compile=cached, render=cached, check=executed`. The resulting greeting matched
the expected contents. This proof is stored locally in the ignored
`.audit/build-cli-proof.json`; it performs no infrastructure operation.

The static census scans `main_test.go` and `*_test.go` under cmd, config,
execroot, internal, rt, runner, target, tools, utils, examples, and buildsys.
It counts `Test*` function declarations excluding TestMain and AST call sites
to `t.Fatal`, `t.Fatalf`, `t.Error`, or `t.Errorf`; fixture guards and helper
entrypoints count, table-driven sites count once, and scratch/dependencies are
excluded. Using `ast-grep` with that predicate measured 136 tests and 672 sites,
up from the preceding 122/576. The default subset is 133/646; tagged files
contribute 3/26, summing to 136/672. The inventory is the preceding release
inventory plus the six files listed above; `cmd/workflow_test.go` was modified
to make its legacy-template selection explicit.

The shared process implementation was unchanged, so its interactive PTY ceremony
was not repeated. Graph children intentionally have no interactive stdin. Cache
and effect declarations do not establish hermeticity or OS sandboxing; see the
build guide and executor trust boundary. No extension implementation was changed.

## Build-system gap closure — 2026-09-30

The completion recheck found that glob inputs could bypass dependencies on
generated directory trees and that affected-target queries missed normalized
local executable paths. The new `buildsys/path_regression_test.go` first
reproduced these defects and related failures before the fixes:

- `TestOutputTreeGlobDependencies`: recursive, wildcard and character-class
  patterns require output-tree producers; unrelated patterns remain valid;
  a target cannot consume its own output tree.
- `TestGeneratedDirectoryGlobBuild`: clean-workspace planning, execution and
  cached directory restoration with generated glob inputs.
- `TestAffectedLocalProgramPaths`: relative, dot-slash and absolute local
  command/tool paths, directory changes, reverse dependencies and unrelated paths.
- `TestAbsoluteGeneratedToolDependencies`: root-aware validation and planning
  for absolute generated executable paths before the file exists.
- `TestGeneratedFileParentInputs`: producer-created directory metadata does not
  invalidate a plan, while source siblings still affect its identity.
- `TestPlanAppliesValidationDefaults`: direct API callers receive plans with
  the same defaults used by execution.

The complete default suite passed:

```sh
go test ./... -count=1 -race -coverprofile=.audit/build-gap-fixes.cover
```

```text
ok  kamaji                          2.609s  coverage: 100.0% of statements
ok  kamaji/buildsys                 20.920s coverage: 84.8% of statements
ok  kamaji/cmd                      1.624s  coverage: 79.7% of statements
ok  kamaji/config                   1.407s  coverage: 94.1% of statements
    kamaji/examples/build-project                coverage: 0.0% of statements
    kamaji/examples/go-rule/rules/hello           coverage: 0.0% of statements
    kamaji/examples/release-workflow/rules/verify coverage: 0.0% of statements
ok  kamaji/execroot                 1.462s  coverage: 82.2% of statements
ok  kamaji/internal/fsutil          2.433s  coverage: 48.3% of statements
ok  kamaji/internal/process         1.840s  coverage: 62.3% of statements
?   kamaji/obj                      [no test files]
ok  kamaji/rt                       1.453s  coverage: 78.8% of statements
ok  kamaji/runner                  11.557s  coverage: 86.0% of statements
ok  kamaji/target                   2.602s  coverage: 88.3% of statements
ok  kamaji/tools                    1.433s  coverage: 87.8% of statements
ok  kamaji/utils                    1.432s  coverage: 74.4% of statements
```

`go tool cover -func=.audit/build-gap-fixes.cover` reports 80.1% aggregate
statement coverage; the preceding graph tree measured 79.7%. Both production
code and test coverage changed. Example execution in the separate integration
command is not included in this default coverage profile.

```sh
go test -tags=integration ./buildsys ./examples/release-workflow -count=1 -race
```

```text
ok  kamaji/buildsys 21.608s
ok  kamaji/examples/release-workflow 2.296s
```

Native build, Linux AMD64 cross-build, `go vet ./...`, `staticcheck ./...`,
`staticcheck -tags=integration ./buildsys ./examples/release-workflow`, and
`go mod verify` passed. `govulncheck ./...` reported `No vulnerabilities found.`
The shared process implementation was unchanged;
interactive terminal validation was not repeated. Linux runtime behavior was
not exercised.

A freshly compiled CLI reproduced both original cases with corrected outcomes:
`affected` includes the local executable's target and its reverse dependencies;
an undeclared directory-glob dependency fails before runtime storage or output
mutation. Adding the dependency executes the producer first and the consumer
reads the new content. Default TOML scaffolding also builds successfully.
Synthetic fixtures and structured results are retained locally under
`.audit/closed-build-gaps-cfl_t4ty/` and are ignored by Git.

The existing test inventory gains only `buildsys/path_regression_test.go` in
this slice. A Python-wrapped AST census was blocked by the workspace hook;
work stopped until the owner explicitly authorized continuation. That command
was not retried, and no new total test/assertion census is claimed. Gograph
remained the source-discovery tool. Passing suites and these focused proofs
close the reported gaps; they are not a proof of exhaustive correctness.

## Build-system requirements recheck — 2026-09-30

An independent pass checked the working tree after the graph gap closure, based
on HEAD `18bd686b43cdc7395b6698bced041b008d830ae5` plus the uncommitted graph
implementation and fixes. This is working-tree evidence, not a claim that the
published v0.2.0 binary contains these features. No source or tests changed in
that verification pass.

These commands all passed:

```sh
go test ./... -count=1 -race
go test -tags=integration ./buildsys ./examples/release-workflow -count=1 -race
go build -o .audit/kamaji-requirements-check .
go vet ./...
staticcheck ./...
git diff --check
```

The tagged suites exercised the compiled Go artifact graph and the local
Python/Go/Ruby/JavaScript release workflow. No test/assertion census or new
coverage profile was produced; the preceding gap-closure coverage measurement
retains its original scope.

A separately compiled CLI was exercised in disposable local projects. The
observed results were:

- Fresh `init`, `validate --all`, `doctor`, `targets`, and `build` succeeded;
  an explicitly selected graph-schema YAML document also built.
- Planning did not create `.kamaji`; a matching `--expect-plan` executed, while
  a source change rejected the old plan and preserved the previous output.
- Repeated builds reused the producer and reran its uncached checker. Cache
  availability did not change the same-input plan ID; source changes caused
  execution, and `--no-cache` bypassed reuse.
- Altered outputs were restored; corrupt cache payloads caused regeneration
  and repair rather than acceptance.
- `affected` selected a changed source's producer and consumer. Capabilities,
  plans, results, history, and event records parsed as JSON. History matched the
  emitted result, and child output stayed on stderr during event streaming.
- Declared external effects were denied without creating a marker and permitted
  with `--allow-effects`. This proof used synthetic local file creation.
- A child exit of 7 was preserved; its dependent was blocked without executing.
  A timed-out child returned 124.
- Cleanup previews preserved files. Requested output/cache/history cleanup
  removed those artifacts while preserving source files.

Local detailed evidence is retained in ignored
`.audit/requirements-verification.txt` and
`.audit/requirements-check-dc2m7je5/requirements-result.json`; these are not
published project dependencies. Tests and documented example commands provide
reproducible checks for another checkout. See the
[Go build walkthrough](../examples/build-project/README.md) and
[agent workflow example](AGENT_WORKFLOWS.md).

This establishes the exercised declared-input behavior, not exhaustive
correctness, hermeticity, OS sandboxing, remote execution, or automatic import
inference. No infrastructure extension, live network service, interactive PTY,
Bazel performance comparison, or other-platform runtime was checked in this
pass. The read-only gograph session completed with 7 successful commands and no
failed commands; plan/review were not run because there were no Go edits.

### Documentation examples checked on 2026-09-30

The subsequent documentation-only refresh built the CLI with `make build` and
checked command help against the graph reference. It executed the Python block
in [Agent workflows](AGENT_WORKFLOWS.md) unchanged: planning, plan-bound build,
and equality of the emitted result and saved history all passed.

The [Go walkthrough](../examples/build-project/README.md) passed repeated cache
reuse, cleanup preview, output deletion/restoration, affected selection, explain,
and explicit cache bypass. Repeated and restored builds reported `cached` for
`compile` and `render`, and `executed` for `check`. Forced execution reported
`executed` for every target. The focused integration command produced:

```text
=== RUN   TestCompiledArtifactGraph
--- PASS: TestCompiledArtifactGraph (1.21s)
PASS
ok  	kamaji/buildsys	1.612s
```

Command: `go test -tags=integration ./buildsys -run '^TestCompiledArtifactGraph$' -count=1 -v`.
The test is in `buildsys/example_integration_test.go`. Local Markdown file and
heading links and `git diff --check` were also checked. No Go source or test
changed, and no assertion census or full-suite rerun was needed for this prose
refresh; the preceding full validation remains separately recorded above.

## v0.3.0 release preparation — 2026-09-30

The release includes the graph implementation and regression fixes recorded
above, documentation, and Homebrew packaging. No Go source or test was changed
by the packaging slice. The full offline command was:

```sh
go test ./... -count=1 -race
```

```text
ok  kamaji                     2.496s
ok  kamaji/buildsys           20.529s
ok  kamaji/cmd                 1.712s
ok  kamaji/config              1.415s
?   kamaji/examples/build-project [no test files]
?   kamaji/examples/go-rule/rules/hello [no test files]
?   kamaji/examples/release-workflow/rules/verify [no test files]
ok  kamaji/execroot            1.539s
ok  kamaji/internal/fsutil     2.419s
ok  kamaji/internal/process    1.888s
?   kamaji/obj [no test files]
ok  kamaji/rt                  1.645s
ok  kamaji/runner             11.741s
ok  kamaji/target              2.671s
ok  kamaji/tools               1.469s
ok  kamaji/utils               1.463s
```

The local integration command was:

```sh
go test -tags=integration ./buildsys ./examples/release-workflow -count=1 -race
```

```text
ok  kamaji/buildsys                  21.709s
ok  kamaji/examples/release-workflow   2.382s
```

`go vet ./...`, `staticcheck ./...`, and `go mod verify` passed;
`govulncheck ./...` reported `No vulnerabilities found.` This is the scanner's
result at release preparation, not a permanent vulnerability guarantee.
No new coverage profile or assertion census was produced. The tagged example
tests are `buildsys/example_integration_test.go` and
`examples/release-workflow/workflow_test.go`.

`make release-assets` produced macOS/Linux archives for AMD64 and ARM64, each
containing only `kamaji` and `LICENSE`. The formula generator first rejected a
deliberately invalid checksum without emitting a formula, then passed using
verified archives. `brew style` initially required a frozen-string comment;
that was added and the generated formula passed with no offenses. A version
smoke assertion initially captured stdout only; the existing version command
writes to stderr, so the formula and smoke capture include stderr. No CLI
behavior was changed to satisfy that assertion.

The packaged macOS ARM64 binary reported v0.3.0 and passed capabilities,
read-only planning, execution bound to the plan ID, output restoration from
cache, and exact saved-history equality. Other platform assets were cross-built,
not executed. Interactive PTY and real infrastructure-extension ceremonies were
not repeated by this packaging slice. Homebrew installation and its formula
test are post-publication checks, separate from these preparation results.
