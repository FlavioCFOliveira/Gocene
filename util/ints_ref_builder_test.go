// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestIntsRefBuilder_StartsEmpty(t *testing.T) {
	b := NewIntsRefBuilder()
	if b.Length() != 0 {
		t.Fatalf("fresh Length=%d want 0", b.Length())
	}
	if len(b.Ints()) != 0 {
		t.Fatalf("fresh Ints()=%v want empty", b.Ints())
	}
}

func TestIntsRefBuilder_AppendGrows(t *testing.T) {
	b := NewIntsRefBuilder()
	for i := 0; i < 130; i++ {
		b.Append(i * 7)
	}
	if b.Length() != 130 {
		t.Fatalf("Length=%d want 130", b.Length())
	}
	for i := 0; i < 130; i++ {
		if b.IntAt(i) != i*7 {
			t.Fatalf("IntAt(%d)=%d want %d", i, b.IntAt(i), i*7)
		}
	}
}

func TestIntsRefBuilder_SetIntAtAndClear(t *testing.T) {
	b := NewIntsRefBuilder()
	for i := 0; i < 4; i++ {
		b.Append(i)
	}
	b.SetIntAt(2, 99)
	if b.IntAt(2) != 99 {
		t.Fatalf("SetIntAt did not persist")
	}
	b.Clear()
	if b.Length() != 0 {
		t.Fatalf("Length after Clear=%d want 0", b.Length())
	}
	// Buffer must be retained for reuse.
	if len(b.Ints()) == 0 {
		t.Fatalf("Clear should not free buffer")
	}
}

func TestIntsRefBuilder_SetLength(t *testing.T) {
	b := NewIntsRefBuilder()
	b.Grow(10)
	b.SetLength(5)
	if b.Length() != 5 {
		t.Fatalf("SetLength: Length=%d want 5", b.Length())
	}
}

func TestIntsRefBuilder_CopyInts(t *testing.T) {
	b := NewIntsRefBuilder()
	src := []int{1, 2, 3, 4, 5}
	b.CopyInts(src, 1, 3)
	if b.Length() != 3 {
		t.Fatalf("Length=%d want 3", b.Length())
	}
	for i, want := range []int{2, 3, 4} {
		if b.IntAt(i) != want {
			t.Fatalf("IntAt(%d)=%d want %d", i, b.IntAt(i), want)
		}
	}
}

func TestIntsRefBuilder_CopyIntsRefThenGetThenToIntsRef(t *testing.T) {
	b := NewIntsRefBuilder()
	src := NewIntsRefFromSlice([]int{10, 20, 30}, 0, 3)
	b.CopyIntsRef(src)

	got := b.Get()
	if got.Length != 3 || got.Offset != 0 {
		t.Fatalf("Get: offset=%d length=%d", got.Offset, got.Length)
	}
	if !IntsRefEquals(got, src) {
		t.Fatalf("Get contents differ from src")
	}
	deep := b.ToIntsRef()
	if &deep.Ints[0] == &b.Ints()[0] {
		t.Fatalf("ToIntsRef must not alias builder buffer")
	}
}

func TestIntsRefBuilder_GrowNoCopyDiscardsContents(t *testing.T) {
	b := NewIntsRefBuilder()
	for i := 0; i < 5; i++ {
		b.Append(i + 1)
	}
	prev := b.Ints()
	b.GrowNoCopy(1024)
	if &b.Ints()[0] == &prev[0] {
		// Allowed if existing storage already big enough; force a real grow.
		t.Logf("storage reused (cap=%d)", len(prev))
	}
	// Note: contents are explicitly NOT preserved by GrowNoCopy.
	// The builder.Length remains 5 but Ints() is now uninitialized.
	if b.Length() != 5 {
		t.Fatalf("GrowNoCopy must not change Length, got %d", b.Length())
	}
}

func TestIntsRefBuilder_GrowPreservesContents(t *testing.T) {
	b := NewIntsRefBuilder()
	for i := 0; i < 5; i++ {
		b.Append(i + 1)
	}
	b.Grow(2048)
	for i := 0; i < 5; i++ {
		if b.IntAt(i) != i+1 {
			t.Fatalf("Grow lost contents at %d: %d", i, b.IntAt(i))
		}
	}
}

func TestIntsRefBuilder_CopyUTF8Bytes_ASCII(t *testing.T) {
	br := NewBytesRef([]byte("hello"))
	b := NewIntsRefBuilder()
	b.CopyUTF8Bytes(br)
	want := []rune("hello")
	if b.Length() != len(want) {
		t.Fatalf("Length=%d want %d", b.Length(), len(want))
	}
	for i, r := range want {
		if b.IntAt(i) != int(r) {
			t.Fatalf("IntAt(%d)=%d want %d", i, b.IntAt(i), int(r))
		}
	}
}

func TestIntsRefBuilder_CopyUTF8Bytes_Unicode(t *testing.T) {
	// "héllo\U0001F680" mixes 1-, 2-, and 4-byte UTF-8 sequences.
	br := NewBytesRef([]byte("héllo\U0001F680"))
	b := NewIntsRefBuilder()
	b.CopyUTF8Bytes(br)
	want := []rune("héllo\U0001F680")
	if b.Length() != len(want) {
		t.Fatalf("Length=%d want %d", b.Length(), len(want))
	}
	for i, r := range want {
		if b.IntAt(i) != int(r) {
			t.Fatalf("IntAt(%d)=%d want %d", i, b.IntAt(i), int(r))
		}
	}
}

func TestIntsRefBuilder_GetPanicsOnNonZeroOffset(t *testing.T) {
	b := NewIntsRefBuilder()
	b.Append(1)
	b.ref.Offset = 1 // intentional: shouldn't happen via public API.

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on Get with non-zero offset")
		}
	}()
	b.Get()
}
