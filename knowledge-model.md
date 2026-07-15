# Gocene Knowledge Graph Model

This file describes the shape of the Label Property Graph (LPG) maintained in `rmp graph` for the Gocene project. It must be kept in sync with the live graph.

## Node Labels

| Label | Purpose | Key Properties |
|---|---|---|
| `Package` | A Go package / package directory in the project | `name` (package import path or directory) |
| `SourceFile` | A non-test Go source file | `name` |
| `TestFile` | A Go test file (`*_test.go`) | `name`, `packageDir` |
| `TestFunction` | An individual Go test function | `name` |
| `Test` | A test scenario or grouped test concept | varies |
| `Feature` | A project feature or capability | `name`, `file`, `status`, `task`, `commit` |
| `Component` | A high-level subsystem / component | `name` |
| `Task` | An `rmp` task | `name`, `title`, `status`, `created_at` |
| `Sprint` | An `rmp` sprint | `name` |
| `Commit` | A git commit | `hash`, `message`, `date` |
| `File` | A project file tracked in the graph | `name` |
| `Document` | A documentation file | `name` |
| `Blocker` | A deferred-test blocker reason | varies |

## Edge Types

| Type | Meaning |
|---|---|
| `IN_PACKAGE` | File/function belongs to a package |
| `IN_FILE` | Function is contained in a file |
| `contains` | Container / contained relationship (generic) |
| `PART_OF` | Entity is part of another entity |
| `HAS_SUBTASK` | Parent task has a child/sub task |
| `BELONGS_TO` | Entity belongs to a sprint/task/etc |
| `IMPLEMENTED_IN` / `IMPLEMENTS` / `implements` | Feature implemented in file / file implements feature |
| `TESTS` / `TESTED_BY` / `COVERS` / `COVERED_BY` | Test coverage relationships |
| `USES` | Dependency / usage relationship |
| `PASSED_AT` | Test passed at a given commit |
| `REALIZES` / `DELIVERS` / `delivers` | Task realizes/delivers feature |
| `VALIDATED_BY` / `VERIFIES` | Validation relationships |
| `CLOSED_BY` / `CLOSES` | Task closed by commit |
| `HAS_COMMIT` | Entity has an associated commit |
| `TOUCHES` / `updates` | Commit touches/updates file or entity |
| `PROGRESSES` | Progression relationship |
| `PROVIDES` | Provides capability |
| `BLOCKED_BY` | Entity blocked by another |

## Provenance

Every node and edge should carry provenance where available:

| Property | Meaning |
|---|---|
| `gitCommit` | Hash of the commit when the element was last confirmed |
| `gitDate` | ISO date of that commit, e.g. `"2026-07-15"` |

## Notes

- The graph contains historical edge-type duplication (e.g. `IMPLEMENTS` and `implements`). The model records both as they exist in the live graph; new writes should prefer the UPPER_CASE forms for consistency where no existing edge of the same relation already exists.
- The `null` label count of 1 indicates a single legacy node with no label; this should be reviewed and labelled or removed during a future graph-hygiene pass.
