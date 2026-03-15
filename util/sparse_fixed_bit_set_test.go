// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/bits"
	"testing"
)

// TestSparseFixedBitSet_Creation tests basic creation of SparseFixedBitSet
func TestSparseFixedBitSet_Creation(t *testing.T) {
	// Test normal creation
	sfs, err := NewSparseFixedBitSet(100)
	if err != nil {
		t.Fatalf("Failed to create SparseFixedBitSet: %v", err)
	}
	if sfs == nil {
		t.Fatal("Expected non-nil SparseFixedBitSet")
	}
	if sfs.Length() != 100 {
		t.Errorf("Expected length 100, got %d", sfs.Length())
	}

	// Test empty bitset - should error since length must be >= 1
	_, err = NewSparseFixedBitSet(0)
	if err == nil {
		t.Error("Expected error for length 0")
	}

	// Test negative size
	_, err = NewSparseFixedBitSet(-1)
	if err == nil {
		t.Error("Expected error for negative size")
	}
}

// TestSparseFixedBitSet_SetAndGet tests basic set and get operations
func TestSparseFixedBitSet_SetAndGet(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(10000)

	// Initially all bits should be 0
	if sfs.Get(50) {
		t.Error("Expected bit 50 to be unset initially")
	}

	// Set a bit
	sfs.Set(50)
	if !sfs.Get(50) {
		t.Error("Expected bit 50 to be set")
	}

	// Other bits should still be 0
	if sfs.Get(51) {
		t.Error("Expected bit 51 to be unset")
	}

	// Test boundary
	sfs.Set(0)
	if !sfs.Get(0) {
		t.Error("Expected bit 0 to be set")
	}

	sfs.Set(9999)
	if !sfs.Get(9999) {
		t.Error("Expected bit 9999 to be set")
	}
}

// TestSparseFixedBitSet_GetAndSet tests the getAndSet operation
func TestSparseFixedBitSet_GetAndSet(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(100)

	// GetAndSet on unset bit should return false
	if sfs.GetAndSet(50) {
		t.Error("Expected GetAndSet to return false for unset bit")
	}

	// Bit should now be set
	if !sfs.Get(50) {
		t.Error("Expected bit 50 to be set after GetAndSet")
	}

	// GetAndSet on set bit should return true
	if !sfs.GetAndSet(50) {
		t.Error("Expected GetAndSet to return true for set bit")
	}

	// Bit should still be set
	if !sfs.Get(50) {
		t.Error("Expected bit 50 to still be set")
	}
}

// TestSparseFixedBitSet_Clear tests single bit clear operation
func TestSparseFixedBitSet_Clear(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(100)

	sfs.Set(50)
	if !sfs.Get(50) {
		t.Fatal("Bit should be set before clearing")
	}

	sfs.Clear(50)
	if sfs.Get(50) {
		t.Error("Expected bit 50 to be cleared")
	}
}

// TestSparseFixedBitSet_ClearAll tests clearing all bits
func TestSparseFixedBitSet_ClearAll(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(10000)

	// Set bits in different blocks
	sfs.Set(10)
	sfs.Set(5000)
	sfs.Set(9999)

	sfs.ClearAll()

	if !sfs.IsEmpty() {
		t.Error("Expected bitset to be empty after ClearAll")
	}

	// Check all bits are cleared
	for i := 0; i < 10000; i++ {
		if sfs.Get(i) {
			t.Errorf("Expected bit %d to be cleared after ClearAll", i)
			break
		}
	}
}

// TestSparseFixedBitSet_Cardinality tests the cardinality method
func TestSparseFixedBitSet_Cardinality(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(10000)

	if sfs.Cardinality() != 0 {
		t.Errorf("Expected initial cardinality 0, got %d", sfs.Cardinality())
	}

	// Set some bits
	sfs.Set(10)
	sfs.Set(50)
	sfs.Set(90)
	sfs.Set(5000)
	sfs.Set(9999)

	if sfs.Cardinality() != 5 {
		t.Errorf("Expected cardinality 5, got %d", sfs.Cardinality())
	}

	// Clear a bit
	sfs.Clear(50)
	if sfs.Cardinality() != 4 {
		t.Errorf("Expected cardinality 4 after clear, got %d", sfs.Cardinality())
	}
}

