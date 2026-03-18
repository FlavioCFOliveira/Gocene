// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/util/TestArrayUtil.java
// Purpose: Tests for ArrayUtil - array manipulation utilities including
// growth patterns, parsing, sorting, and selection algorithms.

package util

import (
	"cmp"
	"math"
	"math/rand"
	"slices"
	"testing"
	"time"
)

// ==================== Growth Pattern Tests ====================

// TestArrayUtil_Growth ensures ArrayUtil.Oversize gives linear amortized cost of realloc/copy
func TestArrayUtil_Growth(t *testing.T) {
	currentSize := 0
	copyCost := int64(0)

	// Make sure ArrayUtil hits MaxArrayLength, if we insist
	for currentSize != MaxArrayLength {
		nextSize := Oversize(1+currentSize, NumBytesObjectRef)
		if nextSize <= currentSize {
			t.Fatalf("Next size %d should be greater than current size %d", nextSize, currentSize)
		}
		if currentSize > 0 {
			copyCost += int64(currentSize)
			copyCostPerElement := float64(copyCost) / float64(currentSize)
			if copyCostPerElement >= 10.0 {
				t.Errorf("Copy cost per element %f should be less than 10.0", copyCostPerElement)
			}
		}
		currentSize = nextSize
	}
}

// TestArrayUtil_MaxSize tests max size limits
func TestArrayUtil_MaxSize(t *testing.T) {
	// Intentionally pass invalid elemSizes
	for elemSize := 0; elemSize < 10; elemSize++ {
		if Oversize(MaxArrayLength, elemSize) != MaxArrayLength {
			t.Errorf("Expected MaxArrayLength for elemSize %d when requesting MaxArrayLength", elemSize)
		}
		if Oversize(MaxArrayLength-1, elemSize) != MaxArrayLength {
			t.Errorf("Expected MaxArrayLength for elemSize %d when requesting MaxArrayLength-1", elemSize)
		}
	}
}

// TestArrayUtil_TooBig tests that oversize throws for sizes exceeding max
func TestArrayUtil_TooBig(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for size exceeding MaxArrayLength")
		}
	}()

	Oversize(MaxArrayLength+1, 1)
}

// TestArrayUtil_ExactLimit tests exact limit at MaxArrayLength
func TestArrayUtil_ExactLimit(t *testing.T) {
	result := Oversize(MaxArrayLength, 1)
	if result != MaxArrayLength {
		t.Errorf("Expected MaxArrayLength (%d), got %d", MaxArrayLength, result)
	}
}

// TestArrayUtil_InvalidElementSizes tests with random invalid element sizes
func TestArrayUtil_InvalidElementSizes(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	num := AtLeast(r, 10000)

	for i := 0; i < num; i++ {
		minTargetSize := r.Intn(MaxArrayLength)
		elemSize := r.Intn(11) // 0-10
		v := Oversize(minTargetSize, elemSize)
		if v < minTargetSize {
			t.Errorf("Result %d should be >= minTargetSize %d", v, minTargetSize)
		}
	}
}

// ==================== ParseInt Tests ====================

// parseIntTest helper that creates a char array with random padding
func parseIntTest(r *rand.Rand, s string) int {
	start := r.Intn(5)
	chars := make([]rune, len(s)+start+r.Intn(4))
	for i, c := range s {
		chars[start+i] = c
	}
	return ParseInt(chars, start, len(s))
}

// TestArrayUtil_ParseInt tests integer parsing from char arrays
func TestArrayUtil_ParseInt(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Test empty string
	func() {
		defer func() {
			if rec := recover(); rec == nil {
				t.Error("Expected panic for empty string")
			}
		}()
		parseIntTest(r, "")
	}()

	// Test invalid string "foo"
	func() {
		defer func() {
			if rec := recover(); rec == nil {
				t.Error("Expected panic for 'foo'")
			}
		}()
		parseIntTest(r, "foo")
	}()

	// Test overflow (Long.MAX_VALUE)
	func() {
		defer func() {
			if rec := recover(); rec == nil {
				t.Error("Expected panic for Long.MAX_VALUE")
			}
		}()
		parseIntTest(r, "9223372036854775807")
	}()

	// Test decimal "0.34"
	func() {
		defer func() {
			if rec := recover(); rec == nil {
				t.Error("Expected panic for '0.34'")
			}
		}()
		parseIntTest(r, "0.34")
	}()

	// Test valid values
	if parseIntTest(r, "1") != 1 {
		t.Error("Expected 1")
	}
	if parseIntTest(r, "-10000") != -10000 {
		t.Error("Expected -10000")
	}
	if parseIntTest(r, "1923") != 1923 {
		t.Error("Expected 1923")
	}
	if parseIntTest(r, "-1") != -1 {
		t.Error("Expected -1")
	}

	// Test with offset
	chars := []rune("foo 1923 bar")
	if ParseInt(chars, 4, 4) != 1923 {
		t.Error("Expected 1923 from offset")
	}
}

