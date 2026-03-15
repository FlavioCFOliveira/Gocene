// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/util/ArrayUtil.java
// Purpose: Methods for manipulating arrays including growth, parsing,
// sorting utilities, and selection algorithms.

package util

import (
	"encoding/binary"
	"math"
)

// Constants for array sizing
const (
	// NumBytesArrayHeader is the estimated header size for arrays in Go
	// This is an approximation based on typical Go slice overhead
	NumBytesArrayHeader = 24

	// MaxArrayLength is the maximum length for an array
	MaxArrayLength = math.MaxInt32 - NumBytesArrayHeader

	// NumBytesObjectRef is the size of a reference/pointer in bytes (8 on 64-bit)
	NumBytesObjectRef = 8
)

// Oversize returns an array size >= minTargetSize, generally over-allocating
// exponentially to achieve amortized linear-time cost as the array grows.
//
// This was originally borrowed from Python 2.4.2 listobject.c sources,
// but has now been substantially changed based on discussions from java-dev.
//
// minTargetSize: Minimum required value to be returned.
// bytesPerElement: Bytes used by each element of the array.
func Oversize(minTargetSize, bytesPerElement int) int {
	if minTargetSize < 0 {
	// catch usage that accidentally overflows int
	panic("invalid array size")
	}

	if minTargetSize == 0 {
	// wait until at least one element is requested
	return 0
	}

	if minTargetSize > MaxArrayLength {
	panic("requested array size exceeds maximum array length")
	}

	// asymptotic exponential growth by 1/8th, favors
	// spending a bit more CPU to not tie up too much wasted RAM:
	extra := minTargetSize >> 3

	if extra < 3 {
	// for very small arrays, where constant overhead of
	// realloc is presumably relatively high, we grow faster
	extra = 3
	}

	newSize := minTargetSize + extra

	// add 7 to allow for worst case byte alignment addition below:
	if newSize+7 < 0 || newSize+7 > MaxArrayLength {
	// int overflowed, or we exceeded the maximum array length
	return MaxArrayLength
	}

	// Round up to alignment based on bytesPerElement
	// In 64-bit Go, we align to 8 bytes
	return roundUpToAlignment(newSize, bytesPerElement)
}

// roundUpToAlignment rounds up size based on element size for memory alignment
func roundUpToAlignment(size, bytesPerElement int) int {
	switch bytesPerElement {
	case 4:
	// round up to multiple of 2
	return (size + 1) &^ 1
	case 2:
	// round up to multiple of 4
	return (size + 3) &^ 3
	case 1:
	// round up to multiple of 8
	return (size + 7) &^ 7
	case 8:
	// no rounding needed for 8-byte elements
	return size
	default:
	// odd (invalid?) size - return as-is
	return size
	}
}

// ==================== ParseInt Functions ====================

// ParseInt parses a rune array into an int with default radix 10.
// chars: the character array
// offset: The offset into the array
// len: The length
// Returns the int value or panics with a NumberFormatException equivalent
func ParseInt(chars []rune, offset, length int) int {
	return ParseIntRadix(chars, offset, length, 10)
}

// ParseIntRadix parses the rune array as an int value with the specified radix.
// chars: a string representation of an int quantity.
// radix: the base to use for conversion.
// Returns int the value represented by the argument
func ParseIntRadix(chars []rune, offset, length, radix int) int {
	if chars == nil || radix < 2 || radix > 36 {
	panic("NumberFormatException: invalid radix or null chars")
	}

	if length == 0 {
	panic("NumberFormatException: chars length is 0")
	}

	i := 0
	negative := chars[offset+i] == '-'
	if negative && i+1 == length {
	panic("NumberFormatException: can't convert to an int")
	}

	if negative {
	offset++
	length--
	}

	return parseIntInternal(chars, offset, length, radix, negative)
}

func parseIntInternal(chars []rune, offset, length, radix int, negative bool) int {
	max := math.MinInt32 / radix
	result := 0

	for i := 0; i < length; i++ {
	c := chars[i+offset]

	// Convert rune to digit
	var digit int
	if c >= '0' && c <= '9' {
		digit = int(c - '0')
	} else if c >= 'a' && c <= 'z' {
		digit = int(c-'a') + 10
	} else if c >= 'A' && c <= 'Z' {
		digit = int(c-'A') + 10
	} else {
		panic("NumberFormatException: Unable to parse")
	}

	if digit >= radix {
		panic("NumberFormatException: Unable to parse")
	}

	if max > result {
		panic("NumberFormatException: Unable to parse")
	}

	next := result*radix - digit
	if next > result {
		panic("NumberFormatException: Unable to parse")
	}
	result = next
	}

	if !negative {
	result = -result
	if result < 0 {
		panic("NumberFormatException: Unable to parse")
	}
	}

	return result
}

