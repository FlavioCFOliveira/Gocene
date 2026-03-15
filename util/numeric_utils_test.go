// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math"
	"math/big"
	"testing"
)

// float64Equal compares two float64 values for equality, handling NaN
func float64Equal(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return a == b
}

// compareFloat64 compares two float64 values: -1 if a < b, 0 if a == b, 1 if a > b
func compareFloat64(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// TestLongConversionAndOrdering generates a series of encoded longs, each numerical one bigger
// than the one before. Checks for correct ordering of the encoded bytes and that values round-trip.
func TestLongConversionAndOrdering(t *testing.T) {
	var previous *BytesRef
	current := &BytesRef{
		Bytes:  make([]byte, 8), // Long.BYTES = 8
		Offset: 0,
		Length: 8,
	}

	for value := int64(-100000); value < 100000; value++ {
		LongToSortableBytes(value, current.Bytes, current.Offset)

		if previous == nil {
			previous = &BytesRef{
				Bytes:  make([]byte, 8),
				Offset: 0,
				Length: 8,
			}
		} else {
			// Test if current is bigger than previous
			if BytesRefCompare(previous, current) >= 0 {
				t.Errorf("current should be bigger than previous at value %d", value)
			}
		}

		// Test if forward and back conversion works
		decoded := SortableBytesToLong(current.Bytes, current.Offset)
		if decoded != value {
			t.Errorf("forward and back conversion should generate same long: expected %d, got %d", value, decoded)
		}

		// Next step: copy current to previous
		copy(previous.Bytes[previous.Offset:], current.Bytes[current.Offset:current.Offset+current.Length])
	}
}

// TestIntConversionAndOrdering generates a series of encoded ints, each numerical one bigger
// than the one before. Checks for correct ordering of the encoded bytes and that values round-trip.
func TestIntConversionAndOrdering(t *testing.T) {
	var previous *BytesRef
	current := &BytesRef{
		Bytes:  make([]byte, 4), // Integer.BYTES = 4
		Offset: 0,
		Length: 4,
	}

	for value := int32(-100000); value < 100000; value++ {
		IntToSortableBytes(value, current.Bytes, current.Offset)

		if previous == nil {
			previous = &BytesRef{
				Bytes:  make([]byte, 4),
				Offset: 0,
				Length: 4,
			}
		} else {
			// Test if current is bigger than previous
			if BytesRefCompare(previous, current) >= 0 {
				t.Errorf("current should be bigger than previous at value %d", value)
			}
		}

		// Test if forward and back conversion works
		decoded := SortableBytesToInt(current.Bytes, current.Offset)
		if decoded != value {
			t.Errorf("forward and back conversion should generate same int: expected %d, got %d", value, decoded)
		}

		// Next step: copy current to previous
		copy(previous.Bytes[previous.Offset:], current.Bytes[current.Offset:current.Offset+current.Length])
	}
}

// TestBigIntConversionAndOrdering generates a series of encoded BigIntegers, each numerical one bigger
// than the one before. Checks for correct ordering of the encoded bytes and that values round-trip.
func TestBigIntConversionAndOrdering(t *testing.T) {
	// We need at least 3 bytes of storage
	size := 8 // Using 8 bytes for testing
	var previous *BytesRef
	current := &BytesRef{
		Bytes:  make([]byte, size),
		Offset: 0,
		Length: size,
	}

	for value := int64(-100000); value < 100000; value++ {
		bigInt := big.NewInt(value)
		err := BigIntToSortableBytes(bigInt, size, current.Bytes, current.Offset)
		if err != nil {
			t.Fatalf("Failed to encode BigInt %d: %v", value, err)
		}

		if previous == nil {
			previous = &BytesRef{
				Bytes:  make([]byte, size),
				Offset: 0,
				Length: size,
			}
		} else {
			// Test if current is bigger than previous
			if BytesRefCompare(previous, current) >= 0 {
				t.Errorf("current should be bigger than previous at value %d", value)
			}
		}

		// Test if forward and back conversion works
		decoded := SortableBytesToBigInt(current.Bytes, current.Offset, current.Length)
		if decoded.Cmp(bigInt) != 0 {
			t.Errorf("forward and back conversion should generate same BigInteger: expected %v, got %v", bigInt, decoded)
		}

		// Next step: copy current to previous
		copy(previous.Bytes[previous.Offset:], current.Bytes[current.Offset:current.Offset+current.Length])
	}
}

// TestLongSpecialValues checks extreme values of longs for correct ordering
// of the encoded bytes and that values round-trip.
func TestLongSpecialValues(t *testing.T) {
	values := []int64{
		math.MinInt64,
		math.MinInt64 + 1,
		math.MinInt64 + 2,
		-5003400000000,
		-4000,
		-3000,
		-2000,
		-1000,
		-1,
		0,
		1,
		10,
		300,
		50006789999999999,
		math.MaxInt64 - 2,
		math.MaxInt64 - 1,
		math.MaxInt64,
	}

	encoded := make([]*BytesRef, len(values))

	for i, value := range values {
		encoded[i] = &BytesRef{
			Bytes:  make([]byte, 8),
			Offset: 0,
			Length: 8,
		}
		LongToSortableBytes(value, encoded[i].Bytes, encoded[i].Offset)

		// Check forward and back conversion
		decoded := SortableBytesToLong(encoded[i].Bytes, encoded[i].Offset)
		if decoded != value {
			t.Errorf("forward and back conversion should generate same long at index %d: expected %d, got %d", i, value, decoded)
		}
	}

	// Check sort order (encoded values should be ascending)
	for i := 1; i < len(encoded); i++ {
		if BytesRefCompare(encoded[i-1], encoded[i]) >= 0 {
			t.Errorf("check sort order failed at index %d: previous should be less than current", i)
		}
	}
}

// TestIntSpecialValues checks extreme values of ints for correct ordering
// of the encoded bytes and that values round-trip.
func TestIntSpecialValues(t *testing.T) {
	values := []int32{
		math.MinInt32,
		math.MinInt32 + 1,
		math.MinInt32 + 2,
		-64765767,
		-4000,
		-3000,
		-2000,
		-1000,
		-1,
		0,
		1,
		10,
		300,
		765878989,
		math.MaxInt32 - 2,
		math.MaxInt32 - 1,
		math.MaxInt32,
	}

	encoded := make([]*BytesRef, len(values))

	for i, value := range values {
		encoded[i] = &BytesRef{
			Bytes:  make([]byte, 4),
			Offset: 0,
			Length: 4,
		}
		IntToSortableBytes(value, encoded[i].Bytes, encoded[i].Offset)

		// Check forward and back conversion
		decoded := SortableBytesToInt(encoded[i].Bytes, encoded[i].Offset)
		if decoded != value {
			t.Errorf("forward and back conversion should generate same int at index %d: expected %d, got %d", i, value, decoded)
		}
	}

	// Check sort order (encoded values should be ascending)
	for i := 1; i < len(encoded); i++ {
		if BytesRefCompare(encoded[i-1], encoded[i]) >= 0 {
			t.Errorf("check sort order failed at index %d: previous should be less than current", i)
		}
	}
}

// TestBigIntSpecialValues checks extreme values of big integers (4 bytes) for correct ordering
// of the encoded bytes and that values round-trip.
func TestBigIntSpecialValues(t *testing.T) {
	values := []*big.Int{
		big.NewInt(math.MinInt32),
		big.NewInt(math.MinInt32 + 1),
		big.NewInt(math.MinInt32 + 2),
		big.NewInt(-64765767),
		big.NewInt(-4000),
		big.NewInt(-3000),
		big.NewInt(-2000),
		big.NewInt(-1000),
		big.NewInt(-1),
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(10),
		big.NewInt(300),
		big.NewInt(765878989),
		big.NewInt(math.MaxInt32 - 2),
		big.NewInt(math.MaxInt32 - 1),
		big.NewInt(math.MaxInt32),
	}

	encoded := make([]*BytesRef, len(values))

	for i, value := range values {
		encoded[i] = &BytesRef{
			Bytes:  make([]byte, 4),
			Offset: 0,
			Length: 4,
		}
		err := BigIntToSortableBytes(value, 4, encoded[i].Bytes, encoded[i].Offset)
		if err != nil {
			t.Fatalf("Failed to encode BigInt at index %d: %v", i, err)
		}

		// Check forward and back conversion
		decoded := SortableBytesToBigInt(encoded[i].Bytes, encoded[i].Offset, 4)
		if decoded.Cmp(value) != 0 {
			t.Errorf("forward and back conversion should generate same big integer at index %d: expected %v, got %v", i, value, decoded)
		}
	}

	// Check sort order (encoded values should be ascending)
	for i := 1; i < len(encoded); i++ {
		if BytesRefCompare(encoded[i-1], encoded[i]) >= 0 {
			t.Errorf("check sort order failed at index %d: previous should be less than current", i)
		}
	}
}

// TestDoubles checks various sorted values of doubles (including extreme values)
// for correct ordering of the encoded bytes and that values round-trip.
func TestDoubles(t *testing.T) {
	values := []float64{
		math.Inf(-1),
		-2.3e25,
		-1.0e15,
		-1.0,
		-1.0e-1,
		-1.0e-2,
		math.Copysign(0, -1), // -0.0
		0.0,
		1.0e-2,
		1.0e-1,
		1.0,
		1.0e15,
		2.3e25,
		math.Inf(1),
		math.NaN(),
	}

	encoded := make([]int64, len(values))

	// Check forward and back conversion
	for i, value := range values {
		encoded[i] = DoubleToSortableLong(value)
		decoded := SortableLongToDouble(encoded[i])
		if !float64Equal(value, decoded) {
			t.Errorf("forward and back conversion should generate same double at index %d: expected %v, got %v", i, value, decoded)
		}
	}

	// Check sort order (encoded values should be ascending)
	for i := 1; i < len(encoded); i++ {
		if encoded[i-1] >= encoded[i] {
			t.Errorf("check sort order failed at index %d: previous should be less than current", i)
		}
	}
}

// doubleNaNs contains various NaN representations
var doubleNaNs = []float64{
	math.NaN(),
	math.Float64frombits(0x7ff0000000000001),
	math.Float64frombits(0x7fffffffffffffff),
	math.Float64frombits(0xfff0000000000001),
	math.Float64frombits(0xffffffffffffffff),
}

// TestSortableDoubleNaN checks that all NaN values sort after positive infinity.
func TestSortableDoubleNaN(t *testing.T) {
	plusInf := DoubleToSortableLong(math.Inf(1))
	for _, nan := range doubleNaNs {
		if !math.IsNaN(nan) {
			t.Errorf("Expected NaN, got %v", nan)
		}
		sortable := DoubleToSortableLong(nan)
		if sortable <= plusInf {
			t.Errorf("Double not sorted correctly: %v, long repr: %d, positive inf.: %d", nan, sortable, plusInf)
		}
	}
}

// TestFloats checks various sorted values of floats (including extreme values)
// for correct ordering of the encoded bytes and that values round-trip.
func TestFloats(t *testing.T) {
	values := []float32{
		float32(math.Inf(-1)),
		-2.3e25,
		-1.0e15,
		-1.0,
		-1.0e-1,
		-1.0e-2,
		float32(math.Copysign(0, -1)), // -0.0
		0.0,
		1.0e-2,
		1.0e-1,
		1.0,
		1.0e15,
		2.3e25,
		float32(math.Inf(1)),
		float32(math.NaN()),
	}

	encoded := make([]int32, len(values))

	// Check forward and back conversion
	for i, value := range values {
		encoded[i] = FloatToSortableInt(value)
		decoded := SortableIntToFloat(encoded[i])
		if compareFloats(value, decoded) != 0 {
			t.Errorf("forward and back conversion should generate same float at index %d: expected %v, got %v", i, value, decoded)
		}
	}

	// Check sort order (encoded values should be ascending)
	for i := 1; i < len(encoded); i++ {
		if encoded[i-1] >= encoded[i] {
			t.Errorf("check sort order failed at index %d: previous should be less than current", i)
		}
	}
}

// floatNaNs contains various NaN representations
var floatNaNs = []float32{
	float32(math.NaN()),
	math.Float32frombits(0x7f800001),
	math.Float32frombits(0x7fffffff),
	math.Float32frombits(0xff800001),
	math.Float32frombits(0xffffffff),
}

// TestSortableFloatNaN checks that all NaN values sort after positive infinity.
func TestSortableFloatNaN(t *testing.T) {
	plusInf := FloatToSortableInt(float32(math.Inf(1)))
	for _, nan := range floatNaNs {
		if !math.IsNaN(float64(nan)) {
			t.Errorf("Expected NaN, got %v", nan)
		}
		sortable := FloatToSortableInt(nan)
		if sortable <= plusInf {
			t.Errorf("Float not sorted correctly: %v, int repr: %d, positive inf.: %d", nan, sortable, plusInf)
		}
	}
}

// compareFloats compares two float32 values like Float.compare in Java
func compareFloats(a, b float32) int {
	return compareFloat64(float64(a), float64(b))
}

// TestAdd tests the Add function with random BigIntegers.
func TestAdd(t *testing.T) {
	iters := 1000
	for iter := 0; iter < iters; iter++ {
		numBytes := RandomIntN(100) + 1
		if numBytes < 1 {
			numBytes = 1
		}

		// Generate random BigIntegers (positive, up to 8*numBytes-1 bits)
		v1 := new(big.Int).Rand(GetRandom(), new(big.Int).Lsh(big.NewInt(1), uint(8*numBytes-1)))
		v2 := new(big.Int).Rand(GetRandom(), new(big.Int).Lsh(big.NewInt(1), uint(8*numBytes-1)))

		v1Bytes := make([]byte, numBytes)
		v1RawBytes := v1.Bytes()
		if len(v1RawBytes) > numBytes {
			v1RawBytes = v1RawBytes[len(v1RawBytes)-numBytes:]
		}
		copy(v1Bytes[numBytes-len(v1RawBytes):], v1RawBytes)

		v2Bytes := make([]byte, numBytes)
		v2RawBytes := v2.Bytes()
		if len(v2RawBytes) > numBytes {
			v2RawBytes = v2RawBytes[len(v2RawBytes)-numBytes:]
		}
		copy(v2Bytes[numBytes-len(v2RawBytes):], v2RawBytes)

		result := make([]byte, numBytes)
		err := Add(numBytes, 0, v1Bytes, v2Bytes, result)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		sum := new(big.Int).Add(v1, v2)
		resultBigInt := new(big.Int).SetBytes(result)

		if sum.Cmp(resultBigInt) != 0 {
			t.Errorf("sum=%v v1=%v v2=%v but result=%v", sum, v1, v2, resultBigInt)
		}
	}
}

// TestIllegalAdd tests that Add throws an error on overflow.
func TestIllegalAdd(t *testing.T) {
	bytes := make([]byte, 4)
	for i := range bytes {
		bytes[i] = 0xff
	}
	one := make([]byte, 4)
	one[3] = 1

	result := make([]byte, 4)
	err := Add(4, 0, bytes, one, result)
	if err == nil {
		t.Error("Expected error for overflow, got nil")
	}
}

// TestSubtract tests the Subtract function with random BigIntegers.
func TestSubtract(t *testing.T) {
	iters := 1000
	for iter := 0; iter < iters; iter++ {
		numBytes := RandomIntN(100) + 1
		if numBytes < 1 {
			numBytes = 1
		}

		// Generate random BigIntegers (positive, up to 8*numBytes-1 bits)
		v1 := new(big.Int).Rand(GetRandom(), new(big.Int).Lsh(big.NewInt(1), uint(8*numBytes-1)))
		v2 := new(big.Int).Rand(GetRandom(), new(big.Int).Lsh(big.NewInt(1), uint(8*numBytes-1)))

		// Ensure v1 >= v2
		if v1.Cmp(v2) < 0 {
			v1, v2 = v2, v1
		}

		v1Bytes := make([]byte, numBytes)
		v1RawBytes := v1.Bytes()
		if len(v1RawBytes) > numBytes {
			v1RawBytes = v1RawBytes[len(v1RawBytes)-numBytes:]
		}
		copy(v1Bytes[numBytes-len(v1RawBytes):], v1RawBytes)

		v2Bytes := make([]byte, numBytes)
		v2RawBytes := v2.Bytes()
		if len(v2RawBytes) > numBytes {
			v2RawBytes = v2RawBytes[len(v2RawBytes)-numBytes:]
		}
		copy(v2Bytes[numBytes-len(v2RawBytes):], v2RawBytes)

		result := make([]byte, numBytes)
		err := Subtract(numBytes, 0, v1Bytes, v2Bytes, result)
		if err != nil {
			t.Fatalf("Subtract failed: %v", err)
		}

		diff := new(big.Int).Sub(v1, v2)
		resultBigInt := new(big.Int).SetBytes(result)

		if diff.Cmp(resultBigInt) != 0 {
			t.Errorf("diff=%v vs result=%v v1=%v v2=%v", diff, resultBigInt, v1, v2)
		}
	}
}

// TestIllegalSubtract tests that Subtract throws an error when a < b.
func TestIllegalSubtract(t *testing.T) {
	v1 := make([]byte, 4)
	v1[3] = 0xf0
	v2 := make([]byte, 4)
	v2[3] = 0xf1

	result := make([]byte, 4)
	err := Subtract(4, 0, v1, v2, result)
	if err == nil {
		t.Error("Expected error when a < b, got nil")
	}
}

// TestIntsRoundTrip tests round-trip encoding of random integers.
func TestIntsRoundTrip(t *testing.T) {
	encoded := make([]byte, 4)

	for i := 0; i < 10000; i++ {
		value := int32(RandomInt())
		IntToSortableBytes(value, encoded, 0)
		decoded := SortableBytesToInt(encoded, 0)
		if decoded != value {
			t.Errorf("Round-trip failed: expected %d, got %d", value, decoded)
		}
	}
}

// TestLongsRoundTrip tests round-trip encoding of random longs.
func TestLongsRoundTrip(t *testing.T) {
	encoded := make([]byte, 8)

	for i := 0; i < 10000; i++ {
		// Generate random int64
		value := int64(RandomInt())<<32 | int64(RandomInt())
		LongToSortableBytes(value, encoded, 0)
		decoded := SortableBytesToLong(encoded, 0)
		if decoded != value {
			t.Errorf("Round-trip failed: expected %d, got %d", value, decoded)
		}
	}
}

// TestFloatsRoundTrip tests round-trip encoding of random floats.
func TestFloatsRoundTrip(t *testing.T) {
	encoded := make([]byte, 4)

	for i := 0; i < 10000; i++ {
		value := math.Float32frombits(uint32(RandomInt()))
		IntToSortableBytes(FloatToSortableInt(value), encoded, 0)
		actual := SortableIntToFloat(SortableBytesToInt(encoded, 0))
		if math.Float32bits(value) != math.Float32bits(actual) {
			t.Errorf("Round-trip failed: expected %v, got %v", value, actual)
		}
	}
}

// TestDoublesRoundTrip tests round-trip encoding of random doubles.
func TestDoublesRoundTrip(t *testing.T) {
	encoded := make([]byte, 8)

	for i := 0; i < 10000; i++ {
		value := math.Float64frombits(uint64(RandomInt())<<32 | uint64(RandomInt()))
		LongToSortableBytes(DoubleToSortableLong(value), encoded, 0)
		actual := SortableLongToDouble(SortableBytesToLong(encoded, 0))
		if math.Float64bits(value) != math.Float64bits(actual) {
			t.Errorf("Round-trip failed: expected %v, got %v", value, actual)
		}
	}
}

// TestBigIntsRoundTrip tests round-trip encoding of random big integers.
func TestBigIntsRoundTrip(t *testing.T) {
	for i := 0; i < 10000; i++ {
		// Generate random BigInteger (up to 16 bytes)
		maxLength := RandomIntN(16) + 1
		value := new(big.Int).Rand(GetRandom(), new(big.Int).Lsh(big.NewInt(1), uint(8*maxLength)))

		length := len(value.Bytes())
		if length == 0 {
			length = 1
		}

		// Make sure sign extension is tested: sometimes pad to more bytes when encoding
		maxLength = RandomIntN(4) + length
		encoded := make([]byte, maxLength)
		err := BigIntToSortableBytes(value, maxLength, encoded, 0)
		if err != nil {
			t.Fatalf("Failed to encode BigInt: %v", err)
		}
		decoded := SortableBytesToBigInt(encoded, 0, maxLength)
		if decoded.Cmp(value) != 0 {
			t.Errorf("Round-trip failed: expected %v, got %v", value, decoded)
		}
	}
}

// TestIntsCompare checks sort order of random integers consistent with Integer.Compare.
func TestIntsCompare(t *testing.T) {
	left := &BytesRef{
		Bytes:  make([]byte, 4),
		Offset: 0,
		Length: 4,
	}
	right := &BytesRef{
		Bytes:  make([]byte, 4),
		Offset: 0,
		Length: 4,
	}

	for i := 0; i < 10000; i++ {
		leftValue := int32(RandomInt())
		IntToSortableBytes(leftValue, left.Bytes, left.Offset)

		rightValue := int32(RandomInt())
		IntToSortableBytes(rightValue, right.Bytes, right.Offset)

		expectedSign := signum(int64(int32Compare(leftValue, rightValue)))
		actualSign := signum(int64(BytesRefCompare(left, right)))

		if expectedSign != actualSign {
			t.Errorf("Compare mismatch: left=%d, right=%d, expected sign %d, got %d", leftValue, rightValue, expectedSign, actualSign)
		}
	}
}

// TestLongsCompare checks sort order of random longs consistent with Long.Compare.
func TestLongsCompare(t *testing.T) {
	left := &BytesRef{
		Bytes:  make([]byte, 8),
		Offset: 0,
		Length: 8,
	}
	right := &BytesRef{
		Bytes:  make([]byte, 8),
		Offset: 0,
		Length: 8,
	}

	for i := 0; i < 10000; i++ {
		leftValue := int64(RandomInt())<<32 | int64(RandomInt())
		LongToSortableBytes(leftValue, left.Bytes, left.Offset)

		rightValue := int64(RandomInt())<<32 | int64(RandomInt())
		LongToSortableBytes(rightValue, right.Bytes, right.Offset)

		expectedSign := signum(int64(int64Compare(leftValue, rightValue)))
		actualSign := signum(int64(BytesRefCompare(left, right)))

		if expectedSign != actualSign {
			t.Errorf("Compare mismatch: left=%d, right=%d, expected sign %d, got %d", leftValue, rightValue, expectedSign, actualSign)
		}
	}
}

// TestFloatsCompare checks sort order of random floats consistent with Float.Compare.
func TestFloatsCompare(t *testing.T) {
	left := &BytesRef{
		Bytes:  make([]byte, 4),
		Offset: 0,
		Length: 4,
	}
	right := &BytesRef{
		Bytes:  make([]byte, 4),
		Offset: 0,
		Length: 4,
	}

	for i := 0; i < 10000; i++ {
		leftValue := math.Float32frombits(uint32(RandomInt()))
		IntToSortableBytes(FloatToSortableInt(leftValue), left.Bytes, left.Offset)

		rightValue := math.Float32frombits(uint32(RandomInt()))
		IntToSortableBytes(FloatToSortableInt(rightValue), right.Bytes, right.Offset)

		expectedSign := signum(int64(compareFloats(leftValue, rightValue)))
		actualSign := signum(int64(BytesRefCompare(left, right)))

		if expectedSign != actualSign {
			t.Errorf("Compare mismatch: left=%v, right=%v, expected sign %d, got %d", leftValue, rightValue, expectedSign, actualSign)
		}
	}
}

// TestDoublesCompare checks sort order of random doubles consistent with Double.Compare.
func TestDoublesCompare(t *testing.T) {
	left := &BytesRef{
		Bytes:  make([]byte, 8),
		Offset: 0,
		Length: 8,
	}
	right := &BytesRef{
		Bytes:  make([]byte, 8),
		Offset: 0,
		Length: 8,
	}

	for i := 0; i < 10000; i++ {
		leftValue := math.Float64frombits(uint64(RandomInt())<<32 | uint64(RandomInt()))
		LongToSortableBytes(DoubleToSortableLong(leftValue), left.Bytes, left.Offset)

		rightValue := math.Float64frombits(uint64(RandomInt())<<32 | uint64(RandomInt()))
		LongToSortableBytes(DoubleToSortableLong(rightValue), right.Bytes, right.Offset)

		expectedSign := signum(int64(compareFloat64(leftValue, rightValue)))
		actualSign := signum(int64(BytesRefCompare(left, right)))

		if expectedSign != actualSign {
			t.Errorf("Compare mismatch: left=%v, right=%v, expected sign %d, got %d", leftValue, rightValue, expectedSign, actualSign)
		}
	}
}

// TestBigIntsCompare checks sort order of random big integers consistent with BigInteger.CompareTo.
func TestBigIntsCompare(t *testing.T) {
	for i := 0; i < 10000; i++ {
		maxLength := RandomIntN(16) + 1

		leftValue := new(big.Int).Rand(GetRandom(), new(big.Int).Lsh(big.NewInt(1), uint(8*maxLength)))
		left := &BytesRef{
			Bytes:  make([]byte, maxLength),
			Offset: 0,
			Length: maxLength,
		}
		err := BigIntToSortableBytes(leftValue, maxLength, left.Bytes, left.Offset)
		if err != nil {
			t.Fatalf("Failed to encode left BigInt: %v", err)
		}

		rightValue := new(big.Int).Rand(GetRandom(), new(big.Int).Lsh(big.NewInt(1), uint(8*maxLength)))
		right := &BytesRef{
			Bytes:  make([]byte, maxLength),
			Offset: 0,
			Length: maxLength,
		}
		err = BigIntToSortableBytes(rightValue, maxLength, right.Bytes, right.Offset)
		if err != nil {
			t.Fatalf("Failed to encode right BigInt: %v", err)
		}

		expectedSign := signum(int64(leftValue.Cmp(rightValue)))
		actualSign := signum(int64(BytesRefCompare(left, right)))

		if expectedSign != actualSign {
			t.Errorf("Compare mismatch: left=%v, right=%v, expected sign %d, got %d", leftValue, rightValue, expectedSign, actualSign)
		}
	}
}

// Helper functions

func int32Compare(a, b int32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func int64Compare(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func signum(x int64) int {
	if x < 0 {
		return -1
	}
	if x > 0 {
		return 1
	}
	return 0
}

// Additional tests for edge cases

// TestSortableDoubleBits tests the sortable double bits conversion directly.
func TestSortableDoubleBits(t *testing.T) {
	tests := []struct {
		input    uint64
		expected int64
	}{
		{0x0000000000000000, 0x0000000000000000},                    // +0.0
		{0x8000000000000000, int64(0x8000000000000000)},                    // -0.0
		{0x3ff0000000000000, 0x3ff0000000000000},                    // 1.0
		{0xbff0000000000000, 0x4000000000000000},                    // -1.0
		{0x7ff0000000000000, 0x7ff0000000000000},                    // +Inf
		{0xfff0000000000000, 0x8000000000000000},                    // -Inf
		{0x7ff8000000000000, 0x7ff8000000000000},                    // NaN
		{0x8000000000000001, 0x7fffffffffffffff},                    // Smallest negative
	}

	for _, tc := range tests {
		result := SortableDoubleBits(tc.input)
		if result != tc.expected {
			t.Errorf("SortableDoubleBits(0x%016x) = 0x%016x, expected 0x%016x", tc.input, result, tc.expected)
		}
	}
}

// TestSortableFloatBits tests the sortable float bits conversion directly.
func TestSortableFloatBits(t *testing.T) {
	tests := []struct {
		input    uint32
		expected int32
	}{
		{0x00000000, 0x00000000}, // +0.0
		{0x80000000, int32(0x80000000)}, // -0.0
		{0x3f800000, 0x3f800000}, // 1.0
		{0xbf800000, 0x40000000}, // -1.0
		{0x7f800000, 0x7f800000}, // +Inf
		{0xff800000, int32(0x80000000)}, // -Inf
		{0x7fc00000, 0x7fc00000}, // NaN
	}

	for _, tc := range tests {
		result := SortableFloatBits(tc.input)
		if result != tc.expected {
			t.Errorf("SortableFloatBits(0x%08x) = 0x%08x, expected 0x%08x", tc.input, result, tc.expected)
		}
	}
}

// TestIntToSortableBytesEdgeCases tests edge cases for int encoding.
func TestIntToSortableBytesEdgeCases(t *testing.T) {
	tests := []int32{
		0,
		1,
		-1,
		math.MaxInt32,
		math.MinInt32,
	}

	for _, value := range tests {
		encoded := make([]byte, 4)
		IntToSortableBytes(value, encoded, 0)
		decoded := SortableBytesToInt(encoded, 0)
		if decoded != value {
			t.Errorf("Round-trip failed for %d: got %d", value, decoded)
		}
	}
}

// TestLongToSortableBytesEdgeCases tests edge cases for long encoding.
func TestLongToSortableBytesEdgeCases(t *testing.T) {
	tests := []int64{
		0,
		1,
		-1,
		math.MaxInt64,
		math.MinInt64,
	}

	for _, value := range tests {
		encoded := make([]byte, 8)
		LongToSortableBytes(value, encoded, 0)
		decoded := SortableBytesToLong(encoded, 0)
		if decoded != value {
			t.Errorf("Round-trip failed for %d: got %d", value, decoded)
		}
	}
}

// TestBigIntToSortableBytesEdgeCases tests edge cases for BigInt encoding.
func TestBigIntToSortableBytesEdgeCases(t *testing.T) {
	tests := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(-1),
		new(big.Int).SetBytes([]byte{0x7f, 0xff, 0xff, 0xff}), // Large positive
		new(big.Int).Neg(new(big.Int).SetBytes([]byte{0x7f, 0xff, 0xff, 0xff})),
	}

	for _, value := range tests {
		size := 8
		encoded := make([]byte, size)
		err := BigIntToSortableBytes(value, size, encoded, 0)
		if err != nil {
			t.Errorf("Failed to encode %v: %v", value, err)
			continue
		}
		decoded := SortableBytesToBigInt(encoded, 0, size)
		if decoded.Cmp(value) != 0 {
			t.Errorf("Round-trip failed for %v: got %v", value, decoded)
		}
	}
}

// TestAddEdgeCases tests edge cases for the Add function.
func TestAddEdgeCases(t *testing.T) {
	// Test adding zero
	a := []byte{0, 0, 0, 1}
	b := []byte{0, 0, 0, 0}
	result := make([]byte, 4)
	err := Add(4, 0, a, b, result)
	if err != nil {
		t.Errorf("Add with zero failed: %v", err)
	}
	if !bytes.Equal(result, []byte{0, 0, 0, 1}) {
		t.Errorf("Add with zero gave wrong result: %v", result)
	}

	// Test adding to max value (should overflow)
	maxVal := []byte{0xff, 0xff, 0xff, 0xfe}
	one := []byte{0, 0, 0, 1}
	err = Add(4, 0, maxVal, one, result)
	if err != nil {
		t.Errorf("Add should not overflow: %v", err)
	}
}

// TestSubtractEdgeCases tests edge cases for the Subtract function.
func TestSubtractEdgeCases(t *testing.T) {
	// Test subtracting zero
	a := []byte{0, 0, 0, 5}
	b := []byte{0, 0, 0, 0}
	result := make([]byte, 4)
	err := Subtract(4, 0, a, b, result)
	if err != nil {
		t.Errorf("Subtract zero failed: %v", err)
	}
	if !bytes.Equal(result, []byte{0, 0, 0, 5}) {
		t.Errorf("Subtract zero gave wrong result: %v", result)
	}

	// Test subtracting same value
	a = []byte{0, 0, 0, 5}
	b = []byte{0, 0, 0, 5}
	err = Subtract(4, 0, a, b, result)
	if err != nil {
		t.Errorf("Subtract same value failed: %v", err)
	}
	if !bytes.Equal(result, []byte{0, 0, 0, 0}) {
		t.Errorf("Subtract same value gave wrong result: %v", result)
	}
}

// TestBytesRefOrderingWithNumericUtils tests that BytesRef comparison works correctly
// with NumericUtils encoded values.
func TestBytesRefOrderingWithNumericUtils(t *testing.T) {
	// Test that encoded values maintain proper ordering
	values := []int64{-100, -50, -10, 0, 10, 50, 100}
	encoded := make([]*BytesRef, len(values))

	for i, v := range values {
		encoded[i] = &BytesRef{
			Bytes:  make([]byte, 8),
			Offset: 0,
			Length: 8,
		}
		LongToSortableBytes(v, encoded[i].Bytes, encoded[i].Offset)
	}

	// Verify ascending order
	for i := 1; i < len(encoded); i++ {
		cmp := BytesRefCompare(encoded[i-1], encoded[i])
		if cmp >= 0 {
			t.Errorf("Expected ascending order at index %d, got comparison result %d", i, cmp)
		}
	}
}
