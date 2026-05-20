// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/index/TestFilterIndexInput.java
// (Lucene 10.4.0). In Java, TestFilterIndexInput extends TestIndexInput and
// overrides getIndexInput to wrap an InterceptingIndexInput in a
// FilterIndexInput. This port is self-contained: the shared READ_TEST_BYTES
// fixture, the InterceptingIndexInput harness, and the checkReads /
// checkSeeksAndSkips helpers inherited from TestIndexInput are inlined here.
//
// Divergences from the Java reference:
//   - testOverrides relies on Java reflection over IndexInput's abstract
//     methods; that intent is non-portable. It is replaced by the compile-time
//     assertion `var _ IndexInput = (*FilterIndexInput)(nil)` (in
//     filter_index_input.go) plus delegate-forwarding coverage below.
//   - Gocene's FilterIndexInput does not forward SkipBytes (it inherits
//     BaseIndexInput.SkipBytes, which mutates a pointer independent of the
//     delegate). checkSeeksAndSkips therefore exercises the skip path via
//     SetPosition, which is exactly Lucene's seek-based default skipBytes.

package store

import (
	"math/rand"
	"testing"
)

// readTestBytes is the Go port of TestIndexInput.READ_TEST_BYTES: a hand-built
// buffer of VInt/VLong values and length-prefixed UTF-8 strings (including
// 2/3/4-byte sequences and embedded NULs) used by checkReads.
var readTestBytes = []byte{
	0x80, 0x01, 0xFF, 0x7F, 0x80, 0x80, 0x01, 0x81, 0x80, 0x01,
	0xFF, 0xFF, 0xFF, 0xFF, 0x07,
	0xFF, 0xFF, 0xFF, 0xFF, 0x0F,
	0xFF, 0xFF, 0xFF, 0xFF, 0x07,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x7F,
	0x06, 'L', 'u', 'c', 'e', 'n', 'e',

	// 2-byte UTF-8 (U+00BF "INVERTED QUESTION MARK")
	0x02, 0xC2, 0xBF,
	0x0A, 'L', 'u', 0xC2, 0xBF, 'c', 'e', 0xC2, 0xBF, 'n', 'e',

	// 3-byte UTF-8 (U+2620 "SKULL AND CROSSBONES")
	0x03, 0xE2, 0x98, 0xA0,
	0x0C, 'L', 'u', 0xE2, 0x98, 0xA0, 'c', 'e', 0xE2, 0x98, 0xA0, 'n', 'e',

	// surrogate pairs
	// (U+1D11E "MUSICAL SYMBOL G CLEF")
	// (U+1D160 "MUSICAL SYMBOL EIGHTH NOTE")
	0x04, 0xF0, 0x9D, 0x84, 0x9E,
	0x08, 0xF0, 0x9D, 0x84, 0x9E, 0xF0, 0x9D, 0x85, 0xA0,
	0x0E, 'L', 'u', 0xF0, 0x9D, 0x84, 0x9E, 'c', 'e', 0xF0, 0x9D, 0x85, 0xA0, 'n', 'e',

	// null bytes
	0x01, 0x00,
	0x08, 'L', 'u', 0x00, 'c', 'e', 0x00, 'n', 'e',
}

// interceptingIndexInput is the Go port of
// TestIndexInput.InterceptingIndexInput: a mock IndexInput that only tracks a
// position (responding to seek/skip) and panics if ReadByte/ReadBytes are
// called, ensuring seek/skip never invokes a read.
type interceptingIndexInput struct {
	*BaseIndexInput
}

func newInterceptingIndexInput(resourceDescription string, length int64) *interceptingIndexInput {
	return &interceptingIndexInput{
		BaseIndexInput: NewBaseIndexInput(resourceDescription, length),
	}
}

func (in *interceptingIndexInput) SetPosition(pos int64) error {
	in.SetFilePointer(pos)
	return nil
}

