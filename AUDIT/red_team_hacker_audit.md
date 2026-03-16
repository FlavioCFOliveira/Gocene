# Gocene Security Audit Report

**Audit Date:** 2026-03-16
**Auditor:** Red Team Security Analysis
**Scope:** Full codebase analysis (484 Go files)
**Packages Audited:** index, search, codecs, store, analysis, util, document, queryparser, facets, join, grouping, highlight

---

## Executive Summary

This security audit analyzed the Gocene codebase, a Go port of Apache Lucene. The codebase is in early development stages with significant portions being stubs or partial implementations. While no critical vulnerabilities (RCE, data loss) were identified, several security concerns were found ranging from MEDIUM to LOW severity, primarily related to:

1. Path traversal risks in file operations
2. Integer overflow potential in buffer allocations
3. Race conditions in concurrent merge scheduling
4. Resource exhaustion vectors in query parsing
5. TOCTOU (Time-of-Check to Time-of-Use) vulnerabilities

**Overall Risk Assessment:** MEDIUM - The codebase requires hardening before production use, particularly around file system operations and resource management.

---

## Detailed Findings

### 1. Path Traversal Vulnerabilities

#### FINDING-001: Insufficient Path Validation in FSDirectory
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/fs_directory.go`
- **Lines:** 111, 122, 160, 207, 232-233, 268, 380, 413
- **Severity:** HIGH
- **CWE:** CWE-22 (Path Traversal), CWE-73 (External Control of File Name or Path)

**Description:**
The `FSDirectory` implementation uses `filepath.Join()` to construct file paths without validating that the `name` parameter does not contain path traversal sequences (`../`, `..\`). An attacker could potentially:
- Read files outside the intended directory
- Write files to arbitrary locations
- Delete files outside the index directory

**Vulnerable Code Pattern:**
```go
// Line 111 in FileExists
path := filepath.Join(d.directory, name)

// Line 160 in DeleteFile
path := filepath.Join(d.directory, name)

// Line 232-233 in Rename
sourcePath := filepath.Join(d.directory, source)
destPath := filepath.Join(d.directory, dest)
```

**Proof of Concept:**
```go
// An attacker could call:
dir.OpenInput("../../../etc/passwd", ctx)  // Reads system files
dir.CreateOutput("../../../tmp/malicious", ctx)  // Writes outside index
dir.DeleteFile("../../../important/file")  // Deletes arbitrary files
```

**Remediation:**
```go
// Add path validation function
func validateFileName(name string) error {
    // Reject absolute paths
    if filepath.IsAbs(name) {
        return fmt.Errorf("absolute paths not allowed: %s", name)
    }
    // Reject paths containing ..
    if strings.Contains(name, "..") {
        return fmt.Errorf("path traversal not allowed: %s", name)
    }
    // Reject paths with null bytes
    if strings.Contains(name, "\x00") {
        return fmt.Errorf("null bytes not allowed: %s", name)
    }
    // Validate against allowed pattern
    validName := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
    if !validName.MatchString(name) {
        return fmt.Errorf("invalid filename: %s", name)
    }
    return nil
}

// Use in all file operations
func (d *FSDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
    if err := validateFileName(name); err != nil {
        return nil, err
    }
    path := filepath.Join(d.directory, name)
    // ... rest of implementation
}
```

---

#### FINDING-002: Path Traversal in NativeFSLockFactory
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/lock.go`
- **Line:** 94
- **Severity:** MEDIUM
- **CWE:** CWE-22 (Path Traversal)

**Description:**
The `NativeFSLockFactory.ObtainLock()` method constructs lock file paths without validating the `lockName` parameter, allowing path traversal attacks.

**Vulnerable Code:**
```go
lockFile := filepath.Join(path, lockName+".lock")
```

**Remediation:**
Apply the same `validateFileName()` function to the `lockName` parameter before constructing the path.

---

### 2. Integer Overflow and Buffer Issues

