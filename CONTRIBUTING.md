# Contributing to Gocene

Thank you for your interest in contributing. This document explains the
development workflow, conventions, and tools required to contribute to Gocene.

---

## Prerequisites

- Go 1.25 or later
- Git
- `rmp` CLI (roadmap management tool — see below)
- Access to the Lucene 10.4.0 source tree at `/tmp/lucene` (see below)

### Lucene reference source

The authoritative reference for all porting work is Apache Lucene 10.4.0:

```bash
git clone --depth=1 --branch releases/lucene/10.4.0 \
    https://github.com/apache/lucene.git /tmp/lucene
```

The reference tree is read-only. Never modify it.

---

## Development workflow

Gocene follows a strict four-step cycle for every change:

```
Specify → Implement → Test → Document
```

1. **Specify.** Identify the task in the roadmap via `rmp`. Read functional
   requirements, technical requirements, and acceptance criteria in full before
   writing any code. Raise questions before starting if any requirement is
   ambiguous.

2. **Implement.** Read every file you intend to modify before writing to it.
   Port behaviour faithfully from the Lucene Java source; document any
   intentional divergences in the task summary.

3. **Test.** Run the full validation pipeline (see below) and confirm that all
   acceptance criteria are verifiable by command output.

4. **Document.** Update godoc, `CHANGELOG.md`, and any relevant docs. Close
   the task in `rmp` with a concise summary before committing.

---

## Roadmap management with `rmp`

All tasks and sprints are tracked with the `rmp` CLI. It is the single source
of truth for planning and execution.

### Basic task lifecycle

```bash
# List tasks in the active sprint
rmp sprint tasks -r gocene <sprint-id>

# Mark a task as started
rmp task stat -r gocene <task-id> DOING

# Mark a task as completed with a summary
rmp task stat -r gocene <task-id> TESTING
rmp task stat -r gocene <task-id> COMPLETED --summary "Brief description of what was done."
```

### Discovering subcommands

Always run `rmp -h` and `rmp <subcommand> -h` before using a new subcommand.
Never guess flags or formats.

---

## Gitflow conventions

Gocene uses gitflow branch naming:

| Branch type | Prefix | Example |
|---|---|---|
| New feature or port | `feature/` | `feature/bkd-writer-3301` |
| Urgent bug fix | `hotfix/` | `hotfix/codec-header-crc` |
| Release preparation | `release/` | `release/v0.2.0` |

### Branch lifecycle for a task

```bash
git checkout main
git checkout -b feature/<slug>-<task-id>
# ... implement, test, validate ...
git add <files>
git commit -m "<type>(<scope>): <summary>"
git checkout main
git merge --no-ff feature/<slug>-<task-id> -m "merge: feature/<slug>-<task-id> into main"
```

Close the task in `rmp` **before** committing. All git operations must be
confirmed with the user before execution.

### Commit message format (Conventional Commits)

```
<type>(<scope>): <imperative summary, ≤72 chars>

<body: why, not what; wrap at 72 chars>

<footer: Refs, Closes>
```

Types: `feat`, `fix`, `refactor`, `perf`, `test`, `docs`, `build`, `chore`,
`ci`, `style`.

Do **not** include `Co-Authored-By:` trailers — a pre-commit hook rejects them.

---

## Binary compatibility (mandatory)

Gocene is bound by a **non-negotiable byte-level compatibility mandate** with
Apache Lucene 10.4.0. Every artefact Gocene writes must be readable by Lucene
10.4.0 unchanged, and every artefact Lucene 10.4.0 writes must be readable by
Gocene without reinterpretation. The full statement of the mandate is in
[`CLAUDE.md`](CLAUDE.md), section *Binary Compatibility Mandate*.

### Running the compat suite locally

The suite requires JDK 21 and Maven 3.6+ in addition to the Go toolchain.

```bash
export JAVA_HOME=/path/to/jdk-21
export PATH="$JAVA_HOME/bin:$PATH"

# Build the Java fixture harness (Lucene 10.4.0).
mvn -B -q -f tools/lucene-fixtures/pom.xml verify

# Generate the baseline corpus into a tmp dir.
make -f tools/lucene-fixtures/Makefile corpus-baseline

# Run the per-package and combined-scenario compat tests.
export GOCENE_COMPAT_HARNESS=1
export LUCENE_FIXTURES_JAR="$PWD/tools/lucene-fixtures/target/lucene-fixtures.jar"
go test -tags compat ./... -timeout 900s
```

A short-form invocation that exercises only the end-to-end combined scenarios:

```bash
GOCENE_COMPAT_HARNESS=1 \
LUCENE_FIXTURES_JAR="$PWD/tools/lucene-fixtures/target/lucene-fixtures.jar" \
  go test ./internal/compat/scenarios/... -timeout 600s
```

### What a divergence looks like

When a Gocene-emitted byte sequence diverges from Lucene 10.4.0, the harness
exits with code `4` and prints a single-line JSON diagnostic on stdout that
names the affected file, byte offset, and the expected vs actual byte values.
Example, captured by mutating one byte of `s1-hits.tsv` at offset 100 in the
S1 fixture:

```text
{"file":"s1-hits.tsv","offset":100,"expected":53,"actual":159}
```

The corresponding Go test (`TestMutationDiagnostic` in
`internal/compat/scenarios/`) re-parses this JSON and asserts that `file`,
`offset`, `expected`, and `actual` are all populated and that `expected !=
actual`. Any contributor change that introduces a divergence will surface
through the same record in CI.

### CI enforcement

