# Gocene Performance Benchmarks — Baseline

> **Scope note:** These benchmarks reflect the **current partial port** state
> (pre-1.0). Results may change significantly as deferred features (NRT reader,
> full IndexWriter pipeline, RandomIndexWriter test infrastructure) are
> implemented. This is not representative of final production performance.
> Updated: 2026-06-11.

This document records the baseline benchmark results for performance-critical
components in the Gocene port of Apache Lucene 10.4.0.

## Environment

| Property | Value |
|---|---|
| Platform | linux/arm64 (Raspberry Pi 5) |
| Go version | go1.25 |
| CPU | ARM Cortex-A76 (4 cores) |
| Benchmark flags | `-bench=. -benchmem -benchtime=3s -count=1` |
| Measured on | 2026-05-24 |

## Results

### codecs — ForUtil (Frame of Reference encoding)

ForUtil encodes and decodes blocks of 256 int32 values using bit-packing.
Benchmark uses 10 bits per value, representative of typical posting-list delta
sequences.

| Benchmark | Iterations | ns/op | MB/s | B/op | allocs/op |
|---|---|---|---|---|---|
| BenchmarkForUtilEncode | 1 636 195 | 2045 | 500.62 | 660 | 1 |
| BenchmarkForUtilDecode | 2 131 222 | 1589 | 644.39 | 4 | 1 |

### codecs — PForUtil (Patched Frame of Reference encoding)

PForUtil extends ForUtil with exception handling for outlier values. Benchmark
uses a typical posting-list block: 90% of values fit in 7 bits, 10% are
exceptions up to 14 bits.

| Benchmark | Iterations | ns/op | MB/s | B/op | allocs/op |
|---|---|---|---|---|---|
| BenchmarkPForUtilEncode | 888 985 | 3384 | 302.62 | 1211 | 1 |
| BenchmarkPForUtilDecode | 1 766 878 | 1973 | 518.94 | 4 | 1 |

### search — BM25Similarity

ScoreBM25 computes the per-document BM25 relevance score given term frequency,
document length, average document length, and IDF. Setup: 1 000 000 documents,
50 000 matching, term frequency 3, document length 100, average 120.

| Benchmark | Iterations | ns/op | B/op | allocs/op |
|---|---|---|---|---|
| BenchmarkBM25Score | 1 000 000 000 | 0.84 | 0 | 0 |

### index — FieldInfos

FieldInfos stores field metadata for a segment. The benchmark measures a full
round-trip: Add 32 fields, Freeze, then look up each field by name and by
number (simulating segment flush and reader open).

| Benchmark | Iterations | ns/op | B/op | allocs/op |
|---|---|---|---|---|
| BenchmarkFieldInfosSerialisation | 130 029 | 29 883 | 13 280 | 85 |

## Notes

- BM25Score is zero-alloc and runs in sub-nanosecond time because it is a
  pure arithmetic expression with no heap escapes.
- ForUtil and PForUtil decode allocate 4 B/op due to a small `buf` escape in
  `splitInts`; encode allocates slightly more due to the directory write path.
- FieldInfos allocations (85 per call, 13 KB) reflect the map + slice growth
  for 32 fields; this is one-time cost at segment open, not on the query path.
- All benchmarks were measured without the `-race` flag because the
  ThreadSanitizer TSAN VMA layout is incompatible with the host kernel
  (linux/arm64 RPi kernel).
