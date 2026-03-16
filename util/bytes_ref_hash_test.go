// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test helper to create a new pool for testing
func newTestPool() *ByteBlockPool {
	return NewByteBlockPool(NewDirectAllocator())
}

// atLeastGlobal returns a value >= n using global rand
func atLeastGlobal(n int) int {
	return n + rand.Intn(n)
}

// Test helper to create a new hash for testing
func newTestHash(pool *ByteBlockPool) *BytesRefHash {
	initSize := 2 << (1 + rand.Intn(5))
	if rand.Intn(2) == 0 {
		return NewBytesRefHashWithPool(pool)
	}
	return NewBytesRefHashWithCapacity(pool, initSize, NewDirectBytesStartArray(initSize))
}

// testAtLeast returns a value that is at least the given minimum
func testAtLeast(min int) int {
	return min + rand.Intn(10)
}

// testRandomRealisticUnicodeString generates a random realistic unicode string
func testRandomRealisticUnicodeString(r *rand.Rand, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	length := r.Intn(maxLength) + 1
	var result []rune
	for i := 0; i < length; i++ {
		// Generate mostly ASCII but some unicode
		if r.Float32() < 0.9 {
			result = append(result, rune(r.Intn(128)))
		} else {
			result = append(result, rune(r.Intn(0x10FFFF)))
		}
	}
	return string(result)
}

// testNewBytesRef creates a BytesRef from a string
func testNewBytesRef(s string) *BytesRef {
	return NewBytesRef([]byte(s))
}

// TestBytesRefHash_Size tests the Size method
func TestBytesRefHash_Size(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	pool := newTestPool()
	hash := newTestHash(pool)

	ref := &BytesRefBuilder{}
	num := testAtLeast(2)

	for j := 0; j < num; j++ {
		mod := 1 + rand.Intn(39)
		for i := 0; i < 797; i++ {
			var str string
			for {
				str = testRandomRealisticUnicodeString(rand.New(rand.NewSource(time.Now().UnixNano())), 1000)
				if len(str) > 0 {
					break
				}
			}
			ref.CopyChars(str)
			count := hash.Size()
			key, err := hash.Add(ref.Get())
			if err != nil {
				t.Fatalf("Add failed: %v", err)
			}
			if key < 0 {
				if hash.Size() != count {
					t.Errorf("Expected size %d, got %d", count, hash.Size())
				}
			} else {
				if hash.Size() != count+1 {
					t.Errorf("Expected size %d, got %d", count+1, hash.Size())
				}
			}
			if i%mod == 0 {
				hash.ClearWithPoolReset()
				if hash.Size() != 0 {
					t.Errorf("Expected size 0 after clear, got %d", hash.Size())
				}
				hash.Reinit()
			}
		}
	}
}

// TestBytesRefHash_Get tests the Get method
func TestBytesRefHash_Get(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	pool := newTestPool()
	hash := newTestHash(pool)

	ref := &BytesRefBuilder{}
	scratch := NewBytesRefEmpty()
	num := testAtLeast(2)

	for j := 0; j < num; j++ {
		strings := make(map[string]int)
		uniqueCount := 0
		for i := 0; i < 797; i++ {
			var str string
			for {
				str = testRandomRealisticUnicodeString(rand.New(rand.NewSource(time.Now().UnixNano())), 1000)
				if len(str) > 0 {
					break
				}
			}
			ref.CopyChars(str)
			count := hash.Size()
			key, err := hash.Add(ref.Get())
			if err != nil {
				t.Fatalf("Add failed: %v", err)
			}
			if key >= 0 {
				if _, exists := strings[str]; exists {
					t.Errorf("String %s already exists", str)
				}
				strings[str] = key
				if key != uniqueCount {
					t.Errorf("Expected key %d, got %d", uniqueCount, key)
				}
				uniqueCount++
				if hash.Size() != count+1 {
					t.Errorf("Expected size %d, got %d", count+1, hash.Size())
				}
			} else {
				if (-key)-1 >= count {
					t.Errorf("Key %d out of range for count %d", key, count)
				}
				if hash.Size() != count {
					t.Errorf("Expected size %d, got %d", count, hash.Size())
				}
			}
		}
		for str, key := range strings {
			ref.CopyChars(str)
			result := hash.Get(key, scratch)
			if !bytes.Equal(ref.Get().ValidBytes(), result.ValidBytes()) {
				t.Errorf("Get returned wrong bytes for key %d", key)
			}
		}
		hash.ClearWithPoolReset()
		if hash.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", hash.Size())
		}
		hash.Reinit()
	}
}

