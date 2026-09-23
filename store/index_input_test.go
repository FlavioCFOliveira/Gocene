// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexInput.java
// (Apache Lucene 10.5.0). The Java class lives in package index but tests only
// store types (ByteArrayDataInput, Directory inputs, IndexInput.skipBytes), so
// the Go port lives beside those types.
//
// LuceneTestCase.newDirectory() picks a random Directory implementation from
// the test framework; that randomisation is not ported, so the port uses
// ByteBuffersDirectory, one of the implementations newDirectory() can return.

import (
	"math"
	"math/rand/v2"
	"sync"
	"testing"
	"time"
)

// TestIndexInput.READ_TEST_BYTES is ported as readTestBytes in
// filter_index_input_test.go (TestFilterIndexInput extends TestIndexInput in
// Java and shares the fixture and InterceptingIndexInput).

// indexInputCount mirrors COUNT = RANDOM_MULTIPLIER * 65536 (RANDOM_MULTIPLIER
// defaults to 1).
const indexInputCount = 1 * 65536

// indexInputTestNightly mirrors LuceneTestCase.TEST_NIGHTLY (false by default).
const indexInputTestNightly = false

var (
	indexInputFixtureOnce sync.Once
	indexInputInts        []int32
	indexInputLongs       []int64
	indexInputRandomBytes []byte
	indexInputSeed        uint64
)

// indexInputBeforeClass mirrors the @BeforeClass method of TestIndexInput.
func indexInputBeforeClass(t *testing.T) {
	t.Helper()
	indexInputFixtureOnce.Do(func() {
		indexInputSeed = uint64(time.Now().UnixNano())
		random := rand.New(rand.NewPCG(indexInputSeed, 0))
		indexInputInts = make([]int32, indexInputCount)
		indexInputLongs = make([]int64, indexInputCount)
		indexInputRandomBytes = make([]byte, indexInputCount*(5+4+9+8))
		bdo := NewByteArrayDataOutput(indexInputRandomBytes)
		for i := 0; i < indexInputCount; i++ {
			i1 := int32(random.Uint32())
			indexInputInts[i] = i1
			mustWrite(bdo.WriteVInt(i1))
			mustWrite(bdo.WriteInt(i1))

			var l1 int64
			if rarely(random) {
				// a long with lots of zeroes at the end
				l1 = nextLong(random, 0, math.MaxInt32) << 32
			} else {
				l1 = nextLong(random, 0, math.MaxInt64)
			}
			indexInputLongs[i] = l1
			mustWrite(bdo.WriteVLong(l1))
			mustWrite(bdo.WriteLong(l1))
		}
	})
	t.Logf("beforeClass random seed: %d", indexInputSeed)
}

func mustWrite(err error) {
	if err != nil {
		panic(err)
	}
}

// rarely mirrors LuceneTestCase.rarely(Random) for the default, non-nightly
// configuration: p = 1, so the call returns true for 1% of draws.
func rarely(r *rand.Rand) bool {
	return r.IntN(100) >= 99
}

// nextLong mirrors TestUtil.nextLong(Random, long start, long end): a uniform
// value in [start, end], both ends inclusive.
func nextLong(r *rand.Rand, start, end int64) int64 {
	if start > end {
		panic("start must be <= end")
	}
	span := uint64(end) - uint64(start) + 1
	if span == 0 {
		return int64(r.Uint64())
	}
	return start + int64(r.Uint64N(span))
}

func checkIndexInputReads(t *testing.T, is DataInput) {
	t.Helper()
	wantVInts := []int32{128, 16383, 16384, 16385, math.MaxInt32, -1}
	for _, want := range wantVInts {
		got, err := is.ReadVInt()
		mustNoErr(t, err)
		if got != want {
			t.Fatalf("readVInt: got %d, want %d", got, want)
		}
	}
	for _, want := range []int64{math.MaxInt32, math.MaxInt64} {
		got, err := is.ReadVLong()
		mustNoErr(t, err)
		if got != want {
			t.Fatalf("readVLong: got %d, want %d", got, want)
		}
	}
	wantStrings := []string{
		"Lucene",
		"¿",
		"Lu¿ce¿ne",
		"☠",
		"Lu☠ce☠ne",
		"\U0001D11E",
		"\U0001D11E\U0001D160",
		"Lu\U0001D11Ece\U0001D160ne",
		"\u0000",
		"Lu\u0000ce\u0000ne",
	}
	for _, want := range wantStrings {
		got, err := is.ReadString()
		mustNoErr(t, err)
		if got != want {
			t.Fatalf("readString: got %q, want %q", got, want)
		}
	}
}