// TestSparseFixedBitSet_ApproximateCardinality tests approximate cardinality
func TestSparseFixedBitSet_ApproximateCardinality(t *testing.T) {
	// Test with sparse set
	sfs, _ := NewSparseFixedBitSet(10000)

	// Set bits with large intervals (sparse)
	first := RandomIntN(1000)
	interval := 200 + RandomIntN(1000)
	count := 0
	for i := first; i < sfs.Length(); i += interval {
		sfs.Set(i)
		count++
	}

	actual := sfs.Cardinality()
	approx := sfs.ApproximateCardinality()

	// Approximate cardinality should be within reasonable range
	// The linear counting algorithm has some variance
	diff := abs(approx - actual)
	if diff > 20 {
		t.Errorf("Approximate cardinality %d too far from actual %d (diff %d)", approx, actual, diff)
	}
}

// TestSparseFixedBitSet_ApproximateCardinalityOnDenseSet tests approximate cardinality on dense set
func TestSparseFixedBitSet_ApproximateCardinalityOnDenseSet(t *testing.T) {
	// Test with all bits set (dense)
	numDocs := 1000 + RandomIntN(10000)
	sfs, _ := NewSparseFixedBitSet(numDocs)

	for i := 0; i < sfs.Length(); i++ {
		sfs.Set(i)
	}

	approx := sfs.ApproximateCardinality()
	if approx != numDocs {
		t.Errorf("Expected approximate cardinality %d for dense set, got %d", numDocs, approx)
	}
}

// TestSparseFixedBitSet_IsEmpty tests the IsEmpty method
func TestSparseFixedBitSet_IsEmpty(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(100)

	if !sfs.IsEmpty() {
		t.Error("Expected empty bitset initially")
	}

	sfs.Set(50)
	if sfs.IsEmpty() {
		t.Error("Expected non-empty after setting bit")
	}

	sfs.Clear(50)
	if !sfs.IsEmpty() {
		t.Error("Expected empty after clearing bit")
	}
}

// TestSparseFixedBitSet_NextSetBit tests nextSetBit operation
func TestSparseFixedBitSet_NextSetBit(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(10000)

	sfs.Set(10)
	sfs.Set(50)
	sfs.Set(5000)
	sfs.Set(9999)

	// Find first set bit
	next := sfs.NextSetBit(0)
	if next != 10 {
		t.Errorf("Expected next set bit from 0 to be 10, got %d", next)
	}

	// Find next set bit
	next = sfs.NextSetBit(11)
	if next != 50 {
		t.Errorf("Expected next set bit from 11 to be 50, got %d", next)
	}

	// Find next set bit across blocks
	next = sfs.NextSetBit(51)
	if next != 5000 {
		t.Errorf("Expected next set bit from 51 to be 5000, got %d", next)
	}

	// Find last set bit
	next = sfs.NextSetBit(5001)
	if next != 9999 {
		t.Errorf("Expected next set bit from 5001 to be 9999, got %d", next)
	}

	// No more set bits
	next = sfs.NextSetBit(10000)
	if next != -1 {
		t.Errorf("Expected -1 after last set bit, got %d", next)
	}
}

// TestSparseFixedBitSet_NextSetBitInRange tests nextSetBit with upper bound
func TestSparseFixedBitSet_NextSetBitInRange(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(1000)

	sfs.Set(100)
	sfs.Set(500)
	sfs.Set(900)

	// Find within range
	next := sfs.NextSetBit(0, 200)
	if next != 100 {
		t.Errorf("Expected next set bit in range [0, 200) to be 100, got %d", next)
	}

	// No set bit in range
	next = sfs.NextSetBit(200, 400)
	if next != -1 {
		t.Errorf("Expected -1 for no set bit in range, got %d", next)
	}

	// Upper bound excludes the bit
	next = sfs.NextSetBit(100, 100)
	if next != -1 {
		t.Errorf("Expected -1 when upper bound equals start, got %d", next)
	}
}