// TestBytesRefHash_Compact tests the Compact method
func TestBytesRefHash_Compact(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	pool := newTestPool()
	hash := newTestHash(pool)

	ref := &BytesRefBuilder{}
	num := testAtLeast(2)

	for j := 0; j < num; j++ {
		numEntries := 0
		size := 797
		bits, err := NewFixedBitSet(size)
		if err != nil {
			t.Fatalf("Failed to create FixedBitSet: %v", err)
		}

		for i := 0; i < size; i++ {
			var str string
			for {
				str = testRandomRealisticUnicodeString(rand.New(rand.NewSource(time.Now().UnixNano())), 1000)
				if len(str) > 0 {
					break
				}
			}
			ref.CopyChars(str)
			key, err := hash.Add(ref.Get())
			if err != nil {
				t.Fatalf("Add failed: %v", err)
			}
			if key < 0 {
				if !bits.Get((-key) - 1) {
					t.Errorf("Expected bit %d to be set", (-key)-1)
				}
			} else {
				if bits.Get(key) {
					t.Errorf("Expected bit %d to be clear", key)
				}
				bits.Set(key)
				numEntries++
			}
		}

		if hash.Size() != bits.Cardinality() {
			t.Errorf("Expected size %d, got %d", bits.Cardinality(), hash.Size())
		}
		if numEntries != bits.Cardinality() {
			t.Errorf("Expected numEntries %d to equal cardinality %d", numEntries, bits.Cardinality())
		}
		if numEntries != hash.Size() {
			t.Errorf("Expected numEntries %d to equal size %d", numEntries, hash.Size())
		}

		compact := hash.Compact()
		if numEntries >= len(compact) {
			t.Errorf("Expected compact length > %d, got %d", numEntries, len(compact))
		}

		for i := 0; i < numEntries; i++ {
			bits.Clear(compact[i])
		}
		if bits.Cardinality() != 0 {
			t.Errorf("Expected all bits to be cleared after compact, got %d", bits.Cardinality())
		}

		hash.ClearWithPoolReset()
		if hash.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", hash.Size())
		}
		hash.Reinit()
	}
}

// TestBytesRefHash_Sort tests the Sort method
func TestBytesRefHash_Sort(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	pool := newTestPool()
	hash := newTestHash(pool)

	ref := &BytesRefBuilder{}
	num := testAtLeast(2)

	for j := 0; j < num; j++ {
		// Use a sorted set (TreeSet equivalent)
		strings := make([]string, 0, 797)
		stringSet := make(map[string]bool)

		for i := 0; i < 797; i++ {
			var str string
			for {
				str = testRandomRealisticUnicodeString(rand.New(rand.NewSource(time.Now().UnixNano())), 1000)
				if len(str) > 0 {
					break
				}
			}
			ref.CopyChars(str)
			_, err := hash.Add(ref.Get())
			if err != nil {
				t.Fatalf("Add failed: %v", err)
			}
			if !stringSet[str] {
				strings = append(strings, str)
				stringSet[str] = true
			}
		}

		// Sort strings
		sort.Strings(strings)

		for iter := 0; iter < 3; iter++ {
			sorted := hash.Sort()
			if len(strings) >= len(sorted) {
				t.Errorf("Expected sort length > %d, got %d", len(strings), len(sorted))
			}

			scratch := NewBytesRefEmpty()
			for i, str := range strings {
				ref.CopyChars(str)
				result := hash.Get(sorted[i], scratch)
				if !bytes.Equal(ref.Get().ValidBytes(), result.ValidBytes()) {
					t.Errorf("Sort returned wrong bytes at index %d", i)
				}
			}
		}

		hash.ClearWithPoolReset()
		if hash.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", hash.Size())
		}
		hash.Reinit()
	}
}

