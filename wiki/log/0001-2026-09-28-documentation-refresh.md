## [2026-09-28] docs | Kamaji core documentation refresh

Updated current guides against the implemented Go core: commands, strict YAML,
rule argument handling, dependency inventory, storage/retention, installation,
Python selection, and the executor trust boundary. Added a complete minimal
rule example and indexed the guides in the repository-owned wiki.

The tree is uncommitted at base HEAD
`84aadbd23ef831c332107363934a03c6d335d1c5`; this entry records no new code commit.
Historical audits, measurements, and scratch notes retain their original bodies
with status pointers. Extension fixes and v0.1.0 remain deferred. No parent wiki,
extension source, or Go source was changed by this documentation slice.

Validation covers current CLI help/version output, source-backed configuration
semantics, documentation examples, relative Markdown file links, and whitespace.
The Markdown inventory (README.md, PROBLEMS.md, docs/*.md, wiki/**/*.md) passed
checks for 14 files, 52 relative file links, local anchors, balanced code fences,
and five single-document YAML examples. The documented
Python example printed its expected greeting and extra argument in an isolated
in-memory check; no bundled extension or infrastructure command was run.
The full implementation race-suite evidence remains in docs/TESTING.md; a
documentation-only refresh does not create a new coverage measurement.

Adjacent limits remain documented in docs/RUNTIME_LIFECYCLE.md: there is no total
disk quota for active working copies/cache, abrupt termination can leave scratch
files, and rules run with user permissions. These are declined as new work in
this documentation slice; no product requirement was added.
