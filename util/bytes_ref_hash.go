// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"fmt"
)

// BytesRefHash is a special purpose hash-map like data-structure optimized for
// BytesRef instances. BytesRefHash maintains mappings of byte arrays to ids
// (Map<BytesRef,int>) storing the hashed bytes efficiently in continuous storage.
// The mapping to the id is encapsulated inside BytesRefHash and is guaranteed
// to be increased for each added BytesRef.
//
// Note: The maximum capacity BytesRef instance passed to Add must not be
// longer than ByteBlockPool.BYTE_BLOCK_SIZE-2. The internal storage is limited
// to 2GB total byte storage.
//
// This is the Go port of Lucene's org.apache.lucene.util.BytesRefHash.
type BytesRefHash struct {
	// Package private fields needed by comparator
	pool       *BytesRefBlockPool
	bytesStart []int

	hashSize     int
	hashHalfSize int
	hashMask     int
	highMask     int
	count        int
	lastCount    int

	// The ids array serves a dual purpose:
	// 1. When the value is -1, it indicates an empty slot in the hash table.
	// 2. When the value is not -1, it stores:
	//    - The actual index into the bytesStart array (low bits, masked by hashMask)
	//    - The high bits of the original hashcode (high bits, masked by highMask)
	ids []int

	bytesStartArray BytesStartArray
	bytesUsed       *Counter
}

// DefaultCapacity is the default initial capacity for BytesRefHash.
const DefaultCapacity = 16

// MaxBytesLengthExceededException is thrown if a BytesRef exceeds the limit.
type MaxBytesLengthExceededException struct {
	message string
}

func (e *MaxBytesLengthExceededException) Error() string {
	return e.message
}

// NewMaxBytesLengthExceededException creates a new MaxBytesLengthExceededException.
func NewMaxBytesLengthExceededException(message string) *MaxBytesLengthExceededException {
	return &MaxBytesLengthExceededException{message: message}
}

// NewBytesRefHash creates a new BytesRefHash with a ByteBlockPool using a DirectAllocator.
func NewBytesRefHash() *BytesRefHash {
	return NewBytesRefHashWithPool(NewByteBlockPool(NewDirectAllocator()))
}

// NewBytesRefHashWithPool creates a new BytesRefHash with the given ByteBlockPool.
func NewBytesRefHashWithPool(pool *ByteBlockPool) *BytesRefHash {
	return NewBytesRefHashWithCapacity(pool, DefaultCapacity, NewDirectBytesStartArray(DefaultCapacity))
}

// NewBytesRefHashWithCapacity creates a new BytesRefHash with the given capacity and BytesStartArray.
func NewBytesRefHashWithCapacity(pool *ByteBlockPool, capacity int, bytesStartArray BytesStartArray) *BytesRefHash {
	if capacity <= 0 {
		panic(fmt.Sprintf("capacity must be greater than 0, got %d", capacity))
	}

	if !IsPowerOfTwo(capacity) {
		panic(fmt.Sprintf("capacity must be a power of two, got %d", capacity))
	}

	hash := &BytesRefHash{
		hashSize:        capacity,
		hashHalfSize:    capacity >> 1,
		hashMask:        capacity - 1,
		highMask:        ^(capacity - 1),
		pool:            NewBytesRefBlockPool(pool),
		ids:             make([]int, capacity),
		bytesStartArray: bytesStartArray,
		lastCount:       -1,
	}

	// Initialize ids with -1 (empty slots)
	for i := range hash.ids {
		hash.ids[i] = -1
	}

	hash.bytesStart = bytesStartArray.Init()
	bytesUsed := bytesStartArray.BytesUsed()
	if bytesUsed == nil {
		bytesUsed = NewCounter()
	}
	hash.bytesUsed = bytesUsed
	hash.bytesUsed.AddAndGet(int64(capacity * 4)) // Integer.BYTES = 4

	return hash
}

// Size returns the number of BytesRef values in this BytesRefHash.
func (h *BytesRefHash) Size() int {
	return h.count
}

