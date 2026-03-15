// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/big"
	"testing"
)

// bitCount returns the number of set bits in a big.Int
func bitCount(x *big.Int) int {
	count := 0
	// Make a copy to avoid modifying the original
	temp := new(big.Int).Set(x)
	for temp.Sign() > 0 {
		if temp.Bit(0) != 0 {
			count++
		}
		temp.Rsh(temp, 1)
	}
	return count
}

// doGet tests that the various ways of accessing the bits are equivalent
func doGet(t *testing.T, a *big.Int, b *LongBitSet) {
	if int64(bitCount(a)) != b.Cardinality() {
		t.Errorf("cardinality mismatch: BitSet=%d, LongBitSet=%d", bitCount(a), b.Cardinality())
	}
	max := b.Length()
	for i := int64(0); i < max; i++ {
		bitA := a.Bit(int(i)) != 0
		bitB := b.Get(i)
		if bitA != bitB {
			t.Errorf("mismatch: BitSet[%d]=%v, LongBitSet[%d]=%v", i, bitA, i, bitB)
		}
	}
}

// doNextSetBit tests nextSetBit operations
func doNextSetBit(t *testing.T, a *big.Int, b *LongBitSet) {
	if int64(bitCount(a)) != b.Cardinality() {
		t.Errorf("cardinality mismatch: BitSet=%d, LongBitSet=%d", bitCount(a), b.Cardinality())
	}
	aa := -1
	bb := int64(-1)
	for {
		// Find next set bit in big.Int
		nextA := -1
		for i := aa + 1; i < int(b.Length()); i++ {
			if a.Bit(i) != 0 {
				nextA = i
				break
			}
		}
		aa = nextA

		// Find next set bit in LongBitSet
		if bb < b.Length()-1 {
			bb = b.NextSetBit(bb + 1)
		} else {
			bb = -1
		}

		if int64(aa) != bb {
			t.Errorf("nextSetBit mismatch: BitSet=%d, LongBitSet=%d", aa, bb)
		}
		if aa < 0 {
			break
		}
	}
}

// doPrevSetBit tests prevSetBit operations
func doPrevSetBit(t *testing.T, a *big.Int, b *LongBitSet) {
	if int64(bitCount(a)) != b.Cardinality() {
		t.Errorf("cardinality mismatch: BitSet=%d, LongBitSet=%d", bitCount(a), b.Cardinality())
	}
	aa := int(b.Length()) + RandomIntN(100)
	bb := int64(aa)
	for {
		// Find prev set bit in big.Int
		aa--
		for aa >= 0 && a.Bit(aa) == 0 {
			aa--
		}

		// Find prev set bit in LongBitSet
		if b.Length() == 0 {
			bb = -1
		} else if bb > b.Length()-1 {
			bb = b.PrevSetBit(b.Length() - 1)
		} else if bb < 1 {
			bb = -1
		} else {
			bb = b.PrevSetBit(bb - 1)
		}

		if int64(aa) != bb {
			t.Errorf("prevSetBit mismatch: BitSet=%d, LongBitSet=%d", aa, bb)
		}
		if aa < 0 {
			break
		}
	}
}

