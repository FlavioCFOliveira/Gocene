// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import (
	"errors"
	"testing"
)

// TestLucene912RWPostingsFormat_Name verifies the format name.
func TestLucene912RWPostingsFormat_Name(t *testing.T) {
	f := NewLucene912PostingsFormat()
	if got := f.Name(); got != "Lucene912" {
		t.Errorf("Name() = %q, want %q", got, "Lucene912")
	}
}

// TestLucene912RWPostingsFormat_FieldsConsumerError verifies that FieldsConsumer
// returns ErrWriteNotSupported.
func TestLucene912RWPostingsFormat_FieldsConsumerError(t *testing.T) {
	f := NewLucene912PostingsFormat()
	_, err := f.FieldsConsumer(nil)
	if err == nil {
		t.Fatal("FieldsConsumer: expected error, got nil")
	}
	if !errors.Is(err, ErrWriteNotSupported) {
		t.Errorf("FieldsConsumer error = %v, want ErrWriteNotSupported", err)
	}
}

// TestLucene912RWPostingsFormat_FieldsProducerError verifies that FieldsProducer
// returns an error (backward format not available for reading postings).
func TestLucene912RWPostingsFormat_FieldsProducerError(t *testing.T) {
	f := NewLucene912PostingsFormat()
	_, err := f.FieldsProducer(nil)
	if err == nil {
		t.Fatal("FieldsProducer: expected error, got nil")
	}
}

// TestLucene912RWPostingsFormat_IntBlockTermState verifies the term state constructor.
func TestLucene912RWPostingsFormat_IntBlockTermState(t *testing.T) {
	s := NewIntBlockTermState()
	if s.LastPosBlockOffset != -1 {
		t.Errorf("LastPosBlockOffset = %d, want -1", s.LastPosBlockOffset)
	}
	if s.SingletonDocID != -1 {
		t.Errorf("SingletonDocID = %d, want -1", s.SingletonDocID)
	}
	if s.BlockTermState == nil {
		t.Error("BlockTermState is nil")
	}
}