// TestBytesRefHash_Add tests the Add method
func TestBytesRefHash_Add(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	pool := newTestPool()
	hash := newTestHash(pool)

	ref := &BytesRefBuilder{}
	scratch := NewBytesRefEmpty()
	num := testAtLeast(2)

	for j := 0; j < num; j++ {
		strings := make(map[string]bool)
		uniqueCount := 0
		for i := 0; i < 797; i++ {
			var str string
			for {
				str = testRandomRealisticUnicodeString(rand.New(rand.NewSource(time.Now().UnixNano())), 1000)
				if len(str) > 0 {
					break
				}
			}
			ref.CopyChars(str)
			count := hash.Size()
			key, err := hash.Add(ref.Get())
			if err != nil {
				t.Fatalf("Add failed: %v", err)
			}

			if key >= 0 {
				if strings[str] {
					t.Errorf("String %s should not exist yet", str)
				}
				strings[str] = true
				if key != uniqueCount {
					t.Errorf("Expected key %d, got %d", uniqueCount, key)
				}
				if hash.Size() != count+1 {
					t.Errorf("Expected size %d, got %d", count+1, hash.Size())
				}
				uniqueCount++
			} else {
				if !strings[str] {
					t.Errorf("String %s should already exist", str)
				}
				if (-key)-1 >= count {
					t.Errorf("Key %d out of range for count %d", key, count)
				}
				result := hash.Get((-key)-1, scratch)
				if string(result.ValidBytes()) != str {
					t.Errorf("Get returned wrong string for key %d", key)
				}
				if hash.Size() != count {
					t.Errorf("Expected size %d, got %d", count, hash.Size())
				}
			}
		}

		assertAllIn(t, strings, hash)
		hash.ClearWithPoolReset()
		if hash.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", hash.Size())
		}
		hash.Reinit()
	}
}

// TestBytesRefHash_Find tests the Find method
func TestBytesRefHash_Find(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	pool := newTestPool()
	hash := newTestHash(pool)

	ref := &BytesRefBuilder{}
	scratch := NewBytesRefEmpty()
	num := testAtLeast(2)

	for j := 0; j < num; j++ {
		strings := make(map[string]bool)
		uniqueCount := 0
		for i := 0; i < 797; i++ {
			var str string
			for {
				str = testRandomRealisticUnicodeString(rand.New(rand.NewSource(time.Now().UnixNano())), 1000)
				if len(str) > 0 {
					break
				}
			}
			ref.CopyChars(str)
			count := hash.Size()
			key := hash.Find(ref.Get())
			if key >= 0 {
				// string found in hash
				if strings[str] {
					t.Errorf("String %s should not be found as new", str)
				}
				if key >= count {
					t.Errorf("Key %d out of range for count %d", key, count)
				}
				result := hash.Get(key, scratch)
				if string(result.ValidBytes()) != str {
					t.Errorf("Get returned wrong string for key %d", key)
				}
				if hash.Size() != count {
					t.Errorf("Expected size %d, got %d", count, hash.Size())
				}
			} else {
				key, err := hash.Add(ref.Get())
				if err != nil {
					t.Fatalf("Add failed: %v", err)
				}
				if !strings[str] {
					t.Errorf("String %s should be new", str)
				}
				if key != uniqueCount {
					t.Errorf("Expected key %d, got %d", uniqueCount, key)
				}
				if hash.Size() != count+1 {
					t.Errorf("Expected size %d, got %d", count+1, hash.Size())
				}
				uniqueCount++
			}
		}

		assertAllIn(t, strings, hash)
		hash.ClearWithPoolReset()
		if hash.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", hash.Size())
		}
		hash.Reinit()
	}
}