// Get populates and returns a BytesRef with the bytes for the given bytesID.
// Note: the given bytesID must be a positive integer less than the current size (Size()).
func (h *BytesRefHash) Get(bytesID int, ref *BytesRef) *BytesRef {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}
	if bytesID < 0 || bytesID >= len(h.bytesStart) {
		panic(fmt.Sprintf("bytesID out of range: %d (len: %d)", bytesID, len(h.bytesStart)))
	}
	h.pool.FillBytesRef(ref, h.bytesStart[bytesID])
	return ref
}

// Compact returns the ids array in arbitrary order. Valid ids start at offset of 0
// and end at a limit of Size() - 1.
// Note: This is a destructive operation. Clear() must be called in order to reuse
// this BytesRefHash instance.
func (h *BytesRefHash) Compact() []int {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}

	// id is the sequence number when bytes added to the pool
	for i := 0; i < h.count; i++ {
		h.ids[i] = i
	}
	for i := h.count; i < h.hashSize; i++ {
		h.ids[i] = -1
	}

	h.lastCount = h.count
	return h.ids
}

// Sort returns the values array sorted by the referenced byte values.
// Note: This is a destructive operation. Clear() must be called in order to reuse
// this BytesRefHash instance.
func (h *BytesRefHash) Sort() []int {
	compact := h.Compact()
	// For simplicity, we use IntroSort on the compacted ids
	// The Java version uses a more complex StringSorter with MSBStringRadixSorter
	// but for Go port, we'll use a simpler approach

	// Create a slice of indices to sort
	type entry struct {
		id   int
		bytes []byte
	}
	entries := make([]entry, h.count)
	scratch := &BytesRef{}
	for i := 0; i < h.count; i++ {
		h.Get(compact[i], scratch)
		entries[i] = entry{id: compact[i], bytes: scratch.Clone().ValidBytes()}
	}

	// Sort entries by bytes
	IntroSort(entries, func(a, b entry) int {
		return bytes.Compare(a.bytes, b.bytes)
	})

	// Update compact with sorted ids
	for i := 0; i < h.count; i++ {
		compact[i] = entries[i].id
	}

	// Fill remaining slots with -1
	for i := h.count; i < len(compact); i++ {
		compact[i] = -1
	}

	return compact
}

// bytes.Compare is not in the standard library for Go < 1.21, so we implement it
func bytesCompare(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func (h *BytesRefHash) shrink(targetSize int) bool {
	// Cannot use ArrayUtil.shrink because we require power of 2
	newSize := h.hashSize
	for newSize >= 8 && newSize/4 > targetSize {
		newSize /= 2
	}
	if newSize != h.hashSize {
		h.bytesUsed.AddAndGet(int64(-4 * (h.hashSize - newSize))) // Integer.BYTES = 4
		h.hashSize = newSize
		h.ids = make([]int, newSize)
		for i := range h.ids {
			h.ids[i] = -1
		}
		h.hashHalfSize = newSize / 2
		h.hashMask = newSize - 1
		h.highMask = ^h.hashMask
		return true
	}
	return false
}

// Clear clears the BytesRefHash. If resetPool is true, the pool is also reset.
func (h *BytesRefHash) Clear(resetPool bool) {
	h.lastCount = h.count
	h.count = 0
	if resetPool {
		h.pool.Reset()
	}
	h.bytesStart = h.bytesStartArray.Clear()
	if h.lastCount != -1 && h.shrink(h.lastCount) {
		// shrink clears the hash entries
		return
	}
	for i := range h.ids {
		h.ids[i] = -1
	}
}

// ClearWithPoolReset clears the BytesRefHash and resets the pool.
func (h *BytesRefHash) ClearWithPoolReset() {
	h.Clear(true)
}

// Close closes the BytesRefHash and releases all internally used memory.
func (h *BytesRefHash) Close() {
	h.Clear(true)
	h.ids = nil
	h.bytesUsed.AddAndGet(int64(-4 * h.hashSize)) // Integer.BYTES = 4
}

// Add adds a new BytesRef.
// Returns the id the given bytes are hashed if there was no mapping for the given bytes,
// otherwise (-(id)-1). This guarantees that the return value will always be >= 0
// if the given bytes haven't been hashed before.
func (h *BytesRefHash) Add(bytes *BytesRef) (int, error) {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}

	// Check max length
	if bytes.Length > ByteBlockSize-2 {
		return 0, NewMaxBytesLengthExceededException(
			fmt.Sprintf("bytes can be at most %d in length; got %d", ByteBlockSize-2, bytes.Length))
	}

	hashcode := doHash(bytes.Bytes, bytes.Offset, bytes.Length)
	hashPos := h.findHash(bytes, hashcode)
	e := h.ids[hashPos]

	if e == -1 {
		// new entry
		if h.count >= len(h.bytesStart) {
			h.bytesStart = h.bytesStartArray.Grow()
		}
		offset, err := h.pool.AddBytesRef(bytes)
		if err != nil {
			return 0, err
		}
		h.bytesStart[h.count] = offset
		e = h.count
		h.count++
		h.ids[hashPos] = e | (hashcode & h.highMask)

		if h.count == h.hashHalfSize {
			h.rehash(2*h.hashSize, true)
		}
		return e, nil
	}
	e = e & h.hashMask
	return -(e + 1), nil
}