#### FINDING-003: Potential Integer Overflow in ByteArrayDataInput
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/index_input.go`
- **Lines:** 196-203, 250-270
- **Severity:** MEDIUM
- **CWE:** CWE-190 (Integer Overflow), CWE-122 (Heap-based Buffer Overflow)

**Description:**
The `ReadBytesN()` and `ReadString()` methods allocate buffers based on user-controlled length values without overflow checks. A corrupted index file could specify extremely large lengths, causing:
- Memory exhaustion
- Integer overflow leading to undersized buffer allocation
- Potential buffer overflow

**Vulnerable Code:**
```go
// Line 196-203
func (in *ByteArrayDataInput) ReadBytesN(n int) ([]byte, error) {
    if in.pos+n > len(in.bytes) {  // Integer overflow possible here
        return nil, io.EOF
    }
    result := make([]byte, n)  // Could allocate huge buffer
    // ...
}

// Line 250-270 - VInt reading without bounds
func (in *ByteArrayDataInput) ReadVInt() (int32, error) {
    // No maximum shift limit until shift >= 32
    if shift >= 32 {
        return 0, fmt.Errorf("corrupted VInt")
    }
}
```

**Remediation:**
```go
// Add maximum buffer size constant
const MaxBufferSize = 1 << 30 // 1GB limit

func (in *ByteArrayDataInput) ReadBytesN(n int) ([]byte, error) {
    if n < 0 {
        return nil, fmt.Errorf("negative length: %d", n)
    }
    if n > MaxBufferSize {
        return nil, fmt.Errorf("length exceeds maximum: %d", n)
    }
    if in.pos+n > len(in.bytes) {
        return nil, io.EOF
    }
    result := make([]byte, n)
    copy(result, in.bytes[in.pos:in.pos+n])
    in.pos += n
    return result, nil
}
```

---

#### FINDING-004: Unbounded Memory Growth in LRUQueryCache
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/search/lru_query_cache.go`
- **Lines:** 52-66, 137-159
- **Severity:** MEDIUM
- **CWE:** CWE-770 (Allocation of Resources Without Limits), CWE-400 (Uncontrolled Resource Consumption)

**Description:**
The `LRUQueryCache` allows setting `maxRamBytes` to 0 (unlimited), which could lead to unbounded memory growth. Additionally, the cache uses `maxSize` but doesn't properly account for the actual memory usage of cached Weight objects.

**Vulnerable Code:**
```go
// Line 58-66 - maxRamBytes of 0 means unlimited
func NewLRUQueryCache(maxSize int, maxRamBytes int64) *LRUQueryCache {
    return &LRUQueryCache{
        maxSize:        maxSize,
        maxRamBytes:    maxRamBytes,  // 0 = unlimited
        // ...
    }
}

// Line 153-159 - TODO indicates unimplemented RAM tracking
func (c *LRUQueryCache) shouldEvict() bool {
    if c.maxSize > 0 && len(c.cache) >= c.maxSize {
        return true
    }
    // TODO: Check maxRamBytes  // UNIMPLEMENTED
    return false
}
```

**Remediation:**
```go
// Enforce minimum and maximum cache sizes
const (
    MinCacheSize = 10
    MaxCacheSize = 100000
    DefaultMaxRamBytes = 100 * 1024 * 1024 // 100MB default
)

func NewLRUQueryCache(maxSize int, maxRamBytes int64) *LRUQueryCache {
    if maxSize <= 0 {
        maxSize = MinCacheSize
    }
    if maxSize > MaxCacheSize {
        maxSize = MaxCacheSize
    }
    if maxRamBytes <= 0 {
        maxRamBytes = DefaultMaxRamBytes
    }
    // ...
}
```

---

### 3. Concurrency Issues

