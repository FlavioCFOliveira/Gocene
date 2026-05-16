// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// FilterIndexInput is an IndexInput implementation that delegates calls to
// another IndexInput. This is the decorator pattern applied to IndexInput.
//
// This is the Go port of org.apache.lucene.store.FilterIndexInput from
// Apache Lucene 10.4.0. It is intended for adding limitations or test
// instrumentation on top of an existing IndexInput; new IndexInput
// implementations should extend IndexInput / DataInput directly rather than
// reuse this wrapper.
//
// All methods forward to the wrapped delegate. Subclasses (embedders) may
// override any method to inject behaviour; the delegate is accessible via
// GetDelegate.
type FilterIndexInput struct {
	*BaseIndexInput
	in IndexInput
}

// NewFilterIndexInput creates a FilterIndexInput that wraps the given delegate
// IndexInput. resourceDescription mirrors Lucene's super(resourceDescription)
// constructor argument and is exposed via the embedded BaseIndexInput.
func NewFilterIndexInput(resourceDescription string, in IndexInput) *FilterIndexInput {
	return &FilterIndexInput{
		BaseIndexInput: NewBaseIndexInput(resourceDescription, in.Length()),
		in:             in,
	}
}

// GetDelegate returns the delegate that was passed in at construction.
func (f *FilterIndexInput) GetDelegate() IndexInput {
	return f.in
}

// UnwrapFilterIndexInput unwraps all FilterIndexInputs until it finds a
// non-FilterIndexInput delegate. Mirrors Lucene's FilterIndexInput.unwrap.
func UnwrapFilterIndexInput(in IndexInput) IndexInput {
	for {
		f, ok := in.(*FilterIndexInput)
		if !ok {
			return in
		}
		in = f.in
	}
}

// Close closes the wrapped IndexInput.
func (f *FilterIndexInput) Close() error { return f.in.Close() }

// GetFilePointer returns the current position from the wrapped input.
func (f *FilterIndexInput) GetFilePointer() int64 { return f.in.GetFilePointer() }

// SetPosition forwards the seek to the wrapped input.
func (f *FilterIndexInput) SetPosition(pos int64) error { return f.in.SetPosition(pos) }

// Length returns the wrapped input's length.
func (f *FilterIndexInput) Length() int64 { return f.in.Length() }

// Slice forwards to the wrapped input.
func (f *FilterIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	return f.in.Slice(desc, offset, length)
}

// Clone forwards to the wrapped input.
func (f *FilterIndexInput) Clone() IndexInput { return f.in.Clone() }

// ReadByte forwards to the wrapped input.
func (f *FilterIndexInput) ReadByte() (byte, error) { return f.in.ReadByte() }

// ReadBytes forwards to the wrapped input.
func (f *FilterIndexInput) ReadBytes(b []byte) error { return f.in.ReadBytes(b) }

// ReadBytesN forwards to the wrapped input.
func (f *FilterIndexInput) ReadBytesN(n int) ([]byte, error) { return f.in.ReadBytesN(n) }

// ReadShort forwards to the wrapped input.
func (f *FilterIndexInput) ReadShort() (int16, error) { return f.in.ReadShort() }

// ReadInt forwards to the wrapped input.
func (f *FilterIndexInput) ReadInt() (int32, error) { return f.in.ReadInt() }

// ReadLong forwards to the wrapped input.
func (f *FilterIndexInput) ReadLong() (int64, error) { return f.in.ReadLong() }

// ReadString forwards to the wrapped input.
func (f *FilterIndexInput) ReadString() (string, error) { return f.in.ReadString() }

// Compile-time assertion that FilterIndexInput satisfies IndexInput.
var _ IndexInput = (*FilterIndexInput)(nil)