// Find returns the id of the given BytesRef, or -1 if there is no mapping for the given bytes.
func (h *BytesRefHash) Find(bytes *BytesRef) int {
	hashcode := doHash(bytes.Bytes, bytes.Offset, bytes.Length)
	id := h.ids[h.findHash(bytes, hashcode)]
	if id == -1 {
		return -1
	}
	return id & h.hashMask
}

func (h *BytesRefHash) findHash(bytes *BytesRef, hashcode int) int {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}

	code := hashcode
	hashPos := code & h.hashMask
	e := h.ids[hashPos]
	highBits := hashcode & h.highMask

	// Conflict; use linear probe to find an open slot (see LUCENE-5604)
	for e != -1 && ((e&h.highMask) != highBits || !h.pool.Equals(h.bytesStart[e&h.hashMask], bytes)) {
		code++
		hashPos = code & h.hashMask
		e = h.ids[hashPos]
	}

	return hashPos
}

// AddByPoolOffset adds an arbitrary int offset instead of a BytesRef term.
// This is used in the indexer to hold the hash for term vectors.
func (h *BytesRefHash) AddByPoolOffset(offset int) int {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}

	code := offset
	hashPos := offset & h.hashMask
	e := h.ids[hashPos]

	// Conflict; use linear probe to find an open slot (see LUCENE-5604)
	for e != -1 && h.bytesStart[e&h.hashMask] != offset {
		code++
		hashPos = code & h.hashMask
		e = h.ids[hashPos]
	}

	if e == -1 {
		// new entry
		if h.count >= len(h.bytesStart) {
			h.bytesStart = h.bytesStartArray.Grow()
		}
		e = h.count
		h.bytesStart[e] = offset
		h.count++
		h.ids[hashPos] = e

		if h.count == h.hashHalfSize {
			h.rehash(2*h.hashSize, false)
		}
		return e
	}
	return -(e + 1)
}

// rehash is called when hash is too small (> 50% occupied) or too large (< 20% occupied).
func (h *BytesRefHash) rehash(newSize int, hashOnData bool) {
	newMask := newSize - 1
	newHighMask := ^newMask
	h.bytesUsed.AddAndGet(int64(4 * newSize)) // Integer.BYTES = 4
	newHash := make([]int, newSize)
	for i := range newHash {
		newHash[i] = -1
	}

	for i := 0; i < h.hashSize; i++ {
		e0 := h.ids[i]
		if e0 != -1 {
			e0 &= h.hashMask
			var hashcode, code int
			if hashOnData {
				hashcode = h.pool.Hash(h.bytesStart[e0])
				code = hashcode
			} else {
				code = h.bytesStart[e0]
				hashcode = 0
			}

			hashPos := code & newMask

			// Conflict; use linear probe to find an open slot (see LUCENE-5604)
			for newHash[hashPos] != -1 {
				code++
				hashPos = code & newMask
			}

			newHash[hashPos] = e0 | (hashcode & newHighMask)
		}
	}

	h.hashMask = newMask
	h.highMask = newHighMask
	h.bytesUsed.AddAndGet(int64(-4 * len(h.ids))) // Integer.BYTES = 4
	h.ids = newHash
	h.hashSize = newSize
	h.hashHalfSize = newSize / 2
}

