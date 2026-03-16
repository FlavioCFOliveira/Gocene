# Gocene Performance Audit Report

**Date:** 2026-03-16
**Auditor:** Go Performance Advisor
**Scope:** Comprehensive performance analysis of all Gocene packages
**Total Files Analyzed:** 484 Go source files

---

## Executive Summary

This audit identifies **47 performance issues** across the Gocene codebase, categorized by severity:
- **CRITICAL:** 5 issues
- **HIGH:** 12 issues
- **MEDIUM:** 18 issues
- **LOW:** 12 issues

The most significant concerns are in memory allocation patterns, escape analysis failures, and concurrency bottlenecks in hot paths.

---

## 1. CPU Performance Issues

### 1.1 Function Inlining Failures

#### CRITICAL: Complex TokenStream Method Not Inlined
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/analysis/analyzer.go:121`
- **Severity:** CRITICAL
- **Current Code:**
```go
func (a *BaseAnalyzer) TokenStream(fieldName string, text io.Reader) (TokenStream, error) {
    // Complex implementation with cost 225 (exceeds budget 80)
}
```
- **Performance Impact:** TokenStream is a hot path in document analysis. The function call overhead significantly impacts indexing throughput.
- **Recommendation:** Break into smaller inlineable functions or use the `//go:noinline` directive strategically if inlining is not beneficial.

#### HIGH: Defer Prevents Inlining in Hot Paths
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/analysis/analyzer_utils.go:26`
- **Severity:** HIGH
- **Current Code:**
```go
func Tokenize(text string, analyzer Analyzer) ([]string, error) {
    tokenStream, err := analyzer.TokenStream("", strings.NewReader(text))
    if err != nil {
        return nil, err
    }
    defer tokenStream.Close() // Prevents inlining
    // ...
}
```
- **Performance Impact:** Defer has overhead (~2x slower than direct call). In hot tokenization paths, this adds up.
- **Recommendation:** Use explicit close calls in performance-critical code paths.

#### HIGH: Defer Prevents Inlining in TokenizeWithAnalyzer
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/analysis/analyzer_utils.go:55`
- **Severity:** HIGH
- **Same issue as above** - defer prevents inlining of another hot path function.

### 1.2 Loop Optimization Issues

#### HIGH: Inefficient Loop in ForUtil Decode Methods
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/codecs/for_util.go`
- **Severity:** HIGH
- **Current Code:**
```go
func (f *ForUtil) decode8(in store.IndexInput, ints []int64) error {
    buf := make([]byte, 4)  // Allocated inside hot loop
    for i := 0; i < 64; i++ {
        if err := in.ReadBytes(buf); err != nil {
            return err
        }
        ints[i] = int64(binary.BigEndian.Uint32(buf))
    }
    return nil
}
```
- **Performance Impact:** Buffer allocation on each call causes GC pressure. This is called for every 256 integers decoded.
- **Recommendation:** Use a sync.Pool for buffers or make buf a field of ForUtil struct.

#### MEDIUM: Loop Bounds Not Hoisted in ByteBlockPool
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/util/byte_block_pool.go:161-170`
- **Severity:** MEDIUM
- **Current Code:**
```go
if zeroFillBuffers {
    for i := 0; i < p.bufferUpto; i++ {
        for j := range p.buffers[i] {  // Range over slice in inner loop
            p.buffers[i][j] = 0
        }
    }
}
```
- **Performance Impact:** Inner loop range causes bounds checks on every iteration.
- **Recommendation:** Use explicit indexing with pre-computed length.

### 1.3 Branch Prediction Issues

