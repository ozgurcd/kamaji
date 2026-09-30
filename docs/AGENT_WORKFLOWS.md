# Build automation for coding agents

This guide describes the graph engine introduced in v0.3.0.
The v0.2.0 binaries provide the legacy rule runner. Install v0.3.0 or newer
and put `kamaji` on PATH before following this guide. Kamaji runs without a model
provider, daemon, or language SDK.

## Review a plan, execute it, and retrieve evidence

Start in a project containing `kamaji.toml`. The checked-in
[Go build example](../examples/build-project/README.md) supplies the `check`
target used below. `kamaji capabilities` reports supported schema identifiers;
`kamaji plan check --json` inspects the selected dependency closure without
creating runtime storage or executing commands.

This Python standard-library example passes the actual plan ID to the build and
retrieves its saved result. It captures machine output while leaving diagnostics
on stderr. Run it from `examples/build-project` with `kamaji` v0.3.0 or newer on
PATH:

```python
import json
import subprocess

def invoke(*args):
    process = subprocess.run(
        ["kamaji", *args], text=True, stdout=subprocess.PIPE, check=False
    )
    try:
        document = json.loads(process.stdout)
    except json.JSONDecodeError:
        raise SystemExit(f"Kamaji returned no JSON document (exit {process.returncode})")
    if process.returncode != 0:
        # A failed build may contain per-target evidence; retain it for diagnosis.
        print(json.dumps(document, indent=2))
        raise SystemExit(process.returncode)
    return document

plan = invoke("plan", "check", "--json")
if plan.get("schema") != "kamaji.plan.v1":
    raise SystemExit("Unsupported plan schema")
print(json.dumps(plan, indent=2))
# Apply your review policy here before executing trusted project commands.
result = invoke("build", "check", "--expect-plan", plan["id"], "--json")
if result.get("schema") != "kamaji.result.v1" or not result.get("success"):
    raise SystemExit("Unexpected build result")
if result["plan"] != plan["id"]:
    raise SystemExit("Result does not identify the reviewed plan")
record = invoke("history", result["id"], "--json")
if record != result:
    raise SystemExit("Saved result differs from build output")
print(json.dumps({"run": result["id"], "targets": result["targets"]}, indent=2))
```

The example executes after printing the plan; it does not implement a human
approval prompt. Integrate your own review step at the marked point when needed.
Plan JSON omits argument and environment values, so review the trusted build
configuration as well. Use the same project, file selection, and target roots
when planning and building. A plan mismatch requires investigating the change
and requesting a fresh plan, rather than silently dropping `--expect-plan`.

The ID binds declared source/configuration state, including source tool content;
cache availability does not change it. Generated inputs may not exist yet, and
dependent cache decisions can be `pending` until producers finish. Execution
rechecks source state as actions become ready. This is not a filesystem snapshot
or protection from hostile concurrent writers and undeclared inputs.

## JSON and process contracts

| Command/output | Schema | Main fields |
| --- | --- | --- |
| `capabilities` | `kamaji.capabilities.v1` | `version`, `build_formats`, `build_commands`, `contracts`, `os_sandbox` |
| `plan --json` | `kamaji.plan.v1` | `id`, `targets` |
| `build --json`, final event-stream record, `history` | `kamaji.result.v1` | `id`, `plan`, `started`, `finished`, `success`, `targets` |
| Build event | `kamaji.event.v1` | `target`, `state` |
| `affected --json` | `kamaji.affected.v1` | `targets` |
| `clean --json` | `kamaji.clean.v1` | `dry_run`, `paths` |
| Structured command failure | `kamaji.error.v1` | `error` |

Each document has a `schema` field. Check it before interpreting the payload.
`history` always emits JSON on success; its `--json` flag requests structured
error output. Flags/argument parsing can fail before a command's JSON handler,
so also handle nonzero exits with absent or non-JSON stdout.

A planned target includes `name`, `argument_count`, `effect`, `cache`, `slots`,
`status`, `reason`, and `fingerprint`. Optional fields include `description`,
`deps`, `inputs`, `outputs`, `program`, and `environment_names`. Its status is
`run`, `cached`, `pending`, or `aggregate`; this is a forecast, not execution
evidence.

A result target includes `name`, `status`, `exit_code`, and `duration_ms`.
Depending on what ran, it can include an action `key`, `inputs`, `outputs`,
`tools`, `definition_sha256`, and `error`. Artifact entries contain `path` and
numeric permission `mode`, plus `sha256` for files or `directory: true` for
directories. Early failures and blocked targets can lack digests. Do not infer
success from the presence of a key: inspect `success`, target statuses, and the
process exit status.

For progress reporting:

```sh
kamaji build check --events
```

Read one JSON object per stdout line. Events identify target transitions such
as `started`, `executed`, `cached`, `aggregate`, `failed`, and `blocked`; the
last document is the build result when execution reaches result reporting.
Independent targets' events can interleave. A preflight error can occur instead
of a result. `--events` and `--json` are mutually exclusive. Child stdout and
stderr go to Kamaji's stderr in both modes; never merge stderr into the JSON
stream. Saved run records contain structured evidence, not child logs. Capture
stderr separately if logs are needed, outside declared source-input patterns.

## Select affected work and handle effects

From the example project:

```sh
kamaji affected main.go --json
kamaji plan check --json
```

`affected` takes project-relative changed or deleted paths; it does not query
Git. Results include matching declared inputs, outputs, local executable/tool
paths and reverse dependencies. Deleted or unreadable paths are conservatively
treated as possible directories. Results may include extra work and cannot
cover undeclared dependencies. If an automation receives an empty target list,
skip execution explicitly: invoking `build` with no targets selects defaults
(or all targets when no defaults are configured).

Targets declaring `effect = "external"` require `build --allow-effects` and
cannot be cached. Grant that flag only after reviewing the selected closure,
and combine it with `--expect-plan`. The flag is build-wide, not a target-specific
approval list. Declaring `effect = "build"` does not prevent a command from
accessing the network or modifying other files. Kamaji is not an OS sandbox.

Plans and results expose paths, descriptions and content digests even though
argument/environment values are omitted. Child logs can contain arbitrary
output. Store evidence according to your project's data-handling requirements.
For cache assumptions, configuration, cleanup, and execution limits, see
[Usage](HOW_TO_USE.md), [Executor security](EXECUTOR_SECURITY.md), and
[Resource limits](RESOURCE_LIMITS.md).