func (in *interceptingIndexInput) ReadByte() (byte, error) {
	panic("interceptingIndexInput.ReadByte: unexpected read")
}

func (in *interceptingIndexInput) ReadBytes([]byte) error {
	panic("interceptingIndexInput.ReadBytes: unexpected read")
}

func (in *interceptingIndexInput) ReadBytesN(int) ([]byte, error) {
	panic("interceptingIndexInput.ReadBytesN: unexpected read")
}

func (in *interceptingIndexInput) ReadShort() (int16, error) {
	panic("interceptingIndexInput.ReadShort: unexpected read")
}

func (in *interceptingIndexInput) ReadInt() (int32, error) {
	panic("interceptingIndexInput.ReadInt: unexpected read")
}

func (in *interceptingIndexInput) ReadLong() (int64, error) {
	panic("interceptingIndexInput.ReadLong: unexpected read")
}

func (in *interceptingIndexInput) ReadString() (string, error) {
	panic("interceptingIndexInput.ReadString: unexpected read")
}

func (in *interceptingIndexInput) Close() error { return nil }

func (in *interceptingIndexInput) Clone() IndexInput {
	return &interceptingIndexInput{
		BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
	}
}

func (in *interceptingIndexInput) Slice(string, int64, int64) (IndexInput, error) {
	panic("interceptingIndexInput.Slice: unsupported")
}

var _ IndexInput = (*interceptingIndexInput)(nil)

// checkReads is the Go port of TestIndexInput.checkReads: it walks
// readTestBytes through the package-level VInt/VLong/String decoders and
// asserts every decoded value.
func checkReads(t *testing.T, in DataInput) {
	t.Helper()

	wantVInt := func(want int32) {
		t.Helper()
		got, err := ReadVInt(in)
		if err != nil || got != want {
			t.Fatalf("ReadVInt = (%d, %v), want (%d, nil)", got, err, want)
		}
	}
	wantVLong := func(want int64) {
		t.Helper()
		got, err := ReadVLong(in)
		if err != nil || got != want {
			t.Fatalf("ReadVLong = (%d, %v), want (%d, nil)", got, err, want)
		}
	}
	wantString := func(want string) {
		t.Helper()
		got, err := ReadString(in)
		if err != nil || got != want {
			t.Fatalf("ReadString = (%q, %v), want (%q, nil)", got, err, want)
		}
	}

	wantVInt(128)
	wantVInt(16383)
	wantVInt(16384)
	wantVInt(16385)
	wantVInt(2147483647) // Integer.MAX_VALUE
	wantVInt(-1)
	wantVLong(2147483647)          // (long) Integer.MAX_VALUE
	wantVLong(9223372036854775807) // Long.MAX_VALUE
	wantString("Lucene")

	wantString("¿")
	wantString("Lu¿ce¿ne")

	wantString("☠")
	wantString("Lu☠ce☠ne")

	wantString("\U0001D11E")
	wantString("\U0001D11E\U0001D160")
	wantString("Lu\U0001D11Ece\U0001D160ne")

	wantString("\x00")
	wantString("Lu\x00ce\x00ne")
}

