// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"testing"
)

// stubPointReader checks that PointReader can be satisfied by an
// arbitrary implementation; it exists only for compile-time
// interface conformance.
type stubPointReader struct{ closed bool }

func (s *stubPointReader) Next() (bool, error) { return false, nil }
func (s *stubPointReader) PointValue() PointValue {
	return &stubPointValue{}
}
func (s *stubPointReader) Close() error { s.closed = true; return nil }

// stubPointWriter mirrors stubPointReader.
type stubPointWriter struct{ n int64 }

func (s *stubPointWriter) Append(packedValue []byte, docID int) error      { s.n++; return nil }
func (s *stubPointWriter) AppendPointValue(pointValue PointValue) error    { s.n++; return nil }
func (s *stubPointWriter) GetReader(startPoint, length int64) (PointReader, error) {
	return &stubPointReader{}, nil
}
func (s *stubPointWriter) Count() int64    { return s.n }
func (s *stubPointWriter) Destroy() error  { return nil }
func (s *stubPointWriter) Close() error    { return nil }

func TestPointReaderInterface(t *testing.T) {
	var r PointReader = &stubPointReader{}
	ok, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ok {
		t.Fatalf("Next: got true want false")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestPointWriterInterface(t *testing.T) {
	var w PointWriter = &stubPointWriter{}
	if err := w.Append([]byte{1, 2, 3, 4}, 7); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.AppendPointValue(&stubPointValue{}); err != nil {
		t.Fatalf("AppendPointValue: %v", err)
	}
	if w.Count() != 2 {
		t.Fatalf("Count: got %d want 2", w.Count())
	}
	r, err := w.GetReader(0, 2)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	if r == nil {
		t.Fatalf("GetReader returned nil")
	}
	if err := w.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
