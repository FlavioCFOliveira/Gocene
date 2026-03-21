# SIMD Optimization Design Notes (GC-832)

## Status
COMPLETED 2026-03-21 - Design document created

## Overview
SIMD (Single Instruction, Multiple Data) optimizations for ForUtil encoding/decoding.

## Limitations

### Go Language Constraints
- Go does not have native SIMD intrinsics like C/C++
- Options available:
  1. **Assembly**: Write Go assembly code for hot paths
  2. **CGO**: Call C libraries with SIMD code
  3. **Compiler intrinsics**: Not available in standard Go
  4. **Auto-vectorization**: Go compiler does limited auto-vectorization

### Recommended Approach

For production implementation, consider:

1. **Assembly for critical paths**:
   - ForUtil.encode/decode loops
   - PForDelta unpacking
   - Bit packing operations

2. **Use existing libraries**:
   - `github.com/minio/c2goasm` for C to Go assembly
   - `golang.org/x/sys/cpu` for CPU feature detection

3. **Focus areas**:
   - Bit unpacking (most common operation)
   - Bulk copies with aligned memory
   - XOR operations for compression

## Implementation Priority

### Phase 1: Assembly Implementation
- Target: ForUtil.decode8 and decode16
- Expected improvement: 2-4x for bulk operations

### Phase 2: Auto-vectorization Hints
- Use compiler-friendly patterns
- Avoid pointer aliasing
- Use fixed-size arrays where possible

## Benchmarking

Required benchmarks before implementation:
```go
func BenchmarkForUtilDecode(b *testing.B) {
    // Current implementation
}

func BenchmarkForUtilDecodeSIMD(b *testing.B) {
    // SIMD implementation
}
```

## Conclusion

SIMD optimizations require careful consideration of Go's limitations.
Assembly implementation is recommended for critical paths, but adds
maintenance burden and platform-specific code.

For Gocene Phase 50+, consider revisiting when Go adds better SIMD support.