#### FINDING-005: Race Condition in ConcurrentMergeScheduler
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/index/concurrent_merge_scheduler.go`
- **Lines:** 287-301, 334-370
- **Severity:** MEDIUM
- **CWE:** CWE-362 (Race Condition), CWE-667 (Improper Locking)

**Description:**
The `spawnMergeThread()` method accesses shared state (`mergeThreads`, `pendingMerges`) without proper synchronization. The `mergeMu` is unlocked before spawning the goroutine, allowing race conditions.

**Vulnerable Code:**
```go
// Line 287-301
s.mergeMu.Lock()
activeThreads := len(s.mergeThreads)
s.mergeMu.Unlock()

if activeThreads < maxThreadCount {
    s.spawnMergeThread(source, merge)
} else {
    s.waitForMergeThread()
    s.mergeMu.Lock()
    s.pendingMerges = append(s.pendingMerges, merge)  // Race here
    s.mergeMu.Unlock()
}
```

**Remediation:**
```go
// Use proper synchronization for all shared state access
func (s *ConcurrentMergeScheduler) Merge(source MergeSource, trigger MergeTrigger) error {
    // ...
    s.mergeMu.Lock()
    activeThreads := len(s.mergeThreads)
    canSpawn := activeThreads < maxThreadCount
    s.mergeMu.Unlock()

    if canSpawn {
        s.spawnMergeThread(source, merge)
    } else {
        // Use channel-based synchronization instead of polling
        select {
        case s.mergeThreadAvailable <- struct{}{}:
            s.mergeMu.Lock()
            s.pendingMerges = append(s.pendingMerges, merge)
            s.mergeMu.Unlock()
        case <-s.ctx.Done():
            return fmt.Errorf("scheduler closed")
        }
    }
}
```

---

#### FINDING-006: Lock Copying in IndexReader
- **File:** Multiple files
- **Lines:** Various
- **Severity:** MEDIUM
- **CWE:** CWE-662 (Improper Synchronization), CWE-609 (Double-Checked Locking)

**Description:**
`go vet` reported that `IndexReader` contains `sync/atomic.Bool` which should not be copied. The struct is being passed by value in several locations.

**go vet Output:**
```
index/index_writer.go:497:9: range var reader copies lock: github.com/FlavioCFOliveira/Gocene/index.IndexReader contains sync/atomic.Bool contains sync/atomic.noCopy
join/join_util.go:67:60: BuildBitSet passes lock by value: github.com/FlavioCFOliveira/Gocene/index.IndexReader contains sync/atomic.Bool contains sync/atomic.noCopy
facets/taxonomy_reader.go:249:38: NewTaxonomyReaderFactory passes lock by value
```

**Remediation:**
Pass `IndexReader` by pointer (`*IndexReader`) instead of by value, or remove the `noCopy` sentinel from the struct if copying is intentional.

---

### 4. Resource Management Issues

#### FINDING-007: File Descriptor Leak in MMapDirectory
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/mmap_directory.go`
- **Lines:** 141-172
- **Severity:** MEDIUM
- **CWE:** CWE-404 (Improper Resource Shutdown), CWE-772 (Missing Release of Resource)

**Description:**
In `OpenInput()`, if memory mapping fails after opening files, the cleanup code may not properly close all file descriptors, especially in multi-chunk scenarios.

**Vulnerable Code:**
```go
// Line 141-172
for i := 0; i < numChunks; i++ {
    // ...
    if i == 0 {
        f = file
    } else {
        f, err = os.Open(path)  // Opens new fd
        // ...
    }

    chunk, err := mmap(f, remaining)
    if err != nil {
        // Cleanup may not close all fds properly
        for j := 0; j < i; j++ {
            chunks[j].unmap()
            chunks[j].close()
        }
        if i > 0 {
            f.Close()
        } else {
            file.Close()
        }
        return nil, err
    }
}
```

**Remediation:**
Use `defer` for cleanup and ensure all file descriptors are tracked:
```go
func (d *MMapDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
    // ...
    var openFiles []*os.File
    defer func() {
        if err != nil {
            for _, f := range openFiles {
                f.Close()
            }
        }
    }()

    for i := 0; i < numChunks; i++ {
        var f *os.File
        if i == 0 {
            f = file
        } else {
            f, err = os.Open(path)
            if err != nil {
                return nil, err
            }
            openFiles = append(openFiles, f)
        }
        // ... map and add to chunks
    }
    openFiles = nil // Success, don't close in defer
    return input, nil
}
```

