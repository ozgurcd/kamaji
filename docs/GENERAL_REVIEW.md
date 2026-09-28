# General code review — 2026-09-28

For current behavior, start with [Usage](HOW_TO_USE.md) and
[Runtime lifecycle](RUNTIME_LIFECYCLE.md). The later usability additions are
documented in [CLI reference](CLI_REFERENCE.md). The findings and line numbers below
are historical; the implementation follow-up at the end records their closure.

Reviewed the Kamaji core working tree at HEAD
`84aadbd23ef831c332107363934a03c6d335d1c5`, including its existing uncommitted
changes. Evidence comes from current source, offline probes, and fresh checks;
project documentation was not treated as proof. Extension rule source remains
outside scope at the owner's request. No production code was changed in the
review itself. The findings below were open at review time; their subsequent
implementation is recorded in the follow-up at the end of this document.

Scope includes CLI dispatch, user/workspace/build/schema configuration, process
execution, Python setup, cache/download/extraction, file copying, installation,
resource bounds, and the test suite. This is a bounded review, not proof that
all possible defects have been found.

## Prioritized findings

### GR01 — P2 security: download failures expose URL query data

`target/target.go:273` (`downloadFile`) returns the HTTP client's error directly.
`main.go:9` prints the resulting error. Transport errors include the requested
URL, including its query string. Signed artifact URLs or query-based access
values can therefore reach stderr and CI logs even without debug logging.

Reproduced offline using an in-memory transport and a public dummy query marker.
The returned error contained that marker; no real credential or network request
was used. This is an error-path disclosure, separate from command debug logging.

Recommended: sanitize user-visible transport errors before they reach stderr;
include the operation and safe endpoint identity, excluding query and userinfo.
Add regression coverage for signed-query URLs and redirect/transport failures.

### GR02 — P2 correctness/usability: malformed user configuration is ignored

`config/UserConfig.go:12` returns an empty map for home lookup, file-open, and
YAML decoding failures. `cmd/root.go:29` then selects interpreter defaults.
An invalid configured interpreter entry can therefore silently turn into a
different interpreter rather than an actionable configuration error.

Source-confirmed; the existing `TestLoadUserConfig` explicitly expects malformed
and wrong-type input to yield an empty map. Those tests passed in this review.

Recommended: treat a missing optional file as normal, but propagate permission
and parse errors with a safe file location. Reserve fallback for absent values.

### GR03 — P2 correctness: configuration typos and trailing documents are ignored

`rt/runtime.go:18` decodes workspace YAML without known-field enforcement and
does not check for a second document. `target/target.go:21` likewise unmarshals
build configuration without rejecting unknown struct fields.

Offline probes reproduced both workspace cases: `max_download_byte: 1` was
ignored and resolved to the default 512 MiB limit; a second YAML document with
a different rules directory was silently ignored. The typo can make a requested
resource restriction ineffective. The malformed-field behavior was measured
for workspace limits; other fields are a source-derived extension of the risk.

Recommended: reject unknown fields in fixed configuration structs and require
exactly one document. Keep dynamic target option maps flexible where intended.
Report the field and location without dumping configuration values.

### GR04 — P2 correctness: duplicate dependency names select the first entry

`target/target.go:170` returns the first matching third-party entry. Unlike
duplicate selected build targets, duplicate dependency names are not rejected.
Reordering a workspace can silently change the selected dependency definition.

Reproduced with two synthetic definitions sharing one name and different
artifact paths: the first path was selected without an error.

Recommended: reject duplicate dependency names during workspace validation,
before performing downloads or starting a process.

### GR05 — P3 performance: repeated references redo cache validation and writes

`target/target.go:159` initializes every top-level `@@` reference independently.
Each successful `validateCachedFile` hashes the full payload and recreates its
metadata (`target/target.go:245`, `tools/tools.go:33,106`). Cold publication also
hashes before rename and again during validation. Archive bytes are copied and
extracted afresh for each execution (`execroot/execroot.go:174`).

The offline probe measured three validations/metadata replacements for three
references to one already-cached dependency, with no extra downloads. This is
an operation count, not a wall-clock benchmark or claimed speedup.

Recommended first step: deduplicate dependency names within one initialization
and avoid rewriting identical metadata. Retain checksum verification of cached
content. Consider extraction caching only with a design preserving immutable
verified content and per-execution write isolation.

### GR06 — P2 reliability: failed setup work and execution copies accumulate

`runner/runner.go:65` creates an execution root and copies/extracts dependencies
before checking that the rule exists. It retains the directory on failure as
well as success. Isolated execution copies the whole working directory, and
there is no cumulative storage quota. `--cleanup` removes all execution/cache
directories without coordinating with active runs (`cmd/root.go:29`).

The offline probe measured three retained execution directories after three
missing-rule failures. Source shows that configured dependency extraction can
precede the same failure; the probe did not create large archives or exhaust disk.

Recommended: validate inexpensive prerequisites before downloading/extracting;
remove incomplete execution roots on setup failure. Make successful-run
retention explicit, with a bounded policy and a keep-for-debugging option.
Coordinate cleanup with active runs rather than deleting in-use state.

### GR07 — P3 hardening: copy helper follows existing destination symlinks