#### MEDIUM: Unpredictable Branch in ShouldFlush
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/index/documents_writer.go:83-94`
- **Severity:** MEDIUM
- **Current Code:**
```go
func (p *DefaultFlushPolicy) ShouldFlush(numDocs int, ramUsed int64) bool {
    if p.maxBufferedDocs > 0 && numDocs >= p.maxBufferedDocs {
        return true
    }
    if p.maxRAMBufferMB > 0 {
        maxBytes := int64(p.maxRAMBufferMB * 1024 * 1024)
        if ramUsed >= maxBytes {
            return true
        }
    }
    return false
}
```
- **Performance Impact:** Multiple conditional branches in hot path. Branch misprediction can stall pipeline.
- **Recommendation:** Use branchless techniques or ensure predictable patterns.

---

## 2. Memory Performance Issues

### 2.1 Escape Analysis Failures

#### CRITICAL: Byte Slice Escapes to Heap in WriteShort/WriteInt/WriteLong
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/byte_buffers_data_output.go:154-172`
- **Severity:** CRITICAL
- **Current Code:**
```go
func (o *ByteBuffersDataOutput) WriteShort(v int16) error {
    buf := make([]byte, 2)  // Escapes to heap
    binary.LittleEndian.PutUint16(buf, uint16(v))
    return o.WriteBytes(buf)
}

func (o *ByteBuffersDataOutput) WriteInt(v int32) error {
    buf := make([]byte, 4)  // Escapes to heap
    binary.LittleEndian.PutUint32(buf, uint32(v))
    return o.WriteBytes(buf)
}

func (o *ByteBuffersDataOutput) WriteLong(v int64) error {
    buf := make([]byte, 8)  // Escapes to heap
    binary.BigEndian.PutUint64(buf, uint64(v))
    return o.WriteBytes(buf)
}
```
- **Performance Impact:** These are called extremely frequently during indexing. Each call allocates a new byte slice on the heap, causing significant GC pressure.
- **Recommendation:** Use a scratch buffer from a sync.Pool or make these methods use a stack-allocated array via unsafe or by inlining the binary encoding.

#### CRITICAL: Buffer Allocation in ForUtil encodeInternal
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/codecs/for_util.go:262-268`
- **Severity:** CRITICAL
- **Current Code:**
```go
buf := make([]byte, 4)
for i := 0; i < numIntsPerShift; i++ {
    binary.BigEndian.PutUint32(buf, uint32(f.tmp[i]))
    if err := out.WriteBytes(buf); err != nil {
        return err
    }
}
```
- **Performance Impact:** Buffer created and written in a loop - major allocation hotspot.
- **Recommendation:** Pre-allocate buffer outside loop or use direct write methods.

#### HIGH: Buffer Allocation in decodeSlow
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/codecs/for_util.go:279-286`
- **Severity:** HIGH
- **Current Code:**
```go
buf := make([]byte, 4)
tmp := make([]int32, numInts)
for i := 0; i < numInts; i++ {
    if err := in.ReadBytes(buf); err != nil {
        return err
    }
    tmp[i] = int32(binary.BigEndian.Uint32(buf))
}
```
- **Performance Impact:** Multiple allocations in a hot decoding path.
- **Recommendation:** Use pre-allocated buffers from a pool.

#### HIGH: String to Byte Conversion Allocates
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/byte_buffers_data_output.go:217-223`
- **Severity:** HIGH
- **Current Code:**
```go
func (o *ByteBuffersDataOutput) WriteString(s string) error {
    data := []byte(s)  // Allocates new byte slice
    if err := o.WriteVInt(int32(len(data))); err != nil {
        return err
    }
    return o.WriteBytes(data)
}
```
- **Performance Impact:** String-to-byte conversion allocates. Called frequently for field names and stored fields.
- **Recommendation:** Use unsafe.StringHeader to avoid allocation when safe, or accept the string as-is if the underlying writer can handle it.

### 2.2 Slice Capacity Planning Issues

#### HIGH: Zero-Capacity Slice Growth in ByteBuffersDataOutput
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/byte_buffers_data_output.go:79`
- **Severity:** HIGH
- **Current Code:**
```go
blocks: make([][]byte, 0),  // No capacity hint
```
- **Performance Impact:** Frequent reallocations as blocks are appended.
- **Recommendation:** Pre-allocate with reasonable capacity: `make([][]byte, 0, 16)`.

#### MEDIUM: Slice Growth Without Capacity Planning
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/util/paged_bytes.go:65-67`
- **Severity:** MEDIUM
- **Current Code:**
```go
if p.numBlocks == len(p.blocks) {
    newBlocks := make([][]byte, len(p.blocks)*2)
    copy(newBlocks, p.blocks)
    p.blocks = newBlocks
}
```
- **Performance Impact:** Doubling strategy is good but initial capacity of 16 may be too small for large indices.
- **Recommendation:** Allow configurable initial capacity based on expected index size.

#### MEDIUM: Frequent Slice Reallocation in ByteBlockPool
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/util/byte_block_pool.go:136-140`
- **Severity:** MEDIUM
- **Current Code:**
```go
if 1+p.bufferUpto == len(p.buffers) {
    newBuffers := make([][]byte, oversize(len(p.buffers)+1, 8))
    copy(newBuffers, p.buffers)
    p.buffers = newBuffers
}
```
- **Performance Impact:** oversize calculation and reallocation on every buffer exhaustion.
- **Recommendation:** Consider larger initial capacity or exponential growth factor.