func checkIndexInputRandomReads(t *testing.T, is DataInput) {
	t.Helper()
	for i := 0; i < indexInputCount; i++ {
		vi, err := is.ReadVInt()
		mustNoErr(t, err)
		if vi != indexInputInts[i] {
			t.Fatalf("readVInt[%d]: got %d, want %d", i, vi, indexInputInts[i])
		}
		ii, err := is.ReadInt()
		mustNoErr(t, err)
		if ii != indexInputInts[i] {
			t.Fatalf("readInt[%d]: got %d, want %d", i, ii, indexInputInts[i])
		}
		vl, err := is.ReadVLong()
		mustNoErr(t, err)
		if vl != indexInputLongs[i] {
			t.Fatalf("readVLong[%d]: got %d, want %d", i, vl, indexInputLongs[i])
		}
		ll, err := is.ReadLong()
		mustNoErr(t, err)
		if ll != indexInputLongs[i] {
			t.Fatalf("readLong[%d]: got %d, want %d", i, ll, indexInputLongs[i])
		}
	}
}

func checkIndexInputSeeksAndSkips(t *testing.T, is IndexInput, random *rand.Rand) {
	t.Helper()
	length := is.Length()

	iterations := 10
	if indexInputTestNightly {
		iterations = 1_000
	}
	for i := 0; i < iterations; i++ {
		mustNoErr(t, is.SetPosition(0)) // make sure we're at the start

		for curr := int64(0); curr < length; {
			maxSkipTo := length - 1
			// if we're close to the end, just skip all the way
			var skipTo int64
			if length-curr < 10 {
				skipTo = maxSkipTo
			} else {
				skipTo = nextLong(random, curr, maxSkipTo)
			}
			skipDelta := skipTo - curr

			// first reposition using seek
			startByte1, err := is.ReadByte()
			mustNoErr(t, err)
			mustNoErr(t, is.SetPosition(skipTo))
			endByte1, err := is.ReadByte()
			mustNoErr(t, err)

			// do the same thing but with skipBytes
			mustNoErr(t, is.SetPosition(curr))
			startByte2, err := is.ReadByte()
			mustNoErr(t, err)
			mustNoErr(t, is.SetPosition(curr))
			mustNoErr(t, is.SkipBytes(skipDelta))
			endByte2, err := is.ReadByte()
			mustNoErr(t, err)

			if startByte1 != startByte2 {
				t.Fatalf("start byte: %d != %d", startByte1, startByte2)
			}
			if endByte1 != endByte2 {
				t.Fatalf("end byte: %d != %d", endByte1, endByte2)
			}
			// +1 since we read the byte we seek/skip to
			if got, want := is.GetFilePointer(), curr+skipDelta+1; got != want {
				t.Fatalf("getFilePointer: got %d, want %d", got, want)
			}

			curr = is.GetFilePointer()
		}
	}
}

// testRawIndexInputRead checks the IndexInput methods of any impl.
func TestIndexInput_RawIndexInputRead(t *testing.T) {
	indexInputBeforeClass(t)
	seed := uint64(time.Now().UnixNano())
	t.Logf("random seed: %d", seed)
	random := rand.New(rand.NewPCG(seed, 1))
	for i := 0; i < 10; i++ {
		dir := NewByteBuffersDirectory()
		os, err := dir.CreateOutput("foo", IOContextDefault)
		mustNoErr(t, err)
		mustNoErr(t, os.WriteBytes(readTestBytes, 0, len(readTestBytes)))
		mustNoErr(t, os.Close())
		is, err := dir.OpenInput("foo", IOContextDefault)
		mustNoErr(t, err)
		checkIndexInputReads(t, is)
		checkIndexInputSeeksAndSkips(t, is, random)
		mustNoErr(t, is.Close())

		os, err = dir.CreateOutput("bar", IOContextDefault)
		mustNoErr(t, err)
		mustNoErr(t, os.WriteBytes(indexInputRandomBytes, 0, len(indexInputRandomBytes)))
		mustNoErr(t, os.Close())
		is, err = dir.OpenInput("bar", IOContextDefault)
		mustNoErr(t, err)
		checkIndexInputRandomReads(t, is)
		checkIndexInputSeeksAndSkips(t, is, random)
		mustNoErr(t, is.Close())
		mustNoErr(t, dir.Close())
	}
}

func TestIndexInput_ByteArrayDataInput(t *testing.T) {
	indexInputBeforeClass(t)
	is := NewByteArrayDataInput(readTestBytes)
	checkIndexInputReads(t, is)
	is = NewByteArrayDataInput(indexInputRandomBytes)
	checkIndexInputRandomReads(t, is)
}

func TestIndexInput_NoReadOnSkipBytes(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	t.Logf("random seed: %d", seed)
	random := rand.New(rand.NewPCG(seed, 2))
	length := int64(1_000_000)
	if indexInputTestNightly {
		length = math.MaxInt64
	}
	maxSeekPos := length - 1
	is := getInterceptingIndexInput(length)

	for is.GetFilePointer() < maxSeekPos {
		seekPos := nextLong(random, is.GetFilePointer(), maxSeekPos)
		skipDelta := seekPos - is.GetFilePointer()
		mustNoErr(t, is.SkipBytes(skipDelta))
		if got := is.GetFilePointer(); got != seekPos {
			t.Fatalf("getFilePointer: got %d, want %d", got, seekPos)
		}
	}
}

// getInterceptingIndexInput mirrors TestIndexInput.getIndexInput(long).
func getInterceptingIndexInput(length int64) IndexInput {
	return newInterceptingIndexInput("foo", length)
}
