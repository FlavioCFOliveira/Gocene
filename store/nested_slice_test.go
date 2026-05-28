// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "testing"

// TestNestedSliceComposesOffset is the rmp #4747 regression test: slicing a
// slice must compose the parent's base offset. The compound-file reader slices
// a sub-file out of the .cfs, and codecs then slice within that sub-file; if the
// nested slice ignores the parent base, reads land on the wrong bytes and the
// block-tree term dictionary / postings headers decode garbage.
//
// Underlying file is bytes [0,1,...,99]. s1 = Slice(10,40) covers file[10:50];
// s2 = s1.Slice(5,20) must cover file[15:35], so s2's first byte is file[15]=15.
func TestNestedSliceComposesOffset(t *testing.T) {
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}

	run := func(t *testing.T, in IndexInput) {
		s1, err := in.Slice("s1", 10, 40)
		if err != nil {
			t.Fatalf("s1 = Slice(10,40): %v", err)
		}
		s2, err := s1.Slice("s2", 5, 20)
		if err != nil {
			t.Fatalf("s2 = s1.Slice(5,20): %v", err)
		}
		if got := s2.Length(); got != 20 {
			t.Errorf("s2.Length() = %d, want 20", got)
		}
		got, err := s2.ReadByte()
		if err != nil {
			t.Fatalf("s2.ReadByte(): %v", err)
		}
		if got != 15 {
			t.Errorf("nested slice first byte = %d, want 15 (file[10+5]); parent offset not composed", got)
		}
		// Second byte must continue from the composed base.
		if got, err := s2.ReadByte(); err != nil || got != 16 {
			t.Errorf("nested slice second byte = %d (err %v), want 16", got, err)
		}
	}

	t.Run("ByteBuffers", func(t *testing.T) {
		dir := NewByteBuffersDirectory()
		defer dir.Close()
		out, _ := dir.CreateOutput("d", IOContext{})
		_ = out.WriteBytes(data)
		_ = out.Close()
		in, err := dir.OpenInput("d", IOContext{})
		if err != nil {
			t.Fatalf("OpenInput: %v", err)
		}
		defer in.Close()
		run(t, in)
	})

	t.Run("SimpleFS", func(t *testing.T) {
		dir, err := NewSimpleFSDirectory(t.TempDir())
		if err != nil {
			t.Fatalf("NewSimpleFSDirectory: %v", err)
		}
		defer dir.Close()
		out, _ := dir.CreateOutput("d", IOContext{})
		_ = out.WriteBytes(data)
		_ = out.Close()
		in, err := dir.OpenInput("d", IOContext{})
		if err != nil {
			t.Fatalf("OpenInput: %v", err)
		}
		defer in.Close()
		run(t, in)
	})

	t.Run("MMap", func(t *testing.T) {
		dir, err := NewMMapDirectory(t.TempDir())
		if err != nil {
			t.Skipf("MMapDirectory unavailable: %v", err)
		}
		defer dir.Close()
		out, _ := dir.CreateOutput("d", IOContext{})
		_ = out.WriteBytes(data)
		_ = out.Close()
		in, err := dir.OpenInput("d", IOContext{})
		if err != nil {
			t.Fatalf("OpenInput: %v", err)
		}
		defer in.Close()
		run(t, in)
	})
}
