// Package store implements org.apache.lucene.misc.store.
package store

import "io"

// ByteTrackingIndexOutput is the IndexOutput wrapper that records the total
// bytes written. Mirrors
// org.apache.lucene.misc.store.ByteTrackingIndexOutput.
type ByteTrackingIndexOutput struct {
	Writer io.Writer
	Bytes  int64
}

// NewByteTrackingIndexOutput wraps w.
func NewByteTrackingIndexOutput(w io.Writer) *ByteTrackingIndexOutput {
	return &ByteTrackingIndexOutput{Writer: w}
}

// Write tracks the byte count and forwards to the wrapped writer.
func (o *ByteTrackingIndexOutput) Write(p []byte) (int, error) {
	n, err := o.Writer.Write(p)
	o.Bytes += int64(n)
	return n, err
}

// ByteWritesTrackingDirectoryWrapper wraps a Directory and counts the bytes
// written across every IndexOutput it creates. Mirrors
// org.apache.lucene.misc.store.ByteWritesTrackingDirectoryWrapper.
type ByteWritesTrackingDirectoryWrapper struct {
	Delegate  Directory
	BytesSeen int64
}

// Directory is the minimum contract this wrapper needs.
type Directory interface {
	CreateOutput(name string) (io.Writer, error)
}

// NewByteWritesTrackingDirectoryWrapper wraps delegate.
func NewByteWritesTrackingDirectoryWrapper(delegate Directory) *ByteWritesTrackingDirectoryWrapper {
	return &ByteWritesTrackingDirectoryWrapper{Delegate: delegate}
}

// CreateOutput wraps the underlying output in a byte tracker that shares
// the parent counter.
func (d *ByteWritesTrackingDirectoryWrapper) CreateOutput(name string) (io.Writer, error) {
	w, err := d.Delegate.CreateOutput(name)
	if err != nil {
		return nil, err
	}
	return &sharedTracker{parent: d, w: w}, nil
}

type sharedTracker struct {
	parent *ByteWritesTrackingDirectoryWrapper
	w      io.Writer
}

func (t *sharedTracker) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	t.parent.BytesSeen += int64(n)
	return n, err
}

// DirectIODirectory is the Directory backed by direct-I/O reads/writes.
// Mirrors org.apache.lucene.misc.store.DirectIODirectory. The Go port keeps
// the surface and defers concrete O_DIRECT handling to platform-specific
// builds.
type DirectIODirectory struct {
	Path string
}

// NewDirectIODirectory builds a DirectIODirectory rooted at path.
func NewDirectIODirectory(path string) *DirectIODirectory {
	return &DirectIODirectory{Path: path}
}

// HardlinkCopyDirectoryWrapper copies files via filesystem hardlinks rather
// than a full byte copy. Mirrors
// org.apache.lucene.misc.store.HardlinkCopyDirectoryWrapper.
type HardlinkCopyDirectoryWrapper struct {
	Delegate Directory
}

// NewHardlinkCopyDirectoryWrapper builds the wrapper.
func NewHardlinkCopyDirectoryWrapper(delegate Directory) *HardlinkCopyDirectoryWrapper {
	return &HardlinkCopyDirectoryWrapper{Delegate: delegate}
}

// RAFDirectory is the Directory backed by RandomAccessFile-style I/O.
// Mirrors org.apache.lucene.misc.store.RAFDirectory.
type RAFDirectory struct {
	Path string
}

// NewRAFDirectory builds the directory.
func NewRAFDirectory(path string) *RAFDirectory {
	return &RAFDirectory{Path: path}
}