Every pull request triggers two workflows: the fast `build-and-test` job
(pure Go, Linux only) and the dedicated `compat` job
(`ubuntu-latest`/`macos-latest`/`windows-latest` × `go 1.25.x`/`stable`).
The `compat` job builds the Java harness, generates the baseline corpus,
runs the combined scenarios, runs every `-tags compat` test, and uploads a
`compat-failure-<run-id>-<os>-<go>.tar.gz` artefact on failure.

**The `compat` job must be green to merge.** Windows is currently marked
`continue-on-error: true` because the GitHub-hosted Windows runners
intermittently fail JDK 21/Maven installation; treat Windows results as
informational until that is stabilised. Linux and macOS must pass on both
Go versions.

#### Branch-protection setup (maintainer task)

The `compat` job is enforced via branch protection, configured manually:

> **GitHub → Settings → Branches → Branch protection rule for `main` →
> Require status checks to pass before merging →** select every matrix
> instance of `Compat (Lucene 10.4.0 byte-parity)` except the Windows
> entries, plus `Build, Vet, and Test`.

---

## Validation pipeline

Before closing any task, run the following in the repository root:

```bash
# Quick validation (via Makefile):
make test          # standard tests
make lint          # check for undocumented t.Fatal/t.Skip blockers
make race-test     # race detector (x86_64 only, see below)

# Or manually:
gofmt -l .          # must produce no output
goimports -l .      # must produce no output
go vet ./...
go build ./...
go test ./... -timeout 300s
staticcheck ./...
govulncheck ./...
```

All steps must pass with no errors. If any step fails, stop, report the raw
output, and do not declare the task complete.

### Race detector and the ARM64 limitation

Run the race detector on concurrency-sensitive changes:

```bash
# Via Makefile (recommended):
make race-test

# Or directly:
go test -race ./... -timeout 900s
```

The Go race detector (ThreadSanitizer) requires a **48-bit virtual address
space**. Some ARM64 hosts — including common Raspberry Pi / aarch64
development boards — expose only a **47-bit VMA**, on which `-race` aborts at
startup (`unexpected fault address`, `runtime: address space conflict`). This
is a platform limitation, not a Gocene bug, and it cannot be worked around in
the code.

If you develop on such an ARM64 host:

- You will not be able to run `-race` locally. Rely on the CI **"Race detector
  (x86_64)"** job (GitHub's `ubuntu-latest` runners are x86_64 and support the
  detector) to catch data races in your pull request.
- Where feasible, run `-race` once on an x86_64 machine before opening a PR
  that touches goroutines, channels, or shared mutable state.

No local hardware change is required to contribute; the x86_64 CI job is the
authoritative race gate.

---

## Fuzz testing

Gocene processes untrusted input on two fronts: user-supplied query strings
and document text on the analysis/query side, and Lucene-authored binary
artefacts on the codec/store side. Both are fuzzed with Go's native fuzzing
engine (`FuzzXxx` functions, `f.Fuzz`, `f.Add` seed corpora).

The invariant every fuzz target asserts is **no crash on untrusted input**:

- **Parsers** (query parser, tokenizers) must never panic. A returned error
  for malformed syntax is correct; a panic is a bug. Tokenizers additionally
  must **terminate** — the token stream always reaches end-of-input.
- **Codec readers** must reject malformed, truncated, or adversarial bytes
  with an error, never a panic or out-of-bounds access. They parse bytes they
  did not author, so a corrupt file must degrade gracefully.

Current targets:

| Target                              | Package        | Property                          |
| ----------------------------------- | -------------- | --------------------------------- |
| `FuzzQueryParser`                   | `queryparser/` | classic parser never panics       |
| `FuzzStandardTokenizer`             | `analysis/`    | tokenizer never panics, terminates|
| `FuzzLucene90CompoundEntriesRead`   | `codecs/`      | `.cfe` reader errors, never panics |

Run a single target locally for a bounded time:

```bash
go test -run='^$' -fuzz='^FuzzQueryParser$' -fuzztime=30s ./queryparser/
go test -run='^$' -fuzz='^FuzzStandardTokenizer$' -fuzztime=30s ./analysis/
go test -run='^$' -fuzz='^FuzzLucene90CompoundEntriesRead$' -fuzztime=30s ./codecs/
```

CI runs all three targets for a short, bounded window on every PR (the
**"Fuzz (parsing smoke)"** job). This is a smoke test, not exhaustive
fuzzing; run longer windows (`-fuzztime=5m` or more) locally when changing
parsing or codec-reader code.

If a fuzz run finds a crasher, the engine writes the failing input under the
package's `testdata/fuzz/<TargetName>/` directory. **Commit that file**: it
becomes a permanent regression seed replayed by `go test` on every run. Fix
the production code so the input is handled (error, not panic) before closing
the task.

When adding a new parser or codec reader that consumes untrusted input, add a
matching `FuzzXxx` target and wire it into the CI `fuzz` job.

---

## Coding conventions

- Go 1.25+ idioms; generics where they add clarity.
- `context.Context` is the first parameter of every function that performs I/O
  or orchestrates goroutines.
- Errors: wrap with `fmt.Errorf("context: %w", err)`; sentinels as `ErrXxx`;
  inspect with `errors.Is` / `errors.As`.
- Logging: `log/slog`, injected as a dependency — no global loggers.
- Performance: correctness first; optimise only after `go test -bench` or
  `pprof` identifies a hotspot. Prefer zero-alloc and lock-free designs on hot
  paths.
- Concurrency: introduce goroutines only when there is a measurable benefit
  demonstrated by a benchmark.

For the full coding standard, see the project's CLAUDE.md.

---

## Asking questions

If any requirement is ambiguous, contradictory, or incomplete, ask before
writing code. Present multiple options (a, b, c, …) and indicate which you
recommend. Ask one question at a time.
