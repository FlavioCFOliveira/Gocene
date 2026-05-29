// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

// TestStorePrimitivesAreLittleEndian asserts that WriteShort/WriteInt/WriteLong
// on every IndexOutput implementation emit little-endian bytes (low byte
// first), matching Lucene 10.x org.apache.lucene.store.DataOutput (rmp #4786),
// and that the on-disk bytes are identical across implementations.
//
// The framing primitives (CodecUtil header/footer) remain big-endian via the
// explicit store.WriteInt32/WriteInt64 helpers and are intentionally NOT
// covered here.
func TestStorePrimitivesAreLittleEndian(t *testing.T) {
	const (
		shortVal = int16(0x0102)
		intVal   = int32(0x01020304)
		longVal  = int64(0x0102030405060708)
	)

	// Lucene DataOutput.writeShort/writeInt/writeLong emit low byte first.
	wantShort := []byte{0x02, 0x01}
	wantInt := []byte{0x04, 0x03, 0x02, 0x01}
	wantLong := []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}
	want := append(append(append([]byte{}, wantShort...), wantInt...), wantLong...)

	writeAll := func(out DataOutput) error {
		if err := out.WriteShort(shortVal); err != nil {
			return err
		}
		if err := out.WriteInt(intVal); err != nil {
			return err
		}
		return out.WriteLong(longVal)
	}

	t.Run("ByteArrayDataOutput", func(t *testing.T) {
		out := NewByteArrayDataOutput(16)
		if err := writeAll(out); err != nil {
			t.Fatalf("write: %v", err)
		}
		assertBytes(t, out.GetBytes(), want)
	})

	t.Run("ByteBuffersDataOutput", func(t *testing.T) {
		out := NewByteBuffersDataOutput()
		if err := writeAll(out); err != nil {
			t.Fatalf("write: %v", err)
		}
		assertBytes(t, out.ToArrayCopy(), want)
	})

	t.Run("SimpleFSIndexOutput", func(t *testing.T) {
		dir, err := NewSimpleFSDirectory(t.TempDir())
		if err != nil {
			t.Fatalf("NewSimpleFSDirectory: %v", err)
		}
		defer dir.Close()
		assertDirectoryLE(t, dir, writeAll, want)
	})

	t.Run("NIOFSIndexOutput", func(t *testing.T) {
		dir, err := NewNIOFSDirectory(t.TempDir())
		if err != nil {
			t.Fatalf("NewNIOFSDirectory: %v", err)
		}
		defer dir.Close()
		assertDirectoryLE(t, dir, writeAll, want)
	})

	t.Run("ByteBuffersDirectory", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()
		assertDirectoryLE(t, dir, writeAll, want)
	})
}

