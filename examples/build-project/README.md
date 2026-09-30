# Compile, generate, and verify with Go

This example requires Kamaji v0.3.0 or newer; the v0.2.0 binary cannot run it. It uses Go's standard library and
synthetic local input. No Python, external Go modules, or cloud services are
needed. Install the Go toolchain required by the repository's `go.mod`.

From the Kamaji repository root:

```sh
make build
cd examples/build-project
../../kamaji plan --json
../../kamaji build --jobs 2
cat out/greeting.txt
```

The greeting is `Hello from a build graph`. The [project file](kamaji.toml)
declares this pipeline:

| Target | Command purpose | Declared output | Cache |
| --- | --- | --- | --- |
| `compile` | Compile [main.go](main.go) into a native executable | `out/greet` | Enabled |
| `render` | Run that executable to write a greeting | `out/greeting.txt` | Enabled |
| `check` | Run the executable to verify the greeting contents | None | Disabled |

`check` is the default root. Explicit `deps` order the stages; raising `--jobs`
does not run dependent stages concurrently. `render` declares the generated
executable as an input. It can be absent when planning because `compile` owns
it and is a dependency. `check` reruns on each successful build, even when the
two producers are cached.

The compiler target passes `GOCACHE`, `GOPATH`, and `GOTMPDIR` when provided by
the caller. It disables user Go configuration, automatic toolchain selection,
telemetry, and CGO. It invokes `go build` explicitly; Kamaji does not infer Go
imports or insert compilation commands. Go's compiler cache is separate from
Kamaji's output cache.

## Reuse and restore outputs

Continue in this example directory:

```sh
../../kamaji build --json
../../kamaji clean --dry-run
../../kamaji clean
../../kamaji build --json
cat out/greeting.txt
```

With unchanged inputs and environment, the second build reports `cached` for
`compile` and `render`, and `executed` for `check`. `clean` removes the declared
outputs but preserves the local artifact cache and run history. The subsequent
build verifies cached payloads, restores both outputs, and executes `check`.
The greeting remains the same. Existing cache state can make the first build
cached too.

To force execution without reading or writing the artifact cache:

```sh
../../kamaji build --no-cache
```

The example's `.gitignore` excludes `out/` and `.kamaji/`. `clean --cache` also
deletes the whole project's artifact cache; `clean --history` additionally
deletes saved results. Preview either operation with `--dry-run` first.

## Inspect changes and automate the workflow

```sh
../../kamaji affected main.go --json
../../kamaji explain check --json
```

Changing `main.go` affects `compile` and its consumers `render` and `check`.
Editing a command or its message changes the action configuration and invalidates
the relevant cache key. `explain` returns the selected graph plan; it does not
execute the verifier.

[Agent workflows](../../docs/AGENT_WORKFLOWS.md) provides a complete Python
wrapper that plans `check`, builds with the returned `--expect-plan` ID, and
retrieves saved JSON evidence with `history`. For all graph fields and cache
limitations, see [Usage](../../docs/HOW_TO_USE.md).

The integration check compiles the real Go program, runs the graph, removes its
outputs, and verifies restoration and the uncached checker. Run from the
repository root:

```sh
go test -tags=integration ./buildsys -run '^TestCompiledArtifactGraph$' -count=1 -v
```
