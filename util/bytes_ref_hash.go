// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"time"
)

// BytesRefHash is a special purpose hash-map like data-structure optimized for
// BytesRef instances. BytesRefHash maintains mappings of byte arrays to ids
// (Map<BytesRef,int>) storing the hashed bytes efficiently in continuous storage.
// The mapping to the id is encapsulated inside BytesRefHash and is guaranteed
// to be increased for each added BytesRef.
//
// Note: The maximum capacity BytesRef instance passed to Add must not be
// longer than ByteBlockPool.BYTE_BLOCK_SIZE-2. The internal storage is limited to 2GB
// total byte storage.
//
// This is the Go port of Lucene's org.apache.lucene.util.BytesRefHash.
type BytesRefHash struct {
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

// RamBytesUsed returns the total amount of RAM, in bytes, consumed by
// this object and any sub-objects it owns.
func (h *BytesRefHash) RamBytesUsed() int64 {
	return h.bytesUsed.Get()
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
		pool:            NewBytesRefBlockPoolWithPool(pool),
		ids:             make([]int, capacity),
		bytesStartArray: bytesStartArray,
		lastCount:       -1,
	}

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
func (h *BytesRefHash) Compact() []int {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}

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
func (h *BytesRefHash) Sort() []int {
	compact := h.Compact()
	count := h.count
	if count == 0 {
		return compact
	}

	// We use a specialized Sortable that implements the bucket cache optimization.
	ss := &bytesRefHashSortable{
		hash:     h,
		indices:  compact,
		count:    count,
		scratch1: NewBytesRefEmpty(),
		scratch2: NewBytesRefEmpty(),
		pivot:    NewBytesRefEmpty(),
	}

	maxLength := 0
	for i := 0; i < count; i++ {
		ref := NewBytesRefEmpty()
		h.Get(compact[i], ref)
		if ref.Length > maxLength {
			maxLength = ref.Length
		}
	}

	NewMSBRadixSorter(maxLength).Sort(ss, 0, count)

	for i := count; i < len(compact); i++ {
		compact[i] = -1
	}

	return compact
}

type bytesRefHashSortable struct {
	hash     *BytesRefHash
	indices  []int
	count    int
	scratch1 *BytesRef
	scratch2 *BytesRef
	pivot    *BytesRef
}

func (s *bytesRefHashSortable) Compare(i, j int) int {
	s.get(s.scratch1, i)
	s.get(s.scratch2, j)
	return bytesCompare(s.scratch1.ValidBytes(), s.scratch2.ValidBytes())
}

func (s *bytesRefHashSortable) Swap(i, j int) {
	s.indices[i], s.indices[j] = s.indices[j], s.indices[i]
}

func (s *bytesRefHashSortable) ByteAt(i, k int) int {
	s.get(s.scratch1, i)
	if k >= s.scratch1.Length {
		return -1
	}
	return int(s.scratch1.Bytes[s.scratch1.Offset+k])
}

func (s *bytesRefHashSortable) SetPivot(i int) {
	s.get(s.pivot, i)
}

func (s *bytesRefHashSortable) ComparePivot(j int) int {
	s.get(s.scratch1, j)
	return bytesCompare(s.pivot.ValidBytes(), s.scratch1.ValidBytes())
}

func (s *bytesRefHashSortable) get(dest *BytesRef, i int) {
	if dest == nil {
		dest = NewBytesRefEmpty()
	}
	s.hash.Get(s.indices[i], dest)
}

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
	newSize := h.hashSize
	for newSize >= 8 && newSize/4 > targetSize {
		newSize /= 2
	}
	if newSize != h.hashSize {
		h.bytesUsed.AddAndGet(int64(-4 * (h.hashSize - newSize)))
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

func (h *BytesRefHash) Clear(resetPool bool) {
	h.lastCount = h.count
	h.count = 0
	if resetPool {
		h.pool.Reset()
	}
	h.bytesStartArray.Clear()
	h.bytesStart = h.bytesStartArray.Init()
	if h.lastCount != -1 && h.shrink(h.lastCount) {
		return
	}
	for i := range h.ids {
		h.ids[i] = -1
	}
}

func (h *BytesRefHash) ClearWithPoolReset() {
	h.Clear(true)
}

func (h *BytesRefHash) Close() {
	h.Clear(true)
	h.ids = nil
	h.bytesUsed.AddAndGet(int64(-4 * h.hashSize))
}

func (h *BytesRefHash) Add(bytes *BytesRef) (int, error) {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}

	if bytes.Length > ByteBlockSize-2 {
		return 0, NewMaxBytesLengthExceededException(
			fmt.Sprintf("bytes can be at most %d in length; got %d", ByteBlockSize-2, bytes.Length))
	}

	hashcode := doHash(bytes.Bytes, bytes.Offset, bytes.Length)
	hashPos := h.findHash(bytes, hashcode)
	e := h.ids[hashPos]

	if e == -1 {
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

	for e != -1 && ((e&h.highMask) != highBits || !h.pool.Equals(h.bytesStart[e&h.hashMask], bytes)) {
		code++
		hashPos = code & h.hashMask
		e = h.ids[hashPos]
	}

	return hashPos
}

func (h *BytesRefHash) AddByPoolOffset(offset int) int {
	if h.bytesStart == nil {
		panic("bytesStart is nil - not initialized")
	}

	code := offset
	hashPos := offset & h.hashMask
	e := h.ids[hashPos]

	for e != -1 && h.bytesStart[e&h.hashMask] != offset {
		code++
		hashPos = code & h.hashMask
		e = h.ids[hashPos]
	}
	if e == -1 {
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

func (h *BytesRefHash) rehash(newSize int, hashOnData bool) {
	newMask := newSize - 1
	newHighMask := ^newMask
	h.bytesUsed.AddAndGet(int64(4 * newSize))
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
			for newHash[hashPos] != -1 {
				code++
				hashPos = code & newMask
			}

			newHash[hashPos] = e0 | (hashcode & newHighMask)
		}
	}

	h.hashMask = newMask
	h.highMask = newHighMask
	h.bytesUsed.AddAndGet(int64(-4 * len(h.ids)))
	h.ids = newHash
	h.hashSize = newSize
	h.hashHalfSize = newSize / 2
}

func doHash(bytes []byte, offset, length int) int {
	return MurmurHash3_x86_32(bytes, offset, length, GoodFastHashSeed)
}

var GoodFastHashSeed = int(uint32(initGoodFastHashSeed()))

func initGoodFastHashSeed() int64 {
	return time.Now().UnixNano()
}

func SetGoodFastHashSeed(seed int) {
	GoodFastHashSeed = seed
}

func IsPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

func (h *BytesRefHash) Reinit() {
	if h.bytesStart == nil {
		h.bytesStart = h.bytesStartArray.Init()
	}

	if h.ids == nil {
		h.ids = make([]int, h.hashSize)
		for i := range h.ids {
			h.ids[i] = -1
		}
		h.bytesUsed.AddAndGet(int64(4 * h.hashSize))
	}
}

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
	Init() []int
	Grow() []int
	Clear() []int
	BytesUsed() *Counter
}

// DirectBytesStartArray is a simple BytesStartArray that tracks memory allocation
// using a private Counter instance.
type DirectBytesStartArray struct {
	initSize   int
	bytesStart []int
	bytesUsed  *Counter
}

func NewDirectBytesStartArray(initSize int) *DirectBytesStartArray {
	return NewDirectBytesStartArrayWithCounter(initSize, NewCounter())
}

func NewDirectBytesStartArrayWithCounter(initSize int, counter *Counter) *DirectBytesStartArray {
	return &DirectBytesStartArray{
		initSize:  initSize,
		bytesUsed: counter,
	}
}

func (a *DirectBytesStartArray) Init() []int {
	a.bytesStart = make([]int, Oversize(a.initSize, 4))
	return a.bytesStart
}

func (a *DirectBytesStartArray) Grow() []int {
	if a.bytesStart == nil {
		panic("bytesStart is nil")
	}
	newSize := Oversize(len(a.bytesStart)+1, 4)
	newBytesStart := make([]int, newSize)
	copy(newBytesStart, a.bytesStart)
	a.bytesStart = newBytesStart
	return a.bytesStart
}

func (a *DirectBytesStartArray) Clear() []int {
	a.bytesStart = nil
	return nil
}

func (a *DirectBytesStartArray) BytesUsed() *Counter {
	return a.bytesUsed
}

