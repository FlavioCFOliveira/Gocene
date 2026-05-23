// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene80DocValuesConsumer_ClosedMethods verifies that all Add* and Close
// methods on a manually-closed consumer return an error or no-op respectively.
func TestLucene80DocValuesConsumer_ClosedMethods(t *testing.T) {
	c := &Lucene80DocValuesConsumer{closed: true}

	if err := c.AddNumericField(nil, nil); err == nil {
		t.Error("AddNumericField on closed consumer: expected error")
	}
	if err := c.AddBinaryField(nil, nil); err == nil {
		t.Error("AddBinaryField on closed consumer: expected error")
	}
	if err := c.AddSortedField(nil, nil); err == nil {
		t.Error("AddSortedField on closed consumer: expected error")
	}
	if err := c.AddSortedSetField(nil, nil); err == nil {
		t.Error("AddSortedSetField on closed consumer: expected error")
	}
	if err := c.AddSortedNumericField(nil, nil); err == nil {
		t.Error("AddSortedNumericField on closed consumer: expected error")
	}
}

// TestLucene80DocValuesConsumer_DoubleClose verifies that Close on an already
// closed consumer is a no-op.
func TestLucene80DocValuesConsumer_DoubleClose(t *testing.T) {
	c := &Lucene80DocValuesConsumer{closed: true}
	if err := c.Close(); err != nil {
		t.Errorf("double Close: got %v", err)
	}
}

// TestLucene80DocValuesConsumer_DeferredAdd verifies that Add* methods on an
// open consumer return a non-nil error (full encoding is deferred).
func TestLucene80DocValuesConsumer_DeferredAdd(t *testing.T) {
	c := &Lucene80DocValuesConsumer{}

	if err := c.AddNumericField(nil, nil); err == nil {
		t.Error("AddNumericField: expected deferred error")
	}
	if err := c.AddBinaryField(nil, nil); err == nil {
		t.Error("AddBinaryField: expected deferred error")
	}
	if err := c.AddSortedField(nil, nil); err == nil {
		t.Error("AddSortedField: expected deferred error")
	}
	if err := c.AddSortedSetField(nil, nil); err == nil {
		t.Error("AddSortedSetField: expected deferred error")
	}
	if err := c.AddSortedNumericField(nil, nil); err == nil {
		t.Error("AddSortedNumericField: expected deferred error")
	}
}

// TestLucene80DocValuesConsumer_ImplementsInterface is a compile-time
// assertion surfaced as a runtime no-op.
func TestLucene80DocValuesConsumer_ImplementsInterface(t *testing.T) {
	var _ codecs.DocValuesConsumer = (*Lucene80DocValuesConsumer)(nil)
}