// TestSparseFixedBitSet_PrevSetBit tests prevSetBit operation
func TestSparseFixedBitSet_PrevSetBit(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(10000)

	sfs.Set(10)
	sfs.Set(50)
	sfs.Set(5000)
	sfs.Set(9999)

	// Find previous set bit
	prev := sfs.PrevSetBit(9998)
	if prev != 5000 {
		t.Errorf("Expected prev set bit from 9998 to be 5000, got %d", prev)
	}

	// Find previous across blocks
	prev = sfs.PrevSetBit(4999)
	if prev != 50 {
		t.Errorf("Expected prev set bit from 4999 to be 50, got %d", prev)
	}

	// Find first set bit
	prev = sfs.PrevSetBit(49)
	if prev != 10 {
		t.Errorf("Expected prev set bit from 49 to be 10, got %d", prev)
	}

	// No previous set bits
	prev = sfs.PrevSetBit(9)
	if prev != -1 {
		t.Errorf("Expected -1 before first set bit, got %d", prev)
	}
}

// TestSparseFixedBitSet_ClearRange tests clearing a range of bits
func TestSparseFixedBitSet_ClearRange(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(1000)

	// Set all bits
	for i := 0; i < 1000; i++ {
		sfs.Set(i)
	}

	// Clear a range
	sfs.ClearRange(100, 200)

	// Check bits before range are still set
	if !sfs.Get(99) {
		t.Error("Expected bit 99 to still be set")
	}

	// Check bits in range are cleared
	for i := 100; i < 200; i++ {
		if sfs.Get(i) {
			t.Errorf("Expected bit %d to be cleared", i)
			break
		}
	}

	// Check bits after range are still set
	if !sfs.Get(200) {
		t.Error("Expected bit 200 to still be set")
	}
}

// TestSparseFixedBitSet_Or tests OR operation with another SparseFixedBitSet
func TestSparseFixedBitSet_Or(t *testing.T) {
	sfs1, _ := NewSparseFixedBitSet(10000)
	sfs2, _ := NewSparseFixedBitSet(10000)

	// Set bits in first set
	sfs1.Set(10)
	sfs1.Set(50)

	// Set bits in second set
	sfs2.Set(50)
	sfs2.Set(90)

	// OR the sets
	sfs1.Or(sfs2)

	// Check results
	if !sfs1.Get(10) {
		t.Error("Expected bit 10 to remain set")
	}
	if !sfs1.Get(50) {
		t.Error("Expected bit 50 to remain set")
	}
	if !sfs1.Get(90) {
		t.Error("Expected bit 90 to be set from OR")
	}
}

// TestSparseFixedBitSet_OrWithEmptySet tests OR with empty set
func TestSparseFixedBitSet_OrWithEmptySet(t *testing.T) {
	sfs1, _ := NewSparseFixedBitSet(100)
	sfs2, _ := NewSparseFixedBitSet(100)

	sfs1.Set(50)

	// OR with empty set
	sfs1.Or(sfs2)

	if !sfs1.Get(50) {
		t.Error("Expected bit 50 to remain set after OR with empty set")
	}

	if sfs1.Cardinality() != 1 {
		t.Errorf("Expected cardinality 1, got %d", sfs1.Cardinality())
	}
}

// TestSparseFixedBitSet_RamBytesUsed tests memory usage tracking
func TestSparseFixedBitSet_RamBytesUsed(t *testing.T) {
	size := 1000 + RandomIntN(10000)
	sfs, _ := NewSparseFixedBitSet(size)

	// Empty set should have some base memory usage
	initialRam := sfs.RamBytesUsed()
	if initialRam <= 0 {
		t.Error("Expected positive RAM bytes used for empty set")
	}

	// Set a few random bits
	for i := 0; i < 3; i++ {
		sfs.Set(RandomIntN(size))
	}
	ramAfterSparse := sfs.RamBytesUsed()
	if ramAfterSparse <= 0 {
		t.Error("Expected positive RAM bytes used after setting bits")
	}

	// Create another set and OR with sparse iterator
	sfs2, _ := NewSparseFixedBitSet(size)
	sfs2.Set(10)
	sfs2.Set(20)

	sfs3, _ := NewSparseFixedBitSet(size)
	for i := 0; i < size; i += 10 + RandomIntN(100) {
		sfs3.Set(i)
	}

	sfs2.Or(sfs3)

	if sfs2.RamBytesUsed() <= initialRam {
		t.Error("Expected increased RAM usage after OR with sparse set")
	}
}