// doRandomSets performs random operations on bitsets and verifies consistency
func doRandomSets(t *testing.T, maxSize int, iter int) {
	var a0 *big.Int
	var b0 *LongBitSet

	for i := 0; i < iter; i++ {
		sz := RandomIntN(maxSize-2) + 2
		a := big.NewInt(0)
		b, err := NewLongBitSet(int64(sz))
		if err != nil {
			t.Fatalf("Failed to create LongBitSet: %v", err)
		}

		// Test the various ways of setting bits
		if sz > 0 {
			nOper := RandomIntN(sz)
			for j := 0; j < nOper; j++ {
				idx := RandomIntN(sz)
				a.SetBit(a, idx, 1)
				b.Set(int64(idx))

				idx = RandomIntN(sz)
				a.SetBit(a, idx, 0)
				b.Clear(int64(idx))

				idx = RandomIntN(sz)
				// Flip in big.Int
				if a.Bit(idx) == 0 {
					a.SetBit(a, idx, 1)
				} else {
					a.SetBit(a, idx, 0)
				}
				b.FlipSingle(int64(idx))

				idx = RandomIntN(sz)
				// Flip again
				if a.Bit(idx) == 0 {
					a.SetBit(a, idx, 1)
				} else {
					a.SetBit(a, idx, 0)
				}
				b.FlipSingle(int64(idx))

				// Test getAndSet
				val2 := b.Get(int64(idx))
				val := b.GetAndSet(int64(idx))
				if val2 != val {
					t.Error("getAndSet returned different value than get")
				}
				if !b.Get(int64(idx)) {
					t.Error("getAndSet should have set the bit")
				}
				a.SetBit(a, idx, 1)

				if !val {
					b.Clear(int64(idx))
					a.SetBit(a, idx, 0)
				}
				if b.Get(int64(idx)) != val {
					t.Error("clear/get mismatch")
				}
			}
		}

		// Test that the various ways of accessing the bits are equivalent
		doGet(t, a, b)

		// Test ranges, including possible extension
		fromIndex := RandomIntN(sz / 2)
		toIndex := fromIndex + RandomIntN(sz-fromIndex)
		aa := new(big.Int).Set(a)
		// Flip range in big.Int
		for k := fromIndex; k < toIndex; k++ {
			if aa.Bit(k) == 0 {
				aa.SetBit(aa, k, 1)
			} else {
				aa.SetBit(aa, k, 0)
			}
		}
		bb := b.Clone()
		bb.Flip(int64(fromIndex), int64(toIndex))

		fromIndex = RandomIntN(sz / 2)
		toIndex = fromIndex + RandomIntN(sz-fromIndex)
		aa = new(big.Int).Set(a)
		// Clear range in big.Int
		for k := fromIndex; k < toIndex; k++ {
			aa.SetBit(aa, k, 0)
		}
		bb = b.Clone()
		bb.ClearRange(int64(fromIndex), int64(toIndex))

		doNextSetBit(t, aa, bb)
		doPrevSetBit(t, aa, bb)

		fromIndex = RandomIntN(sz / 2)
		toIndex = fromIndex + RandomIntN(sz-fromIndex)
		aa = new(big.Int).Set(a)
		// Set range in big.Int
		for k := fromIndex; k < toIndex; k++ {
			aa.SetBit(aa, k, 1)
		}
		bb = b.Clone()
		bb.SetRange(int64(fromIndex), int64(toIndex))

		doNextSetBit(t, aa, bb)
		doPrevSetBit(t, aa, bb)

		if b0 != nil && b0.Length() <= b.Length() {
			if int64(bitCount(a)) != b.Cardinality() {
				t.Errorf("cardinality mismatch before ops: BitSet=%d, LongBitSet=%d", bitCount(a), b.Cardinality())
			}

			// AND
			aAnd := new(big.Int).Set(a)
			aAnd.And(aAnd, a0)
			bAnd := b.Clone()
			if !bAnd.Equals(b) {
				t.Error("clone should be equal")
			}
			bAnd.And(b0)

			// OR
			aOr := new(big.Int).Set(a)
			aOr.Or(aOr, a0)
			bOr := b.Clone()
			bOr.Or(b0)

			// XOR
			aXor := new(big.Int).Set(a)
			aXor.Xor(aXor, a0)
			bXor := b.Clone()
			bXor.Xor(b0)

			// AND NOT
			aAndNot := new(big.Int).Set(a)
			aAndNot.AndNot(aAndNot, a0)
			bAndNot := b.Clone()
			bAndNot.AndNot(b0)

			if int64(bitCount(a0)) != b0.Cardinality() {
				t.Errorf("b0 cardinality mismatch: BitSet=%d, LongBitSet=%d", bitCount(a0), b0.Cardinality())
			}
			if int64(bitCount(aOr)) != bOr.Cardinality() {
				t.Errorf("OR cardinality mismatch: BitSet=%d, LongBitSet=%d", bitCount(aOr), bOr.Cardinality())
			}
			if int64(bitCount(aAnd)) != bAnd.Cardinality() {
				t.Errorf("AND cardinality mismatch: BitSet=%d, LongBitSet=%d", bitCount(aAnd), bAnd.Cardinality())
			}
			if int64(bitCount(aXor)) != bXor.Cardinality() {
				t.Errorf("XOR cardinality mismatch: BitSet=%d, LongBitSet=%d", bitCount(aXor), bXor.Cardinality())
			}
			if int64(bitCount(aAndNot)) != bAndNot.Cardinality() {
				t.Errorf("AND NOT cardinality mismatch: BitSet=%d, LongBitSet=%d", bitCount(aAndNot), bAndNot.Cardinality())
			}
		}

		a0 = a
		b0 = b
	}
}

