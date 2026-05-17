// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import "testing"

func TestFilterCodec_DelegatesToWrapped(t *testing.T) {
	t.Parallel()

	delegate := NewLucene104Codec()
	fc := NewFilterCodec("MyFilteredCodec", delegate)

	if got, want := fc.Name(), "MyFilteredCodec"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if fc.Delegate() != delegate {
		t.Errorf("Delegate() did not return the wrapped codec")
	}
	if fc.PostingsFormat() != delegate.PostingsFormat() {
		t.Errorf("PostingsFormat() did not forward to delegate")
	}
	if fc.StoredFieldsFormat() != delegate.StoredFieldsFormat() {
		t.Errorf("StoredFieldsFormat() did not forward to delegate")
	}
	if fc.FieldInfosFormat() != delegate.FieldInfosFormat() {
		t.Errorf("FieldInfosFormat() did not forward to delegate")
	}
	if fc.SegmentInfosFormat() != delegate.SegmentInfosFormat() {
		t.Errorf("SegmentInfosFormat() did not forward to delegate")
	}
	if fc.TermVectorsFormat() != delegate.TermVectorsFormat() {
		t.Errorf("TermVectorsFormat() did not forward to delegate")
	}
	if fc.DocValuesFormat() != delegate.DocValuesFormat() {
		t.Errorf("DocValuesFormat() did not forward to delegate")
	}
}

// filterCodecOverride embeds FilterCodec and overrides only one component,
// demonstrating the intended composition pattern.
type filterCodecOverride struct {
	*FilterCodec
	custom PostingsFormat
}

func (c *filterCodecOverride) PostingsFormat() PostingsFormat {
	return c.custom
}

func TestFilterCodec_SubclassCanOverrideSingleComponent(t *testing.T) {
	t.Parallel()

	delegate := NewLucene104Codec()
	custom := NewLucene104PostingsFormat()
	override := &filterCodecOverride{
		FilterCodec: NewFilterCodec("Overridden", delegate),
		custom:      custom,
	}

	if override.Name() != "Overridden" {
		t.Errorf("Name() = %q, want %q", override.Name(), "Overridden")
	}
	if override.PostingsFormat() != custom {
		t.Errorf("PostingsFormat() did not return overridden custom format")
	}
	// Non-overridden components still flow through delegate.
	if override.StoredFieldsFormat() != delegate.StoredFieldsFormat() {
		t.Errorf("StoredFieldsFormat() should still forward to delegate")
	}
	if override.FieldInfosFormat() != delegate.FieldInfosFormat() {
		t.Errorf("FieldInfosFormat() should still forward to delegate")
	}
}
