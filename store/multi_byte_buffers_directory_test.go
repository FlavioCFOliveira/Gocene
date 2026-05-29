// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"testing"
)

// TestMultiByteBuffersDirectory is the Gocene port of
// org.apache.lucene.store.TestMultiByteBuffersDirectory.
//
// In Lucene 10.4.0 this test extends BaseChunkedDirectoryTestCase and overrides
// getDirectory(Path, int maxChunkSize) to construct a ByteBuffersDirectory
// wired with a small bitsPerBlock so that data is physically split across many
// ByteBuffer chunks. The parent class then exercises boundary-crossing reads,
// writes, slices, seeks and clones against that chunked layout.
//
// Gocene divergence: ByteBuffersDirectory currently stores each file in a
// single contiguous []byte ([[project-gocene-bbdi-endian-trap]] is not in play
// here, but the underlying constructor accepts no chunk-size knob). There is
// no Gocene equivalent of BaseChunkedDirectoryTestCase yet — that harness is
// owned by Sprint 55 follow-ups. This port therefore:
//
//   - exposes a `newMultiBufferDirectory(t, maxChunkSize)` helper with the
//     same shape as the Java override, so a future chunked constructor can
//     wire in here without rewriting the call sites;
//   - exercises the boundary-crossing test cases that are most load-bearing
//     for ByteBuffersDirectory (cross-boundary bytes, little-endian longs/ints,
//     seek-zero/seek-end, clone/close, sliced seeking) in a chunk-size
//     parameterised loop; and
//   - skips the cases that genuinely require the chunked constructor (random
//     chunk sizes against a real merge pipeline) until the constructor lands.
func TestMultiByteBuffersDirectory(t *testing.T) {
	t.Run("cross_boundary_bytes", func(t *testing.T) {
		// Mirrors BaseChunkedDirectoryTestCase#testBytesCrossBoundary.
		for _, chunk := range []int{16, 64, 256, 1024} {
			chunk := chunk
			t.Run(fmt.Sprintf("chunk=%d", chunk), func(t *testing.T) {
				dir := newMultiBufferDirectory(t, chunk)
				defer dir.Close()

				num := 100
				data := deterministicBytes(t, num)

				out, err := dir.CreateOutput("bytesCrossBoundary", IOContext{})
				if err != nil {
					t.Fatalf("CreateOutput: %v", err)
				}
				if err := out.WriteBytes(data); err != nil {
					t.Fatalf("WriteBytes: %v", err)
				}
				if err := out.Close(); err != nil {
					t.Fatalf("Close output: %v", err)
				}

				in, err := dir.OpenInput("bytesCrossBoundary", IOContext{})
				if err != nil {
					t.Fatalf("OpenInput: %v", err)
				}
				defer in.Close()

				got := make([]byte, num)
				if err := in.ReadBytes(got); err != nil {
					t.Fatalf("ReadBytes: %v", err)
				}
				if !bytes.Equal(got, data) {
					t.Fatalf("full read mismatch")
				}

				// Sub-reads from every offset across the (logical) boundary.
				for offset := 1; offset < num; offset++ {
					if err := in.SetPosition(int64(offset)); err != nil {
						t.Fatalf("SetPosition(%d): %v", offset, err)
					}
					sub := make([]byte, num-offset)
					if err := in.ReadBytes(sub); err != nil {
						t.Fatalf("ReadBytes at %d: %v", offset, err)
					}
					if !bytes.Equal(sub, data[offset:]) {
						t.Fatalf("sub-read mismatch at offset=%d", offset)
					}
				}
			})
		}
	})

	t.Run("little_endian_longs_cross_boundary", func(t *testing.T) {
		// Mirrors BaseChunkedDirectoryTestCase#testLittleEndianLongsCrossBoundary,
		// adapted: Gocene's ByteBuffersIndexOutput emits big-endian longs (see
		// store/byte_buffers_directory.go: WriteLong). We therefore round-trip
		// against the matching big-endian ReadLong and only assert that values
		// survive a cross-boundary write+read. Wire-format byte equivalence
		// with JVM-produced bytes is tracked separately.
		dir := newMultiBufferDirectory(t, 16)
		defer dir.Close()

		out, err := dir.CreateOutput("longs", IOContext{})
		if err != nil {
			t.Fatalf("CreateOutput: %v", err)
		}
		if err := out.WriteByte(2); err != nil {
			t.Fatalf("WriteByte: %v", err)
		}
		want := []int64{3, 0x7FFFFFFFFFFFFFFF, -3}
		for _, v := range want {
			if err := out.WriteLong(v); err != nil {
				t.Fatalf("WriteLong(%d): %v", v, err)
			}
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close output: %v", err)
		}

		in, err := dir.OpenInput("longs", IOContext{})
		if err != nil {
			t.Fatalf("OpenInput: %v", err)
		}
		defer in.Close()

		if length, _ := dir.FileLength("longs"); length != 25 {
			t.Fatalf("length: want 25 got %d", length)
		}
		b, err := in.ReadByte()
		if err != nil || b != 2 {
			t.Fatalf("ReadByte: got=%d err=%v", b, err)
		}
		// WriteLong and ReadLong are both little-endian (Lucene 10.x parity,
		// rmp #4786); read three longs back directly and confirm they survive
		// cross-boundary I/O.
		for i := range want {
			got, err := in.ReadLong()
			if err != nil {
				t.Fatalf("ReadLong[%d]: %v", i, err)
			}
			if got != want[i] {
				t.Fatalf("long[%d]: want %d got %d", i, want[i], got)
			}
		}
	})

	t.Run("seek_zero", func(t *testing.T) {
		// Mirrors BaseChunkedDirectoryTestCase#testSeekZero — seeking to 0 on
		// an empty file is always legal regardless of chunk size.
		for i := 0; i < 3; i++ {
			chunk := 1 << i
			t.Run(fmt.Sprintf("chunk=%d", chunk), func(t *testing.T) {
				dir := newMultiBufferDirectory(t, chunk)
				defer dir.Close()

				out, err := dir.CreateOutput("zeroBytes", IOContext{})
				if err != nil {
					t.Fatalf("CreateOutput: %v", err)
				}
				if err := out.Close(); err != nil {
					t.Fatalf("Close output: %v", err)
				}
				in, err := dir.OpenInput("zeroBytes", IOContext{})
				if err != nil {
					t.Fatalf("OpenInput: %v", err)
				}
				if err := in.SetPosition(0); err != nil {
					t.Fatalf("SetPosition(0): %v", err)
				}
				if err := in.Close(); err != nil {
					t.Fatalf("Close input: %v", err)
				}
			})
		}
	})

	t.Run("seek_end", func(t *testing.T) {
		// Mirrors BaseChunkedDirectoryTestCase#testSeekEnd — seeking to the
		// exact end of file is legal and reads up to that point must match.
		for i := 0; i < 12; i++ {
			chunk := 1 << i
			t.Run(fmt.Sprintf("chunk=%d", chunk), func(t *testing.T) {
				dir := newMultiBufferDirectory(t, chunk)
				defer dir.Close()

				data := deterministicBytes(t, 1<<i)
				out, err := dir.CreateOutput("bytes", IOContext{})
				if err != nil {
					t.Fatalf("CreateOutput: %v", err)
				}
				if err := out.WriteBytes(data); err != nil {
					t.Fatalf("WriteBytes: %v", err)
				}
				if err := out.Close(); err != nil {
					t.Fatalf("Close output: %v", err)
				}

				in, err := dir.OpenInput("bytes", IOContext{})
				if err != nil {
					t.Fatalf("OpenInput: %v", err)
				}
				got := make([]byte, 1<<i)
				if err := in.ReadBytes(got); err != nil {
					t.Fatalf("ReadBytes: %v", err)
				}
				if !bytes.Equal(got, data) {
					t.Fatalf("read mismatch at chunk=%d", chunk)
				}
				if err := in.SetPosition(int64(1 << i)); err != nil {
					t.Fatalf("SetPosition(end): %v", err)
				}
				if err := in.Close(); err != nil {
					t.Fatalf("Close input: %v", err)
				}
			})
		}
	})

	t.Run("seeking", func(t *testing.T) {
		// Mirrors BaseChunkedDirectoryTestCase#testSeeking — read every
		// (start, length) sub-range with explicit seeks. Bounded loop to keep
		// runtime reasonable in the non-nightly profile.
		for i := 0; i < 4; i++ {
			chunk := 1 << i
			t.Run(fmt.Sprintf("chunk=%d", chunk), func(t *testing.T) {
				dir := newMultiBufferDirectory(t, chunk)
				defer dir.Close()

				data := deterministicBytes(t, 1<<(i+1))
				out, err := dir.CreateOutput("bytes", IOContext{})
				if err != nil {
					t.Fatalf("CreateOutput: %v", err)
				}
				if err := out.WriteBytes(data); err != nil {
					t.Fatalf("WriteBytes: %v", err)
				}
				if err := out.Close(); err != nil {
					t.Fatalf("Close output: %v", err)
				}

				in, err := dir.OpenInput("bytes", IOContext{})
				if err != nil {
					t.Fatalf("OpenInput: %v", err)
				}
				defer in.Close()

				got := make([]byte, len(data))
				if err := in.ReadBytes(got); err != nil {
					t.Fatalf("ReadBytes(full): %v", err)
				}
				if !bytes.Equal(got, data) {
					t.Fatalf("full read mismatch")
				}
				for start := 0; start < len(data); start++ {
					for length := 0; length < len(data)-start; length++ {
						if err := in.SetPosition(int64(start)); err != nil {
							t.Fatalf("SetPosition(%d): %v", start, err)
						}
						buf := make([]byte, length)
						if err := in.ReadBytes(buf); err != nil {
							t.Fatalf("ReadBytes(%d,%d): %v", start, length, err)
						}
						if !bytes.Equal(buf, data[start:start+length]) {
							t.Fatalf("slice mismatch start=%d length=%d", start, length)
						}
					}
				}
			})
		}
	})

	t.Run("sliced_seeking", func(t *testing.T) {
		// Mirrors BaseChunkedDirectoryTestCase#testSlicedSeeking — assert
		// slice(offset, length) returns the right bytes for every sub-range.
		for i := 0; i < 4; i++ {
			chunk := 1 << i
			t.Run(fmt.Sprintf("chunk=%d", chunk), func(t *testing.T) {
				dir := newMultiBufferDirectory(t, chunk)
				defer dir.Close()

				data := deterministicBytes(t, 1<<(i+1))
				out, err := dir.CreateOutput("bytes", IOContext{})
				if err != nil {
					t.Fatalf("CreateOutput: %v", err)
				}
				if err := out.WriteBytes(data); err != nil {
					t.Fatalf("WriteBytes: %v", err)
				}
				if err := out.Close(); err != nil {
					t.Fatalf("Close output: %v", err)
				}

				slicer, err := dir.OpenInput("bytes", IOContext{})
				if err != nil {
					t.Fatalf("OpenInput: %v", err)
				}
				defer slicer.Close()

				for start := 0; start < len(data); start++ {
					for length := 0; length < len(data)-start; length++ {
						sl, err := slicer.Slice("bytesSlice", int64(start), int64(length))
						if err != nil {
							t.Fatalf("Slice(%d,%d): %v", start, length, err)
						}
						buf := make([]byte, length)
						if err := sl.ReadBytes(buf); err != nil {
							sl.Close()
							t.Fatalf("ReadBytes(slice %d,%d): %v", start, length, err)
						}
						if !bytes.Equal(buf, data[start:start+length]) {
							sl.Close()
							t.Fatalf("slice mismatch start=%d length=%d", start, length)
						}
						sl.Close()
					}
				}
			})
		}
	})

	t.Run("clone_close", func(t *testing.T) {
		// Mirrors BaseChunkedDirectoryTestCase#testCloneClose — closing a clone
		// must not invalidate other clones (or the parent). Gocene's clones
		// share the underlying content slice, so a Close on one must not zero
		// the parent's read state.
		dir := newMultiBufferDirectory(t, 32)
		defer dir.Close()

		out, err := dir.CreateOutput("bytes", IOContext{})
		if err != nil {
			t.Fatalf("CreateOutput: %v", err)
		}
		// Write a small VInt-shaped payload that Lucene's testCloneClose uses.
		if err := out.WriteByte(5); err != nil {
			t.Fatalf("WriteByte: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close output: %v", err)
		}

		one, err := dir.OpenInput("bytes", IOContext{})
		if err != nil {
			t.Fatalf("OpenInput: %v", err)
		}
		two := one.Clone()
		three := two.Clone() // clone-of-clone

		if err := two.Close(); err != nil {
			t.Fatalf("Close two: %v", err)
		}

		// `one` and `three` must still be usable after `two` is closed.
		gotOne, err := one.ReadByte()
		if err != nil || gotOne != 5 {
			t.Fatalf("one.ReadByte after two.Close: got=%d err=%v", gotOne, err)
		}
		if err := three.SetPosition(0); err != nil {
			t.Fatalf("three.SetPosition: %v", err)
		}
		gotThree, err := three.ReadByte()
		if err != nil || gotThree != 5 {
			t.Fatalf("three.ReadByte after two.Close: got=%d err=%v", gotThree, err)
		}

		if err := one.Close(); err != nil {
			t.Fatalf("Close one: %v", err)
		}
		if err := three.Close(); err != nil {
			t.Fatalf("Close three: %v", err)
		}
	})

	t.Run("random_chunk_sizes", func(t *testing.T) {
		// Mirrors BaseChunkedDirectoryTestCase#testRandomChunkSizes — exercises
		// a real index-write/read cycle over a directory built at a randomised
		// chunk size. Requires (a) a Gocene chunked ByteBuffersDirectory
		// constructor, (b) RandomIndexWriter + MockAnalyzer + MockDirectoryWrapper
		// ports. None exist yet on this branch.
		t.Skip("requires chunked ByteBuffersDirectory constructor and " +
			"RandomIndexWriter/MockDirectoryWrapper ports; tracked under Sprint 55")
	})
}

// newMultiBufferDirectory matches the shape of the Java override in
// TestMultiByteBuffersDirectory#getDirectory(Path, int maxChunkSize). Until
// Gocene's ByteBuffersDirectory grows a chunk-size knob, the maxChunkSize
// argument is recorded for documentation only and the monolithic constructor
// is used. The signature is preserved so that the chunked constructor, when
// added, can be wired in here without touching call sites.
func newMultiBufferDirectory(t *testing.T, maxChunkSize int) *ByteBuffersDirectory {
	t.Helper()
	if maxChunkSize < 1 {
		t.Fatalf("maxChunkSize must be >= 1, got %d", maxChunkSize)
	}
	return NewByteBuffersDirectory()
}

// deterministicBytes returns n bytes from a deterministic per-call PRNG. The
// exact seed source is irrelevant for these tests — what matters is
// reproducibility inside a single `go test` run. Named distinctly from the
// existing randomBytes helper in base_data_output_test_case_test.go to avoid
// the package-level collision.
func deterministicBytes(t *testing.T, n int) []byte {
	t.Helper()
	r := rand.New(rand.NewPCG(0xC0FFEE, uint64(n)))
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte(r.IntN(256))
	}
	return buf
}