// TestLongBitSet_Small tests with small bitsets
func TestLongBitSet_Small(t *testing.T) {
	iter := 100
	doRandomSets(t, 1200, iter)
}

// TestLongBitSet_Equals tests equality
func TestLongBitSet_Equals(t *testing.T) {
	// This test can't handle numBits==0:
	numBits := RandomIntN(2000) + 1
	b1, _ := NewLongBitSet(int64(numBits))
	b2, _ := NewLongBitSet(int64(numBits))

	if !b1.Equals(b2) {
		t.Error("Expected empty bitsets to be equal")
	}
	if !b2.Equals(b1) {
		t.Error("Expected empty bitsets to be equal (reverse)")
	}

	for iter := 0; iter < 100; iter++ {
		idx := RandomIntN(numBits)
		if !b1.Get(int64(idx)) {
			b1.Set(int64(idx))
			if b1.Equals(b2) {
				t.Error("Expected different bitsets to not be equal")
			}
			if b2.Equals(b1) {
				t.Error("Expected different bitsets to not be equal (reverse)")
			}
			b2.Set(int64(idx))
			if !b1.Equals(b2) {
				t.Error("Expected identical bitsets to be equal")
			}
			if !b2.Equals(b1) {
				t.Error("Expected identical bitsets to be equal (reverse)")
			}
		}
	}

	// Try different type of object
	if b1.Equals("not a bitset") {
		t.Error("Expected not equal to string")
	}
}

// TestLongBitSet_HashCodeEquals tests hash code consistency with equals
func TestLongBitSet_HashCodeEquals(t *testing.T) {
	// This test can't handle numBits==0:
	numBits := RandomIntN(2000) + 1
	b1, _ := NewLongBitSet(int64(numBits))
	b2, _ := NewLongBitSet(int64(numBits))

	if !b1.Equals(b2) {
		t.Error("Expected empty bitsets to be equal")
	}
	if !b2.Equals(b1) {
		t.Error("Expected empty bitsets to be equal (reverse)")
	}

	for iter := 0; iter < 100; iter++ {
		idx := RandomIntN(numBits)
		if !b1.Get(int64(idx)) {
			b1.Set(int64(idx))
			if b1.Equals(b2) {
				t.Error("Expected different bitsets to not be equal")
			}
			if b1.HashCode() == b2.HashCode() {
				t.Error("Expected different hash codes for different bitsets")
			}
			b2.Set(int64(idx))
			if !b1.Equals(b2) {
				t.Error("Expected identical bitsets to be equal")
			}
			if b1.HashCode() != b2.HashCode() {
				t.Error("Expected identical hash codes for equal bitsets")
			}
		}
	}
}

// TestLongBitSet_TooLarge tests that creating a too-large bitset panics
func TestLongBitSet_TooLarge(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for too large bitset")
		}
	}()
	Bits2Words(MaxNumBits + 1)
}

// TestLongBitSet_NegativeNumBits tests that negative numBits panics
func TestLongBitSet_NegativeNumBits(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for negative numBits")
		}
	}()
	Bits2Words(-17)
}

// TestLongBitSet_SmallBitSets tests small bitsets (0-10 bits)
func TestLongBitSet_SmallBitSets(t *testing.T) {
	// Make sure size 0-10 bit sets are OK:
	for numBits := 0; numBits < 10; numBits++ {
		b1, err := NewLongBitSet(int64(numBits))
		if err != nil {
			t.Fatalf("Failed to create LongBitSet with %d bits: %v", numBits, err)
		}
		b2, _ := NewLongBitSet(int64(numBits))

		if !b1.Equals(b2) {
			t.Errorf("Expected empty bitsets to be equal for numBits=%d", numBits)
		}
		if b1.HashCode() != b2.HashCode() {
			t.Errorf("Expected empty bitsets to have same hash code for numBits=%d", numBits)
		}
		if b1.Cardinality() != 0 {
			t.Errorf("Expected cardinality 0 for numBits=%d, got %d", numBits, b1.Cardinality())
		}

		if numBits > 0 {
			b1.SetRange(0, int64(numBits))
			if int(b1.Cardinality()) != numBits {
				t.Errorf("Expected cardinality %d, got %d", numBits, b1.Cardinality())
			}
			b1.Flip(0, int64(numBits))
			if b1.Cardinality() != 0 {
				t.Errorf("Expected cardinality 0 after flip, got %d", b1.Cardinality())
			}
		}
	}
}