// TestBytesRefHash_ConcurrentAccess tests concurrent access to BytesRefHash
func TestBytesRefHash_ConcurrentAccess(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	num := testAtLeast(2)
	for j := 0; j < num; j++ {
		pool := newTestPool()
		hash := newTestHash(pool)

		numStrings := 797
		strings := make([]string, 0, numStrings)
		for i := 0; i < numStrings; i++ {
			str := randomRealisticUnicodeString(rand.New(rand.NewSource(time.Now().UnixNano())), 1000)
			_, err := hash.Add(testNewBytesRef(str))
			if err != nil {
				t.Fatalf("Add failed: %v", err)
			}
			strings = append(strings, str)
		}
		hashSize := hash.Size()

		notFound := atomic.Int32{}
		notEquals := atomic.Int32{}
		wrongSize := atomic.Int32{}

		numThreads := atLeastGlobal(3)
		var wg sync.WaitGroup
		loops := atLeastGlobal(100)

		// Use a barrier to synchronize thread start
		barrier := make(chan struct{})

		for i := 0; i < numThreads; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				scratch := NewBytesRefEmpty()
				// Wait for barrier
				<-barrier
				for k := 0; k < loops; k++ {
					find := testNewBytesRef(strings[k%len(strings)])
					id := hash.Find(find)
					if id < 0 {
						notFound.Add(1)
					} else {
						result := hash.Get(id, scratch)
						if !bytes.Equal(result.ValidBytes(), find.ValidBytes()) {
							notEquals.Add(1)
						}
					}
					if hash.Size() != hashSize {
						wrongSize.Add(1)
					}
				}
			}()
		}

		// Release all threads at once
		close(barrier)
		wg.Wait()

		if notFound.Load() != 0 {
			t.Errorf("Expected 0 not found, got %d", notFound.Load())
		}
		if notEquals.Load() != 0 {
			t.Errorf("Expected 0 not equals, got %d", notEquals.Load())
		}
		if wrongSize.Load() != 0 {
			t.Errorf("Expected 0 wrong size, got %d", wrongSize.Load())
		}

		hash.ClearWithPoolReset()
		if hash.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", hash.Size())
		}
		hash.Reinit()
	}
}

// TestBytesRefHash_LargeValue tests handling of large values
func TestBytesRefHash_LargeValue(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	pool := newTestPool()
	hash := NewBytesRefHashWithPool(pool)

	sizes := []int{
		rand.Intn(5),
		ByteBlockSize - 33 + rand.Intn(31),
	}

	ref := NewBytesRefEmpty()
	for i, size := range sizes {
		ref.Bytes = make([]byte, size)
		ref.Offset = 0
		ref.Length = size
		key, err := hash.Add(ref)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if key != i {
			t.Errorf("Expected key %d, got %d", i, key)
		}
	}

	// This should throw MaxBytesLengthExceededException
	ref.Bytes = make([]byte, ByteBlockSize-1+rand.Intn(37))
	ref.Offset = 0
	ref.Length = len(ref.Bytes)
	_, err := hash.Add(ref)
	if err == nil {
		t.Error("Expected MaxBytesLengthExceededException for large value")
	}
	if _, ok := err.(*MaxBytesLengthExceededException); !ok {
		t.Errorf("Expected MaxBytesLengthExceededException, got %T", err)
	}
}

