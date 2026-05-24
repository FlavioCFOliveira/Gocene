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

## Validation pipeline

Before closing any task, run the following in the repository root:

```bash
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