// makeLongBitSet creates a LongBitSet from an array of set bits
func makeLongBitSet(a []int, numBits int) *LongBitSet {
	var bs *LongBitSet
	if RandomBool() {
		bits2words := Bits2Words(int64(numBits))
		words := make([]uint64, bits2words+RandomIntN(100))
		bs, _ = NewLongBitSetFromBits(words, int64(numBits))
	} else {
		bs, _ = NewLongBitSet(int64(numBits))
	}
	for _, e := range a {
		bs.Set(int64(e))
	}
	return bs
}

// makeBitSet creates a big.Int from an array of set bits
func makeBitSet(a []int) *big.Int {
	bs := big.NewInt(0)
	for _, e := range a {
		bs.SetBit(bs, e, 1)
	}
	return bs
}

// checkPrevSetBitArray checks prevSetBit for a specific array
func checkPrevSetBitArray(t *testing.T, a []int, numBits int) {
	obs := makeLongBitSet(a, numBits)
	bs := makeBitSet(a)
	doPrevSetBit(t, bs, obs)
}

// TestLongBitSet_PrevSetBit tests prevSetBit
func TestLongBitSet_PrevSetBit(t *testing.T) {
	checkPrevSetBitArray(t, []int{}, 0)
	checkPrevSetBitArray(t, []int{0}, 1)
	checkPrevSetBitArray(t, []int{0, 2}, 3)
}

// checkNextSetBitArray checks nextSetBit for a specific array
func checkNextSetBitArray(t *testing.T, a []int, numBits int) {
	obs := makeLongBitSet(a, numBits)
	bs := makeBitSet(a)
	doNextSetBit(t, bs, obs)
}

// TestLongBitSet_NextSetBit tests nextSetBit
func TestLongBitSet_NextSetBit(t *testing.T) {
	setBits := make([]int, RandomIntN(1000))
	for i := range setBits {
		setBits[i] = RandomIntN(len(setBits))
	}
	checkNextSetBitArray(t, setBits, len(setBits)+RandomIntN(10))
	checkNextSetBitArray(t, []int{}, len(setBits)+RandomIntN(10))
}

// TestLongBitSet_EnsureCapacity tests ensureCapacity
func TestLongBitSet_EnsureCapacity(t *testing.T) {
	bits, _ := NewLongBitSet(5)
	bits.Set(1)
	bits.Set(4)

	newBits := EnsureCapacity(bits, 8) // grow within the word
	if !newBits.Get(1) {
		t.Error("Expected bit 1 to be set")
	}
	if !newBits.Get(4) {
		t.Error("Expected bit 4 to be set")
	}
	newBits.Clear(1)
	// we align to 64-bits, so even though it shouldn't have, it re-allocated a long[1]
	if !bits.Get(1) {
		t.Error("Original should still have bit 1 set")
	}
	if newBits.Get(1) {
		t.Error("New bits should not have bit 1 set after clear")
	}

	newBits.Set(1)
	newBits = EnsureCapacity(newBits, newBits.Length()-2) // reuse
	if !newBits.Get(1) {
		t.Error("Expected bit 1 to be set after reuse")
	}

	bits.Set(1)
	newBits = EnsureCapacity(bits, 72) // grow beyond one word
	if !newBits.Get(1) {
		t.Error("Expected bit 1 to be set after growing")
	}
	if !newBits.Get(4) {
		t.Error("Expected bit 4 to be set after growing")
	}
	newBits.Clear(1)
	// we grew the long[], so it's not shared
	if !bits.Get(1) {
		t.Error("Original should still have bit 1 set after grow")
	}
	if newBits.Get(1) {
		t.Error("New bits should not have bit 1 set after clear")
	}
}

