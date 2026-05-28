# Skipped-test audit (rmp #4722)

This document categorizes every tracked `*_test.go` file that contains at least
one `t.Skip`/`t.Skipf`, so the large number of skipped tests does not silently
mask regressions. It is paired with a CI guard (`scripts/check-skips.sh`, run by
the `skip-guard` job in `.github/workflows/ci.yml`) that fails the build on a
**new** *undocumented* skip (a `t.Skip()` / `t.Skipf("")` with no reason).

## Snapshot

- Tracked `*_test.go` files: **1857**
- Files containing ≥1 `t.Skip`: **463** (≈25%)

## Categories

Per the rmp #4722 acceptance criteria, skips fall into three buckets:

### (a) Requires infrastructure not yet ported — gated on tracked work

These are legitimately blocked and must carry a reason string pointing at the
gap. They unskip as the gating work lands; they are **not** category (b).

| Sub-category | Approx. files | Gated on |
| --- | --- | --- |
| Read-path / SegmentReader core-readers integration | ~99 | the read-path cascade: rmp #4747 (compound postings read) + #4686 (IndexWriter byte-parity). When these land, `IndexSearcher.Doc` / `DirectoryReader` round-trips work and these unskip (#4665, #4671, #4683, #4701, #4700, #4704 are the same family). |
| Compat-harness / deferred Lucene-fixture scenarios | ~110 | the Java fixture harness (`GOCENE_COMPAT_HARNESS=1`, Maven+JDK21) — exercised in the `compat` CI job, deliberately not in the fast unit job. |
| Unimplemented API / import cycle | ~57 | specific feature gaps (e.g. `IndexWriter.GetReader`, NRT APIs, collector-interface alignment). |
| Test-framework infrastructure | ~34 | ports of lucene-test-framework helpers (`BaseTokenStreamTestCase`, `CannedTokenStream`, `RandomIndexWriter`, `GeoTestUtil`, `expectThrows`). |

### (b) Implementable now — review individually

~152 files matched no category-(a) marker. These are the priority for
incremental implementation: each should either be implemented or, if it turns
out to be gated, have its skip reason rewritten to name the blocking task so it
moves into category (a). Many of these become trivially implementable once the
read-path cascade (#4747/#4686) lands, which is why a follow-up pass after that
cascade is the most efficient way to drain them.

### (c) Intentional / environment-conditional — keep

~11 files skip deliberately and correctly: nightly tests under `-short`, and
similar environment-conditional guards. These are expected and must remain.

## Policy going forward (enforced by CI)

1. Every `t.Skip`/`t.Skipf` MUST carry a non-empty reason. The `skip-guard` CI
   job rejects bare `t.Skip()` / `t.Skipf("")`.
2. A skip that is gated on other work SHOULD name the blocking rmp task or the
   missing capability in its reason string, so this audit stays self-maintaining.
3. New tests should prefer to be implemented rather than skipped; a skip is a
   tracked debt, not a default.

## Remaining work

The dominant category (a) buckets are blocked on the read-path cascade (#4747,
#4686) and the compat harness. The category (b) review/implementation pass is
most efficient once #4747 lands (it removes the largest single blocker). This
audit + the CI guard satisfy the "categorized list" and "CI check" acceptance
criteria of #4722; the bulk unskip is sequenced behind the read-path epics.