// TestSparseFixedBitSet_SparseRepresentation tests the sparse representation invariants
func TestSparseFixedBitSet_SparseRepresentation(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(10000)

	// Set some sparse bits
	sfs.Set(100)
	sfs.Set(5000)
	sfs.Set(9999)

	// Verify internal invariants
	nonZeroLongCount := 0
	for i := 0; i < len(sfs.indices); i++ {
		n := bits.OnesCount64(sfs.indices[i])
		if n != 0 {
			nonZeroLongCount += n
			// Check that unused slots in bits array are zero
			for j := n; j < len(sfs.bits[i]); j++ {
				if sfs.bits[i][j] != 0 {
					t.Errorf("Expected zero at bits[%d][%d]", i, j)
				}
			}
		}
	}

	if nonZeroLongCount != sfs.nonZeroLongCount {
		t.Errorf("Expected nonZeroLongCount %d, got %d", nonZeroLongCount, sfs.nonZeroLongCount)
	}
}

// TestSparseFixedBitSet_LargeSparseSet tests operations on a large sparse set
func TestSparseFixedBitSet_LargeSparseSet(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(100000)

	// Set bits at various positions
	positions := []int{0, 100, 1000, 10000, 50000, 99999}
	for _, pos := range positions {
		sfs.Set(pos)
	}

	// Verify all positions
	for _, pos := range positions {
		if !sfs.Get(pos) {
			t.Errorf("Expected bit %d to be set", pos)
		}
	}

	// Check cardinality
	if sfs.Cardinality() != len(positions) {
		t.Errorf("Expected cardinality %d, got %d", len(positions), sfs.Cardinality())
	}

	// Test nextSetBit
	next := sfs.NextSetBit(0)
	if next != 0 {
		t.Errorf("Expected next set bit from 0 to be 0, got %d", next)
	}

	next = sfs.NextSetBit(101)
	if next != 1000 {
		t.Errorf("Expected next set bit from 101 to be 1000, got %d", next)
	}
}

// TestSparseFixedBitSet_BlockBoundaries tests behavior at 4096-bit block boundaries
func TestSparseFixedBitSet_BlockBoundaries(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(10000)

	// Set bits at block boundaries (4096 bits per block)
	sfs.Set(4095)  // Last bit of first block
	sfs.Set(4096)  // First bit of second block
	sfs.Set(8191)  // Last bit of second block
	sfs.Set(8192)  // First bit of third block

	// Verify all bits
	if !sfs.Get(4095) {
		t.Error("Expected bit 4095 to be set")
	}
	if !sfs.Get(4096) {
		t.Error("Expected bit 4096 to be set")
	}
	if !sfs.Get(8191) {
		t.Error("Expected bit 8191 to be set")
	}
	if !sfs.Get(8192) {
		t.Error("Expected bit 8192 to be set")
	}

	// Test nextSetBit across block boundary
	next := sfs.NextSetBit(4096)
	if next != 4096 {
		t.Errorf("Expected next set bit from 4096 to be 4096, got %d", next)
	}

	next = sfs.NextSetBit(4097)
	if next != 8191 {
		t.Errorf("Expected next set bit from 4097 to be 8191, got %d", next)
	}
}

// TestSparseFixedBitSet_RandomOperations tests random set/clear operations
func TestSparseFixedBitSet_RandomOperations(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(10000)
	expected := make(map[int]bool)

	// Perform random operations
	for i := 0; i < 1000; i++ {
		idx := RandomIntN(10000)
		if RandomBool() {
			sfs.Set(idx)
			expected[idx] = true
		} else {
			sfs.Clear(idx)
			delete(expected, idx)
		}
	}

	// Verify all bits
	for idx := range expected {
		if !sfs.Get(idx) {
			t.Errorf("Expected bit %d to be set", idx)
		}
	}

	// Check cardinality
	if sfs.Cardinality() != len(expected) {
		t.Errorf("Expected cardinality %d, got %d", len(expected), sfs.Cardinality())
	}
}