// ==================== GrowExact Functions ====================

// GrowExact returns a new array whose size is exactly the specified newLength
// without over-allocating. The original array elements are copied to the new array.
func GrowExact[T any](array []T, newLength int) []T {
	if newLength < len(array) {
	panic("IndexOutOfBoundsException: newLength must be >= len(array)")
	}
	copy_ := make([]T, newLength)
	copy(copy_, array)
	return copy_
}

// GrowExactInt16 returns a new int16 array of exactly newLength
func GrowExactInt16(array []int16, newLength int) []int16 {
	if newLength < len(array) {
	panic("IndexOutOfBoundsException")
	}
	copy_ := make([]int16, newLength)
	copy(copy_, array)
	return copy_
}

// GrowExactInt32 returns a new int32 array of exactly newLength
func GrowExactInt32(array []int32, newLength int) []int32 {
	if newLength < len(array) {
	panic("IndexOutOfBoundsException")
	}
	copy_ := make([]int32, newLength)
	copy(copy_, array)
	return copy_
}

// GrowExactInt64 returns a new int64 array of exactly newLength
func GrowExactInt64(array []int64, newLength int) []int64 {
	if newLength < len(array) {
	panic("IndexOutOfBoundsException")
	}
	copy_ := make([]int64, newLength)
	copy(copy_, array)
	return copy_
}

// GrowExactFloat32 returns a new float32 array of exactly newLength
func GrowExactFloat32(array []float32, newLength int) []float32 {
	if newLength < len(array) {
	panic("IndexOutOfBoundsException")
	}
	copy_ := make([]float32, newLength)
	copy(copy_, array)
	return copy_
}

// GrowExactFloat64 returns a new float64 array of exactly newLength
func GrowExactFloat64(array []float64, newLength int) []float64 {
	if newLength < len(array) {
	panic("IndexOutOfBoundsException")
	}
	copy_ := make([]float64, newLength)
	copy(copy_, array)
	return copy_
}

// GrowExactByte returns a new byte array of exactly newLength
func GrowExactByte(array []byte, newLength int) []byte {
	if newLength < len(array) {
	panic("IndexOutOfBoundsException")
	}
	copy_ := make([]byte, newLength)
	copy(copy_, array)
	return copy_
}

// ==================== GrowInRange Functions ====================

// GrowInRange returns an array whose size is at least minLength, generally
// over-allocating exponentially, but never allocating more than maxLength elements.
func GrowInRange(array []int, minLength, maxLength int) []int {
	if minLength < 0 {
	panic("AssertionError: length must be positive")
	}

	if minLength > maxLength {
	panic("IllegalArgumentException: minLength > maxLength")
	}

	if len(array) >= minLength {
	return array
	}

	potentialLength := Oversize(minLength, 4) // int is 4 bytes
	if potentialLength > maxLength {
	potentialLength = maxLength
	}

	return GrowExact(array, potentialLength)
}

// GrowInRangeFloat returns an array whose size is at least minLength but not over maxLength
func GrowInRangeFloat(array []float32, minLength, maxLength int) []float32 {
	if minLength < 0 {
	panic("AssertionError: minLength must be positive")
	}

	if minLength > maxLength {
	panic("IllegalArgumentException: minLength > maxLength")
	}

	if len(array) >= minLength {
	return array
	}

	potentialLength := Oversize(minLength, 4) // float32 is 4 bytes
	if potentialLength > maxLength {
	potentialLength = maxLength
	}

	return GrowExact(array, potentialLength)
}

// CopyOfSubArrayGeneric copies the specified range of the given slice into a new sub-slice.
// This is a generic version that works with any type.
// array: the input slice
// from: the initial index of range to be copied (inclusive)
// to: the final index of range to be copied (exclusive)
func CopyOfSubArrayGeneric[T any](array []T, from, to int) []T {
	if from < 0 || to > len(array) || from > to {
	panic("IndexOutOfBoundsException: invalid range")
	}
	result := make([]T, to-from)
	copy(result, array[from:to])
	return result
}

// ==================== Select Algorithm ====================

// Select reorganizes arr[from:to] so that the element at offset k is at the same
// position as if arr[from:to] was sorted, and all elements on its left are less
// than or equal to it, and all elements on its right are greater than or equal to it.
//
// This runs in linear time on average and in n log(n) time in the worst case.
//
// arr: Array to be re-organized.
// from: Starting index for re-organization.
// to: Ending index (exclusive).
// k: Index of element to sort from.
// comp: Comparison function
func Select[T any](arr []T, from, to, k int, comp func(a, b T) int) {
	if to-from <= 1 {
	return
	}

	// Use introselect algorithm (quickselect with fallback to heapsort)
	selectRecursive(arr, from, to, k, comp, 2*log2(to-from))
}