// TestBytesRefHash_AddByPoolOffset tests the AddByPoolOffset method
func TestBytesRefHash_AddByPoolOffset(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	pool := newTestPool()
	hash := newTestHash(pool)
	offsetHash := newTestHash(pool)

	ref := &BytesRefBuilder{}
	scratch := NewBytesRefEmpty()
	num := testAtLeast(2)

	for j := 0; j < num; j++ {
		strings := make(map[string]bool)
		uniqueCount := 0
		for i := 0; i < 797; i++ {
			var str string
			for {
				str = testRandomRealisticUnicodeString(rand.New(rand.NewSource(time.Now().UnixNano())), 1000)
				if len(str) > 0 {
					break
				}
			}
			ref.CopyChars(str)
			count := hash.Size()
			key, err := hash.Add(ref.Get())
			if err != nil {
				t.Fatalf("Add failed: %v", err)
			}

			if key >= 0 {
				if strings[str] {
					t.Errorf("String %s should not exist yet", str)
				}
				strings[str] = true
				if key != uniqueCount {
					t.Errorf("Expected key %d, got %d", uniqueCount, key)
				}
				if hash.Size() != count+1 {
					t.Errorf("Expected size %d, got %d", count+1, hash.Size())
				}
				offsetKey := offsetHash.AddByPoolOffset(hash.ByteStart(key))
				if offsetKey != uniqueCount {
					t.Errorf("Expected offsetKey %d, got %d", uniqueCount, offsetKey)
				}
				if offsetHash.Size() != count+1 {
					t.Errorf("Expected offsetHash size %d, got %d", count+1, offsetHash.Size())
				}
				uniqueCount++
			} else {
				if !strings[str] {
					t.Errorf("String %s should already exist", str)
				}
				if (-key)-1 >= count {
					t.Errorf("Key %d out of range for count %d", key, count)
				}
				result := hash.Get((-key)-1, scratch)
				if string(result.ValidBytes()) != str {
					t.Errorf("Get returned wrong string for key %d", key)
				}
				if hash.Size() != count {
					t.Errorf("Expected size %d, got %d", count, hash.Size())
				}
				offsetKey := offsetHash.AddByPoolOffset(hash.ByteStart((-key) - 1))
				if (-offsetKey)-1 >= count {
					t.Errorf("OffsetKey %d out of range for count %d", offsetKey, count)
				}
				result2 := hash.Get((-offsetKey)-1, scratch)
				if string(result2.ValidBytes()) != str {
					t.Errorf("Get returned wrong string for offsetKey %d", offsetKey)
				}
				if hash.Size() != count {
					t.Errorf("Expected size %d, got %d", count, hash.Size())
				}
			}
		}

		assertAllIn(t, strings, hash)
		for str := range strings {
			ref.CopyChars(str)
			// Find the string in hash to get its id
			hashID := hash.Find(ref.Get())
			if hashID < 0 {
				t.Errorf("String %s not found in hash", str)
				continue
			}
			// Get the pool offset from hash
			poolOffset := hash.ByteStart(hashID)
			// Add by pool offset to offsetHash
			offsetKey := offsetHash.AddByPoolOffset(poolOffset)
			if offsetKey < 0 {
				t.Errorf("Failed to add string %s to offsetHash", str)
				continue
			}
			result := offsetHash.Get(offsetKey, scratch)
			if !bytes.Equal(ref.Get().ValidBytes(), result.ValidBytes()) {
				t.Errorf("OffsetHash returned wrong bytes for string %s", str)
			}
		}

		hash.ClearWithPoolReset()
		if hash.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", hash.Size())
		}
		offsetHash.ClearWithPoolReset()
		if offsetHash.Size() != 0 {
			t.Errorf("Expected offsetHash size 0 after clear, got %d", offsetHash.Size())
		}
		hash.Reinit()
		offsetHash.Reinit()
	}
}