// TestArrayUtil_ParseIntRadix tests parsing with different radix
func TestArrayUtil_ParseIntRadix(t *testing.T) {
	// Test hex
	chars := []rune("FF")
	if ParseIntRadix(chars, 0, 2, 16) != 255 {
		t.Error("Expected 255 for hex FF")
	}

	// Test binary
	chars = []rune("1010")
	if ParseIntRadix(chars, 0, 4, 2) != 10 {
		t.Error("Expected 10 for binary 1010")
	}

	// Test octal
	chars = []rune("77")
	if ParseIntRadix(chars, 0, 2, 8) != 63 {
		t.Error("Expected 63 for octal 77")
	}
}

// ==================== Sorting Tests ====================

// createRandomArray creates a random Integer-like array for testing
func createRandomArray(r *rand.Rand, maxSize int) []int {
	size := r.Intn(maxSize) + 1
	a := make([]int, size)
	for i := range a {
		a[i] = r.Intn(size)
	}
	return a
}

// createSparseRandomArray creates a sparse random array (values 0-1)
func createSparseRandomArray(r *rand.Rand, maxSize int) []int {
	size := r.Intn(maxSize) + 1
	a := make([]int, size)
	for i := range a {
		a[i] = r.Intn(2)
	}
	return a
}

// TestArrayUtil_IntroSort tests introSort functionality
func TestArrayUtil_IntroSort(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	num := AtLeast(r, 50)

	for i := 0; i < num; i++ {
		a1 := createRandomArray(r, 2000)
		a2 := make([]int, len(a1))
		copy(a2, a1)

		IntroSortOrdered(a1)
		slices.Sort(a2)

		if !slices.Equal(a1, a2) {
			t.Error("IntroSort result doesn't match expected sorted order")
		}

		// Test with reverse order
		a1 = createRandomArray(r, 2000)
		a2 = make([]int, len(a1))
		copy(a2, a1)

		IntroSort(a1, func(a, b int) int {
			if a < b {
				return 1
			}
			if a > b {
				return -1
			}
			return 0
		})
		slices.SortFunc(a2, func(a, b int) int {
			if a < b {
				return 1
			}
			if a > b {
				return -1
			}
			return 0
		})

		if !slices.Equal(a1, a2) {
			t.Error("IntroSort reverse result doesn't match expected sorted order")
		}

		// Reverse back and sort again
		IntroSortOrdered(a1)
		slices.Sort(a2)
		if !slices.Equal(a1, a2) {
			t.Error("IntroSort second sort doesn't match expected sorted order")
		}
	}
}

// TestArrayUtil_QuickToHeapSortFallback tests LUCENE-3054 fix
// This is a test for stack overflow in worst cases
func TestArrayUtil_QuickToHeapSortFallback(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	num := AtLeast(r, 10)

	for i := 0; i < num; i++ {
		a1 := createSparseRandomArray(r, 40000)
		a2 := make([]int, len(a1))
		copy(a2, a1)

		IntroSortOrdered(a1)
		slices.Sort(a2)

		if !slices.Equal(a1, a2) {
			t.Error("QuickToHeapSortFallback result doesn't match expected sorted order")
		}
	}
}

// TestArrayUtil_TimSort tests timSort functionality
func TestArrayUtil_TimSort(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	num := AtLeast(r, 50)

	for i := 0; i < num; i++ {
		a1 := createRandomArray(r, 2000)
		a2 := make([]int, len(a1))
		copy(a2, a1)

		TimSortOrdered(a1)
		slices.Sort(a2)

		if !slices.Equal(a1, a2) {
			t.Error("TimSort result doesn't match expected sorted order")
		}

		// Test with reverse order
		a1 = createRandomArray(r, 2000)
		a2 = make([]int, len(a1))
		copy(a2, a1)

		TimSort(a1, func(a, b int) int {
			if a < b {
				return 1
			}
			if a > b {
				return -1
			}
			return 0
		})
		slices.SortFunc(a2, func(a, b int) int {
			if a < b {
				return 1
			}
			if a > b {
				return -1
			}
			return 0
		})

		if !slices.Equal(a1, a2) {
			t.Error("TimSort reverse result doesn't match expected sorted order")
		}

		// Reverse back and sort again
		TimSortOrdered(a1)
		slices.Sort(a2)
		if !slices.Equal(a1, a2) {
			t.Error("TimSort second sort doesn't match expected sorted order")
		}
	}
}

