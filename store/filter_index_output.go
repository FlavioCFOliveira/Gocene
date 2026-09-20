// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// FilterIndexOutput is an IndexOutput implementation that delegates calls to
// another IndexOutput. This is the decorator pattern applied to IndexOutput.
//
// This is the Go port of org.apache.lucene.store.FilterIndexOutput from
// Apache Lucene 10.4.0. It is intended for adding limitations or test
// instrumentation on top of an existing IndexOutput; new IndexOutput
// implementations should extend IndexOutput / DataOutput directly rather
// than reuse this wrapper.
//
// All methods forward to the wrapped delegate. The delegate is accessible
// via GetDelegate.
type FilterIndexOutput struct {
	*spi.BaseIndexOutput
	out IndexOutput
}

// NewFilterIndexOutput creates a FilterIndexOutput wrapping the given
// delegate. resourceDescription mirrors Lucene's super(resourceDescription,
// name) call and is stored on the embedded spi.BaseIndexOutput.
func NewFilterIndexOutput(resourceDescription, name string, out IndexOutput) *FilterIndexOutput {
	_ = resourceDescription // spi.BaseIndexOutput only tracks name; description is informational
	return &FilterIndexOutput{
		BaseIndexOutput: spi.NewBaseIndexOutput(name),
		out:             out,
	}
}

// GetDelegate returns the delegate that was passed in at construction.
func (f *FilterIndexOutput) GetDelegate() IndexOutput { return f.out }

// UnwrapFilterIndexOutput unwraps all FilterIndexOutputs until it finds a
// non-FilterIndexOutput delegate. Mirrors Lucene's FilterIndexOutput.unwrap.
func UnwrapFilterIndexOutput(out IndexOutput) IndexOutput {
	for {
		f, ok := out.(*FilterIndexOutput)
		if !ok {
			return out
		}
		out = f.out
	}
}

// Close closes the wrapped IndexOutput.
func (f *FilterIndexOutput) Close() error { return f.out.Close() }

// GetFilePointer returns the current file pointer from the wrapped output.
func (f *FilterIndexOutput) GetFilePointer() int64 { return f.out.GetFilePointer() }

// SetPosition forwards to the wrapped output.
func (f *FilterIndexOutput) SetPosition(pos int64) error { return f.out.SetPosition(pos) }

// Length returns the wrapped output's length.
func (f *FilterIndexOutput) Length() int64 { return f.out.Length() }

// WriteByte forwards to the wrapped output.
func (f *FilterIndexOutput) WriteByte(b byte) error { return f.out.WriteByte(b) }

// WriteBytes forwards to the wrapped output.
func (f *FilterIndexOutput) WriteBytes(b []byte, offset, length int) error { return f.out.WriteBytes(b, offset, length) }

// WriteBytesN forwards to the wrapped output.
func (f *FilterIndexOutput) WriteBytesN(b []byte, n int) error { return f.out.WriteBytesN(b, n) }

// WriteShort forwards to the wrapped output.
func (f *FilterIndexOutput) WriteShort(v int16) error { return f.out.WriteShort(v) }

// WriteInt forwards to the wrapped output.
func (f *FilterIndexOutput) WriteInt(v int32) error { return f.out.WriteInt(v) }

// WriteLong forwards to the wrapped output.
func (f *FilterIndexOutput) WriteLong(v int64) error { return f.out.WriteLong(v) }

// WriteString forwards to the wrapped output.
func (f *FilterIndexOutput) WriteString(s string) error { return f.out.WriteString(s) }

// CopyBytes forwards to the wrapped output.
func (f *FilterIndexOutput) CopyBytes(input DataInput, numBytes int64) error { return f.out.CopyBytes(input, numBytes) }

// WriteGroupVInts forwards to the wrapped output.
func (f *FilterIndexOutput) WriteGroupVInts(values []int32, limit int) error { return f.out.WriteGroupVInts(values, limit) }

// WriteSetOfStrings forwards to the wrapped output.
func (f *FilterIndexOutput) WriteSetOfStrings(s []string) error { return f.out.WriteSetOfStrings(s) }

// WriteMapOfStrings forwards to the wrapped output.
func (f *FilterIndexOutput) WriteMapOfStrings(m map[string]string) error { return f.out.WriteMapOfStrings(m) }

// WriteVInt forwards to the wrapped output.
func (f *FilterIndexOutput) WriteVInt(i int32) error { return f.out.WriteVInt(i) }

// WriteVLong forwards to the wrapped output.
func (f *FilterIndexOutput) WriteVLong(i int64) error { return f.out.WriteVLong(i) }

// WriteZInt forwards to the wrapped output.
func (f *FilterIndexOutput) WriteZInt(i int32) error { return f.out.WriteZInt(i) }

// WriteZLong forwards to the wrapped output.
func (f *FilterIndexOutput) WriteZLong(i int64) error { return f.out.WriteZLong(i) }

// Compile-time assertion that FilterIndexOutput satisfies IndexOutput.
var _ IndexOutput = (*FilterIndexOutput)(nil)