// TestLongBitSet_Bits2Words tests bits2words calculation
func TestLongBitSet_Bits2Words(t *testing.T) {
	tests := []struct {
		numBits  int64
		expected int
	}{
		{0, 0},
		{1, 1},
		{63, 1},
		{64, 1},
		{65, 2},
		{127, 2},
		{128, 2},
		{129, 3},
	}

	for _, tc := range tests {
		result := Bits2Words(tc.numBits)
		if result != tc.expected {
			t.Errorf("Bits2Words(%d) = %d, expected %d", tc.numBits, result, tc.expected)
		}
	}

	// Test edge cases
	if Bits2Words(1<<31) != 1<<(31-6) {
		t.Errorf("Bits2Words(1<<31) = %d, expected %d", Bits2Words(1<<31), 1<<(31-6))
	}
	if Bits2Words((1<<31)+1) != (1<<(31-6))+1 {
		t.Errorf("Bits2Words((1<<31)+1) = %d, expected %d", Bits2Words((1<<31)+1), (1<<(31-6))+1)
	}

	// ensure the claimed max num_bits doesn't throw exc
	if Bits2Words(MaxNumBits) <= 0 {
		t.Error("Bits2Words(MaxNumBits) should be > 0")
	}
}

// TestLongBitSet_BasicOperations tests basic set/clear/get operations
func TestLongBitSet_BasicOperations(t *testing.T) {
	b, _ := NewLongBitSet(100)

	// Initially all bits should be clear
	for i := int64(0); i < 100; i++ {
		if b.Get(i) {
			t.Errorf("Expected bit %d to be clear initially", i)
		}
	}

	// Set some bits
	b.Set(0)
	b.Set(50)
	b.Set(99)

	if !b.Get(0) {
		t.Error("Expected bit 0 to be set")
	}
	if !b.Get(50) {
		t.Error("Expected bit 50 to be set")
	}
	if !b.Get(99) {
		t.Error("Expected bit 99 to be set")
	}

	// Clear a bit
	b.Clear(50)
	if b.Get(50) {
		t.Error("Expected bit 50 to be clear")
	}

	// Cardinality
	if b.Cardinality() != 2 {
		t.Errorf("Expected cardinality 2, got %d", b.Cardinality())
	}
}

// TestLongBitSet_GetAndSet tests GetAndSet operation
func TestLongBitSet_GetAndSet(t *testing.T) {
	b, _ := NewLongBitSet(100)

	// GetAndSet on clear bit should return false
	if b.GetAndSet(50) {
		t.Error("GetAndSet on clear bit should return false")
	}
	if !b.Get(50) {
		t.Error("Bit should be set after GetAndSet")
	}

	// GetAndSet on set bit should return true
	if !b.GetAndSet(50) {
		t.Error("GetAndSet on set bit should return true")
	}
}

// TestLongBitSet_GetAndClear tests GetAndClear operation
func TestLongBitSet_GetAndClear(t *testing.T) {
	b, _ := NewLongBitSet(100)
	b.Set(50)

	// GetAndClear on set bit should return true
	if !b.GetAndClear(50) {
		t.Error("GetAndClear on set bit should return true")
	}
	if b.Get(50) {
		t.Error("Bit should be clear after GetAndClear")
	}

	// GetAndClear on clear bit should return false
	if b.GetAndClear(50) {
		t.Error("GetAndClear on clear bit should return false")
	}
}

// TestLongBitSet_Intersects tests intersects operation
func TestLongBitSet_Intersects(t *testing.T) {
	b1, _ := NewLongBitSet(100)
	b2, _ := NewLongBitSet(100)

	// No intersection initially
	if b1.Intersects(b2) {
		t.Error("Expected no intersection for empty bitsets")
	}

	// Set different bits
	b1.Set(10)
	b2.Set(20)
	if b1.Intersects(b2) {
		t.Error("Expected no intersection for different bits")
	}

	// Set same bit
	b2.Set(10)
	if !b1.Intersects(b2) {
		t.Error("Expected intersection")
	}
}

// TestLongBitSet_ScanIsEmpty tests scanIsEmpty
func TestLongBitSet_ScanIsEmpty(t *testing.T) {
	b, _ := NewLongBitSet(100)

	if !b.ScanIsEmpty() {
		t.Error("Expected empty bitset")
	}

	b.Set(50)
	if b.ScanIsEmpty() {
		t.Error("Expected non-empty bitset")
	}

	b.Clear(50)
	if !b.ScanIsEmpty() {
		t.Error("Expected empty bitset after clear")
	}
}