// log2 returns log base 2 of n
func log2(n int) int {
	result := 0
	for n > 1 {
	n >>= 1
	result++
	}
	return result
}

func selectRecursive[T any](arr []T, from, to, k int, comp func(a, b T) int, maxDepth int) {
	for {
	if from >= to {
		return
	}

	if to-from == 1 {
		if from == k {
		return
		}
		return
	}

	if maxDepth == 0 {
		// Fallback to sorting
		heapSortRange(arr, from, to, comp)
		return
	}

	maxDepth--

	// Partition
	pivotIndex := medianOfThree(arr, from, to, comp)
	pivotIndex = partition(arr, from, to, pivotIndex, comp)

	if k == pivotIndex {
		return
	} else if k < pivotIndex {
		to = pivotIndex
	} else {
		from = pivotIndex + 1
	}
	}
}

// medianOfThree finds the median of first, middle, and last elements
func medianOfThree[T any](arr []T, from, to int, comp func(a, b T) int) int {
	mid := from + (to-from)/2
	to--

	// Sort the three elements
	if comp(arr[from], arr[mid]) > 0 {
	arr[from], arr[mid] = arr[mid], arr[from]
	}
	if comp(arr[from], arr[to]) > 0 {
	arr[from], arr[to] = arr[to], arr[from]
	}
	if comp(arr[mid], arr[to]) > 0 {
	arr[mid], arr[to] = arr[to], arr[mid]
	}

	return mid
}

// partition partitions the array around the pivot
func partition[T any](arr []T, from, to, pivotIndex int, comp func(a, b T) int) int {
	pivotValue := arr[pivotIndex]
	arr[pivotIndex], arr[to-1] = arr[to-1], arr[pivotIndex]

	storeIndex := from
	for i := from; i < to-1; i++ {
	if comp(arr[i], pivotValue) < 0 {
		arr[storeIndex], arr[i] = arr[i], arr[storeIndex]
		storeIndex++
	}
	}
	arr[storeIndex], arr[to-1] = arr[to-1], arr[storeIndex]
	return storeIndex
}

// heapSortRange sorts a range using heapsort
func heapSortRange[T any](arr []T, from, to int, comp func(a, b T) int) {
	n := to - from

	// Build heap
	for i := n/2 - 1; i >= 0; i-- {
	heapify(arr, from, n, i, comp)
	}

	// Extract elements from heap one by one
	for i := n - 1; i > 0; i-- {
	arr[from], arr[from+i] = arr[from+i], arr[from]
	heapify(arr, from, i, 0, comp)
	}
}

// heapify maintains heap property
func heapify[T any](arr []T, offset, n, i int, comp func(a, b T) int) {
	largest := i
	left := 2*i + 1
	right := 2*i + 2

	if left < n && comp(arr[offset+left], arr[offset+largest]) > 0 {
	largest = left
	}

	if right < n && comp(arr[offset+right], arr[offset+largest]) > 0 {
	largest = right
	}

	if largest != i {
	arr[offset+i], arr[offset+largest] = arr[offset+largest], arr[offset+i]
	heapify(arr, offset, n, largest, comp)
	}
}

// ==================== CompareUnsigned Functions ====================

// CompareUnsigned4 compares exactly 4 unsigned bytes from the provided arrays.
// Returns negative if a < b, positive if a > b, 0 if equal.
func CompareUnsigned4(a []byte, aOffset int, b []byte, bOffset int) int {
	if len(a) < aOffset+4 || len(b) < bOffset+4 {
	panic("IndexOutOfBoundsException")
	}

	av := binary.BigEndian.Uint32(a[aOffset:])
	bv := binary.BigEndian.Uint32(b[bOffset:])

	if av < bv {
	return -1
	}
	if av > bv {
	return 1
	}
	return 0
}

// CompareUnsigned8 compares exactly 8 unsigned bytes from the provided arrays.
// Returns negative if a < b, positive if a > b, 0 if equal.
func CompareUnsigned8(a []byte, aOffset int, b []byte, bOffset int) int {
	if len(a) < aOffset+8 || len(b) < bOffset+8 {
	panic("IndexOutOfBoundsException")
	}

	av := binary.BigEndian.Uint64(a[aOffset:])
	bv := binary.BigEndian.Uint64(b[bOffset:])

	if av < bv {
	return -1
	}
	if av > bv {
	return 1
	}
	return 0
}

// Note: IntroSort, IntroSortOrdered, TimSort, and TimSortOrdered
// are defined in collection_util.go to avoid circular dependencies.