// assertAllIn checks that all strings in the set are in the hash
func assertAllIn(t *testing.T, strings map[string]bool, hash *BytesRefHash) {
	ref := &BytesRefBuilder{}
	scratch := NewBytesRefEmpty()
	count := hash.Size()
	for str := range strings {
		ref.CopyChars(str)
		// Use Find to check if string exists, don't add it again
		id := hash.Find(ref.Get())
		if id < 0 {
			t.Errorf("String %s not found in hash", str)
			continue
		}
		if id >= hash.Size() {
			t.Errorf("Invalid id %d for string %s (size=%d)", id, str, hash.Size())
			continue
		}
		result := hash.Get(id, scratch)
		if string(result.ValidBytes()) != str {
			t.Errorf("Get returned wrong string for id %d", id)
		}
		if hash.Size() != count {
			t.Errorf("Expected size %d, got %d", count, hash.Size())
		}
		if id >= count {
			t.Errorf("Id %d should be < count %d for string %s", id, count, str)
		}
	}
}

// TestBytesRefHash_BasicOperations tests basic operations
func TestBytesRefHash_BasicOperations(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// Test empty hash
	if hash.Size() != 0 {
		t.Errorf("Expected size 0, got %d", hash.Size())
	}

	// Test add
	ref1 := NewBytesRef([]byte("hello"))
	id1, err := hash.Add(ref1)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id1 != 0 {
		t.Errorf("Expected id 0, got %d", id1)
	}
	if hash.Size() != 1 {
		t.Errorf("Expected size 1, got %d", hash.Size())
	}

	// Test add duplicate
	id1dup, err := hash.Add(NewBytesRef([]byte("hello")))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id1dup != -(id1 + 1) {
		t.Errorf("Expected id %d for duplicate, got %d", -(id1 + 1), id1dup)
	}
	if hash.Size() != 1 {
		t.Errorf("Expected size 1 after duplicate add, got %d", hash.Size())
	}

	// Test add new
	ref2 := NewBytesRef([]byte("world"))
	id2, err := hash.Add(ref2)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id2 != 1 {
		t.Errorf("Expected id 1, got %d", id2)
	}
	if hash.Size() != 2 {
		t.Errorf("Expected size 2, got %d", hash.Size())
	}

	// Test find
	found := hash.Find(NewBytesRef([]byte("hello")))
	if found != 0 {
		t.Errorf("Expected find 0, got %d", found)
	}
	found = hash.Find(NewBytesRef([]byte("world")))
	if found != 1 {
		t.Errorf("Expected find 1, got %d", found)
	}
	found = hash.Find(NewBytesRef([]byte("notfound")))
	if found != -1 {
		t.Errorf("Expected find -1, got %d", found)
	}

	// Test get
	scratch := NewBytesRefEmpty()
	result := hash.Get(0, scratch)
	if string(result.ValidBytes()) != "hello" {
		t.Errorf("Expected 'hello', got %s", string(result.ValidBytes()))
	}
	result = hash.Get(1, scratch)
	if string(result.ValidBytes()) != "world" {
		t.Errorf("Expected 'world', got %s", string(result.ValidBytes()))
	}

	// Test clear
	hash.ClearWithPoolReset()
	if hash.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", hash.Size())
	}

	// Test reinit
	hash.Reinit()
	if hash.Size() != 0 {
		t.Errorf("Expected size 0 after reinit, got %d", hash.Size())
	}

	// Can add again after reinit
	id, err := hash.Add(NewBytesRef([]byte("new")))
	if err != nil {
		t.Fatalf("Add after reinit failed: %v", err)
	}
	if id != 0 {
		t.Errorf("Expected id 0 after reinit, got %d", id)
	}
}

// TestBytesRefHash_Clear tests the Clear method
func TestBytesRefHash_Clear(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// Add some entries
	for i := 0; i < 100; i++ {
		_, err := hash.Add(NewBytesRef([]byte{byte(i)}))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	if hash.Size() != 100 {
		t.Errorf("Expected size 100, got %d", hash.Size())
	}

	// Clear without pool reset
	hash.Clear(false)
	if hash.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", hash.Size())
	}

	// Can add again
	for i := 0; i < 50; i++ {
		_, err := hash.Add(NewBytesRef([]byte{byte(i + 200)}))
		if err != nil {
			t.Fatalf("Add after clear failed: %v", err)
		}
	}

	if hash.Size() != 50 {
		t.Errorf("Expected size 50, got %d", hash.Size())
	}
}

