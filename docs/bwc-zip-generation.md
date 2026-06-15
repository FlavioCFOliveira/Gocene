# Backward-Compatibility ZIP Generation

This document describes how to produce the per-major-version index ZIPs that
`internal/compat/backward_codecs/multi_version_corpora_compat_test.go`
consumes.

## Scope

The multi-version corpus is **outside** the binary-compatibility mandate's
10.4.0 reference pin.  It is maintained as a best-effort backward-compat
asset for future regression testing.

## Target versions

ZIPs are required for every Lucene major version from 7.0 through 10.3:

```
7.0, 8.0, 9.0, 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7, 9.8, 9.9, 10.0, 10.1, 10.2, 10.3
```

## Generation harness (Java)

For each target version:

1. Check out the Lucene release tag (e.g. `releases/lucene/10.3.0`).
2. Build the Lucene core JARs with Maven.
3. Run a small Java program that:
   - Opens a `FSDirectory`
   - Creates an `IndexWriter` with the **default** codec for that release
   - Indexes a deterministic corpus (same field names and token streams as
     `tools/lucene-fixtures` uses)
   - Commits and closes
4. ZIP the resulting directory.
5. Compute SHA-256 and record it in `internal/compat/backward_codecs/SHA256SUMS`.
6. Commit the ZIP to `internal/compat/backward_codecs/testdata/bwc-zips/`.

## Verification (Go)

Once ZIPs are present, the Go test suite:

```bash
GOCENE_COMPAT_HARNESS=1 go test -tags compat ./internal/compat/backward_codecs/...
```

will unpack each ZIP and run Lucene 10.4.0 `CheckIndex` on it.  A clean exit
proves that Gocene can read the backward-compat corpus.

## Deferred status

ZIP generation is blocked on:
- Availability of a Maven/Java build environment
- Time to build ~16 Lucene release branches
- Storage budget for committed ZIPs (~ tens of MB per version)

This is tracked by rmp task **T102**.