// TestLongBitSet_Clone tests clone operation
func TestLongBitSet_Clone(t *testing.T) {
	b1, _ := NewLongBitSet(100)
	b1.Set(10)
	b1.Set(50)

	b2 := b1.Clone()

	if !b2.Get(10) || !b2.Get(50) {
		t.Error("Clone should have same bits set")
	}

	// Modify original
	b1.Set(90)
	if b2.Get(90) {
		t.Error("Clone should be independent of original")
	}
}

// TestLongBitSet_SetRange tests set range operation
func TestLongBitSet_SetRange(t *testing.T) {
	b, _ := NewLongBitSet(100)

	b.SetRange(10, 20)

	for i := int64(10); i < 20; i++ {
		if !b.Get(i) {
			t.Errorf("Expected bit %d to be set", i)
		}
	}
	if b.Get(9) {
		t.Error("Expected bit 9 to be clear")
	}
	if b.Get(20) {
		t.Error("Expected bit 20 to be clear")
	}
}

// TestLongBitSet_ClearRange tests clear range operation
func TestLongBitSet_ClearRange(t *testing.T) {
	b, _ := NewLongBitSet(100)
	b.SetRange(0, 50)

	b.ClearRange(10, 20)

	for i := int64(10); i < 20; i++ {
		if b.Get(i) {
			t.Errorf("Expected bit %d to be clear", i)
		}
	}
	if !b.Get(0) {
		t.Error("Expected bit 0 to still be set")
	}
	if !b.Get(30) {
		t.Error("Expected bit 30 to still be set")
	}
}

// TestLongBitSet_FlipRange tests flip range operation
func TestLongBitSet_FlipRange(t *testing.T) {
	b, _ := NewLongBitSet(100)
	b.SetRange(0, 30)

	b.Flip(10, 20)

	for i := int64(10); i < 20; i++ {
		if b.Get(i) {
			t.Errorf("Expected bit %d to be clear after flip", i)
		}
	}
	if !b.Get(0) {
		t.Error("Expected bit 0 to still be set")
	}
	if !b.Get(25) {
		t.Error("Expected bit 25 to still be set")
	}
}

// TestLongBitSet_LargeBitSet tests with a large bitset
func TestLongBitSet_LargeBitSet(t *testing.T) {
	// Test with a large bitset that spans multiple uint64 words
	b, err := NewLongBitSet(10000)
	if err != nil {
		t.Fatalf("Failed to create large bitset: %v", err)
	}

	// Set bits in different words
	b.Set(0)
	b.Set(63)
	b.Set(64)
	b.Set(127)
	b.Set(128)
	b.Set(9999)

	if !b.Get(0) {
		t.Error("Expected bit 0 to be set")
	}
	if !b.Get(63) {
		t.Error("Expected bit 63 to be set")
	}
	if !b.Get(64) {
		t.Error("Expected bit 64 to be set")
	}
	if !b.Get(9999) {
		t.Error("Expected bit 9999 to be set")
	}

	if b.Cardinality() != 6 {
		t.Errorf("Expected cardinality 6, got %d", b.Cardinality())
	}
}

// TestLongBitSet_NextSetBitEdgeCases tests nextSetBit edge cases
func TestLongBitSet_NextSetBitEdgeCases(t *testing.T) {
	b, _ := NewLongBitSet(100)

	// Empty bitset
	if b.NextSetBit(0) != -1 {
		t.Error("Expected -1 for empty bitset")
	}

	// Set last bit
	b.Set(99)
	if b.NextSetBit(0) != 99 {
		t.Errorf("Expected 99, got %d", b.NextSetBit(0))
	}
	if b.NextSetBit(99) != 99 {
		t.Errorf("Expected 99, got %d", b.NextSetBit(99))
	}
	if b.NextSetBit(100) != -1 {
		t.Errorf("Expected -1, got %d", b.NextSetBit(100))
	}
}

// TestLongBitSet_PrevSetBitEdgeCases tests prevSetBit edge cases
func TestLongBitSet_PrevSetBitEdgeCases(t *testing.T) {
	b, _ := NewLongBitSet(100)

	// Empty bitset
	if b.PrevSetBit(99) != -1 {
		t.Error("Expected -1 for empty bitset")
	}

	// Set first bit
	b.Set(0)
	if b.PrevSetBit(0) != 0 {
		t.Errorf("Expected 0, got %d", b.PrevSetBit(0))
	}
	if b.PrevSetBit(99) != 0 {
		t.Errorf("Expected 0, got %d", b.PrevSetBit(99))
	}
}
