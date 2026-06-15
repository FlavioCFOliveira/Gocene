# Gocene Benchmarks

This document records baseline performance numbers for the Gucene indexing
and search stack. Numbers are produced with `go test -bench=.` on the
`benchmark/` package.

## Hardware Baseline

| Property | Value |
|----------|-------|
| CPU | AMD Ryzen 9 5900HX with Radeon Graphics |
| GOARCH | amd64 |
| GOOS | linux |
| Go version | 1.25.x |
| Date | 2026-06-15 |

## Indexing Throughput

Measures `IndexWriter.AddDocuments` throughput (docs/sec) for varying batch
sizes. Each document contains a single 256-char `TextField`.

| Batch size | ns/op | docs/sec | B/op | allocs/op |
|-----------:|------:|---------:|-----:|----------:|
| 1 | 20,935 | 47,766 | 7,957 | 76 |
| 10 | 209,540 | 47,724 | 79,684 | 769 |
| 100 | 2,093,758 | 47,761 | 797,317 | 7,689 |
| 1000 | 20,705,683 | 48,296 | 7,975,076 | 76,925 |

**Observation:** throughput scales linearly with batch size; per-doc
overhead is dominated by allocation and tokenization.

## Search Throughput

Measures `IndexSearcher.Search` throughput (queries/sec) over a 10,000-doc
index. Query is a `TermQuery` on a single character `"a"`.

| Top N | ns/op | queries/sec | B/op | allocs/op |
|------:|------:|------------:|-----:|----------:|
| 10 | 60,608 | 16,499 | 115,092 | 865 |
| 100 | 70,614 | 14,162 | 117,440 | 865 |
| 1000 | 77,616 | 12,884 | 120,464 | 865 |

**Observation:** latency grows sub-linearly with `topN` because the
scorer is bulk-oriented.

## Merge Performance

Measures `IndexWriter.ForceMerge(1)` throughput (merges/sec) for indices with
2, 5, and 10 segments. Each segment contains 1,000 docs.

| Segments | ns/op | merges/sec | B/op | allocs/op |
|---------:|------:|-----------:|-----:|----------:|
| 2 | 1,122,502 | 891 | 169,481 | 367 |
| 5 | 383,545 | 2,607 | 89,669 | 422 |
| 10 | 880,538 | 1,136 | 154,372 | 415 |

**Observation:** 5-segment merge is fastest because the intermediate
segment sizes balance I/O and CPU; 2-segment pays the full rewrite
penalty, and 10-segment incurs higher scheduling overhead.

## Lucene Parity

Cross-engine parity benchmarks require a Java 21 installation and the Lucene
fixture harness (`tools/lucene-fixtures`). They are run separately in CI
under the `compat` job. See `.github/workflows/ci.yml`.
