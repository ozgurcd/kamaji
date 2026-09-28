# Download and archive limits

Kamaji limits dependency downloads and archive expansion. The defaults approved
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