// sortItem represents an item with value and order for stability testing
type sortItem struct {
	val   int
	order int
}

// TestArrayUtil_TimSortStability verifies TimSort is stable
func TestArrayUtil_TimSortStability(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	items := make([]sortItem, 100)
	for i := range items {
		// Half of the items have value but same order
		// The other half has defined order, but no (-1) value
		equal := r.Intn(2) == 0
		if equal {
			items[i] = sortItem{val: i + 1, order: 0}
		} else {
			items[i] = sortItem{val: -1, order: r.Intn(1000) + 1}
		}
	}

	TimSort(items, func(a, b sortItem) int {
		if a.order < b.order {
			return -1
		}
		if a.order > b.order {
			return 1
		}
		return 0
	})

	last := items[0]
	for i := 1; i < len(items); i++ {
		act := items[i]
		if act.order == 0 {
			// Order of "equal" items should not be mixed up
			if act.val <= last.val {
				t.Errorf("TimSort stability: equal items out of order at index %d", i)
			}
		}
		if act.order < last.order {
			t.Errorf("TimSort stability: items not sorted by order at index %d", i)
		}
		last = act
	}
}

// TestArrayUtil_EmptyArraySort tests sorting empty arrays
func TestArrayUtil_EmptyArraySort(t *testing.T) {
	// Empty int slice
	emptyInt := []int{}
	IntroSortOrdered(emptyInt)
	TimSortOrdered(emptyInt)
	if len(emptyInt) != 0 {
		t.Error("Empty int array should remain empty")
	}

	// Empty string slice
	emptyStr := []string{}
	IntroSort(emptyStr, cmp.Compare[string])
	TimSort(emptyStr, cmp.Compare[string])
	if len(emptyStr) != 0 {
		t.Error("Empty string array should remain empty")
	}
}

// ==================== Select Algorithm Tests ====================

// TestArrayUtil_Select tests the select algorithm
func TestArrayUtil_Select(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for iter := 0; iter < 100; iter++ {
		doTestSelect(t, r)
	}
}

func doTestSelect(t *testing.T, r *rand.Rand) {
	from := r.Intn(5)
	to := from + 1 + r.Intn(10000)
	max := 0
	if r.Intn(2) == 0 {
		max = r.Intn(100)
	} else {
		max = r.Intn(100000)
	}

	arr := make([]int, from+to+r.Intn(5))
	for i := range arr {
		arr[i] = r.Intn(max + 1)
	}

	k := from + r.Intn(to-from)

	expected := make([]int, len(arr))
	copy(expected, arr)
	slices.Sort(expected[from:to])

	actual := make([]int, len(arr))
	copy(actual, arr)
	Select(actual, from, to, k, func(a, b int) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})

	if actual[k] != expected[k] {
		t.Errorf("Select: expected element at k=%d to be %d, got %d", k, expected[k], actual[k])
	}

	for i := 0; i < len(actual); i++ {
		if i < from || i >= to {
			// Elements outside range should be unchanged
			if actual[i] != arr[i] {
				t.Errorf("Select: element at index %d outside range was modified", i)
			}
		} else if i <= k {
			// Elements before k should be <= element at k
			if actual[i] > actual[k] {
				t.Errorf("Select: element at index %d (%d) should be <= element at k (%d)",
					i, actual[i], actual[k])
			}
		} else {
			// Elements after k should be >= element at k
			if actual[i] < actual[k] {
				t.Errorf("Select: element at index %d (%d) should be >= element at k (%d)",
					i, actual[i], actual[k])
			}
		}
	}
}

// ==================== GrowExact Tests ====================

