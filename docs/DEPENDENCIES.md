# Dependencies

This inventory describes the module declarations and current core
on 2026-09-28. Version numbers are pinned values, not a continuing claim that
they are the latest releases.

The Go directive is `go 1.27.1`. Direct dependencies in `go.mod` are:

- `github.com/spf13/cobra v1.10.2`: CLI commands, flags, and help.
- `github.com/h2non/filetype v1.1.3`: dependency payload type detection.
- `go.yaml.in/yaml/v3 v3.0.5`: user, workspace, build, and rule-schema YAML.

Indirect requirements are `github.com/spf13/pflag v1.0.10` and
`github.com/inconshreveable/mousetrap v1.1.0`, through Cobra. An indirect module
declaration does not mean every platform builds its code.

The core uses one YAML library. The earlier YAML v2 dependency and experimental
`golang.org/x/exp/rand` dependency were removed. The Go standard library has no
YAML decoder; retaining YAML configuration therefore requires a parser outside
the standard library. Adopting a standard-library-only format such as JSON
would be a configuration-format change, not an equivalent library substitution.
Current YAML behavior is described in [Usage](HOW_TO_USE.md).

The usability additions introduce no new Go module dependencies. Cancellation,
terminal control, schema constraints, and storage inspection use the standard
library; terminal/process behavior is implemented for the supported macOS/Linux
runtime platforms.

Generic rule execution and Go plugin support also add no module dependencies.
Compiled Go plugins need no Python or Go toolchain at execution time. Custom
interpreted rules require their declared runtime to be installed; Kamaji does
not install it or its packages. See [Rule languages](RULE_LANGUAGES.md).

The separate `requirements.txt` pins PyYAML 6.0.3 and Jinja2 3.1.6 for Python
extensions. They are not Go dependencies or used by the Go configuration parser.
Managed Python setup installs the globally installed requirements file when it
exists. Extension correctness was not established by the core test suite.

Reproduce the declared/resolved Go inventory with `go list -m all`; validate
downloaded module integrity with `go mod verify`. See [Testing](TESTING.md) for
the recorded vulnerability scan and its limits.
