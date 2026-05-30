# Skipped-test audit (rmp #4722)

This document categorizes every tracked `*_test.go` file that contains at least
one `t.Skip`/`t.Skipf`, so the large number of skipped tests does not silently
mask regressions. It is paired with a CI guard (`scripts/check-skips.sh`, run by
the `skip-guard` job in `.github/workflows/ci.yml`) that fails the build on a
**new** *undocumented* skip (a `t.Skip()` / `t.Skipf("")` with no reason).

## Snapshot

Counted over tracked files only (`git ls-files '*_test.go'`; agent worktrees and
other untracked copies are excluded — same scope the CI guard uses).

- Tracked `*_test.go` files: **1896**
- Files containing ≥1 `t.Skip`/`t.Skipf`: **478** (≈25%)
- Total `t.Skip`/`t.Skipf` calls: **1537**
- Files where **all** test functions skip (skip calls ≥ test functions): **321**
  - of those: **246** infra-gated (a), **64** documented but un-bucketed by the
    keyword pass (b), **11** environment-conditional (c)
- Files with only **some** functions skipped (partial): **142**

The all-skipped figure is down from the original **511**: the read-path cascade
that gated the largest family of skips has largely landed this cycle (see
*Remaining work*), and the join / facets / grouping / KNN / sort / ICU / deletes
suites were unskipped as their backing code became real.

## Categories

Per the rmp #4722 acceptance criteria, skips fall into three buckets. Every skip
already carries a non-empty reason (the `skip-guard` job is green), so the
categorization below is about *why* each is still skipped, not about finding bare
skips.

### (a) Requires infrastructure not yet ported — gated on tracked work

The dominant bucket. These carry a reason string naming the missing capability
(e.g. "not yet ported", "deferred until X lands", "requires
KnnVectorsWriter.MergedByteVectorValues", "core readers are nil"). They unskip as
the gating work lands.

| Sub-category | Approx. skip calls | Gated on |
| --- | --- | --- |
| Read-path / SegmentReader core-readers integration | ~520 | the read-path cascade. The principal remaining blocker is **#4686** (IndexWriter byte-parity with `Lucene104Codec`). Its upstream dependencies **#4747** (compound/postings term-block round-trip), **#4683**/**#4701** (join SegmentReader gap), **#4700** (grouping), **#4704** (facets DocValues resolvers), **#4665**/**#4671** (webapi shadow-map removal) are now **COMPLETED**. |
| Test-framework infrastructure | ~145 | ports of lucene-test-framework helpers (`BaseTokenStreamTestCase`, `CannedTokenStream`, `RandomIndexWriter`, `GeoTestUtil`, `expectThrows`, `LuceneTestCase`). |
| Compat-harness / deferred Lucene-fixture scenarios | ~45 | the Java fixture harness (`GOCENE_COMPAT_HARNESS=1`, Maven+JDK21) — exercised in the `compat` CI job, deliberately not in the fast unit job. |
| Unimplemented API / not-yet-ported component | remainder | specific feature gaps (e.g. `BloomPostingsFormat`, `LuceneFixedGap` write path, `AlwaysRefreshDirectoryTaxonomyReader`). Each names the missing component. |

### (b) Implementable now — review individually

The ~64 all-skipped files whose reasons did not match a category-(a) keyword.
On inspection most are in fact infra-gated with prose the keyword pass missed
("not yet ported", "deferred until …"); a genuine minority are implementable
once their (small) dependency lands. These are the priority for the incremental
implementation pass and should each either be implemented or have their skip
reason rewritten to name the blocking task so they move into category (a).

### (c) Intentional / environment-conditional — keep

~11 files skip deliberately and correctly: nightly tests under `-short`,
environment-conditional guards (missing optional data files such as
`coredict.mem`/`.brk`, network-requiring tests), and Java-only behaviours with no
Go equivalent. These are expected and must remain.

## Policy going forward (enforced by CI)

1. Every `t.Skip`/`t.Skipf` MUST carry a non-empty reason. The `skip-guard` CI
   job rejects bare `t.Skip()` / `t.Skipf("")`. The guard is reason-based, not
   count-based, so it cannot false-positive as the skip count drifts.
2. A skip that is gated on other work SHOULD name the blocking rmp task or the
   missing capability in its reason string, so this audit stays self-maintaining.
3. New tests should prefer to be implemented rather than skipped; a skip is a
   tracked debt, not a default.

## Remaining work

With the read-path cascade largely landed (#4747/#4683/#4701/#4700/#4704/#4665/
#4671 COMPLETED), **#4686** (IndexWriter byte-parity with `Lucene104Codec`) is
the principal structural blocker for the largest remaining category-(a) family;
draining the category-(b) review pass is most efficient once it lands. The
compat-harness skips are correct as-is (they run in the `compat` CI job). This
audit + the CI guard satisfy the "categorized list" and "CI check" acceptance
criteria of #4722; the bulk unskip continues to be sequenced behind the
read-path epics.