// TestArrayUtil_GrowExact tests exact array growth
func TestArrayUtil_GrowExact(t *testing.T) {
	// Test short array
	shortArr := []int16{1, 2, 3}
	grownShort := GrowExact(shortArr, 4)
	if len(grownShort) != 4 || grownShort[0] != 1 || grownShort[1] != 2 || grownShort[2] != 3 || grownShort[3] != 0 {
		t.Error("GrowExact short array failed")
	}

	grownShort = GrowExact(shortArr, 5)
	if len(grownShort) != 5 || grownShort[3] != 0 || grownShort[4] != 0 {
		t.Error("GrowExact short array to 5 failed")
	}

	// Test int array
	intArr := []int32{1, 2, 3}
	grownInt := GrowExact(intArr, 4)
	if len(grownInt) != 4 || grownInt[0] != 1 || grownInt[1] != 2 || grownInt[2] != 3 || grownInt[3] != 0 {
		t.Error("GrowExact int array failed")
	}

	grownInt = GrowExact(intArr, 5)
	if len(grownInt) != 5 || grownInt[3] != 0 || grownInt[4] != 0 {
		t.Error("GrowExact int array to 5 failed")
	}

	// Test long array
	longArr := []int64{1, 2, 3}
	grownLong := GrowExact(longArr, 4)
	if len(grownLong) != 4 || grownLong[0] != 1 || grownLong[1] != 2 || grownLong[2] != 3 || grownLong[3] != 0 {
		t.Error("GrowExact long array failed")
	}

	grownLong = GrowExact(longArr, 5)
	if len(grownLong) != 5 || grownLong[3] != 0 || grownLong[4] != 0 {
		t.Error("GrowExact long array to 5 failed")
	}

	// Test float array
	floatArr := []float32{0.1, 0.2, 0.3}
	grownFloat := GrowExact(floatArr, 4)
	if len(grownFloat) != 4 || grownFloat[0] != 0.1 || grownFloat[1] != 0.2 || grownFloat[2] != 0.3 || grownFloat[3] != 0 {
		t.Error("GrowExact float array failed")
	}

	grownFloat = GrowExact(floatArr, 5)
	if len(grownFloat) != 5 || grownFloat[3] != 0 || grownFloat[4] != 0 {
		t.Error("GrowExact float array to 5 failed")
	}

	// Test double array
	doubleArr := []float64{0.1, 0.2, 0.3}
	grownDouble := GrowExact(doubleArr, 4)
	if len(grownDouble) != 4 || grownDouble[0] != 0.1 || grownDouble[1] != 0.2 || grownDouble[2] != 0.3 || grownDouble[3] != 0 {
		t.Error("GrowExact double array failed")
	}

	grownDouble = GrowExact(doubleArr, 5)
	if len(grownDouble) != 5 || grownDouble[3] != 0 || grownDouble[4] != 0 {
		t.Error("GrowExact double array to 5 failed")
	}

	// Test byte array
	byteArr := []byte{1, 2, 3}
	grownByte := GrowExact(byteArr, 4)
	if len(grownByte) != 4 || grownByte[0] != 1 || grownByte[1] != 2 || grownByte[2] != 3 || grownByte[3] != 0 {
		t.Error("GrowExact byte array failed")
	}

	grownByte = GrowExact(byteArr, 5)
	if len(grownByte) != 5 || grownByte[3] != 0 || grownByte[4] != 0 {
		t.Error("GrowExact byte array to 5 failed")
	}

	// Test string array
	strArr := []string{"a", "b", "c"}
	grownStr := GrowExact(strArr, 4)
	if len(grownStr) != 4 || grownStr[0] != "a" || grownStr[1] != "b" || grownStr[2] != "c" || grownStr[3] != "" {
		t.Error("GrowExact string array failed")
	}

	grownStr = GrowExact(strArr, 5)
	if len(grownStr) != 5 || grownStr[3] != "" || grownStr[4] != "" {
		t.Error("GrowExact string array to 5 failed")
	}
}

// TestArrayUtil_GrowExactPanics tests that GrowExact panics when newLength < len(array)
func TestArrayUtil_GrowExactPanics(t *testing.T) {
	// Test short array
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for GrowExact with newLength < length")
			}
		}()
		GrowExact([]int16{1, 2, 3}, 2)
	}()

	// Test int array
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for GrowExact with newLength < length")
			}
		}()
		GrowExact([]int32{1, 2, 3}, 2)
	}()
}

// ==================== GrowInRange Tests ====================