// doHash computes the hash code for the given bytes.
func doHash(bytes []byte, offset, length int) int {
	return MurmurHash3_x86_32(bytes, offset, length, GoodFastHashSeed)
}

// GoodFastHashSeed is a good seed for fast hashing.
const GoodFastHashSeed = 0x5a827999 // A constant from the SHA-1 algorithm

// IsPowerOfTwo returns true if n is a power of two.
func IsPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// Reinit reinitializes the BytesRefHash after a previous Clear() call.
// If Clear() has not been called previously this method has no effect.
func (h *BytesRefHash) Reinit() {
	if h.bytesStart == nil {
		h.bytesStart = h.bytesStartArray.Init()
	}

	if h.ids == nil {
		h.ids = make([]int, h.hashSize)
		for i := range h.ids {
			h.ids[i] = -1
		}
		h.bytesUsed.AddAndGet(int64(4 * h.hashSize)) // Integer.BYTES = 4
	}
}

// ByteStart returns the bytesStart offset into the internally used ByteBlockPool for the given bytesID.
func (h *BytesRefHash) ByteStart(bytesID int) int {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}
	if bytesID < 0 || bytesID >= h.count {
		panic(fmt.Sprintf("bytesID out of range: %d", bytesID))
	}
	return h.bytesStart[bytesID]
}

// BytesStartArray manages allocation of the per-term addresses.
type BytesStartArray interface {
	// Init initializes the BytesStartArray. This call will allocate memory.
	Init() []int
	// Grow grows the BytesStartArray.
	Grow() []int
	// Clear clears the BytesStartArray and returns the cleared instance.
	Clear() []int
	// BytesUsed returns a Counter reference holding the number of bytes used by this BytesStartArray.
	BytesUsed() *Counter
}

// DirectBytesStartArray is a simple BytesStartArray that tracks memory allocation
// using a private Counter instance.
type DirectBytesStartArray struct {
	initSize   int
	bytesStart []int
	bytesUsed  *Counter
}

// NewDirectBytesStartArray creates a new DirectBytesStartArray with the given initial size.
func NewDirectBytesStartArray(initSize int) *DirectBytesStartArray {
	return NewDirectBytesStartArrayWithCounter(initSize, NewCounter())
}

// NewDirectBytesStartArrayWithCounter creates a new DirectBytesStartArray with the given initial size and counter.
func NewDirectBytesStartArrayWithCounter(initSize int, counter *Counter) *DirectBytesStartArray {
	return &DirectBytesStartArray{
		initSize:  initSize,
		bytesUsed: counter,
	}
}

// Init initializes the BytesStartArray.
func (a *DirectBytesStartArray) Init() []int {
	a.bytesStart = make([]int, oversize(a.initSize, 4)) // Integer.BYTES = 4
	return a.bytesStart
}

// Grow grows the BytesStartArray.
func (a *DirectBytesStartArray) Grow() []int {
	if a.bytesStart == nil {
		panic("bytesStart is nil")
	}
	newSize := oversize(len(a.bytesStart)+1, 4)
	newBytesStart := make([]int, newSize)
	copy(newBytesStart, a.bytesStart)
	a.bytesStart = newBytesStart
	return a.bytesStart
}

// Clear clears the BytesStartArray.
func (a *DirectBytesStartArray) Clear() []int {
	a.bytesStart = nil
	return nil
}

// BytesUsed returns the Counter tracking bytes used.
func (a *DirectBytesStartArray) BytesUsed() *Counter {
	return a.bytesUsed
}

// BytesRefBlockPool is a wrapper around ByteBlockPool for BytesRefHash.
type BytesRefBlockPool struct {
	pool *ByteBlockPool
}

// NewBytesRefBlockPool creates a new BytesRefBlockPool wrapping the given ByteBlockPool.
func NewBytesRefBlockPool(pool *ByteBlockPool) *BytesRefBlockPool {
	return &BytesRefBlockPool{pool: pool}
}