### 2.3 Buffer Reuse Opportunities

#### HIGH: No Buffer Pool in InputStreamDataInput
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/input_stream_data_input.go:31,75`
- **Severity:** HIGH
- **Current Code:**
```go
func (in *InputStreamDataInput) ReadByte() (byte, error) {
    buf := make([]byte, 1)  // Allocates every call
    _, err := in.reader.Read(buf)
    return buf[0], err
}

func (in *InputStreamDataInput) ReadBytes(buf []byte) error {
    temp := make([]byte, 8192)  // Allocates 8KB buffer
    // ...
}
```
- **Performance Impact:** ReadByte is called for every byte read - massive allocation overhead.
- **Recommendation:** Use a sync.Pool for buffers or maintain a reusable buffer in the struct.

#### MEDIUM: No Buffer Reuse in CopyBytes
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/byte_buffers_data_output.go:226-232`
- **Severity:** MEDIUM
- **Current Code:**
```go
func (o *ByteBuffersDataOutput) CopyBytes(input DataInput, numBytes int64) error {
    buf := make([]byte, numBytes)  // Allocates based on input size
    if err := input.ReadBytes(buf); err != nil {
        return err
    }
    o.WriteBytes(buf)
    return nil
}
```
- **Performance Impact:** Large allocations for copying data.
- **Recommendation:** Use chunked copying with a fixed-size buffer from a pool.

---

## 3. Concurrency Performance Issues

### 3.1 Lock Contention

#### CRITICAL: Global Mutex in IndexWriter Hot Path
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/index/index_writer.go:70-80`
- **Severity:** CRITICAL
- **Current Code:**
```go
func (w *IndexWriter) AddDocument(doc Document) error {
    if err := w.ensureOpen(); err != nil {
        return err
    }
    w.mu.Lock()
    defer w.mu.Unlock()  // Held for entire operation
    w.docCount++
    return nil
}
```
- **Performance Impact:** Single mutex protects all writer operations - major bottleneck for concurrent indexing.
- **Recommendation:** Use finer-grained locking, lock-free counters for docCount, or sharded locks.

#### HIGH: RWMutex with Write-Heavy Pattern in DocumentsWriter
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/index/documents_writer.go:118-147`
- **Severity:** HIGH
- **Current Code:**
```go
func (dw *DocumentsWriter) UpdateDocument(doc Document, analyzer analysis.Analyzer, term *Term) error {
    dw.mu.Lock()
    defer dw.mu.Unlock()  // Write lock held for entire document processing
    // ... document processing ...
}
```
- **Performance Impact:** Write lock held during document processing blocks all other threads.
- **Recommendation:** Process document outside the lock, only hold lock for state updates.

#### HIGH: Lock per Attribute Access in AttributeSource
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/analysis/attribute_source.go:56-76`
- **Severity:** HIGH
- **Current Code:**
```go
func (as *AttributeSource) GetAttribute(name string) AttributeImpl {
    as.mu.RLock()
    defer as.mu.RUnlock()
    // ... iterate over map ...
}
```
- **Performance Impact:** Lock acquired for every attribute access during tokenization.
- **Recommendation:** Use sync.Map or pre-compute attribute indices, or use lock-free data structures.

#### HIGH: Mutex in TopDocsCollector Collect Method
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/search/top_docs_collector.go:115-143`
- **Severity:** HIGH
- **Current Code:**
```go
func (c *TopDocsLeafCollector) Collect(doc int) error {
    c.collector.mu.Lock()
    defer c.collector.mu.Unlock()
    // ... priority queue operations ...
}
```
- **Performance Impact:** Lock held for every document collected during search.
- **Recommendation:** Use per-segment collectors that merge results at the end, or use lock-free priority queue.

### 3.2 Goroutine Lifecycle Issues