// TestArrayUtil_GrowInRange tests growInRange functionality
func TestArrayUtil_GrowInRange(t *testing.T) {
	array := []int{1, 2, 3}

	// If minLength is negative, should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for negative minLength")
			}
		}()
		GrowInRange(array, -1, 4)
	}()

	// If minLength > maxLength, should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for minLength > maxLength")
			}
		}()
		GrowInRange(array, 1, 0)
	}()

	// If minLength is sufficient, return same array
	result := GrowInRange(array, 1, 4)
	if len(result) != 3 {
		t.Error("Should return same array when minLength is sufficient")
	}

	// Test normal growth
	minLength := 4
	maxLength := math.MaxInt32
	result = GrowInRange([]int{1, 2, 3}, minLength, maxLength)
	if len(result) < minLength {
		t.Error("Grown array should have at least minLength elements")
	}

	// Test when maxLength is limiting
	result = GrowInRange([]int{1, 2, 3}, minLength, minLength)
	if len(result) != minLength {
		t.Errorf("Expected length %d when maxLength is limiting, got %d", minLength, len(result))
	}
}

// TestArrayUtil_GrowInRangeFloat tests growInRange for float arrays
func TestArrayUtil_GrowInRangeFloat(t *testing.T) {
	array := []float32{1, 2, 3}

	// If minLength is negative, should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for negative minLength")
			}
		}()
		GrowInRangeFloat(array, -1, 4)
	}()

	// If minLength > maxLength, should panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for minLength > maxLength")
			}
		}()
		GrowInRangeFloat(array, 1, 0)
	}()

	// If minLength is sufficient, return same array
	result := GrowInRangeFloat(array, 1, 4)
	if len(result) != 3 {
		t.Error("Should return same array when minLength is sufficient")
	}

	// Test normal growth
	minLength := 4
	maxLength := math.MaxInt32
	result = GrowInRangeFloat([]float32{1, 2, 3}, minLength, maxLength)
	if len(result) < minLength {
		t.Error("Grown array should have at least minLength elements")
	}

	// Test when maxLength is limiting
	result = GrowInRangeFloat([]float32{1, 2, 3}, minLength, minLength)
	if len(result) != minLength {
		t.Errorf("Expected length %d when maxLength is limiting, got %d", minLength, len(result))
	}
}

// ==================== CopyOfSubArray Tests ====================

// TestArrayUtil_CopyOfSubArray tests copyOfSubArray functionality
func TestArrayUtil_CopyOfSubArrayGeneric(t *testing.T) {
	// Test short array
	shortArray := []int16{1, 2, 3}
	result := CopyOfSubArrayGeneric(shortArray, 0, 1)
	if len(result) != 1 || result[0] != 1 {
		t.Error("CopyOfSubArray short failed")
	}

	result = CopyOfSubArrayGeneric(shortArray, 0, 3)
	if len(result) != 3 || result[0] != 1 || result[1] != 2 || result[2] != 3 {
		t.Error("CopyOfSubArray short full failed")
	}

	if len(CopyOfSubArrayGeneric(shortArray, 0, 0)) != 0 {
		t.Error("CopyOfSubArray short empty should return empty")
	}

	// Test int array
	intArray := []int32{1, 2, 3}
	resultInt := CopyOfSubArrayGeneric(intArray, 0, 2)
	if len(resultInt) != 2 || resultInt[0] != 1 || resultInt[1] != 2 {
		t.Error("CopyOfSubArray int failed")
	}

	resultInt = CopyOfSubArrayGeneric(intArray, 0, 3)
	if len(resultInt) != 3 {
		t.Error("CopyOfSubArray int full failed")
	}

	if len(CopyOfSubArrayGeneric(intArray, 1, 1)) != 0 {
		t.Error("CopyOfSubArray int empty should return empty")
	}

	// Test long array
	longArray := []int64{1, 2, 3}
	resultLong := CopyOfSubArrayGeneric(longArray, 1, 2)
	if len(resultLong) != 1 || resultLong[0] != 2 {
		t.Error("CopyOfSubArray long failed")
	}

	resultLong = CopyOfSubArrayGeneric(longArray, 0, 3)
	if len(resultLong) != 3 {
		t.Error("CopyOfSubArray long full failed")
	}

	// Test float array
	floatArray := []float32{0.1, 0.2, 0.3}
	resultFloat := CopyOfSubArrayGeneric(floatArray, 1, 3)
	if len(resultFloat) != 2 || resultFloat[0] != 0.2 || resultFloat[1] != 0.3 {
		t.Error("CopyOfSubArray float failed")
	}

	// Test double array
	doubleArray := []float64{0.1, 0.2, 0.3}
	resultDouble := CopyOfSubArrayGeneric(doubleArray, 2, 3)
	if len(resultDouble) != 1 || resultDouble[0] != 0.3 {
		t.Error("CopyOfSubArray double failed")
	}

	// Test byte array
	byteArray := []byte{1, 2, 3}
	resultByte := CopyOfSubArrayGeneric(byteArray, 0, 1)
	if len(resultByte) != 1 || resultByte[0] != 1 {
		t.Error("CopyOfSubArray byte failed")
	}

	// Test string array
	strArray := []string{"a1", "b2", "c3"}
	resultStr := CopyOfSubArrayGeneric(strArray, 0, 1)
	if len(resultStr) != 1 || resultStr[0] != "a1" {
		t.Error("CopyOfSubArray string failed")
	}
}