`utils/utils.go:32` rejects symlinks in the source, but its destination writes
use ordinary `MkdirAll` and `CopyFile`. A pre-existing destination directory
symlink redirects writes outside the requested destination. An offline probe
confirmed an overwrite in a separate synthetic directory within its fixture.

Impact is limited by current call sites: gograph found production callers in
`runner.Run` and `CopyRulesToGlobalDir`, both constructing fresh destinations
under private temporary directories. This is not evidence of an exploitable
cross-user escape through those current flows. The helper's public surface is
less safe than its source-side validation suggests.

Recommended: require a fresh destination or use rooted destination writes;
resolve existing parent aliases before deciding whether destination is inside
source. Keep the confinement checks if the helper is reused elsewhere.

## Additional improvement opportunities

- Installation consistency: `utils.CopyRulesToGlobalDir` preserves the old
  installed requirements file when the new source has none; its existing test
  deliberately expects that behavior. Decide whether replacement should instead
  remove stale requirements or require an explicit preservation option. Its
  multi-file rename/rollback sequence also has no inter-process coordination;
  concurrent installers were not exercised in this review.
- Python setup: `rt.SetupPythonEnv` modifies the live environment before pip
  succeeds. CLI selection checks for an executable Python file, not a completed
  dependency installation. A failed update can leave a selectable partial
  environment. Consider staging and promoting an environment only after setup
  succeeds; real Python/pip failure behavior was not tested here.
- CLI usability: add target listing and a validation-only command, better
  missing-target diagnostics, and explicit version output. Current Cobra
  registration was inspected; these are product improvements, not claims of
  broken existing requirements. Preserve the existing `--` argument boundary.
- Maintainability: mutable global runtime state and package-level test hooks
  couple parsing, validation, setup, and execution. Passing an explicit runtime
  and narrow interfaces would make independent invocations and parallel testing
  easier. This is a refactoring option, not a demonstrated production data race.
- Existing security boundary still applies: rules execute with the user's
  permissions; working-copy isolation is not an OS sandbox; arguments can be
  visible to process inspection. See the source-backed executor review.

## Validation and reproduction

Passed on this working tree:

- `go test ./... -count=1 -race`
- `go build ./...`
- `go vet ./...`
- `staticcheck ./...`
- `go mod verify` — `all modules verified`
- `govulncheck ./...` — `No vulnerabilities found.`

The vulnerability result applies to the tool's analyzed Go symbols and current
database response. It does not certify application logic or extension code.
No dependency upgrade was required by this scan.

Seven additional synthetic probes live only under ignored
`.audit/general-review/`. They are loaded through a Go overlay so they do not
change the permanent package tests or normalize defective behavior as a desired
contract. These probes assert the presence of findings; passing them confirms
reproduction, not remediation. Run from the repository root with:

```
go test -overlay=.audit/general-review/overlay.json ./target ./rt ./utils ./runner -run '^TestGeneralReview' -count=1 -v
```

All seven passed. They cover URL error disclosure, repeated cache validation,
duplicate dependency names, ignored workspace fields, ignored trailing YAML,
destination-symlink writes, and retained directories after missing-rule errors.
All filesystem activity stayed within synthetic repository-local temporary
fixtures. Network responses were in-memory. Real credentials, Python, extensions,
global installation directories, and infrastructure were not accessed.

Implementation priority: URL redaction and configuration validation first;
then cheap early validation, bounded retention, and dependency deduplication.
The observations above were recorded in ignored `PROBLEMS.md`.

## Implementation follow-up — 2026-09-28

GR01–GR07 and the additional installation, Python setup, CLI, and runtime-state
improvements have been implemented. Current behavior and defaults are in
[Runtime lifecycle](RUNTIME_LIFECYCLE.md).

- GR01: transport and body errors omit URL/remote error data; covered by the
  redaction regression and existing download failure/body-closure tests.
- GR02–GR04: user configuration errors propagate, fixed YAML fields and document
  counts are checked, and duplicate dependency names fail before downloads.
  Config, workspace, build, and schema tests exercise these paths, including
  preservation of the existing rule-schema `language` field.
- GR05: dependency names are deduplicated, unchanged metadata is preserved,
  and freshly verified downloads avoid a redundant checksum pass. Checksum
  validation remains mandatory on cache reuse. Private extraction remains per
  execution; no shared writable extraction cache was introduced.
- GR06: prerequisites are checked earlier, normal/error execution roots are
  removed, optional debug retention has count/byte bounds, and cleanup holds
  an exclusive lease across deletion. Independent-process lock tests and
  lifecycle/CLI tests verify the coordination.
- GR07: copy destinations must be newly created, with resolved-parent checks
  rejecting aliases into the source. The overwrite regression now rejects the
  operation and preserves the external fixture.
- Installation removes stale requirements and serializes installation/removal.
  Python setup promotes only completed environments without relocating them;
  tests verify failed updates preserve the old selection and script paths stay
  valid after successful promotion.
- CLI target listing, validation-only, version reporting, an explicit run
  command, and available-target diagnostics are implemented and tested.
- Production runtime configuration and mutable dependency hooks moved into
  instances. Parallel command/executor tests pass under the race detector.

Historical overlay probes above describe the pre-fix tree; they are not expected
to pass against the current instance-based API. Permanent regression tests now
assert the repaired behavior. No extension source or release operation changed.