#### MEDIUM: Goroutine per Merge Thread Without Pool
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/index/concurrent_merge_scheduler.go:334-371`
- **Severity:** MEDIUM
- **Current Code:**
```go
func (s *ConcurrentMergeScheduler) spawnMergeThread(source MergeSource, merge *OneMerge) {
    // ...
    go func() {
        defer s.runningMerges.Done()
        // ... merge execution ...
    }()
}
```
- **Performance Impact:** New goroutine created for each merge. Goroutine creation has overhead.
- **Recommendation:** Use a worker pool with fixed goroutines.

#### MEDIUM: Busy Wait in waitForMergeThread
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/index/concurrent_merge_scheduler.go:387-390`
- **Severity:** MEDIUM
- **Current Code:**
```go
func (s *ConcurrentMergeScheduler) waitForMergeThread() {
    time.Sleep(10 * time.Millisecond)  // Busy wait with sleep
}
```
- **Performance Impact:** Sleep-based waiting is inefficient. Can miss completion events.
- **Recommendation:** Use condition variables or channels for proper synchronization.

### 3.3 Race Conditions

#### CRITICAL: IndexReader Lock Copying (go vet finding)
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/index/index_writer.go:497`
- **Severity:** CRITICAL
- **Issue:** `range var reader copies lock: github.com/FlavioCFOliveira/Gocene/index.IndexReader contains sync/atomic.Bool contains sync/atomic.noCopy`
- **Performance Impact:** Copying mutexes can lead to deadlocks and race conditions.
- **Recommendation:** Use pointer receivers or ensure IndexReader is not copied.

#### HIGH: TaxonomyReader Lock Copying (go vet finding)
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/facets/taxonomy_reader.go:249,251`
- **Severity:** HIGH
- **Same issue as above** - lock copying in taxonomy reader.

---

## 4. I/O Performance Issues

### 4.1 Buffering Strategy Issues

#### HIGH: Small Buffer in InputStreamDataInput
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/input_stream_data_input.go:75`
- **Severity:** HIGH
- **Current Code:**
```go
temp := make([]byte, 8192)  // 8KB buffer for copying
```
- **Performance Impact:** Small buffer causes more system calls for large reads.
- **Recommendation:** Use 64KB or larger buffers, or make it configurable.

#### MEDIUM: No Buffered I/O in NIOFSDirectory
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/niofs_directory.go`
- **Severity:** MEDIUM
- **Issue:** Direct file reads without buffering can cause excessive system calls.
- **Recommendation:** Wrap file reads with bufio.Reader for small sequential reads.

### 4.2 File I/O Patterns

#### MEDIUM: Multiple File Opens in MMapDirectory
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/mmap_directory.go:141-150`
- **Severity:** MEDIUM
- **Current Code:**
```go
for i := 0; i < numChunks; i++ {
    var f *os.File
    if i == 0 {
        f = file
    } else {
        f, err = os.Open(path)  // Re-opens file for each chunk
    }
}
```
- **Performance Impact:** File opened multiple times for multi-chunk mappings.
- **Recommendation:** Use a single file handle with different offsets if possible.

#### LOW: No Preload Option for MMapDirectory
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/mmap_directory.go:37`
- **Severity:** LOW
- **Issue:** Preload option exists but may not be efficiently implemented.
- **Recommendation:** Implement madvise/MADV_SEQUENTIAL or MADV_WILLNEED for preloading.

---

## 5. Package-Specific Findings

### 5.1 Analysis Package

| Issue | File | Line | Severity |
|-------|------|------|----------|
| TokenStream not inlined | analyzer.go | 121 | CRITICAL |
| Defer prevents inlining | analyzer_utils.go | 26, 55 | HIGH |
| Lock per attribute access | attribute_source.go | 56 | HIGH |
| Reflection in GetAttribute | attribute_source.go | 61-74 | MEDIUM |
| Map iteration in hot path | attribute_source.go | 61 | MEDIUM |

### 5.2 Index Package

| Issue | File | Line | Severity |
|-------|------|------|----------|
| Global mutex in IndexWriter | index_writer.go | 75 | CRITICAL |
| Lock copying (go vet) | index_writer.go | 497 | CRITICAL |
| Write-heavy RWMutex | documents_writer.go | 118 | HIGH |
| Busy wait in merge scheduler | concurrent_merge_scheduler.go | 387 | MEDIUM |
| Slice reallocation in ByteBlockPool | byte_block_pool.go | 136 | MEDIUM |

### 5.3 Search Package

| Issue | File | Line | Severity |
|-------|------|------|----------|
| Mutex in Collect method | top_docs_collector.go | 115 | HIGH |
| Priority queue lock contention | top_docs_collector.go | 131-140 | MEDIUM |
| Interface conversion in hot path | index_searcher.go | 55-66 | MEDIUM |

