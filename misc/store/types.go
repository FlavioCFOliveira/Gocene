// Package store implements org.apache.lucene.misc.store.
package store

import (
	"errors"
	"io"
	"os"
)


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

// FSHardlinkCopyDirectoryWrapper wraps an OS directory path and overrides
// CopyFrom to use a filesystem hardlink instead of a byte-by-byte copy when
// the source is also an FSHardlinkCopyDirectoryWrapper on the same filesystem.
// Falls back to io.Copy when hardlinks are unavailable. Mirrors the core
// behaviour of org.apache.lucene.misc.store.HardlinkCopyDirectoryWrapper
// for FS-backed directories.
type FSHardlinkCopyDirectoryWrapper struct {
	// Dir is the absolute OS path of the directory managed by this wrapper.
	Dir string
}

// NewFSHardlinkCopyDirectoryWrapper creates a wrapper rooted at dir.
func NewFSHardlinkCopyDirectoryWrapper(dir string) *FSHardlinkCopyDirectoryWrapper {
	return &FSHardlinkCopyDirectoryWrapper{Dir: dir}
}

// CopyFrom copies srcFile from src into this directory as destFile.
// When src is also an *FSHardlinkCopyDirectoryWrapper on the same mount,
// it attempts os.Link first and falls back to a full byte copy on failure.
func (w *FSHardlinkCopyDirectoryWrapper) CopyFrom(src *FSHardlinkCopyDirectoryWrapper, srcFile, destFile string) error {
	srcPath := src.Dir + string(os.PathSeparator) + srcFile
	dstPath := w.Dir + string(os.PathSeparator) + destFile

	// Attempt hardlink first.
	if err := os.Link(srcPath, dstPath); err == nil {
		return nil
	}
	// Fallback: byte copy.
	return copyFile(srcPath, dstPath)
}

// copyFile performs a byte-level file copy from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	buf := make([]byte, 32*1024)
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
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
