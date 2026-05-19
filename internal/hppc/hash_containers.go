// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hppc

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Constants for primitive maps. Mirror of
// org.apache.lucene.internal.hppc.HashContainers.

// DefaultExpectedElements is the default initial element-count hint used by
// HPPC hash containers when none is supplied.
const DefaultExpectedElements = 4

// DefaultLoadFactor is the default load factor for HPPC hash containers.
const DefaultLoadFactor float64 = 0.75

// MinLoadFactor is the minimal sane load factor (99 empty slots per 100).
const MinLoadFactor float64 = 1.0 / 100.0

// MaxLoadFactor is the maximum sane load factor (1 empty slot per 100).
const MaxLoadFactor float64 = 99.0 / 100.0

// MinHashArrayLength is the minimum hash buffer size.
const MinHashArrayLength = 4

// MaxHashArrayLength is the maximum array size for hash containers
// (power-of-two and still allocable as a non-negative int32 in the JVM).
// Equivalent to Java's 0x80000000 >>> 1, i.e. 1 << 30.
const MaxHashArrayLength = 1 << 30

// iterationSeed is the process-wide counter used to derive per-iteration
// shuffling seeds. It mirrors HPPC's AtomicInteger.
var iterationSeed atomic.Int32

// NextIterationSeed returns the next seed for iteration shuffling.
// It mirrors HashContainers.ITERATION_SEED.incrementAndGet().
func NextIterationSeed() int32 {
	return iterationSeed.Add(1)
}

// IterationIncrement returns the small odd integer used as iteration step
// for a given seed. Mirrors HashContainers.iterationIncrement.
func IterationIncrement(seed int32) int32 {
	return 29 + ((seed & 7) << 1)
}

// NextBufferSize doubles arraySize, returning a BufferAllocationException
// when MaxHashArrayLength has already been reached. arraySize must be a
// power of two greater than one.
func NextBufferSize(arraySize, elements int, loadFactor float64) (int, error) {
	if err := checkPowerOfTwo(arraySize); err != nil {
		return 0, err
	}
	if arraySize == MaxHashArrayLength {
		return 0, &BufferAllocationException{
			Message: fmt.Sprintf(
				"Maximum array size exceeded for this load factor (elements: %d, load factor: %f)",
				elements, loadFactor,
			),
		}
	}
	return arraySize << 1, nil
}

// ExpandAtCount returns the element count at which the buffer of size
// arraySize must be expanded to preserve the hash-container invariant
// (at least one empty slot). arraySize must be a power of two greater
// than one.
func ExpandAtCount(arraySize int, loadFactor float64) int {
	// checkPowerOfTwo mirrors the Java assert. We swallow the error here
	// because callers are internal and supply validated power-of-two
	// sizes; an invalid argument is a programmer bug.
	_ = checkPowerOfTwo(arraySize)
	// Hash-container invariant: at least one empty slot must exist so the
	// lookup loop terminates on either the element or an empty slot.
	ceiling := int(math.Ceil(float64(arraySize) * loadFactor))
	if minusOne := arraySize - 1; minusOne < ceiling {
		return minusOne
	}
	return ceiling
}

// MinBufferSize returns the minimum power-of-two buffer size able to hold
// elements at loadFactor without violating the hash-container invariant.
func MinBufferSize(elements int, loadFactor float64) (int, error) {
	if elements < 0 {
		return 0, fmt.Errorf("number of elements must be >= 0: %d", elements)
	}
	length := int64(math.Ceil(float64(elements) / loadFactor))
	if length == int64(elements) {
		length++
	}
	length = util.NextHighestPowerOfTwoInt64(length)
	if length < MinHashArrayLength {
		length = MinHashArrayLength
	}
	if length > MaxHashArrayLength {
		return 0, &BufferAllocationException{
			Message: fmt.Sprintf(
				"Maximum array size exceeded for this load factor (elements: %d, load factor: %f)",
				elements, loadFactor,
			),
		}
	}
	return int(length), nil
}

// CheckLoadFactor verifies that loadFactor falls within
// [minAllowedInclusive, maxAllowedInclusive], returning a
// BufferAllocationException otherwise.
func CheckLoadFactor(loadFactor, minAllowedInclusive, maxAllowedInclusive float64) error {
	if loadFactor < minAllowedInclusive || loadFactor > maxAllowedInclusive {
		return &BufferAllocationException{
			Message: fmt.Sprintf(
				"The load factor should be in range [%.2f, %.2f]: %f",
				minAllowedInclusive, maxAllowedInclusive, loadFactor,
			),
		}
	}
	return nil
}

// checkPowerOfTwo replicates the Java assert: arraySize must be strictly
// greater than one and a power of two.
func checkPowerOfTwo(arraySize int) error {
	if arraySize <= 1 {
		return fmt.Errorf("arraySize must be > 1: %d", arraySize)
	}
	if util.NextHighestPowerOfTwo(arraySize) != arraySize {
		return fmt.Errorf("arraySize must be a power of two: %d", arraySize)
	}
	return nil
}
