# Rule languages

This guide covers the legacy `run` interface and `rule_definition.yaml` schemas.
The graph engine introduced in v0.3.0 instead runs any language through explicit `command`
argument lists, without a rule schema. It can compile Go code as a declared build
target and cache its outputs; see the [Go build walkthrough](../examples/build-project/README.md).

Generic language execution is available starting with v0.2.0. The older v0.1.0
binaries execute Python rules only; upgrade before using the settings below.

Kamaji's rule interface is a child process. The rule schema selects how the
process starts; no language SDK, in-process plugin ABI or language registry is
required. All modes share variable validation, dependency preparation, working
directory selection, cancellation, exit status handling and execution cleanup.

## Execution settings

Every target's `rule` path must refer to a regular file with a sibling
`rule_definition.yaml`. A schema still requires `variables`, even if it is `{}`.
The execution block has two modes:

```yaml
language: javascript
execution:
  mode: interpreter
  command: [node, --enable-source-maps]
variables: {}
```

Interpreter mode resolves `command[0]` as the executable. Remaining elements
are fixed arguments. Kamaji then appends the absolute rule path, sorted config
options and user arguments. A bare executable name uses PATH; relative paths
containing a directory component resolve from the caller's working directory
before isolation changes the child's directory. A command is an argument list,
not a shell command string. Environment-variable interpolation, shell expansion
and automatic dependency installation are not performed.

Examples of command lists are `[ruby]`, `[bash]`, `[python3, -I]`, and
`[java, -jar]` for a target whose rule is a JAR file. The selected interpreter
must accept the rule file at that argument position; language-specific flags
and runtime dependencies are the rule author's responsibility.

```yaml
language: rust
execution:
  mode: executable
variables: {}
```

Executable mode runs the target's rule file directly, followed by config options
and user arguments. `command` must be omitted or empty in this mode. The rule
must have execute permission and be built for the host OS/architecture. Preflight
checks file existence/type and execute bits; the OS establishes binary-format,
loader and architecture compatibility when the process starts.

With explicit execution settings, `language` is a descriptive label and can name
any language. Two defaults preserve convenient behavior when execution is omitted:

- Missing `language`, or `language: python`: use Kamaji's existing Python
  selection, including `--python`, KAMAJI_PYTHON, user configuration and managed
  environments. Existing rule schemas continue to work.
- `language: go` or `language: golang`: run the precompiled rule directly;
  `golang` is normalized to `go` in diagnostics.

Other language labels require an execution block. Unknown execution modes,
missing interpreter commands, an empty program name, NUL arguments and unknown
schema fields are rejected. A missing schema is an error, including for callers
embedding the executor directly.

Explicit execution settings take precedence over language defaults. In particular,
`language: python` with an explicit interpreter command uses that command and
does not load Kamaji's Python configuration or managed environment. Python flags
and environment selection apply only to the implicit Python mode. Mixed-language
`validate --all` resolves each target separately.

`validate`, `doctor` and `explain` never compile or execute a rule. They do not
check arbitrary trailing arguments, language imports, package availability or
whether the rule's own argument parser accepts its configuration.

## Go example

For a more involved Go rule, see the [release workflow](EXAMPLES.md#a-multilingual-release-workflow):
a compiled Go verifier reads a JSON inventory, accepts a metadata map through
flags, streams file hashes, and returns a failure code when contents change.
The same guide includes runnable Python, Ruby, and JavaScript rules with their
own schemas and argument parsers.

The complete example lives under [examples/go-rule](../examples/go-rule/BUILD.yaml).
Its rule is an ordinary `package main` program using the standard `flag` package.
From the Kamaji repository root:

```sh
make build
go build -o examples/go-rule/rules/hello/hello ./examples/go-rule/rules/hello
cd examples/go-rule
../../kamaji validate hello-go
../../kamaji explain hello-go --json
../../kamaji run hello-go -- Ada "Grace Hopper"
```

The Go toolchain is needed only when compiling the plugin. Kamaji performs no
automatic Go build, module download or build-cache management for legacy rules. It does not load
`.so` files using Go's `plugin` package. The schema belongs next to the resulting
executable, so separate platform binaries in separate directories also need
their own schema next to each binary. The example's compiled output is ignored
by Git.

## Arguments, diagnostics and trust

Config values become `--name=value` arguments: strings are literal; booleans,
numbers, lists and maps use JSON encoding. Trailing arguments follow unchanged.
The rule parses these arguments itself. Standard Go flags handle scalar values;
use `encoding/json` when accepting map/list options. There is no automatic stdin
JSON protocol. Stdout/stderr and terminal input remain attached to the child.

Explain output includes `language`, `execution_mode` and `executable`. Implicit
Python execution also retains the existing `python` and `python_source` fields.
Config values and fixed interpreter arguments are not printed. Executable paths
are visible, so use command arguments for flags rather than embedding them in
an executable name.

All children inherit the environment, with PWD and KAMAJI_ORGANIZATION_DOMAIN
set by Kamaji. Rules labeled `python` additionally receive the configured common
rules directory as PYTHONPATH. Other labels preserve the inherited PYTHONPATH.
Working-copy isolation is not an OS sandbox; compiled plugins have the same
user permissions and access as interpreted rules. Existing deadline, process-group
and retention limits apply to every execution mode.