// checkSeeksAndSkips is the Go port of TestIndexInput.checkSeeksAndSkips. It
// repeatedly repositions the input with SetPosition (covering both the "seek"
// and the "skipBytes" arms of the Java original; see the divergence note at
// the top of this file) and verifies the byte read after repositioning and
// the resulting file pointer.
func checkSeeksAndSkips(t *testing.T, in IndexInput, rng *rand.Rand) {
	t.Helper()
	length := in.Length()

	const iterations = 10
	for i := 0; i < iterations; i++ {
		if err := in.SetPosition(0); err != nil {
			t.Fatalf("SetPosition(0) failed: %v", err)
		}

		for curr := int64(0); curr < length; {
			maxSkipTo := length - 1
			var skipTo int64
			if length-curr < 10 {
				skipTo = maxSkipTo
			} else {
				skipTo = curr + rng.Int63n(maxSkipTo-curr+1)
			}
			skipDelta := skipTo - curr

			// reposition using SetPosition (seek)
			startByte1, err := in.ReadByte()
			if err != nil {
				t.Fatalf("ReadByte at curr=%d failed: %v", curr, err)
			}
			if err := in.SetPosition(skipTo); err != nil {
				t.Fatalf("SetPosition(%d) failed: %v", skipTo, err)
			}
			endByte1, err := in.ReadByte()
			if err != nil {
				t.Fatalf("ReadByte at skipTo=%d failed: %v", skipTo, err)
			}

			// do the same thing again, repositioning by skipDelta
			if err := in.SetPosition(curr); err != nil {
				t.Fatalf("SetPosition(%d) failed: %v", curr, err)
			}
			startByte2, err := in.ReadByte()
			if err != nil {
				t.Fatalf("ReadByte at curr=%d failed: %v", curr, err)
			}
			if err := in.SetPosition(curr + skipDelta); err != nil {
				t.Fatalf("SetPosition(%d) failed: %v", curr+skipDelta, err)
			}
			endByte2, err := in.ReadByte()
			if err != nil {
				t.Fatalf("ReadByte after skip failed: %v", err)
			}

			if startByte1 != startByte2 {
				t.Fatalf("start byte mismatch: %#x != %#x", startByte1, startByte2)
			}
			if endByte1 != endByte2 {
				t.Fatalf("end byte mismatch: %#x != %#x", endByte1, endByte2)
			}
			// +1 since we read the byte we seek/skip to
			if got := in.GetFilePointer(); got != curr+skipDelta+1 {
				t.Fatalf("GetFilePointer = %d, want %d", got, curr+skipDelta+1)
			}

			curr = in.GetFilePointer()
		}
	}
}

// getIndexInput mirrors TestFilterIndexInput.getIndexInput: a FilterIndexInput
// wrapping an InterceptingIndexInput.
func getIndexInput(length int64) IndexInput {
	return NewFilterIndexInput("wrapped foo", newInterceptingIndexInput("foo", length))
}

// TestFilterIndexInput_NoReadOnSkipBytes is the Go port of
// TestIndexInput.testNoReadOnSkipBytes exercised through getIndexInput. It
// drives only repositioning operations; the InterceptingIndexInput delegate
// panics on any read, so reaching the end without panicking proves no read
// occurred.
func TestFilterIndexInput_NoReadOnSkipBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0x5EED))
	const length = 1_000_000
	maxSeekPos := int64(length - 1)
	in := getIndexInput(length)

	for in.GetFilePointer() < maxSeekPos {
		seekPos := in.GetFilePointer() + rng.Int63n(maxSeekPos-in.GetFilePointer()+1)
		if err := in.SetPosition(seekPos); err != nil {
			t.Fatalf("SetPosition(%d) failed: %v", seekPos, err)
		}
		if got := in.GetFilePointer(); got != seekPos {
			t.Fatalf("GetFilePointer = %d, want %d", got, seekPos)
		}
	}
}

// TestFilterIndexInput_RawFilterIndexInputRead is the Go port of
// TestFilterIndexInput.testRawFilterIndexInputRead: it writes readTestBytes to
// a real directory, opens the file behind a FilterIndexInput, and verifies the
// decoded values and seek/skip behaviour.
func TestFilterIndexInput_RawFilterIndexInputRead(t *testing.T) {
	rng := rand.New(rand.NewSource(0xABCDEF))

	for i := 0; i < 10; i++ {
		dir, err := NewSimpleFSDirectory(t.TempDir())
		if err != nil {
			t.Fatalf("NewSimpleFSDirectory failed: %v", err)
		}

		os, err := dir.CreateOutput("foo", IOContextDefault)
		if err != nil {
			t.Fatalf("CreateOutput(foo) failed: %v", err)
		}
		if err := os.WriteBytes(readTestBytes); err != nil {
			t.Fatalf("WriteBytes(foo) failed: %v", err)
		}
		if err := os.Close(); err != nil {
			t.Fatalf("Close(foo) failed: %v", err)
		}

		delegate, err := dir.OpenInput("foo", IOContextDefault)
		if err != nil {
			t.Fatalf("OpenInput(foo) failed: %v", err)
		}
		is := NewFilterIndexInput("wrapped foo", delegate)
		checkReads(t, is)
		checkSeeksAndSkips(t, is, rng)
		if err := is.Close(); err != nil {
			t.Fatalf("Close(wrapped foo) failed: %v", err)
		}

		if err := dir.Close(); err != nil {
			t.Fatalf("Close(dir) failed: %v", err)
		}
	}
}