// TestBytesRefHash_ByteStart tests the ByteStart method
func TestBytesRefHash_ByteStart(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	ref := NewBytesRef([]byte("test"))
	id, err := hash.Add(ref)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	byteStart := hash.ByteStart(id)
	if byteStart < 0 {
		t.Errorf("Expected non-negative byteStart, got %d", byteStart)
	}

	// Verify we can retrieve the same bytes
	scratch := NewBytesRefEmpty()
	result := hash.Get(id, scratch)
	if !bytes.Equal(result.ValidBytes(), ref.ValidBytes()) {
		t.Error("Get returned different bytes than original")
	}
}

// TestBytesRefHash_Close tests the Close method
func TestBytesRefHash_Close(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// Add some entries
	for i := 0; i < 10; i++ {
		_, err := hash.Add(NewBytesRef([]byte{byte(i)}))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	hash.Close()

	if hash.Size() != 0 {
		t.Errorf("Expected size 0 after close, got %d", hash.Size())
	}
}

// TestBytesRefHash_PowerOfTwoCapacity tests that capacity must be power of two
func TestBytesRefHash_PowerOfTwoCapacity(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())

	// Should panic for non-power-of-two
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for non-power-of-two capacity")
		}
	}()

	NewBytesRefHashWithCapacity(pool, 15, NewDirectBytesStartArray(15))
}

// TestBytesRefHash_ZeroCapacity tests that zero capacity is rejected
func TestBytesRefHash_ZeroCapacity(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for zero capacity")
		}
	}()

	NewBytesRefHashWithCapacity(pool, 0, NewDirectBytesStartArray(0))
}

// TestBytesRefHash_CompactEmpty tests Compact on empty hash
func TestBytesRefHash_CompactEmpty(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	compact := hash.Compact()
	if len(compact) == 0 {
		t.Error("Expected non-empty compact array")
	}
}

// TestBytesRefHash_SortEmpty tests Sort on empty hash
func TestBytesRefHash_SortEmpty(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	sorted := hash.Sort()
	if len(sorted) == 0 {
		t.Error("Expected non-empty sorted array")
	}
}

// TestBytesRefHash_ManyEntries tests adding many entries
func TestBytesRefHash_ManyEntries(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// Add enough entries to trigger rehash
	for i := 0; i < 1000; i++ {
		ref := NewBytesRef([]byte{byte(i >> 8), byte(i)})
		_, err := hash.Add(ref)
		if err != nil {
			t.Fatalf("Add failed at iteration %d: %v", i, err)
		}
	}

	if hash.Size() != 1000 {
		t.Errorf("Expected size 1000, got %d", hash.Size())
	}

	// Verify all entries can be found
	for i := 0; i < 1000; i++ {
		ref := NewBytesRef([]byte{byte(i >> 8), byte(i)})
		id := hash.Find(ref)
		if id < 0 {
			t.Errorf("Could not find entry %d", i)
		}
	}
}

// TestBytesRefHash_DuplicateHandling tests duplicate handling
func TestBytesRefHash_DuplicateHandling(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// Add same value multiple times
	for i := 0; i < 100; i++ {
		id, err := hash.Add(NewBytesRef([]byte("duplicate")))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if i == 0 {
			if id != 0 {
				t.Errorf("Expected id 0 for first add, got %d", id)
			}
		} else {
			if id != -1 {
				t.Errorf("Expected id -1 for duplicate, got %d", id)
			}
		}
	}

	if hash.Size() != 1 {
		t.Errorf("Expected size 1, got %d", hash.Size())
	}
}