---

#### FINDING-008: Goroutine Leak in ConcurrentMergeScheduler
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/index/concurrent_merge_scheduler.go`
- **Lines:** 416-444
- **Severity:** MEDIUM
- **CWE:** CWE-404 (Improper Resource Shutdown)

**Description:**
The `Close()` method has a 60-second timeout but doesn't forcefully terminate goroutines if they don't complete, potentially leaving merge goroutines running indefinitely.

**Remediation:**
```go
func (s *ConcurrentMergeScheduler) Close() error {
    s.mu.Lock()
    if s.IsClosed() {
        s.mu.Unlock()
        return nil
    }
    s.mu.Unlock()

    s.cancel() // Signal cancellation

    // Use context with timeout for graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    done := make(chan struct{})
    go func() {
        s.runningMerges.Wait()
        close(done)
    }()

    select {
    case <-done:
        return s.BaseMergeScheduler.Close()
    case <-ctx.Done():
        // Log warning about leaked goroutines
        return fmt.Errorf("timeout waiting for merges to complete")
    }
}
```

---

### 5. Denial of Service Vulnerabilities

#### FINDING-009: Algorithmic Complexity in QueryParser
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/queryparser/query_parser.go`
- **Lines:** 58-68, 96-134
- **Severity:** MEDIUM
- **CWE:** CWE-407 (Inefficient Algorithmic Complexity), CWE-834 (Excessive Iteration)

**Description:**
The query parser uses recursive descent parsing without depth limits. A malicious query with deeply nested expressions could cause:
- Stack overflow
- Excessive CPU consumption
- Memory exhaustion

**Vulnerable Pattern:**
```go
// Recursive parsing without depth limit
func (p *QueryParser) parseExpression() (search.Query, error) {
    left, err := p.parseAndExpression()  // Recursive
    // ...
    for p.match(TokenTypeOR) {
        right, err := p.parseAndExpression()  // More recursion
        left = search.NewBooleanQueryOrWithQueries(left, right)
    }
}
```

**Remediation:**
```go
const MaxQueryDepth = 100

type QueryParser struct {
    // ... existing fields
    depth int
}

func (p *QueryParser) parseExpression() (search.Query, error) {
    p.depth++
    if p.depth > MaxQueryDepth {
        return nil, fmt.Errorf("query too complex: exceeds maximum depth of %d", MaxQueryDepth)
    }
    defer func() { p.depth-- }()

    // ... rest of implementation
}
```

---

#### FINDING-010: Unbounded Input in QueryParserTokenManager
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/queryparser/query_parser_token_manager.go`
- **Lines:** 108-114, 276-345
- **Severity:** LOW
- **CWE:** CWE-770 (Allocation of Resources Without Limits)

**Description:**
The token manager accepts arbitrary-length input strings without limits, potentially causing memory exhaustion with extremely large queries.

**Remediation:**
```go
const MaxQueryLength = 100000 // 100KB limit

func NewQueryParserTokenManager(input string) (*QueryParserTokenManager, error) {
    if len(input) > MaxQueryLength {
        return nil, fmt.Errorf("query exceeds maximum length of %d bytes", MaxQueryLength)
    }
    return &QueryParserTokenManager{
        input: input,
        pos:   0,
        len:   len(input),
    }, nil
}
```

---

### 6. TOCTOU (Time-of-Check to Time-of-Use) Issues

#### FINDING-011: TOCTOU in FSDirectory.Rename
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/fs_directory.go`
- **Lines:** 226-253
- **Severity:** MEDIUM
- **CWE:** CWE-367 (Time-of-check Time-of-use Race Condition)

**Description:**
The `Rename()` method checks if the destination exists before renaming, but this check is not atomic with the actual rename operation.