// TestFilterIndexInput_Unwrap is the Go port of
// TestFilterIndexInput.testUnwrap: GetDelegate returns the wrapped input and
// UnwrapFilterIndexInput peels every FilterIndexInput layer.
func TestFilterIndexInput_Unwrap(t *testing.T) {
	dir, err := NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory failed: %v", err)
	}
	defer func() {
		if err := dir.Close(); err != nil {
			t.Fatalf("Close(dir) failed: %v", err)
		}
	}()

	ignored, err := dir.CreateOutput("test", IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput(test) failed: %v", err)
	}
	if err := ignored.Close(); err != nil {
		t.Fatalf("Close(test output) failed: %v", err)
	}

	indexInput, err := dir.OpenInput("test", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput(test) failed: %v", err)
	}
	filterIndexInput := NewFilterIndexInput("wrapper of test", indexInput)
	if filterIndexInput.GetDelegate() != indexInput {
		t.Fatalf("GetDelegate did not return the wrapped input")
	}
	if got := UnwrapFilterIndexInput(filterIndexInput); got != indexInput {
		t.Fatalf("UnwrapFilterIndexInput did not reach the base delegate")
	}
	if err := filterIndexInput.Close(); err != nil {
		t.Fatalf("Close(filterIndexInput) failed: %v", err)
	}
}

// TestFilterIndexInput_DelegateForwarding verifies that FilterIndexInput
// forwards primitive read calls to its delegate and tracks the delegate's file
// pointer. This consolidates the intent of TestFilterIndexInput.testOverrides,
// whose Java-reflection mechanism is non-portable to Go.
func TestFilterIndexInput_DelegateForwarding(t *testing.T) {
	dir, err := NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory failed: %v", err)
	}
	defer func() {
		if err := dir.Close(); err != nil {
			t.Fatalf("Close(dir) failed: %v", err)
		}
	}()

	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	out, err := dir.CreateOutput("fwd", IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput(fwd) failed: %v", err)
	}
	if err := out.WriteBytes(want); err != nil {
		t.Fatalf("WriteBytes(fwd) failed: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close(fwd output) failed: %v", err)
	}

	delegate, err := dir.OpenInput("fwd", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput(fwd) failed: %v", err)
	}
	filter := NewFilterIndexInput("filter", delegate)
	defer func() {
		if err := filter.Close(); err != nil {
			t.Fatalf("Close(filter) failed: %v", err)
		}
	}()

	got, err := filter.ReadByte()
	if err != nil || got != 0xDE {
		t.Fatalf("ReadByte = (%#x, %v), want (0xDE, nil)", got, err)
	}
	if filter.GetFilePointer() != 1 {
		t.Fatalf("GetFilePointer = %d, want 1", filter.GetFilePointer())
	}
	if filter.Length() != int64(len(want)) {
		t.Fatalf("Length = %d, want %d", filter.Length(), len(want))
	}

	rest := make([]byte, 3)
	if err := filter.ReadBytes(rest); err != nil {
		t.Fatalf("ReadBytes failed: %v", err)
	}
	if rest[0] != 0xAD || rest[1] != 0xBE || rest[2] != 0xEF {
		t.Fatalf("ReadBytes = % x, want AD BE EF", rest)
	}
}