// AddBytesRef adds a BytesRef to the pool and returns the offset.
func (p *BytesRefBlockPool) AddBytesRef(bytes *BytesRef) (int, error) {
	if bytes.Length+2 > ByteBlockSize {
		return 0, NewMaxBytesLengthExceededException(
			fmt.Sprintf("bytes can be at most %d in length; got %d", ByteBlockSize-2, bytes.Length))
	}

	// Get current position before adding
	pos := int(p.pool.GetPosition())

	// Write length as vInt (variable-length int) - simplified to 2 bytes for short lengths
	if bytes.Length < 128 {
		p.pool.Append([]byte{byte(bytes.Length)})
	} else {
		p.pool.Append([]byte{byte(bytes.Length>>8 | 0x80), byte(bytes.Length & 0xFF)})
	}

	// Write the bytes
	p.pool.AppendBytesRef(bytes)

	return pos, nil
}

// FillBytesRef fills the given BytesRef with bytes at the given offset.
func (p *BytesRefBlockPool) FillBytesRef(ref *BytesRef, offset int) {
	// Read length
	firstByte := p.pool.ReadByte(int64(offset))
	length := 0
	if firstByte&0x80 == 0 {
		length = int(firstByte)
		offset++
	} else {
		secondByte := p.pool.ReadByte(int64(offset + 1))
		length = int((firstByte&0x7F)<<8) | int(secondByte)
		offset += 2
	}

	ref.Length = length

	// Check if slice fits in a single block
	bufferIndex := offset >> ByteBlockShift
	pos := offset & ByteBlockMask
	buffer := p.pool.GetBuffer(bufferIndex)

	if pos+length <= ByteBlockSize {
		// Common case: slice lives in a single block
		ref.Bytes = buffer
		ref.Offset = pos
	} else {
		// Uncommon case: slice spans multiple blocks, need to copy
		ref.Bytes = make([]byte, length)
		ref.Offset = 0
		p.pool.ReadBytes(int64(offset), ref.Bytes, 0, length)
	}
}

// Equals checks if the bytes at the given offset equal the given BytesRef.
func (p *BytesRefBlockPool) Equals(offset int, bytes *BytesRef) bool {
	// Read length
	firstByte := p.pool.ReadByte(int64(offset))
	length := 0
	if firstByte&0x80 == 0 {
		length = int(firstByte)
		offset++
	} else {
		secondByte := p.pool.ReadByte(int64(offset + 1))
		length = int((firstByte&0x7F)<<8) | int(secondByte)
		offset += 2
	}

	if length != bytes.Length {
		return false
	}

	// Compare bytes
	bufferIndex := offset >> ByteBlockShift
	pos := offset & ByteBlockMask
	buffer := p.pool.GetBuffer(bufferIndex)

	if pos+length <= ByteBlockSize {
		// Common case: slice lives in a single block
		return bytesEqual(buffer[pos:pos+length], bytes.Bytes[bytes.Offset:bytes.Offset+bytes.Length])
	}

	// Uncommon case: slice spans multiple blocks
	scratch := make([]byte, length)
	p.pool.ReadBytes(int64(offset), scratch, 0, length)
	return bytesEqual(scratch, bytes.Bytes[bytes.Offset:bytes.Offset+bytes.Length])
}

// Hash returns the hash code for the bytes at the given offset.
func (p *BytesRefBlockPool) Hash(offset int) int {
	// Read length
	firstByte := p.pool.ReadByte(int64(offset))
	length := 0
	if firstByte&0x80 == 0 {
		length = int(firstByte)
		offset++
	} else {
		secondByte := p.pool.ReadByte(int64(offset + 1))
		length = int((firstByte&0x7F)<<8) | int(secondByte)
		offset += 2
	}

	// Read bytes and compute hash
	bufferIndex := offset >> ByteBlockShift
	pos := offset & ByteBlockMask
	buffer := p.pool.GetBuffer(bufferIndex)

	if pos+length <= ByteBlockSize {
		// Common case: slice lives in a single block
		return doHash(buffer[pos:], 0, length)
	}

	// Uncommon case: slice spans multiple blocks
	scratch := make([]byte, length)
	p.pool.ReadBytes(int64(offset), scratch, 0, length)
	return doHash(scratch, 0, length)
}

// Reset resets the pool.
func (p *BytesRefBlockPool) Reset() {
	p.pool.Reset(false, false)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