// assertDirectoryLE writes via the directory's IndexOutput, then reads the file
// back both as raw bytes (to assert LE layout) and via the directory's
// IndexInput primitives (to assert read/write agreement).
func assertDirectoryLE(t *testing.T, dir Directory, writeAll func(DataOutput) error, want []byte) {
	t.Helper()
	const name = "primitives.bin"

	out, err := dir.CreateOutput(name, IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := writeAll(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	in, err := dir.OpenInput(name, IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	// Raw bytes must be little-endian.
	raw := make([]byte, len(want))
	if err := in.ReadBytes(raw); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	assertBytes(t, raw, want)

	// Reading the primitives back must round-trip the original values.
	if err := in.SetPosition(0); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	gotShort, err := in.ReadShort()
	if err != nil {
		t.Fatalf("ReadShort: %v", err)
	}
	if gotShort != 0x0102 {
		t.Fatalf("ReadShort: got %#x want %#x", gotShort, 0x0102)
	}
	gotInt, err := in.ReadInt()
	if err != nil {
		t.Fatalf("ReadInt: %v", err)
	}
	if gotInt != 0x01020304 {
		t.Fatalf("ReadInt: got %#x want %#x", gotInt, 0x01020304)
	}
	gotLong, err := in.ReadLong()
	if err != nil {
		t.Fatalf("ReadLong: %v", err)
	}
	if gotLong != 0x0102030405060708 {
		t.Fatalf("ReadLong: got %#x want %#x", gotLong, int64(0x0102030405060708))
	}
}

// TestStoreCrossImplAgreement writes the same primitives via SimpleFS and
// reads the resulting bytes back through SimpleFS, NIOFS and a ByteBuffers
// input, asserting every implementation decodes identical values (rmp #4786).
func TestStoreCrossImplAgreement(t *testing.T) {
	const name = "xbytes.bin"
	const (
		shortVal = int16(-12345)
		intVal   = int32(-987654321)
		longVal  = int64(-1234567890123456789)
	)

	tmp := t.TempDir()
	fsDir, err := NewSimpleFSDirectory(tmp)
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer fsDir.Close()

	out, err := fsDir.CreateOutput(name, IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteShort(shortVal); err != nil {
		t.Fatalf("WriteShort: %v", err)
	}
	if err := out.WriteInt(intVal); err != nil {
		t.Fatalf("WriteInt: %v", err)
	}
	if err := out.WriteLong(longVal); err != nil {
		t.Fatalf("WriteLong: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Read the raw bytes once for the ByteBuffers path.
	fsIn, err := fsDir.OpenInput(name, IOContext{})
	if err != nil {
		t.Fatalf("OpenInput(fs): %v", err)
	}
	raw := make([]byte, 14)
	if err := fsIn.ReadBytes(raw); err != nil {
		fsIn.Close()
		t.Fatalf("ReadBytes: %v", err)
	}
	fsIn.Close()

	readers := map[string]func() (IndexInput, func(), error){
		"SimpleFS": func() (IndexInput, func(), error) {
			in, err := fsDir.OpenInput(name, IOContext{})
			return in, func() { _ = in.Close() }, err
		},
		"NIOFS": func() (IndexInput, func(), error) {
			d, err := NewNIOFSDirectory(tmp)
			if err != nil {
				return nil, func() {}, err
			}
			in, err := d.OpenInput(name, IOContext{})
			return in, func() { _ = in.Close(); _ = d.Close() }, err
		},
		"ByteBuffers": func() (IndexInput, func(), error) {
			// Re-materialise the exact FS bytes inside a ByteBuffersDirectory so
			// the ByteBuffersIndexInput is constructed through its normal path,
			// then assert it decodes the FS-written little-endian bytes
			// identically to the FS/NIOFS inputs.
			bbDir := NewByteBuffersDirectory()
			o, err := bbDir.CreateOutput(name, IOContext{})
			if err != nil {
				return nil, func() { _ = bbDir.Close() }, err
			}
			if err := o.WriteBytes(raw); err != nil {
				_ = o.Close()
				return nil, func() { _ = bbDir.Close() }, err
			}
			if err := o.Close(); err != nil {
				return nil, func() { _ = bbDir.Close() }, err
			}
			in, err := bbDir.OpenInput(name, IOContext{})
			return in, func() {
				if in != nil {
					_ = in.Close()
				}
				_ = bbDir.Close()
			}, err
		},
	}

	for label, open := range readers {
		t.Run(label, func(t *testing.T) {
			in, cleanup, err := open()
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer cleanup()

			gotShort, err := in.ReadShort()
			if err != nil {
				t.Fatalf("ReadShort: %v", err)
			}
			if gotShort != shortVal {
				t.Fatalf("ReadShort: got %d want %d", gotShort, shortVal)
			}
			gotInt, err := in.ReadInt()
			if err != nil {
				t.Fatalf("ReadInt: %v", err)
			}
			if gotInt != intVal {
				t.Fatalf("ReadInt: got %d want %d", gotInt, intVal)
			}
			gotLong, err := in.ReadLong()
			if err != nil {
				t.Fatalf("ReadLong: %v", err)
			}
			if gotLong != longVal {
				t.Fatalf("ReadLong: got %d want %d", gotLong, longVal)
			}
		})
	}
}

func assertBytes(t *testing.T, got, want []byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length: got %d want %d (got=%x want=%x)", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte[%d]: got %#x want %#x (got=%x want=%x)", i, got[i], want[i], got, want)
		}
	}
}