**Vulnerable Code:**
```go
// Line 243-248
if _, err := os.Stat(destPath); err == nil {
    return fmt.Errorf("%w: %s", ErrFileAlreadyExists, dest)
}

if err := os.Rename(sourcePath, destPath); err != nil {
    return fmt.Errorf("failed to rename file: %w", err)
}
```

**Remediation:**
Use atomic operations or handle the error from `os.Rename` directly:
```go
func (d *FSDirectory) Rename(source string, dest string) error {
    // ... validation ...

    // Attempt rename directly - let OS handle atomicity
    if err := os.Rename(sourcePath, destPath); err != nil {
        if os.IsExist(err) {
            return fmt.Errorf("%w: %s", ErrFileAlreadyExists, dest)
        }
        return fmt.Errorf("failed to rename file: %w", err)
    }
    return nil
}
```

---

#### FINDING-012: TOCTOU in FSDirectory.CreateOutput
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/fs_directory.go`
- **Lines:** 407-434
- **Severity:** MEDIUM
- **CWE:** CWE-367 (Time-of-check Time-of-use Race Condition)

**Description:**
Similar to FINDING-011, `CreateOutput()` checks if a file exists before creating it, creating a race condition window.

**Remediation:**
Use `os.O_EXCL` flag for atomic creation:
```go
file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
if err != nil {
    if os.IsExist(err) {
        return nil, fmt.Errorf("%w: %s", ErrFileAlreadyExists, name)
    }
    return nil, fmt.Errorf("failed to create file: %w", err)
}
```

---

### 7. Input Validation Issues

#### FINDING-013: Insufficient Validation in Codec Headers
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/codecs/codec_util.go`
- **Lines:** 100-124, 126-153
- **Severity:** LOW
- **CWE:** CWE-20 (Improper Input Validation)

**Description:**
The `CheckHeader()` and `CheckIndexHeader()` functions validate codec headers but don't have strict enough validation on string lengths and content, potentially allowing malformed data to pass initial checks.

**Remediation:**
Add stricter validation for codec names and suffixes:
```go
func checkCodecName(codec string) error {
    if len(codec) == 0 || len(codec) >= 128 {
        return fmt.Errorf("invalid codec name length: %d", len(codec))
    }
    // Only allow alphanumeric and underscore
    valid := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
    if !valid.MatchString(codec) {
        return fmt.Errorf("invalid codec name characters: %s", codec)
    }
    return nil
}
```

---

#### FINDING-014: Integer Overflow in NumericUtils
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/util/numeric_utils_test.go`
- **Lines:** 823, 827, 847, 851
- **Severity:** LOW
- **CWE:** CWE-190 (Integer Overflow)

**Description:**
Test code contains integer overflow issues with constants exceeding int64/int32 limits.

**go vet Output:**
```
util/numeric_utils_test.go:823:30: constant 9223372036854775808 overflows int64
util/numeric_utils_test.go:847:22: constant 2147483648 overflows int32
```

**Remediation:**
Use proper type casting or constants within valid ranges:
```go
// Instead of:
value := 9223372036854775808 // overflows int64

// Use:
value := uint64(1) << 63
// or
value := math.MaxInt64
```

---

### 8. Information Disclosure

#### FINDING-015: Process ID Disclosure in Lock Files
- **File:** `/Users/flaviocfo/dev/github.com/FlavioCFOliveira/Gocene/store/fs_directory.go`
- **Line:** 284-285
- **Severity:** LOW
- **CWE:** CWE-200 (Exposure of Sensitive Information)

**Description:**
The lock file contains the process ID written in plaintext, which could disclose system information to attackers.

**Vulnerable Code:**
```go
pid := os.Getpid()
fmt.Fprintf(f, "%d\n", pid)
```

**Remediation:**
Either remove the PID from the lock file or use a non-reversible hash:
```go
// Option 1: Remove PID entirely
// Just create empty lock file with O_EXCL