// TestSparseFixedBitSet_Clone tests cloning functionality
func TestSparseFixedBitSet_Clone(t *testing.T) {
	sfs1, _ := NewSparseFixedBitSet(10000)

	sfs1.Set(10)
	sfs1.Set(5000)
	sfs1.Set(9999)

	sfs2 := sfs1.Clone()

	// Verify clone has same bits
	if !sfs2.Get(10) || !sfs2.Get(5000) || !sfs2.Get(9999) {
		t.Error("Clone should have same bits set")
	}

	if sfs2.Cardinality() != sfs1.Cardinality() {
		t.Error("Clone should have same cardinality")
	}

	// Modify original
	sfs1.Set(100)
	if sfs2.Get(100) {
		t.Error("Clone should be independent of original")
	}

	// Modify clone
	sfs2.Set(200)
	if sfs1.Get(200) {
		t.Error("Original should be independent of clone")
	}
}

// TestSparseFixedBitSet_Equals tests equality comparison
func TestSparseFixedBitSet_Equals(t *testing.T) {
	sfs1, _ := NewSparseFixedBitSet(100)
	sfs2, _ := NewSparseFixedBitSet(100)

	// Empty sets should be equal
	if !sfs1.Equals(sfs2) {
		t.Error("Expected empty bitsets to be equal")
	}

	// Set same bits
	sfs1.Set(50)
	sfs2.Set(50)
	if !sfs1.Equals(sfs2) {
		t.Error("Expected bitsets with same bits to be equal")
	}

	// Set different bits
	sfs1.Set(60)
	if sfs1.Equals(sfs2) {
		t.Error("Expected bitsets with different bits to not be equal")
	}

	// Different sizes
	sfs3, _ := NewSparseFixedBitSet(50)
	if sfs1.Equals(sfs3) {
		t.Error("Expected different sizes to not be equal")
	}
}

// TestSparseFixedBitSet_DenseOperations tests operations when set becomes dense
func TestSparseFixedBitSet_DenseOperations(t *testing.T) {
	sfs, _ := NewSparseFixedBitSet(1000)

	// Set many bits to make it dense
	for i := 0; i < 1000; i += 2 {
		sfs.Set(i)
	}

	// Cardinality should be 500
	if sfs.Cardinality() != 500 {
		t.Errorf("Expected cardinality 500, got %d", sfs.Cardinality())
	}

	// Approximate cardinality - note: linear counting algorithm assumes uniform distribution
	// Setting every other bit creates a non-uniform pattern, so we just verify it's positive
	approx := sfs.ApproximateCardinality()
	if approx <= 0 {
		t.Error("Expected positive approximate cardinality")
	}

	// Test nextSetBit
	for i := 0; i < 1000; i += 2 {
		next := sfs.NextSetBit(i)
		if next != i {
			t.Errorf("Expected next set bit from %d to be %d, got %d", i, i, next)
			break
		}
	}
}

// Helper function
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Helper function for min
func testMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Ensure SparseFixedBitSet implements Bits interface
var _ Bits = (*SparseFixedBitSet)(nil)

// BenchmarkSparseFixedBitSet_Set benchmarks the Set operation
func BenchmarkSparseFixedBitSet_Set(b *testing.B) {
	sfs, _ := NewSparseFixedBitSet(100000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sfs.Set(i % 100000)
	}
}

// BenchmarkSparseFixedBitSet_Get benchmarks the Get operation
func BenchmarkSparseFixedBitSet_Get(b *testing.B) {
	sfs, _ := NewSparseFixedBitSet(100000)
	sfs.Set(50000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sfs.Get(i % 100000)
	}
}

// BenchmarkSparseFixedBitSet_Cardinality benchmarks the Cardinality operation
func BenchmarkSparseFixedBitSet_Cardinality(b *testing.B) {
	sfs, _ := NewSparseFixedBitSet(100000)
	for i := 0; i < 100000; i += 100 {
		sfs.Set(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sfs.Cardinality()
	}
}

// BenchmarkSparseFixedBitSet_NextSetBit benchmarks the NextSetBit operation
func BenchmarkSparseFixedBitSet_NextSetBit(b *testing.B) {
	sfs, _ := NewSparseFixedBitSet(100000)
	for i := 0; i < 100000; i += 100 {
		sfs.Set(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sfs.NextSetBit(i % 100000)
	}
}
