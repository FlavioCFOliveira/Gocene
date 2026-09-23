// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// Port of lucene/core/src/test/org/apache/lucene/store/TestMultiMMap.java
// (Apache Lucene 10.5.0).
//
// The Java class extends the test-framework class BaseChunkedDirectoryTestCase,
// whose inherited suite is not part of this port. LuceneTestCase.newIOContext
// randomises the IOContext; that test-framework randomisation is not ported,
// so IOContext.DEFAULT is used, one of the contexts it can return.

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func newMultiMMapTestRandom(t *testing.T) *rand.Rand {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewPCG(seed, 0xB5AD4ECEDA1CE2A9))
}

// getMultiMMapDirectory mirrors TestMultiMMap.getDirectory(Path, int).
func getMultiMMapDirectory(t *testing.T, path string, maxChunkSize int) Directory {
	t.Helper()
	dir, err := NewMMapDirectoryWithChunkSize(path, int64(maxChunkSize))
	if err != nil {
		t.Fatalf("NewMMapDirectoryWithChunkSize: %v", err)
	}
	return dir
}

// getMultiMMapDefaultDirectory mirrors BaseChunkedDirectoryTestCase.getDirectory(Path):
// getDirectory(path, 1 << TestUtil.nextInt(random(), 10, 20)).
func getMultiMMapDefaultDirectory(t *testing.T, path string, random *rand.Rand) Directory {
	t.Helper()
	return getMultiMMapDirectory(t, path, 1<<(10+random.IntN(11)))
}

func TestMultiMMap_SeekingExceptions(t *testing.T) {
	const sliceSize = 128
	dir := getMultiMMapDirectory(t, t.TempDir(), sliceSize)
	defer dir.Close()
	const size = 128 + 63
	out, err := dir.CreateOutput("a", IOContextDefault)
	mustNoErr(t, err)
	for i := 0; i < size; i++ {
		mustNoErr(t, out.WriteByte(0))
	}
	mustNoErr(t, out.Close())

	in, err := dir.OpenInput("a", IOContextDefault)
	mustNoErr(t, err)
	defer in.Close()

	const negativePos = -1234
	err = in.SetPosition(negativePos)
	if err == nil {
		t.Fatal("expected IllegalArgumentException seeking to a negative position")
	}
	if !strings.Contains(err.Error(), "negative position") {
		t.Errorf("does not mention negative position: %v", err)
	}

	const posAfterEOF = size + 123
	eof := in.SetPosition(posAfterEOF)
	if eof == nil {
		t.Fatal("expected EOFException seeking past EOF")
	}
	if !strings.Contains(eof.Error(), fmt.Sprintf("(pos=%d)", posAfterEOF)) {
		t.Errorf("wrong position in error message: %v", eof)
	}

	// this test verifies that the invalid position is transformed back to
	// original one for exception by slicing:
	slice, err := in.Slice("slice", 33, sliceSize+15)
	mustNoErr(t, err)
	// ensure that the slice uses multi-mmap:
	assertCorrectMMapImpl(t, false, slice)
	eof = slice.SetPosition(posAfterEOF)
	if eof == nil {
		t.Fatal("expected EOFException seeking a slice past EOF")
	}
	if !strings.Contains(eof.Error(), fmt.Sprintf("(pos=%d)", posAfterEOF)) {
		t.Errorf("wrong position in error message: %v", eof)
	}
}

func expectAlreadyClosed(t *testing.T, what string, err error) {
	t.Helper()
	var ace *AlreadyClosedException
	if !errors.As(err, &ace) {
		t.Errorf("%s: expected AlreadyClosedException, got %v", what, err)
	}
}

func TestMultiMMap_CloneSafety(t *testing.T) {
	random := newMultiMMapTestRandom(t)
	mmapDir := getMultiMMapDefaultDirectory(t, t.TempDir(), random)
	io, err := mmapDir.CreateOutput("bytes", IOContextDefault)
	mustNoErr(t, err)
	mustNoErr(t, io.WriteVInt(5))
	mustNoErr(t, io.Close())
	one, err := mmapDir.OpenInput("bytes", IOContextDefault)
	mustNoErr(t, err)
	two := one.Clone()
	three := two.Clone() // clone of clone
	mustNoErr(t, one.Close())
	_, err = one.ReadVInt()
	expectAlreadyClosed(t, "one.readVInt", err)
	_, err = two.ReadVInt()
	expectAlreadyClosed(t, "two.readVInt", err)
	_, err = three.ReadVInt()
	expectAlreadyClosed(t, "three.readVInt", err)

	mustNoErr(t, two.Close())
	mustNoErr(t, three.Close())
	// test double close of master:
	mustNoErr(t, one.Close())
	mustNoErr(t, mmapDir.Close())
}