// Option 2: Use hash
h := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", os.Getpid(), time.Now().UnixNano())))
fmt.Fprintf(f, "%x\n", h[:8]) // Only write partial hash
```

---

## Build and Test Analysis

### go vet Results
The following issues were identified by `go vet`:

1. **Lock Copying Issues:** Multiple locations where structs containing `sync/atomic.Bool` are copied by value
2. **Function Signature Mismatch:** `Seek` method signature doesn't match `io.Seeker` interface
3. **Integer Overflow:** Test file constants exceed type limits
4. **Function Redeclaration:** `atLeast` function redeclared in test files

### Race Detector
Running `go test -race ./...` revealed compilation errors that prevent race detection:
- Missing package dependencies
- Type mismatches in test files
- Undefined symbols in test code

**Recommendation:** Fix compilation errors to enable race detection testing.

---

## Positive Security Findings

The following security-positive patterns were observed:

1. **Checksum Validation:** `ChecksumIndexInput` and codec footers provide data integrity checks
2. **Magic Number Validation:** Codec headers use magic numbers to verify file format
3. **Version Checking:** Codec utilities validate version ranges
4. **Proper Mutex Usage:** Most concurrent code uses `sync.Mutex` or `sync.RWMutex` appropriately
5. **Context Cancellation:** `ConcurrentMergeScheduler` uses Go contexts for cancellation
6. **Resource Cleanup:** `IOUtils` provides helper functions for safe resource cleanup
7. **Buffer Validation:** `BufferedIndexInput` validates slice parameters

---

## Recommendations

### Immediate Actions (HIGH Priority)

1. **Implement Path Validation:** Add filename validation to all file operations in `store` package
2. **Fix Race Conditions:** Review and fix synchronization in `ConcurrentMergeScheduler`
3. **Add Resource Limits:** Implement maximum buffer sizes and query complexity limits
4. **Fix TOCTOU Issues:** Use atomic file operations where possible

### Short-term Actions (MEDIUM Priority)

1. **Enable Race Detection:** Fix compilation errors to allow `go test -race`
2. **Add Timeout Controls:** Implement timeouts for all potentially long-running operations
3. **Implement Memory Limits:** Complete RAM tracking in `LRUQueryCache`
4. **Add Input Validation:** Validate all user-controlled input parameters

### Long-term Actions (LOW Priority)

1. **Security Testing:** Add fuzzing tests for file format parsers
2. **Audit Logging:** Add security-relevant logging for file operations
3. **Resource Quotas:** Implement per-index resource quotas
4. **Code Review:** Conduct regular security-focused code reviews

---

## Appendix: Files Audited

### Core Packages
- `store/` - Directory and file I/O implementations
- `index/` - Index writing, reading, and management
- `codecs/` - Index format codecs
- `search/` - Query execution and caching
- `analysis/` - Text analysis and tokenization
- `util/` - Utility functions and data structures
- `document/` - Document handling
- `queryparser/` - Query parsing
- `facets/` - Faceted search
- `join/` - Join operations
- `grouping/` - Result grouping
- `highlight/` - Search result highlighting

### Total Statistics
- **Total Go Files:** 484
- **Lines of Code:** ~50,000+
- **Test Files:** ~150
- **Packages:** 12

---

## Conclusion

The Gocene codebase shows good architectural patterns inherited from Apache Lucene, but requires significant hardening before production deployment. The most critical issues are:

1. **Path traversal vulnerabilities** in file operations
2. **Race conditions** in concurrent merge scheduling
3. **Resource exhaustion** vectors in query processing

Addressing these issues will require:
- Input validation at all entry points
- Proper synchronization for concurrent operations
- Resource limits and quotas
- Comprehensive testing with race detection

**Overall Security Posture:** The codebase is in early development and should be considered **NOT PRODUCTION READY** from a security perspective until the HIGH and MEDIUM severity findings are addressed.

---

*Report generated by automated security analysis tools and manual code review.*
