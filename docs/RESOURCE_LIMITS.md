# Build, download, and archive limits

## Graph builds (v0.3.0 and newer)

The graph engine reads a regular project document of at most 4 MiB. Cache
manifest reads are limited to 4 MiB; invalid or oversized manifests are cache
misses. Saved run-record reads are limited to 8 MiB and reject oversized
records. These are document-read limits, not quotas on child output, generated
artifacts, total cache storage, or run-record creation.

`build --jobs` limits concurrent scheduling slots; a target's `slots` consumes
that budget. It does not limit CPU threads or memory within a child process.
`--timeout` limits the whole build and a target's `timeout` limits its execution;
cancellation terminates child process groups. Synchronous filesystem work such
as hashing or copying is not preempted mid-operation.

Project-local `.kamaji/cache` and `.kamaji/runs` have no automatic size budget or
retention policy. Use `clean --cache` or `clean --history` for explicit removal,
with `--dry-run` to preview; `clean` also selects declared outputs for deletion.
See [Runtime lifecycle](RUNTIME_LIFECYCLE.md) for storage and locking.

The legacy download/archive limits below do not constrain arbitrary graph
commands. A child can download data or write undeclared files with the user's
permissions. Effect declarations and scheduling limits are not an OS sandbox.

## Legacy dependency downloads and archives

The legacy rule runner limits dependency downloads and archive expansion. The defaults approved
on 2026-09-28 are 512 MiB per download, 2 GiB of extracted file data per archive,
and 10,000 archive members per archive. HTTP downloads also have an overall
two-minute timeout.

These are per-download/per-archive limits. Retained execution files have a
separate optional count/byte policy; active working copies and the cache do not
have a total disk quota. See [Runtime lifecycle](RUNTIME_LIFECYCLE.md).

`cache prune --max-bytes` can explicitly bring recognized cached entries within
a selected maintenance budget, with a `--dry-run` preview. It does not impose an
automatic download-time quota. Optional `run --timeout` additionally cancels
downloads through the execution context; the two-minute HTTP timeout still
applies when it expires earlier.

Override the byte and member limits in `kamaji.workspace.yaml`:

```yaml
limits:
  max_download_bytes: 536870912
  max_extract_bytes: 2147483648
  max_archive_entries: 10000
```

Values are integer bytes or member counts. Omitted fields and zero select their
defaults. Negative or overflowing values are errors; zero does not disable a
limit. No target is launched when workspace configuration is invalid.

The download limit applies both to HTTP streams and existing cached payloads.
An oversized declared HTTP body is rejected before opening the download file.
For unknown-length bodies, Kamaji writes at most the allowed bytes and probes
one more byte to detect overflow. Failed downloads never replace a verified
cache payload, and their temporary files are removed.

ZIP member counts and declared expanded sizes are checked before extraction.
Individual file copies are bounded too. Tar limits count the logical members
returned by Go's tar reader, including directories; regular file sizes share
the expanded-data budget. TAR/PAX metadata and padding additionally share a
decoded-stream ceiling of `max_extract_bytes + 1024 * max_archive_entries +
1048576` bytes. This also bounds trailing gzip expansion. Kamaji consumes the
bounded gzip remainder to verify its checksum before accepting the archive.

Both archive formats reject path traversal and symlink entries. Tar hardlinks
and other special entries are unsupported. Extraction occurs in private
execution directories, so a failed extraction is never published as a runnable
dependency. These limits bound downloaded and expanded data; they are not a
general process-memory or execution-time sandbox for extension commands.

Cache payload and metadata publication uses same-directory atomic renames.
Independent processes may download identical content simultaneously, but they
cannot expose a partially written replacement. Executable paths come from each
invocation's configuration, and extraction is never shared. The offline
`TestCacheConcurrentProcesses` regression exercises independent writers and
readers with two aliases. This is a concurrency guarantee, not a promise of
durability across power loss.
