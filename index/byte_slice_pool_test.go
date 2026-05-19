// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"crypto/rand"
	mathrand "math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestByteSlicePool_NewSliceLevelMarker checks that NewSlice marks the last
// byte with the level-0 sentinel 16.
func TestByteSlicePool_NewSliceLevelMarker(t *testing.T) {
	t.Parallel()
	blockPool := util.NewByteBlockPool(util.NewDirectAllocator())
	blockPool.NextBuffer()
	slicePool := NewByteSlicePool(blockPool)

	upto, err := slicePool.NewSlice(ByteSliceFirstLevelSize)
	if err != nil {
		t.Fatalf("NewSlice: %v", err)
	}
	if upto != 0 {
		t.Fatalf("upto = %d, want 0", upto)
	}
	if got := blockPool.Buffer[ByteSliceFirstLevelSize-1]; got != 16 {
		t.Fatalf("last byte = %d, want 16 (level 0)", got)
	}
	if blockPool.ByteUpto != ByteSliceFirstLevelSize {
		t.Fatalf("ByteUpto = %d, want %d", blockPool.ByteUpto, ByteSliceFirstLevelSize)
	}
}

// TestByteSlicePool_NewSliceTooLarge ensures that requesting a slice larger
// than the block size returns an error.
func TestByteSlicePool_NewSliceTooLarge(t *testing.T) {
	t.Parallel()
	blockPool := util.NewByteBlockPool(util.NewDirectAllocator())
	blockPool.NextBuffer()
	slicePool := NewByteSlicePool(blockPool)

	if _, err := slicePool.NewSlice(util.ByteBlockSize + 1); err == nil {
		t.Fatalf("expected error for slice size > block size")
	}
}

// TestByteSlicePool_NewSliceBlockSize verifies that a slice exactly the block
// size is accepted and consumes the whole buffer.
func TestByteSlicePool_NewSliceBlockSize(t *testing.T) {
	t.Parallel()
	blockPool := util.NewByteBlockPool(util.NewDirectAllocator())
	slicePool := NewByteSlicePool(blockPool)

	upto, err := slicePool.NewSlice(util.ByteBlockSize)
	if err != nil {
		t.Fatalf("NewSlice: %v", err)
	}
	if upto != 0 {
		t.Fatalf("upto = %d, want 0", upto)
	}
}

// TestByteSlicePool_AllocKnownSizeSlice exercises the slice-chaining contract:
// when the level-marker byte is hit, AllocKnownSizeSlice should allocate the
// next slice, write the forwarding address, and preserve the 3 data bytes that
// preceded the forwarding address.
func TestByteSlicePool_AllocKnownSizeSlice(t *testing.T) {
	t.Parallel()
	bytesUsed := util.NewCounter()
	blockPool := util.NewByteBlockPool(util.NewDirectTrackingAllocator(bytesUsed))
	blockPool.NextBuffer()
	slicePool := NewByteSlicePool(blockPool)

	rng := mathrand.New(mathrand.NewPCG(42, 7))
	for i := 0; i < 100; i++ {
		var size int
		if rng.IntN(2) == 0 {
			size = 100 + rng.IntN(901) // [100, 1000]
		} else {
			size = 50000 + rng.IntN(50001) // [50000, 100000]
		}
		data := make([]byte, size)
		if _, err := rand.Read(data); err != nil {
			t.Fatalf("rand.Read: %v", err)
		}

		upto, err := slicePool.NewSlice(ByteSliceFirstLevelSize)
		if err != nil {
			t.Fatalf("NewSlice: %v", err)
		}

		for offset := 0; offset < size; {
			if blockPool.Buffer[upto]&16 == 0 {
				blockPool.Buffer[upto] = data[offset]
				upto++
				offset++
			} else {
				offsetAndLength := slicePool.AllocKnownSizeSlice(blockPool.Buffer, upto)
				sliceLength := offsetAndLength & 0xff
				upto = offsetAndLength >> 8
				if blockPool.Buffer[upto+sliceLength-1] == 0 {
					t.Fatalf("level marker at %d unexpectedly zero", upto+sliceLength-1)
				}
				if blockPool.Buffer[upto] != 0 {
					t.Fatalf("first byte of new slice at %d expected 0, got %d", upto, blockPool.Buffer[upto])
				}
				writeLength := sliceLength - 1
				if writeLength > size-offset {
					writeLength = size - offset
				}
				copy(blockPool.Buffer[upto:upto+writeLength], data[offset:offset+writeLength])
				offset += writeLength
				upto += writeLength
			}
		}
	}
}

// TestByteSlicePool_RoundTripWithReader writes a sequence of bytes through the
// pool's chained slices and reads them back via ByteSliceReader, ensuring
// end-to-end byte parity with the Lucene wire format.
func TestByteSlicePool_RoundTripWithReader(t *testing.T) {
	t.Parallel()
	blockPool := util.NewByteBlockPool(util.NewDirectAllocator())
	blockPool.NextBuffer()
	slicePool := NewByteSlicePool(blockPool)

	const size = 4096
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	startOffset := blockPool.ByteOffset + blockPool.ByteUpto
	upto, err := slicePool.NewSlice(ByteSliceFirstLevelSize)
	if err != nil {
		t.Fatalf("NewSlice: %v", err)
	}
	for offset := 0; offset < size; {
		if blockPool.Buffer[upto]&16 == 0 {
			blockPool.Buffer[upto] = data[offset]
			upto++
			offset++
			continue
		}
		offsetAndLength := slicePool.AllocKnownSizeSlice(blockPool.Buffer, upto)
		sliceLength := offsetAndLength & 0xff
		upto = offsetAndLength >> 8
		writeLength := sliceLength - 1
		if writeLength > size-offset {
			writeLength = size - offset
		}
		copy(blockPool.Buffer[upto:upto+writeLength], data[offset:offset+writeLength])
		offset += writeLength
		upto += writeLength
	}
	endOffset := blockPool.ByteOffset + upto

	var reader ByteSliceReader
	if err := reader.Init(blockPool, startOffset, endOffset); err != nil {
		t.Fatalf("reader.Init: %v", err)
	}
	got := make([]byte, size)
	if err := reader.ReadBytes(got); err != nil {
		t.Fatalf("reader.ReadBytes: %v", err)
	}
	for i := range data {
		if got[i] != data[i] {
			t.Fatalf("byte %d: got %d, want %d", i, got[i], data[i])
		}
	}
}