func TestMultiMMap_CloneSliceSafety(t *testing.T) {
	random := newMultiMMapTestRandom(t)
	mmapDir := getMultiMMapDefaultDirectory(t, t.TempDir(), random)
	io, err := mmapDir.CreateOutput("bytes", IOContextDefault)
	mustNoErr(t, err)
	mustNoErr(t, io.WriteInt(1))
	mustNoErr(t, io.WriteInt(2))
	mustNoErr(t, io.Close())
	slicer, err := mmapDir.OpenInput("bytes", IOContextDefault)
	mustNoErr(t, err)
	one, err := slicer.Slice("first int", 0, 4)
	mustNoErr(t, err)
	two, err := slicer.Slice("second int", 4, 4)
	mustNoErr(t, err)
	three := one.Clone() // clone of clone
	four := two.Clone()  // clone of clone
	mustNoErr(t, slicer.Close())
	_, err = one.ReadInt()
	expectAlreadyClosed(t, "one.readInt", err)
	_, err = two.ReadInt()
	expectAlreadyClosed(t, "two.readInt", err)
	_, err = three.ReadInt()
	expectAlreadyClosed(t, "three.readInt", err)
	_, err = four.ReadInt()
	expectAlreadyClosed(t, "four.readInt", err)

	mustNoErr(t, one.Close())
	mustNoErr(t, two.Close())
	mustNoErr(t, three.Close())
	mustNoErr(t, four.Close())
	// test double-close of slicer:
	mustNoErr(t, slicer.Close())
	mustNoErr(t, mmapDir.Close())
}

// test has asserts specific to mmap impl...
func TestMultiMMap_Implementations(t *testing.T) {
	random := newMultiMMapTestRandom(t)
	for i := 2; i < 12; i++ {
		chunkSize := 1 << i
		mmapDir := getMultiMMapDirectory(t, t.TempDir(), chunkSize)
		io, err := mmapDir.CreateOutput("bytes", IOContextDefault)
		mustNoErr(t, err)
		size := random.IntN(chunkSize*2) + 3 // add some buffer of 3 for slice tests
		bs := make([]byte, size)
		for k := range bs {
			bs[k] = byte(random.Uint32())
		}
		mustNoErr(t, io.WriteBytes(bs, 0, len(bs)))
		mustNoErr(t, io.Close())
		ii, err := mmapDir.OpenInput("bytes", IOContextDefault)
		mustNoErr(t, err)
		actual := make([]byte, size) // first read all bytes
		mustNoErr(t, ii.ReadBytes(actual, 0, len(actual)))
		if !bytes.Equal(bs, actual) {
			t.Fatalf("chunkSize=%d: bytes read differ from bytes written", chunkSize)
		}
		// reinit:
		mustNoErr(t, ii.SetPosition(0))

		// check impl (we must check size < chunksize: currently, if
		// size==chunkSize, we get 2 buffers, the second one empty:
		assertCorrectMMapImpl(t, size < chunkSize, ii)

		// clone tests:
		if reflect.TypeOf(ii) != reflect.TypeOf(ii.Clone()) {
			t.Errorf("clone class %T differs from %T", ii.Clone(), ii)
		}

		// slice test (offset 0)
		sliceSize := random.IntN(size)
		slice, err := ii.Slice("slice", 0, int64(sliceSize))
		mustNoErr(t, err)
		assertCorrectMMapImpl(t, sliceSize < chunkSize, slice)

		// slice test (offset > 0 )
		offset := random.IntN(size-1) + 1
		sliceSize = random.IntN(size - offset + 1)
		slice, err = ii.Slice("slice", int64(offset), int64(sliceSize))
		mustNoErr(t, err)
		assertCorrectMMapImpl(t, offset%chunkSize+sliceSize < chunkSize, slice)

		mustNoErr(t, ii.Close())
		mustNoErr(t, mmapDir.Close())
	}
}

var (
	singleMMapImplName = regexp.MustCompile(`^Single\w+Impl$`)
	multiMMapImplName  = regexp.MustCompile(`^Multi\w+Impl$`)
)

// assertCorrectMMapImpl mirrors TestMultiMMap.assertCorrectImpl: the concrete
// type name must be a Single...Impl or Multi...Impl class.
func assertCorrectMMapImpl(t *testing.T, isSingle bool, ii IndexInput) {
	t.Helper()
	typ := reflect.TypeOf(ii)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	name := typ.Name()
	if isSingle {
		if !singleMMapImplName.MatchString(name) {
			t.Errorf("Require a single impl, got %s", typ)
		}
	} else {
		if !multiMMapImplName.MatchString(name) {
			t.Errorf("Require a multi impl, got %s", typ)
		}
	}
}