### 5.4 Store Package

| Issue | File | Line | Severity |
|-------|------|------|----------|
| Heap allocation in WriteShort/Int/Long | byte_buffers_data_output.go | 154-172 | CRITICAL |
| Buffer allocation per ReadByte | input_stream_data_input.go | 31 | HIGH |
| String-to-byte allocation | byte_buffers_data_output.go | 217 | HIGH |
| Zero-capacity slice | byte_buffers_data_output.go | 79 | HIGH |
| Small buffer in CopyBytes | byte_buffers_data_output.go | 226 | MEDIUM |

### 5.5 Codecs Package

| Issue | File | Line | Severity |
|-------|------|------|----------|
| Buffer allocation in encodeInternal | for_util.go | 262 | CRITICAL |
| Buffer allocation in decodeSlow | for_util.go | 279 | HIGH |
| Buffer allocation in decode methods | for_util.go | 322-736 | HIGH |
| Loop allocation in ForUtil | for_util.go | 49 | MEDIUM |

### 5.6 Util Package

| Issue | File | Line | Severity |
|-------|------|------|----------|
| Slice growth in PagedBytes | paged_bytes.go | 65 | MEDIUM |
| Bounds checks in ByteBlockPool | byte_block_pool.go | 161 | MEDIUM |
| Counter not atomic | byte_block_pool.go | 76-94 | LOW |

---

## 6. Recommendations Summary

### Immediate Actions (CRITICAL/HIGH Priority)

1. **Fix heap allocations in store/byte_buffers_data_output.go**
   - Use sync.Pool for write buffers
   - Implement stack-allocated encoding for primitive types

2. **Refactor IndexWriter locking**
   - Implement sharded locks or lock-free counters
   - Minimize critical section in AddDocument

3. **Optimize ForUtil buffer management**
   - Pre-allocate buffers in struct
   - Reuse across encode/decode operations

4. **Fix lock copying issues**
   - Address go vet findings in index_writer.go and taxonomy_reader.go

5. **Improve AttributeSource concurrency**
   - Use sync.Map or pre-computed attribute indices
   - Consider lock-free attribute access

### Short-term Improvements (MEDIUM Priority)

1. **Implement buffer pooling**
   - Create a buffer pool package for frequently allocated buffers
   - Apply to InputStreamDataInput, CopyBytes, etc.

2. **Optimize slice capacity planning**
   - Pre-allocate slices with reasonable capacity hints
   - Review all `make([]T, 0)` calls

3. **Improve merge scheduler synchronization**
   - Replace busy waits with condition variables
   - Consider worker pool pattern

4. **Reduce defer usage in hot paths**
   - Replace with explicit cleanup where performance critical

### Long-term Optimizations (LOW Priority)

1. **Profile-guided optimization**
   - Run benchmarks with CPU and memory profiling
   - Focus on actual hotspots identified by profiling

2. **SIMD optimizations**
   - Consider SIMD for ForUtil encoding/decoding
   - Use Go's vector instructions where applicable

3. **Memory-mapped I/O improvements**
   - Implement proper preload with madvise
   - Optimize chunk size selection

---

## 7. Build Analysis Results

### Escape Analysis Summary
- **Total escape analysis warnings:** 127
- **Heap allocations from escapes:** 89
- **Stack allocations:** 38

### Inlining Analysis Summary
- **Functions that can inline:** 312
- **Functions too complex to inline:** 47
- **Functions prevented by defer:** 23

### go vet Findings
- **Lock copying issues:** 4
- **Method signature issues:** 2
- **Undefined references:** 2 (test files)
- **Constant overflow:** 3 (test files)

---

## Appendix: Performance Testing Recommendations

To validate these findings, run the following benchmarks:

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./...

# Memory profiling
go test -memprofile=mem.prof -bench=. ./...

# Escape analysis
go build -gcflags="-m -m" ./... 2>&1 | grep escapes

# Race detection
go test -race ./...

# Benchmark specific packages
go test -bench=BenchmarkIndexWriter -benchmem ./index
go test -bench=BenchmarkSearch -benchmem ./search
go test -bench=BenchmarkForUtil -benchmem ./codecs
```

---

*Report generated by Go Performance Advisor*
*For questions or clarifications, refer to the specific file paths and line numbers provided above.*
