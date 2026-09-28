---
co_versioned: true
---

# Kamaji repository

This repository owns its wiki. It does not use the parent Development wiki.
This page is co-versioned with the Kamaji source. The v0.2.0 release adds generic
language support and runnable examples to the core fixes and usability improvements
from v0.1.0. See the [release notes](../../RELEASE_NOTES.md) and
[release build guide](../../docs/RELEASING.md).

## Current core

The current source tree executes YAML-defined targets through configured
interpreters or compiled executables, including Go, starting with v0.2.0. The
older v0.1.0 release remains Python-only. Start with
[README](../../README.md) and the complete [usage example](../../docs/HOW_TO_USE.md).
The current implementation includes:

- Strict configuration parsing, required/default variable checks, duplicate-name
  rejection, safe YAML/download diagnostics, and a single YAML v3 parser.
- Direct argument-vector execution, correctly resolved working directories and
  interpreters, and independent copies for optional isolation.
- Bounded downloads and archive extraction, atomic cache publication, checksum
  verification on cache reuse, and deduplicated references per initialization.
- Private execution directories, default cleanup, optional bounded retention,
  shared/exclusive maintenance leases, and staged installation with rollback.
- Transactional Python environment selection and instance-owned runtime state.
- Shared execution preflight, all-target validation, doctor, and redacted explain.
- Workspace scaffolding, target descriptions/completions, user installation, and
  explicitly selected Python requirements.
- Optional execution deadlines, graceful process-group cancellation, preserved
  child exit codes, and interactive terminal restoration.
- Cache/run inspection, deletion previews, an explicit cache-pruning budget, and
  extended schemas with optional unknown-option rejection.

[CLI reference](../../docs/CLI_REFERENCE.md) describes command syntax and limits.
[Rule languages](../../docs/RULE_LANGUAGES.md) defines the execution contract and Go example.
[Runnable examples](../../docs/EXAMPLES.md) includes a Go greeting and a release
workflow with Python inventory, compiled Go verification, Ruby policy checks,
and JavaScript reporting. The workflow is explicitly sequenced by the caller;
Kamaji does not infer target dependencies.

[Runtime lifecycle](../../docs/RUNTIME_LIFECYCLE.md),
[resource limits](../../docs/RESOURCE_LIMITS.md), and
[executor security](../../docs/EXECUTOR_SECURITY.md) define the documented
behavior and limits. Rules keep user permissions; isolation is not a sandbox.

The Go directive is 1.27.1. [Dependencies](../../docs/DEPENDENCIES.md) records
the module versions and distinguishes Go libraries from Python requirements.
The usability implementation adds no external Go libraries.
[Testing](../../docs/TESTING.md) records the latest implementation validation
and retains earlier measurements with their original scope.

## Review status

The [initial audit](../audit-2026-09-28.md) and the finding bodies in
[general review](../../docs/GENERAL_REVIEW.md) are historical evidence. Core
fixes and the general-review improvements have been implemented and tested.
The locally ignored `PROBLEMS.md` retains findings and follow-up status.
Extension fixes remain deferred at the owner's request. The owner authorized
the v0.1.0 commit, push and release on 2026-09-28.

The initial scope included extension investigation. Later implementation and
this documentation refresh are limited to Kamaji core; no real infrastructure
operations establish the claims here. A finite review is not proof of
exhaustive correctness. Current facts should be rechecked against code and
tests when the implementation changes.

## Commit ledger

| Date | Revision | Slice |
| --- | --- | --- |
| 2026-09-28 | co-versioned | Core fixes, usability improvements, documentation and v0.1.0 packaging. |
| 2026-09-29 | co-versioned | Generic rule execution, Go support, multilingual examples and v0.2.0 packaging. |
