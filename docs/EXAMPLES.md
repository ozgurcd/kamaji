# Runnable build and rule examples

The graph example requires Kamaji v0.3.0 or newer. The legacy
Go and multilingual rule examples run with published Kamaji v0.2.0; the older
v0.1.0 binaries support Python rules only. Paths in the commands below assume you
start at the Kamaji repository root. All inputs are synthetic local files;
none of these examples downloads dependencies or runs infrastructure tools.

## Graph build example

The [build-project walkthrough](../examples/build-project/README.md)
uses the new single-file TOML build model. From the repository root:

```sh
make build
cd examples/build-project
../../kamaji plan --json
../../kamaji build --jobs 2
../../kamaji build --json
../../kamaji affected main.go --json
../../kamaji clean --dry-run
```

The first build executes `compile`, `render`, and `check` in dependency order.
The second restores/reuses `compile` and `render`, then executes the non-cacheable
`check` again. The example's compiler target explicitly passes Go cache/temp
settings when present and disables user Go configuration. No external modules
are used by its generator. Its graph integration test compiles the actual Go
program, deletes both outputs, rebuilds, and checks cache restoration plus the
verification target:

```sh
go test -tags=integration ./buildsys -run '^TestCompiledArtifactGraph$' -count=1 -v
```

Run that test command from the repository root. See [the build guide](HOW_TO_USE.md)
for configuration and [Agent workflows](AGENT_WORKFLOWS.md) for the JSON interface. The following examples use
the legacy rule-runner interface, available in v0.2.0.

## Go: a compiled greeting rule

The [Go greeting source](../examples/go-rule/rules/hello/main.go) uses `flag`
for a string option and positional names. Its [schema](../examples/go-rule/rules/hello/rule_definition.yaml)
declares `language: go`; the [target](../examples/go-rule/BUILD.yaml) points
to the compiled binary, not to `main.go`.

```sh
make build
go build -o examples/go-rule/rules/hello/hello ./examples/go-rule/rules/hello
cd examples/go-rule
../../kamaji validate hello-go
../../kamaji run hello-go -- Ada "Grace Hopper"
```

Expected output:

```text
Hello from Go, Ada!
Hello from Go, Grace Hopper!
```

`config.greeting` becomes one `--greeting=Hello from Go` argument. `Ada` and
`Grace Hopper` are separate positional arguments. Put positional arguments
after `--`; the Go `flag` parser stops processing flags at its first positional
argument. Kamaji's separator itself is not forwarded.

Go is needed to compile the rule. A prebuilt Go rule needs neither Go nor Python
installed to run. Compile for the deployment machine's OS and architecture and
ship `rule_definition.yaml` alongside the executable. Kamaji does not build
source automatically or load Go shared-object plugins.

## A multilingual release workflow

The [release workspace](../examples/release-workflow/BUILD.yaml) turns a release
check into explicit targets:

| Target | Implementation | Input → result |
| --- | --- | --- |
| `inventory` | [Python](../examples/release-workflow/rules/inventory/rule.py) | A list of paths and metadata map → `out/inventory.json` with sizes and SHA256 hashes |
| `verify-go` | [Go](../examples/release-workflow/rules/verify/main.go) | Inventory and expected metadata → verification of the actual local files |
| `policy` | [Ruby](../examples/release-workflow/rules/policy/rule.rb) | Inventory, required files, byte budget, and empty-file policy → pass or exit 3 |
| `report` | [JavaScript](../examples/release-workflow/rules/report/rule.js) | Inventory, title, and selected columns → `out/report.md` |

Install `python3`, `ruby`, and `node` on PATH; Go is required for the build step.
The examples use standard libraries only. The Python schema explicitly selects
`[python3, -I]`, so it uses that executable and does not read Kamaji's user Python
configuration. Ruby and JavaScript likewise declare interpreter argument lists.
The Go schema uses the `language: go` default for native execution.

From the repository root, build both Kamaji and the Go verifier:

```sh
make build
go build -o examples/release-workflow/rules/verify/verify ./examples/release-workflow/rules/verify
cd examples/release-workflow
../../kamaji targets
../../kamaji validate --all
../../kamaji explain verify-go --json
```

Validation checks schemas and executable availability without running the rules.
It succeeds even before `out/inventory.json` exists: rule-owned input files are
checked by the rule during execution. Explain identifies the Go rule's
`language: go`, `execution_mode: executable`, and absolute executable path.
Option values remain redacted.

Run the pipeline from that example directory:

```sh
(
  set -e
  ../../kamaji run inventory --timeout 30s
  ../../kamaji run verify-go --timeout 30s
  ../../kamaji run policy --timeout 30s
  ../../kamaji run report --timeout 30s
)
```

