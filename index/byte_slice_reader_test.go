// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestByteSliceReader.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// byteSliceReaderClass holds the static fields RANDOM_DATA, BLOCK_POOL and
// BLOCK_POOL_END that @BeforeClass initializes once for the class.
var byteSliceReaderClass struct {
	once         sync.Once
	randomData   []byte
	blockPool    *util.ByteBlockPool
	blockPoolEnd int
}

// byteSliceReaderBeforeClass ports beforeClass().
func byteSliceReaderBeforeClass(t *testing.T) {
	t.Helper()
	c := &byteSliceReaderClass
	c.once.Do(func() {
		length := atLeast(100)
		c.randomData = make([]byte, length)
		rand.Read(c.randomData)

		c.blockPool = util.NewByteBlockPool(util.NewDirectAllocator())
		c.blockPool.NextBuffer()
		slicePool := index.NewByteSlicePool(c.blockPool)
		buffer := c.blockPool.Buffer
		upto, err := slicePool.NewSlice(index.ByteSliceFirstLevelSize)
		if err != nil {
			t.Fatalf("newSlice: %v", err)
		}
		for _, randomByte := range c.randomData {
			if buffer[upto]&16 != 0 {
				upto = slicePool.AllocSlice(buffer, upto)
				buffer = c.blockPool.Buffer
			}
			buffer[upto] = randomByte
			upto++
		}
		c.blockPoolEnd = upto
	})
}

func byteSliceReaderReadByte(t *testing.T, r *index.ByteSliceReader) byte {
	t.Helper()
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("readByte: %v", err)
	}
	return b
}

func TestByteSliceReaderReadByte(t *testing.T) {
	byteSliceReaderBeforeClass(t)
	c := &byteSliceReaderClass
	sliceReader := &index.ByteSliceReader{}
	if err := sliceReader.Init(c.blockPool, 0, c.blockPoolEnd); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, expected := range c.randomData {
		if got := byteSliceReaderReadByte(t, sliceReader); got != expected {
			t.Fatalf("expected %d, got %d", expected, got)
		}
	}
}

func TestByteSliceReaderSkipBytes(t *testing.T) {
	byteSliceReaderBeforeClass(t)
	c := &byteSliceReaderClass
	sliceReader := &index.ByteSliceReader{}

	maxSkipTo := len(c.randomData) - 1
	iterations := atLeast(10)
	for i := 0; i < iterations; i++ {
		if err := sliceReader.Init(c.blockPool, 0, c.blockPoolEnd); err != nil {
			t.Fatalf("init: %v", err)
		}
		// skip random chunks of bytes until exhausted
		for curr := 0; curr < maxSkipTo; {
			skipTo := nextInt(curr, maxSkipTo)
			step := skipTo - curr
			if err := sliceReader.SkipBytes(int64(step)); err != nil {
				t.Fatalf("skipBytes: %v", err)
			}
			if got := byteSliceReaderReadByte(t, sliceReader); got != c.randomData[skipTo] {
				t.Fatalf("expected %d, got %d", c.randomData[skipTo], got)
			}
			curr = skipTo + 1 // +1 for read byte
		}
	}
}
