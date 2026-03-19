// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"hash"
)

// xxHash32 implements the xxHash32 algorithm.
// This is a fast non-cryptographic hash algorithm.
type xxHash32 struct {
	state  uint32
	seed   uint32
	buffer []byte
	length int
}

// xxHash32 constants
const (
	xxHash32Prime1 uint32 = 2654435761
	xxHash32Prime2 uint32 = 2246822519
	xxHash32Prime3 uint32 = 3266489917
	xxHash32Prime4 uint32 = 668265263
	xxHash32Prime5 uint32 = 374761393
)

// NewXXHash32 creates a new xxHash32 hash with default seed.
func NewXXHash32() hash.Hash32 {
	return NewXXHash32WithSeed(0)
}

// NewXXHash32WithSeed creates a new xxHash32 hash with the given seed.
func NewXXHash32WithSeed(seed uint32) hash.Hash32 {
	h := &xxHash32{
		seed:   seed,
		buffer: make([]byte, 0, 16),
	}
	h.Reset()
	return h
}

// Write adds more data to the running hash.
func (h *xxHash32) Write(p []byte) (n int, err error) {
	h.buffer = append(h.buffer, p...)
	h.length += len(p)
	return len(p), nil
}

// Sum32 returns the 32-bit hash value.
func (h *xxHash32) Sum32() uint32 {
	if h.length == 0 {
		return h.seed + xxHash32Prime5
	}

	// Process full 4-byte chunks
	state := h.state
	for len(h.buffer) >= 4 {
		chunk := binary.LittleEndian.Uint32(h.buffer)
		state = xxHash32Round(state, chunk)
		h.buffer = h.buffer[4:]
	}

	// Process remaining bytes
	result := state + uint32(h.length)
	for _, b := range h.buffer {
		result += uint32(b) * xxHash32Prime5
		result = (result << 11) | (result >> 21)
		result *= xxHash32Prime1
	}

	// Final mix
	result ^= result >> 15
	result *= xxHash32Prime2
	result ^= result >> 13
	result *= xxHash32Prime3
	result ^= result >> 16

	return result
}

// Sum appends the current hash to b and returns the resulting slice.
func (h *xxHash32) Sum(b []byte) []byte {
	s := h.Sum32()
	return append(b, byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// Reset resets the hash to its initial state.
func (h *xxHash32) Reset() {
	h.state = h.seed + xxHash32Prime1 + xxHash32Prime2
	h.buffer = h.buffer[:0]
	h.length = 0
}

// Size returns the number of bytes Sum will return.
func (h *xxHash32) Size() int {
	return 4
}

// BlockSize returns the hash's underlying block size.
func (h *xxHash32) BlockSize() int {
	return 4
}

// xxHash32Round performs a single round of xxHash32.
func xxHash32Round(state, chunk uint32) uint32 {
	state += chunk * xxHash32Prime3
	state = (state << 13) | (state >> 19)
	state *= xxHash32Prime1
	return state
}

// xxHash64 implements the xxHash64 algorithm.
// This is a fast non-cryptographic hash algorithm with 64-bit output.
type xxHash64 struct {
	state  uint64
	seed   uint64
	buffer []byte
	length int
}

// xxHash64 constants
const (
	xxHash64Prime1 uint64 = 11400714785074694791
	xxHash64Prime2 uint64 = 14029467366897019727
	xxHash64Prime3 uint64 = 1609587929392839161
	xxHash64Prime4 uint64 = 9650029242287828579
	xxHash64Prime5 uint64 = 2870177450012600261
)

// NewXXHash64 creates a new xxHash64 hash with default seed.
func NewXXHash64() hash.Hash64 {
	return NewXXHash64WithSeed(0)
}

// NewXXHash64WithSeed creates a new xxHash64 hash with the given seed.
func NewXXHash64WithSeed(seed uint64) hash.Hash64 {
	h := &xxHash64{
		seed:   seed,
		buffer: make([]byte, 0, 32),
	}
	h.Reset()
	return h
}

// Write adds more data to the running hash.
func (h *xxHash64) Write(p []byte) (n int, err error) {
	h.buffer = append(h.buffer, p...)
	h.length += len(p)
	return len(p), nil
}

// Sum64 returns the 64-bit hash value.
func (h *xxHash64) Sum64() uint64 {
	if h.length == 0 {
		return h.seed + xxHash64Prime5
	}

	// Process full 8-byte chunks
	state := h.state
	for len(h.buffer) >= 8 {
		chunk := binary.LittleEndian.Uint64(h.buffer)
		state = xxHash64Round(state, chunk)
		h.buffer = h.buffer[8:]
	}

	// Process remaining bytes
	result := state + uint64(h.length)
	for _, b := range h.buffer {
		result += uint64(b) * xxHash64Prime5
		result = (result << 27) | (result >> 37)
		result *= xxHash64Prime1
	}

	// Final mix
	result ^= result >> 33
	result *= xxHash64Prime2
	result ^= result >> 29
	result *= xxHash64Prime3
	result ^= result >> 32

	return result
}

// Sum appends the current hash to b and returns the resulting slice.
func (h *xxHash64) Sum(b []byte) []byte {
	s := h.Sum64()
	return append(b, byte(s>>56), byte(s>>48), byte(s>>40), byte(s>>32),
		byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}

// Reset resets the hash to its initial state.
func (h *xxHash64) Reset() {
	h.state = h.seed + xxHash64Prime1 + xxHash64Prime2
	h.buffer = h.buffer[:0]
	h.length = 0
}

// Size returns the number of bytes Sum will return.
func (h *xxHash64) Size() int {
	return 8
}

// BlockSize returns the hash's underlying block size.
func (h *xxHash64) BlockSize() int {
	return 8
}

// xxHash64Round performs a single round of xxHash64.
func xxHash64Round(state, chunk uint64) uint64 {
	state += chunk * xxHash64Prime3
	state = (state << 13) | (state >> 51)
	state *= xxHash64Prime1
	return state
}

// Sum32 returns the lower 32 bits of the 64-bit hash.
func (h *xxHash64) Sum32() uint32 {
	return uint32(h.Sum64())
}
