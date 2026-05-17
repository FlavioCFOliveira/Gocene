// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// FilterCodec is a codec that wraps another codec and forwards every component
// accessor to the wrapped delegate. It is the Go port of
// org.apache.lucene.codecs.FilterCodec from Apache Lucene 10.4.0.
//
// Subclasses typically embed FilterCodec, choose a new codec name, and override
// only the accessors they need to customize (for example, replacing
// PostingsFormat while keeping the remaining components intact).
//
// Example:
//
//	type MyCodec struct {
//	    *FilterCodec
//	}
//
//	func NewMyCodec() *MyCodec {
//	    return &MyCodec{
//	        FilterCodec: NewFilterCodec("MyCodec", NewLucene104Codec()),
//	    }
//	}
//
//	func (c *MyCodec) PostingsFormat() PostingsFormat {
//	    return NewMyPostingsFormat()
//	}
type FilterCodec struct {
	*BaseCodec
	delegate Codec
}

// NewFilterCodec creates a FilterCodec that delegates every component lookup to
// the supplied codec. The name argument is used as the FilterCodec's own
// reported name (matching the Java constructor signature
// `protected FilterCodec(String name, Codec delegate)`).
//
// The delegate must be non-nil; callers passing nil will receive nil-pointer
// dereferences from the component accessors, mirroring the Java behavior that
// requires a delegate at construction time.
func NewFilterCodec(name string, delegate Codec) *FilterCodec {
	return &FilterCodec{
		BaseCodec: NewBaseCodec(name),
		delegate:  delegate,
	}
}

// Delegate returns the underlying codec this FilterCodec forwards to.
// This mirrors Java's `protected Codec delegate()` accessor used by subclasses.
func (c *FilterCodec) Delegate() Codec {
	return c.delegate
}

// PostingsFormat returns the delegate's postings format.
func (c *FilterCodec) PostingsFormat() PostingsFormat {
	return c.delegate.PostingsFormat()
}

// StoredFieldsFormat returns the delegate's stored fields format.
func (c *FilterCodec) StoredFieldsFormat() StoredFieldsFormat {
	return c.delegate.StoredFieldsFormat()
}

// FieldInfosFormat returns the delegate's field infos format.
func (c *FilterCodec) FieldInfosFormat() FieldInfosFormat {
	return c.delegate.FieldInfosFormat()
}

// SegmentInfosFormat returns the delegate's segment infos format.
func (c *FilterCodec) SegmentInfosFormat() SegmentInfosFormat {
	return c.delegate.SegmentInfosFormat()
}

// TermVectorsFormat returns the delegate's term vectors format.
func (c *FilterCodec) TermVectorsFormat() TermVectorsFormat {
	return c.delegate.TermVectorsFormat()
}

// DocValuesFormat returns the delegate's doc values format.
func (c *FilterCodec) DocValuesFormat() DocValuesFormat {
	return c.delegate.DocValuesFormat()
}

// Ensure FilterCodec satisfies the Codec interface.
var _ Codec = (*FilterCodec)(nil)
