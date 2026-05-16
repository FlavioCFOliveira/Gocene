// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package quantization

import (
	"errors"
	"testing"
)

// stubQuantizedVectorsReader is a minimal implementation that proves
// the interface shape compiles.
type stubQuantizedVectorsReader struct {
	values QuantizedByteVectorValues
	state  ScalarQuantizer
	closed bool
}

func (s *stubQuantizedVectorsReader) GetQuantizedVectorValues(fieldName string) (QuantizedByteVectorValues, error) {
	if fieldName == "" {
		return nil, errors.New("empty fieldName")
	}
	return s.values, nil
}

func (s *stubQuantizedVectorsReader) GetQuantizationState(fieldName string) ScalarQuantizer {
	return s.state
}

func (s *stubQuantizedVectorsReader) RamBytesUsed() int64 { return 0 }
func (s *stubQuantizedVectorsReader) Close() error        { s.closed = true; return nil }

func TestQuantizedVectorsReaderInterface(t *testing.T) {
	r := &stubQuantizedVectorsReader{}
	var qvr QuantizedVectorsReader = r

	if _, err := qvr.GetQuantizedVectorValues(""); err == nil {
		t.Fatalf("expected error for empty fieldName")
	}
	if _, err := qvr.GetQuantizedVectorValues("field"); err != nil {
		t.Fatalf("GetQuantizedVectorValues: %v", err)
	}
	if got := qvr.GetQuantizationState("field"); got != nil {
		t.Errorf("GetQuantizationState: got non-nil from nil stub")
	}
	if err := qvr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !r.closed {
		t.Fatalf("Close did not set closed=true")
	}
	if qvr.RamBytesUsed() != 0 {
		t.Errorf("RamBytesUsed: got %d want 0", qvr.RamBytesUsed())
	}
}
