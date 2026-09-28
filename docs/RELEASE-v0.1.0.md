# Kamaji v0.1.0

Initial versioned release of the Kamaji Go core for running YAML-defined targets
through Python rules.

## Included

- Strict YAML configuration and rule schemas with defaults, required values,
  descriptions, enums, numeric bounds and optional unknown-option rejection.
- Workspace scaffolding, target listing/completion, shared execution preflight,
  `validate --all`, `doctor` and redacted human/JSON `explain`.
- SHA256-verified downloads and cache reuse, atomic publication, bounded
  downloads and archive expansion, and confined extraction paths.
- Independent runtime instances, private execution directories, cleanup,
  bounded optional retention, maintenance locks and cache/run inspection.
- Transactional Python setup, user-scoped rules installation and explicit
  requirements selection.
- Argument-vector execution, optional working-directory copies, execution
  deadlines, graceful process-group cancellation, child exit status preservation
  and interactive terminal restoration.
- Go 1.27.1, updated dependency pins and one YAML v3 parser.

## Downloads

Core binaries are available for macOS and Linux on AMD64 and ARM64. Each archive
contains the executable and MIT license; `SHA256SUMS` covers all binary archives.
Run `kamaji version` to confirm `v0.1.0`. The archives do not bundle Python or
extensions. Start with `kamaji init`, then install Python if needed to run rules.
See [the build and verification guide](https://github.com/ozgurcd/kamaji/blob/v0.1.0/docs/RELEASING.md).

## Scope and limits

Extensions and real infrastructure operations are not certified by the core
test suite. Existing extensions remain unchanged; extension fixes are deferred.
Rules run with the user's permissions and inherited environment: isolation is a
working-directory copy, not a security sandbox. Deadlines cancel HTTP requests
and managed children, but do not preempt synchronous filesystem operations.
Descendants that leave the managed process group are outside cancellation.
Cache budgets apply through explicit maintenance commands, not automatic eviction.
Schemas validate outer map/list types rather than recursive structures.
Binaries are unsigned and not notarized. Cross-compilation is not runtime
validation on every platform.

See [usage](https://github.com/ozgurcd/kamaji/blob/v0.1.0/docs/HOW_TO_USE.md)
and [validation evidence](https://github.com/ozgurcd/kamaji/blob/v0.1.0/docs/TESTING.md).