// TestArrayUtil_CopyOfSubArrayPanics tests that CopyOfSubArray panics on invalid range
func TestArrayUtil_CopyOfSubArrayPanics(t *testing.T) {
	shortArray := []int16{1, 2, 3}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for out of bounds")
			}
		}()
		CopyOfSubArrayGeneric(shortArray, 0, 4)
	}()
}

// ==================== CompareUnsigned Tests ====================

// TestArrayUtil_CompareUnsigned4 tests compareUnsigned4
func TestArrayUtil_CompareUnsigned4(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	aOffset := r.Intn(4)
	a := make([]byte, 4+aOffset)
	bOffset := r.Intn(4)
	b := make([]byte, 4+bOffset)

	for i := 0; i < 4; i++ {
		a[aOffset+i] = byte(r.Intn(256))
		for {
			b[bOffset+i] = byte(r.Intn(256))
			if b[bOffset+i] != a[aOffset+i] {
				break
			}
		}
	}

	for i := 0; i < 4; i++ {
		expected := compareUnsignedBytes(a, aOffset, aOffset+4, b, bOffset, bOffset+4)
		actual := CompareUnsigned4(a, aOffset, b, bOffset)

		if Signum(expected) != Signum(actual) {
			t.Errorf("CompareUnsigned4: expected sign %d, got %d", Signum(expected), Signum(actual))
		}

		b[bOffset+i] = a[aOffset+i]
	}

	if CompareUnsigned4(a, aOffset, b, bOffset) != 0 {
		t.Error("CompareUnsigned4 should return 0 for equal arrays")
	}
}

// TestArrayUtil_CompareUnsigned8 tests compareUnsigned8
func TestArrayUtil_CompareUnsigned8(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	aOffset := r.Intn(8)
	a := make([]byte, 8+aOffset)
	bOffset := r.Intn(8)
	b := make([]byte, 8+bOffset)

	for i := 0; i < 8; i++ {
		a[aOffset+i] = byte(r.Intn(256))
		for {
			b[bOffset+i] = byte(r.Intn(256))
			if b[bOffset+i] != a[aOffset+i] {
				break
			}
		}
	}

	for i := 0; i < 8; i++ {
		expected := compareUnsignedBytes(a, aOffset, aOffset+8, b, bOffset, bOffset+8)
		actual := CompareUnsigned8(a, aOffset, b, bOffset)

		if Signum(expected) != Signum(actual) {
			t.Errorf("CompareUnsigned8: expected sign %d, got %d", Signum(expected), Signum(actual))
		}

		b[bOffset+i] = a[aOffset+i]
	}

	if CompareUnsigned8(a, aOffset, b, bOffset) != 0 {
		t.Error("CompareUnsigned8 should return 0 for equal arrays")
	}
}

// ==================== Helper Functions (package-local) ====================

// NextInt returns a random int in [min, max]
func NextInt(r *rand.Rand, min, max int) int {
	if min >= max {
		return min
	}
	return min + r.Intn(max-min+1)
}

// Signum returns the sign of an integer
func Signum(x int) int {
	if x < 0 {
		return -1
	}
	if x > 0 {
		return 1
	}
	return 0
}

// compareUnsignedBytes compares two byte ranges as unsigned
func compareUnsignedBytes(a []byte, aFrom, aTo int, b []byte, bFrom, bTo int) int {
	alen := aTo - aFrom
	blen := bTo - bFrom
	minLen := alen
	if blen < minLen {
		minLen = blen
	}

	for i := 0; i < minLen; i++ {
		au := a[aFrom+i]
		bu := b[bFrom+i]
		if au != bu {
			if au < bu {
				return -1
			}
			return 1
		}
	}

	if alen < blen {
		return -1
	}
	if alen > blen {
		return 1
	}
	return 0
}