Legacy `BUILD.yaml` targets do not declare a graph; the newer `kamaji.toml`
build model uses explicit `deps` for dependency scheduling. Here, the shell
subshell gives this sequence fail-fast behavior without changing your interactive
shell's settings. In CI, use equivalent fail-on-error steps. The report target
only renders an inventory; it does not certify that earlier checks ran.

For the checked-in inputs, the rules print:

```text
Inventoried 2 files -> out/inventory.json
Verified 2 files and expected metadata
Policy passed: 2 files, 78 bytes
Wrote out/report.md
```

Inspect `out/inventory.json` and `out/report.md`. The report contains a title and
a table with path, size, and SHA256 columns. Successful reruns replace these
outputs. Changing an input requires a fresh inventory; changing an input after
inventory generation makes `verify-go` fail with exit 1, even when the byte
length stays the same.

### How the Go verifier accepts structured options

The Go target contains:

```yaml
- name: verify-go
  rule: verify/verify
  config:
    expected_metadata: {project: demo, channel: preview}
```

The sibling schema declares:

```yaml
language: go
allow_unknown: false
variables:
  manifest: {type: string, default: out/inventory.json}
  expected_metadata: {type: map, mandatory: true}
```

Kamaji inserts the manifest default and JSON-encodes the map as one flag value.
The Go program uses `flag.String` to receive `--expected_metadata`, then
`encoding/json` to decode it into a `map[string]string`. It checks each expected
metadata entry, streams each file through `crypto/sha256`, and compares the
size and digest with the inventory. Missing files, malformed inventory JSON,
duplicate or invalid relative paths, metadata mismatches, and changed content
produce a nonzero exit. Extra metadata keys in the inventory are permitted.

The same compiled verifier can run without Kamaji. From the release example
directory, after generating the inventory:

```sh
./rules/verify/verify --manifest=out/inventory.json '--expected_metadata={"project":"demo","channel":"preview"}'
```

The quotes preserve JSON as one shell argument. No Kamaji-specific Go SDK or
external Go library is required. File paths in the inventory are resolved from
the process working directory, not from the inventory file's directory.

### Lists, booleans, constraints, and application checks

Python parses the `files` JSON list and `metadata` JSON map with `json.loads`;
Ruby uses `JSON.parse` for `required_files` and explicitly parses the literal
strings `true`/`false`; JavaScript uses `JSON.parse` for its selected columns.
Kamaji sends the same `--name=value` contract to each language.

The policy schema sets `minimum: 1` for `max_total_bytes` and a default for
`fail_on_empty`. Each schema uses `allow_unknown: false`. Kamaji validates
outer map/list types; the example rules validate their entries. For instance,
JavaScript rejects an unsupported report column during execution.

To customize the example, edit `BUILD.yaml`: add local paths to `files`, match
them in `required_files`, change the byte budget, or select `[path, size]` for a
shorter report. Keep `metadata` and `expected_metadata` consistent. Kamaji has
no generic config override flag; trailing arguments only work when the rule's
own parser supports them.

### A deliberate CI failure using an alternate build file

After generating the inventory, run:

```sh
../../kamaji run policy --build BUILD.strict.yaml
```

This alternate build chooses a one-byte budget. Ruby prints
`Policy rejected: total bytes exceed budget` to stderr and Kamaji exits with
status 3. `validate policy --build BUILD.strict.yaml` still succeeds: the budget
is valid configuration, while its failure depends on the inventory's contents.
This distinguishes schema errors from a rule's business-policy rejection.

### Retain isolated output for inspection

After generating the inventory in the original workspace, run:

```sh
../../kamaji run report --isolated --keep-execroot --timeout 30s
../../kamaji runs list
```

The existing inventory is copied with the working directory. The report is
written inside the retained copy; the original `out/report.md` is unchanged.
Kamaji prints the retained location, and `runs list` lists it. Copy an artifact
out explicitly if you need it permanently: retained runs are bounded debugging
storage, and outputs are not promoted automatically. Copying the working
directory is not an OS sandbox; these rules still have your user permissions.

## Reproduce the example integration check

From the repository root:

```sh
go test -tags=integration ./examples/release-workflow -run '^TestReleaseWorkflow$' -count=1 -v
```

The test compiles the Go verifier and runs the checked-in workflow through
Kamaji with real Python, Ruby, and Node processes in a temporary workspace. It
checks the report, exit 3 policy rejection, a same-length input mutation, and
retained isolated output. It skips explicitly if a required runtime is missing;
a skipped run is not proof that the workflow ran. The test does not call cloud
services or bundled extensions. Select this package explicitly: the separate
terminal integration test elsewhere in the repository requires interactive input.
