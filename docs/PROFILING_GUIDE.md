# Profile-Guided Optimization Guide (GC-834)

## Status
COMPLETED 2026-03-21 - Profiling guide created

## Overview
This document provides guidelines for profile-guided optimization of Gocene.

## Profiling Tools

### CPU Profiling
```bash
# Run benchmarks with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof
```

### Memory Profiling
```bash
# Run benchmarks with memory profiling
go test -bench=. -memprofile=mem.prof ./...
go tool pprof mem.prof
```

### Execution Tracing
```bash
# Generate execution trace
go test -trace=trace.out ./...
go tool trace trace.out
```

## Key Metrics to Monitor

1. **Hot Paths**
   - ForUtil encode/decode
   - ByteBlockPool operations
   - IndexSearcher query processing
   - Merge operations

2. **Memory Allocations**
   - ByteBuffersDataOutput
   - CopyBytes operations
   - TokenStream processing

3. **Lock Contention**
   - IndexWriter locks
   - DocumentsWriter locks
   - MergeScheduler synchronization

## Benchmarking Commands

### Full Benchmark Suite
```bash
go test -bench=. -benchmem -count=5 ./... | tee benchmark_results.txt
```

### Specific Component Benchmarks
```bash
# Store benchmarks
go test -bench=BenchmarkByteBuffersDataOutput -benchmem ./store/...

# Util benchmarks
go test -bench=BenchmarkByteBlockPool -benchmem ./util/...

# Index benchmarks
go test -bench=BenchmarkIndexWriter -benchmem ./index/...
```

## Identified Hotspots (Phase 49)

Based on static analysis and audit:

1. **COMPLETED**: CopyBytes - Buffer pooling implemented
2. **COMPLETED**: ByteBlockPool - Capacity and growth optimized
3. **COMPLETED**: PagedBytes - Growth strategy improved
4. **COMPLETED**: MergeScheduler - sync.Cond for synchronization
5. **COMPLETED**: MMapDirectory - Single file handle for multi-chunk

## Recommended Profiling Workflow

1. **Establish baseline**: Run benchmarks before changes
2. **Profile**: Identify actual hotspots (not predicted)
3. **Optimize**: Focus on top 20% of time consumers
4. **Verify**: Run benchmarks after changes
5. **Compare**: Ensure improvement vs baseline

## Continuous Profiling

Consider setting up:
- Nightly benchmark runs
- Automated regression detection
- Performance dashboards

## Conclusion

Profile-guided optimization should focus on actual measured hotspots.
The Phase 49 optimizations address predicted hotspots based on code audit.
Future phases should validate these assumptions through profiling.
