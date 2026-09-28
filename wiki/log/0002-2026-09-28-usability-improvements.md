## [2026-09-28] implementation | Kamaji usability improvements

Implemented the source-reviewed usability suggestions: shared preflight,
validate-all/doctor, scoped flags, non-overwriting scaffolding, user installation,
explicit requirements, redacted explain, target descriptions/completion,
process-group cancellation and deadlines, child exit statuses, terminal handoff,
run/cache inspection and maintenance previews, and extended schema constraints.

Current command behavior is in docs/CLI_REFERENCE.md and docs/HOW_TO_USE.md;
runtime/security guides and the local repository index were updated. Validation
and implementation limitations are recorded in docs/TESTING.md. No external Go
dependency was added. Extension source and release operations remain excluded.

The implementation remains uncommitted at base HEAD
`84aadbd23ef831c332107363934a03c6d335d1c5`; this entry does not record a new commit.
The prior documentation refresh is a historical slice and retains its original
measurements. All repository-specific wiki changes stay in this local wiki.

Adjacent limits are recorded rather than expanded into new work: schema map/list
types are not recursive schemas; cache budgets are explicit maintenance policies;
process groups are not a sandbox; synchronous filesystem operations are not
preempted by the execution deadline. These are declined as additional scope in
this slice, with the user-facing limits documented at their command descriptions.
