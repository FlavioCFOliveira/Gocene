# Monster Tests

Gocene ports several tests from Apache Lucene that are annotated with `@Monster`
or `@Nightly`. These tests exercise large data structures (>2 GB allocations,
millions of points, adversarial inputs) and are **excluded from the default
`go test ./...` run** so that everyday development stays fast and
memory-safe.

## Running Monster Tests Locally

Set the environment variable `GOCENE_RUN_MONSTERS=1`:

```bash
GOCENE_RUN_MONSTERS=1 go test ./... -run 'Test2GBCharBlockArray|TestBKD_RandomBinaryBig|TestTimSorterWorstCase|Test2BPagedBytes|TestStressRamUsageEstimator|TestManyKnnDocs' -timeout 1800s
```

## Memory Requirements

| Test | Approx. Memory | Notes |
|------|--------------|-------|
| `Test2GBCharBlockArray_Monster2GBChars` | >2 GiB | Allocates a single `CharBlockArray` larger than 2 GB |
| `TestBKD_RandomBinaryBig` | ~1 GiB | 200 k points in a BKD tree |
| `TestTimSorterWorstCase` | ~1 GiB | Adversarial 140 M-element array |
| `Test2BPagedBytes` | ~2.5 GiB | Paged bytes spanning >2 GB |
| `TestStressRamUsageEstimator` | ~1 GiB | Large object graphs for `RamUsageEstimator` |
| `TestManyKnnDocs_LargeSegment` | ~1 GiB | Many KNN float vectors in one segment |

**Recommendation:** at least **16 GB of RAM** is advised when running the full
monster suite concurrently. Machines with 8 GB may swap heavily or hit the Go
runtime's GC limit.

## CI Schedule

A dedicated GitHub Actions workflow (`.github/workflows/monster.yml`) runs the
full monster suite **once per week** (Sunday 03:17 UTC) on both Go 1.25.x and
the latest stable Go release, with and without the race detector.
