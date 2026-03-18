// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"time"
)

var (
	// testRandom is the random source for testing
	testRandom = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// RandomInt returns a random int
func RandomInt() int {
	return testRandom.Int()
}

// RandomIntN returns a random int in the range [0, n)
func RandomIntN(n int) int {
	if n <= 0 {
		return 0
	}
	return testRandom.Intn(n)
}

// RandomBool returns a random boolean
func RandomBool() bool {
	return testRandom.Intn(2) == 0
}

// RandomBinaryTerm returns a random binary term for testing
func RandomBinaryTerm() []byte {
	length := RandomIntN(20) + 1
	bytes := make([]byte, length)
	for i := range bytes {
		bytes[i] = byte(RandomIntN(256))
	}
	return bytes
}

// SetRandomSeed sets the random seed for reproducible tests
func SetRandomSeed(seed int64) {
	testRandom = rand.New(rand.NewSource(seed))
}

// GetRandom returns the global random source
func GetRandom() *rand.Rand {
	return testRandom
}

// RandomSimpleString returns a random simple string with characters from 'a' to 'z'
// The string length will be between min and max (inclusive)
func RandomSimpleString(rng *rand.Rand, min, max int) string {
	if min < 0 {
		min = 0
	}
	if max < min {
		max = min
	}
	length := min
	if max > min {
		length += rng.Intn(max - min + 1)
	}
	chars := make([]byte, length)
	for i := range chars {
		chars[i] = byte('a' + rng.Intn(26))
	}
	return string(chars)
}