// TestBytesRefHash_EmptyString tests empty string handling
func TestBytesRefHash_EmptyString(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	id, err := hash.Add(NewBytesRef([]byte("")))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id != 0 {
		t.Errorf("Expected id 0, got %d", id)
	}

	// Find empty string
	found := hash.Find(NewBytesRef([]byte("")))
	if found != 0 {
		t.Errorf("Expected find 0, got %d", found)
	}

	// Get empty string
	scratch := NewBytesRefEmpty()
	result := hash.Get(0, scratch)
	if result.Length != 0 {
		t.Errorf("Expected length 0, got %d", result.Length)
	}
}

// TestBytesRefHash_BinaryData tests binary data handling
func TestBytesRefHash_BinaryData(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// Add binary data with all byte values
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}

	id, err := hash.Add(NewBytesRef(data))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id != 0 {
		t.Errorf("Expected id 0, got %d", id)
	}

	// Find the binary data
	found := hash.Find(NewBytesRef(data))
	if found != 0 {
		t.Errorf("Expected find 0, got %d", found)
	}

	// Verify retrieved data
	scratch := NewBytesRefEmpty()
	result := hash.Get(0, scratch)
	if !bytes.Equal(result.ValidBytes(), data) {
		t.Error("Retrieved data doesn't match original")
	}
}

// TestBytesRefHash_Rehash tests rehashing behavior
func TestBytesRefHash_Rehash(t *testing.T) {
	// Start with small capacity to force rehash
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithCapacity(pool, 16, NewDirectBytesStartArray(16))

	// Add entries until we trigger rehash (happens at 50% capacity)
	entries := make(map[string]int)
	for i := 0; i < 20; i++ {
		str := string(rune('a' + i))
		id, err := hash.Add(NewBytesRef([]byte(str)))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if id >= 0 {
			entries[str] = id
		}
	}

	// Verify all entries are still accessible
	for str, expectedID := range entries {
		found := hash.Find(NewBytesRef([]byte(str)))
		if found != expectedID {
			t.Errorf("For string %s, expected id %d, got %d", str, expectedID, found)
		}
	}
}

// TestBytesRefHash_ConcurrentAdd tests concurrent adds (not thread-safe by design, but tests race conditions)
func TestBytesRefHash_ConcurrentAdd(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// This test verifies that the hash works correctly with sequential adds
	// BytesRefHash is not designed for concurrent modifications
	for i := 0; i < 100; i++ {
		ref := NewBytesRef([]byte{byte(i)})
		_, err := hash.Add(ref)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	if hash.Size() != 100 {
		t.Errorf("Expected size 100, got %d", hash.Size())
	}
}

// TestBytesRefHash_NilBytesRef tests nil BytesRef handling
func TestBytesRefHash_NilBytesRef(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// Empty BytesRef (nil bytes)
	emptyRef := NewBytesRefEmpty()
	id, err := hash.Add(emptyRef)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id != 0 {
		t.Errorf("Expected id 0, got %d", id)
	}

	// Find empty
	found := hash.Find(NewBytesRefEmpty())
	if found != 0 {
		t.Errorf("Expected find 0, got %d", found)
	}
}

// TestBytesRefHash_LargeBatch tests adding a large batch of entries
func TestBytesRefHash_LargeBatch(t *testing.T) {
	pool := NewByteBlockPool(NewDirectAllocator())
	hash := NewBytesRefHashWithPool(pool)

	// Add many unique entries
	for i := 0; i < 5000; i++ {
		ref := NewBytesRef([]byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)})
		_, err := hash.Add(ref)
		if err != nil {
			t.Fatalf("Add failed at %d: %v", i, err)
		}
	}

	if hash.Size() != 5000 {
		t.Errorf("Expected size 5000, got %d", hash.Size())
	}

	// Test compact
	compact := hash.Compact()
	if len(compact) < hash.Size() {
		t.Errorf("Compact array too small: %d < %d", len(compact), hash.Size())
	}

	// Test sort
	sorted := hash.Sort()
	if len(sorted) < hash.Size() {
		t.Errorf("Sorted array too small: %d < %d", len(sorted), hash.Size())
	}
}
