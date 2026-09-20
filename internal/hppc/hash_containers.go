package hppc

import (
	"math"
	"sync/atomic"
)

const (
	DefaultExpectedElements = 4
	DefaultLoadFactor       = 0.75
	MinLoadFactor           = 1.0 / 100.0
	MaxLoadFactor           = 99.0 / 100.0
	MinHashArrayLength      = 4
	MaxHashArrayLength      = 0x80000000 >> 1
)

var iterationSeed atomic.Int32

// LongIntHashMap is a map from int64 to int32.
type LongIntHashMap map[int64]int32

func IterationIncrement(seed int32) int32 {
	return 29 + ((seed & 7) << 1)
}

func NextBufferSize(arraySize, elements int, loadFactor float64) int {
	if arraySize == MaxHashArrayLength {
		panic(NewBufferAllocationException("Maximum array size exceeded for this load factor (elements: %d, load factor: %f)", elements, loadFactor))
	}
	return arraySize << 1
}

func ExpandAtCount(arraySize int, loadFactor float64) int {
	res := int(math.Ceil(float64(arraySize) * loadFactor))
	if res > arraySize-1 {
		return arraySize - 1
	}
	return res
}

func MinBufferSize(elements int, loadFactor float64) int {
	if elements < 0 {
		panic(NewBufferAllocationException("Number of elements must be >= 0: %d", elements))
	}

	length := math.Ceil(float64(elements) / loadFactor)
	if length == float64(elements) {
		length++
	}

	// nextHighestPowerOfTwo implementation
	l := int(length)
	if l < MinHashArrayLength {
		l = MinHashArrayLength
	} else {
		if (l & (l - 1)) != 0 {
			l = nextHighestPowerOfTwo(l)
		}
	}

	if l > MaxHashArrayLength {
		panic(NewBufferAllocationException("Maximum array size exceeded for this load factor (elements: %d, load factor: %f)", elements, loadFactor))
	}

	return l
}

func nextHighestPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32 // for 64-bit ints
	n++
	return n
}

func CheckLoadFactor(loadFactor, minAllowedInclusive, maxAllowedInclusive float64) {
	if loadFactor < minAllowedInclusive || loadFactor > maxAllowedInclusive {
		panic(NewBufferAllocationException("The load factor should be in range [%.2f, %.2f]: %f", minAllowedInclusive, maxAllowedInclusive, loadFactor))
	}
}
