// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package nrt provides Near-Real-Time replication primitives for Lucene
// segment-level replication between a primary and one or more replica nodes.
//
// Architecture summary
//
// A PrimaryNode holds an IndexWriter and serves CopyState snapshots on
// demand. Each CopyState captures the current SegmentInfos (serialised as
// bytes) plus per-file header/footer/checksum metadata needed for identity
// comparison.
//
// A ReplicaNode receives a new NRT version notification via NewNRTPoint,
// determines which files it is missing (by comparing local FileMetaData
// against the primary's CopyState), and drives a CopyJob to fetch them.
// Once all files have been transferred, SegmentInfosSearcherManager cuts
// over to the new SegmentInfos, making changes searchable.
//
// Deviations from Java
//
//   - IndexWriter, SearcherManager, and the full IndexSearcher stack are not
//     yet wired in; PrimaryNode.setCurrentInfos, ReplicaNode.start, and the
//     concurrent CopyJob machinery remain stubs.
//   - ReplicaNode.NewNRTPoint implements the version-guard and primary-gen
//     cut-over logic but does not yet launch a background CopyJob.
//   - CopyOneFile.Copy performs the actual byte-for-byte I/O that the Java
//     visit() loop does, but the parent CopyJob orchestration is a stub.
//   - ReplicaFileDeleter calls Directory.DeleteFile when the refcount reaches
//     zero; callers that pass nil for dir receive safe no-op deletions.
//
// Port of org.apache.lucene.replicator.nrt.
package nrt

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// NodeCommunicationException
//
// Port of org.apache.lucene.replicator.nrt.NodeCommunicationException.
// ---------------------------------------------------------------------------

// NodeCommunicationException is a non-fatal error signalling a problem
// communicating between NRT nodes.
//
// Port of org.apache.lucene.replicator.nrt.NodeCommunicationException.
type NodeCommunicationException struct {
	when  string
	cause error
}

// NewNodeCommunicationException constructs a NodeCommunicationException.
//
// Port of org.apache.lucene.replicator.nrt.NodeCommunicationException(String,Throwable).
func NewNodeCommunicationException(when string, cause error) *NodeCommunicationException {
	if cause == nil {
		panic("cause must not be nil")
	}
	return &NodeCommunicationException{when: when, cause: cause}
}

// Error implements the error interface.
func (e *NodeCommunicationException) Error() string {
	return fmt.Sprintf("NodeCommunicationException(%s): %v", e.when, e.cause)
}

// Unwrap returns the underlying cause.
func (e *NodeCommunicationException) Unwrap() error { return e.cause }

// ---------------------------------------------------------------------------
// FileMetaData
//
// Port of org.apache.lucene.replicator.nrt.FileMetaData.
// ---------------------------------------------------------------------------

// FileMetaData holds identity metadata for a single segment file.
//
// The Header bytes are the full index header (codec magic + codec name +
// version + 16-byte segment ID + suffix), read from position 0. The Footer
// bytes are the 16-byte codec footer (footer magic + algo + CRC checksum),
// read from the end of the file.
//
// Both slices must match between primary and replica for a file to be
// considered identical. This matches Lucene's readIndexHeader / readFooter
// convention.
//
// Port of org.apache.lucene.replicator.nrt.FileMetaData.
type FileMetaData struct {
	// Header is the full codec index-header bytes (from position 0).
	Header []byte
	// Footer is the last footerLength() bytes of the file.
	Footer []byte
	// Length is the total byte length of the file.
	Length int64
	// Checksum is the CRC32 value stored inside the footer.
	Checksum int64
}

// String returns a human-readable representation.
func (f *FileMetaData) String() string {
	return fmt.Sprintf("FileMetaData(length=%d checksum=%d)", f.Length, f.Checksum)
}

// ---------------------------------------------------------------------------
// CopyState
//
// Port of org.apache.lucene.replicator.nrt.CopyState.
// ---------------------------------------------------------------------------

// CopyState holds a snapshot of the files at one point-in-time on the primary.
// It is passed from primary to replica to drive a copy operation.
//
// Port of org.apache.lucene.replicator.nrt.CopyState.
type CopyState struct {
	// Files maps filename → FileMetaData for all files in this state.
	Files map[string]*FileMetaData
	// Version is the NRT generation version.
	Version int64
	// Gen is the segment-infos generation.
	Gen int64
	// InfosBytes is the serialised SegmentInfos bytes.
	InfosBytes []byte
	// CompletedMergeFiles is the set of files that finished merging.
	CompletedMergeFiles map[string]struct{}
	// PrimaryGen increments each time a new primary is elected.
	PrimaryGen int64
	// Infos is the live SegmentInfos on the primary (nil on the replica side).
	// Represented as interface{} because SegmentInfos full wiring is deferred.
	Infos interface{}
}

// String returns a human-readable representation.
func (s *CopyState) String() string {
	return fmt.Sprintf("CopyState(version=%d)", s.Version)
}

// ---------------------------------------------------------------------------
// CopyJob — abstract base
//
// Port of org.apache.lucene.replicator.nrt.CopyJob.
// ---------------------------------------------------------------------------

// copyJobCounter provides globally unique ordinals for CopyJob instances.
var copyJobCounter atomic.Int64

// OnceDone is called exactly once when a CopyJob finishes or is cancelled.
//
// Port of org.apache.lucene.replicator.nrt.CopyJob.OnceDone.
type OnceDone func(job *CopyJob) error

// CopyJob coordinates the copy of a set of segment files from the primary to
// the replica.
//
// The full background-copy machinery (start / runBlocking / finish, transfer-
// and-cancel, per-file conflict tracking) requires the IndexWriter and network
// transport layer and remains a stub. The fields and Cancel/GetFailed
// primitives are fully functional.
//
// Port of org.apache.lucene.replicator.nrt.CopyJob.
type CopyJob struct {
	mu           sync.Mutex
	Ord          int64
	HighPriority bool
	Reason       string
	onceDone     OnceDone

	Files map[string]*FileMetaData

	exc          error
	cancelReason string

	TotBytes       int64
	TotBytesCopied int64
}

// NewCopyJob constructs a CopyJob.
//
// Port of org.apache.lucene.replicator.nrt.CopyJob(String,Map,ReplicaNode,boolean,OnceDone).
func NewCopyJob(reason string, files map[string]*FileMetaData, highPriority bool, onceDone OnceDone) *CopyJob {
	j := &CopyJob{
		Ord:          copyJobCounter.Add(1),
		HighPriority: highPriority,
		Reason:       reason,
		onceDone:     onceDone,
		Files:        files,
	}
	return j
}

// Cancel marks this job as cancelled with the supplied reason and optional
// error. Subsequent calls are no-ops (first cancellation wins).
//
// Port of org.apache.lucene.replicator.nrt.CopyJob.cancel.
func (j *CopyJob) Cancel(reason string, exc error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.exc != nil {
		// Already cancelled — first write wins.
		return
	}
	if exc == nil {
		exc = errors.New(reason)
	}
	j.cancelReason = reason
	j.exc = exc
}

// GetFailed reports whether this job encountered an error or was cancelled.
func (j *CopyJob) GetFailed() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.exc != nil
}

// ---------------------------------------------------------------------------
// CopyOneFile
//
// Port of org.apache.lucene.replicator.nrt.CopyOneFile.
// ---------------------------------------------------------------------------

// copyBufSize is the per-chunk copy size used by Copy, matching Java's
// CopyOneFile buffer which is sized for 10 × 64 KB = 640 KB per visit.
const copyBufSize = 64 * 1024

// CopyOneFile copies a single file from a source IndexInput to a temporary
// IndexOutput in the replica directory.
//
// The Java implementation receives a DataInput from a network connection and
// writes to a local temp output. Gocene's Copy method mirrors visit()'s
// chunked byte-loop followed by checksum verification: it reads
// (metaData.Length − 8) data bytes in 64 KB chunks, then reads the big-endian
// 8-byte checksum written by the primary, verifies it against the output's
// running CRC, and writes it to the output.
//
// Deviations
//
//   - The Java constructor also creates the temp output via dest.createTempOutput;
//     Gocene's constructor only stores the already-created names.
//   - CopyOneFile(CopyOneFile, DataInput) transfer constructor is not yet
//     implemented (requires the full CopyJob transfer-and-cancel machinery).
//
// Port of org.apache.lucene.replicator.nrt.CopyOneFile.
type CopyOneFile struct {
	name     string
	tmpName  string
	metaData *FileMetaData

	// bytesCopied tracks progress.
	bytesCopied int64
}

// NewCopyOneFile constructs a CopyOneFile.
//
// name is the canonical segment file name; tmpName is the temporary file
// written to the replica directory during the copy; metaData holds the
// expected length and checksum.
//
// Port of org.apache.lucene.replicator.nrt.CopyOneFile(DataInput,ReplicaNode,String,FileMetaData,byte[]).
func NewCopyOneFile(name, tmpName string, metaData *FileMetaData) *CopyOneFile {
	return &CopyOneFile{name: name, tmpName: tmpName, metaData: metaData}
}

// BytesCopied returns the number of bytes copied so far.
func (c *CopyOneFile) BytesCopied() int64 { return c.bytesCopied }

// FileName returns the canonical target file name.
func (c *CopyOneFile) FileName() string { return c.name }

// TmpFileName returns the temporary file name used during copy.
func (c *CopyOneFile) TmpFileName() string { return c.tmpName }

// Close is a no-op; the caller is responsible for closing in and out.
func (c *CopyOneFile) Close() error { return nil }

// Copy reads metaData.Length bytes from in and writes them to out, verifying
// the CRC32 checksum embedded in the last 8 bytes.
//
// The source layout (matching Java CopyOneFile.visit) is:
//
//	[0 .. Length−9]    data bytes (written chunked)
//	[Length−8 .. end]  big-endian int64 CRC32 checksum
//
// out must implement the checksumWriter interface (i.e. *store.ChecksumIndexOutput)
// so that we can retrieve the running checksum after all data bytes are written.
//
// Port of org.apache.lucene.replicator.nrt.CopyOneFile.visit().
func (c *CopyOneFile) Copy(in store.IndexInput, out store.IndexOutput) error {
	cw, ok := out.(interface{ GetChecksum() uint32 })
	if !ok {
		return fmt.Errorf("CopyOneFile.Copy: out must expose GetChecksum() (use *store.ChecksumIndexOutput)")
	}

	// Java: bytesToCopy = metaData.length() - Long.BYTES (i.e. exclude the footer checksum long).
	bytesToCopy := c.metaData.Length - 8
	if bytesToCopy < 0 {
		return fmt.Errorf("CopyOneFile.Copy: file %q length %d is shorter than 8 bytes", c.name, c.metaData.Length)
	}

	buf := make([]byte, copyBufSize)
	remaining := bytesToCopy
	for remaining > 0 {
		chunk := int64(len(buf))
		if remaining < chunk {
			chunk = remaining
		}
		b := buf[:chunk]
		if err := in.ReadBytes(b); err != nil {
			return fmt.Errorf("CopyOneFile.Copy: reading %q: %w", c.name, err)
		}
		if err := out.WriteBytes(b); err != nil {
			return fmt.Errorf("CopyOneFile.Copy: writing %q: %w", c.name, err)
		}
		c.bytesCopied += chunk
		remaining -= chunk
	}

	// Verify the running checksum matches what the primary encoded in the file.
	checksum := int64(cw.GetChecksum())
	if checksum != c.metaData.Checksum {
		return fmt.Errorf("file %q: checksum mismatch after copy (bits flipped during transfer?) "+
			"after-copy checksum=%d vs expected=%d",
			c.name, checksum, c.metaData.Checksum)
	}

	// Read and verify the big-endian checksum long that the primary appended.
	// Java: CodecUtil.readBELong(in) → verify → CodecUtil.writeBELong(out, checksum).
	var checksumBuf [8]byte
	if err := in.ReadBytes(checksumBuf[:]); err != nil {
		return fmt.Errorf("CopyOneFile.Copy: reading checksum long for %q: %w", c.name, err)
	}
	// Big-endian decode (matches Java's DataInput.readBELong via CodecUtil).
	srcChecksum := int64(checksumBuf[0])<<56 | int64(checksumBuf[1])<<48 |
		int64(checksumBuf[2])<<40 | int64(checksumBuf[3])<<32 |
		int64(checksumBuf[4])<<24 | int64(checksumBuf[5])<<16 |
		int64(checksumBuf[6])<<8 | int64(checksumBuf[7])
	if srcChecksum != checksum {
		return fmt.Errorf("file %q: checksum claimed by primary (%d) disagrees with file footer (%d)",
			c.name, srcChecksum, checksum)
	}
	// Write the verified checksum to the output (big-endian, as Lucene's writeBELong).
	outBuf := [8]byte{
		byte(checksum >> 56), byte(checksum >> 48), byte(checksum >> 40), byte(checksum >> 32),
		byte(checksum >> 24), byte(checksum >> 16), byte(checksum >> 8), byte(checksum),
	}
	if err := out.WriteBytes(outBuf[:]); err != nil {
		return fmt.Errorf("CopyOneFile.Copy: writing checksum for %q: %w", c.name, err)
	}
	c.bytesCopied += 8

	return nil
}

// ---------------------------------------------------------------------------
// ReplicaFileDeleter
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.
// ---------------------------------------------------------------------------

// ReplicaFileDeleter manages reference counts for files held by the replica.
// Files whose reference count drops to zero are deleted from the directory.
//
// Thread safety: all public methods are protected by an internal mutex,
// matching Java's synchronized qualifier on every method.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.
type ReplicaFileDeleter struct {
	mu        sync.Mutex
	refCounts map[string]int
	dir       store.Directory // may be nil in test / stub contexts
	node      *Node
}

// NewReplicaFileDeleter constructs a ReplicaFileDeleter.
//
// dir may be nil when used in unit tests that do not require actual deletion.
// node may be nil; it is used only for verbose file-level log messages.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter(Node,Directory).
func NewReplicaFileDeleter(node interface{}, dir interface{}) *ReplicaFileDeleter {
	d := &ReplicaFileDeleter{
		refCounts: make(map[string]int),
	}
	if n, ok := node.(*Node); ok {
		d.node = n
	}
	if sd, ok := dir.(store.Directory); ok {
		d.dir = sd
	}
	return d
}

// IncRef increments the reference count for each of the given file names.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.incRef.
func (d *ReplicaFileDeleter) IncRef(fileNames []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, f := range fileNames {
		d.refCounts[f]++
	}
	return nil
}

// DecRef decrements the reference count for each file. When the count reaches
// zero the file is deleted from the directory (if dir is non-nil).
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.decRef.
func (d *ReplicaFileDeleter) DecRef(fileNames []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var firstErr error
	for _, f := range fileNames {
		if d.refCounts[f] <= 1 {
			delete(d.refCounts, f)
			if d.dir != nil {
				if d.node != nil && d.node.verboseFiles {
					d.node.message("deleting file " + f + " (refCount reached 0)")
				}
				if err := d.dir.DeleteFile(f); err != nil && firstErr == nil {
					firstErr = err
				}
			}
		} else {
			d.refCounts[f]--
		}
	}
	return firstErr
}

// GetRefCount returns the current reference count for a file.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.getRefCount.
func (d *ReplicaFileDeleter) GetRefCount(fileName string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.refCounts[fileName]
}

// DeleteIfNoRef deletes the file from the directory if its reference count is
// already zero. If the file is still referenced this is a no-op.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.deleteIfNoRef.
func (d *ReplicaFileDeleter) DeleteIfNoRef(fileName string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.refCounts[fileName] == 0 {
		delete(d.refCounts, fileName)
		if d.dir != nil {
			if d.node != nil && d.node.verboseFiles {
				d.node.message("deleteIfNoRef: deleting " + fileName)
			}
			return d.dir.DeleteFile(fileName)
		}
	}
	return nil
}

// ForceDeleteFile unconditionally deletes a file from the directory and
// removes it from the reference-count map regardless of its current count.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.forceDeleteFile.
func (d *ReplicaFileDeleter) ForceDeleteFile(fileName string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.refCounts, fileName)
	if d.dir != nil {
		return d.dir.DeleteFile(fileName)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Node — abstract base
//
// Port of org.apache.lucene.replicator.nrt.Node.
// ---------------------------------------------------------------------------

// Node is the abstract base for NRT primary and replica nodes.
//
// Port of org.apache.lucene.replicator.nrt.Node.
type Node struct {
	// ID is the compact ordinal for this node.
	ID int

	// verboseFiles controls whether file-level messages are emitted.
	verboseFiles bool

	// printStream is the log destination; nil silences all messages.
	printStream io.Writer

	// Dir is the underlying store.Directory for this node.
	// Typed as store.Directory so that ReadLocalFileMetaData can open files.
	Dir store.Directory

	// lastFileMetaData caches the FileMetaData map from the last successful sync.
	// Primary uses it for setCurrentInfos; replica uses it for fileIsIdentical.
	lastFileMetaData map[string]*FileMetaData

	// state tracks the lifecycle ("idle", "syncing", "closing", "closed", …).
	state string
}

const (
	// PrimaryGenKey is the commit user-data key for the primary generation.
	PrimaryGenKey = "__primaryGen"
	// VersionKey is the commit user-data key for the NRT version.
	VersionKey = "__version"
)

// NewNode constructs a Node.
//
// dir may be nil in test / stub contexts.
func NewNode(id int, dir interface{}, out io.Writer) *Node {
	n := &Node{
		ID:          id,
		printStream: out,
		state:       "idle",
	}
	if sd, ok := dir.(store.Directory); ok {
		n.Dir = sd
	}
	return n
}

// IsVerboseFiles reports whether file-level logging is enabled.
func (n *Node) IsVerboseFiles() bool { return n.verboseFiles }

// SetVerboseFiles controls file-level logging.
func (n *Node) SetVerboseFiles(v bool) { n.verboseFiles = v }

// Message emits a log line.
func (n *Node) Message(msg string) {
	if n.printStream != nil {
		fmt.Fprintln(n.printStream, msg)
	}
}

// message is the unexported alias used internally (mirrors Java's message()).
func (n *Node) message(msg string) { n.Message(msg) }

// Close is a no-op; subtype-specific close logic lives in PrimaryNode/ReplicaNode.
func (n *Node) Close() error { return nil }

// ReadLocalFileMetaData opens fileName in this node's directory, reads the
// full codec index header, the codec footer, and the CRC32 checksum, and
// returns a FileMetaData. Returns nil (with no error) when the file does not
// exist or is corrupt/truncated — callers treat nil as "must copy".
//
// The header bytes returned are the raw bytes from position 0 to
// indexHeaderLength (codec magic + codec name + version + 16-byte ID +
// suffix). The footer bytes are the last footerLength() == 16 bytes.
//
// Port of org.apache.lucene.replicator.nrt.Node.readLocalFileMetaData.
func (n *Node) ReadLocalFileMetaData(fileName string) (*FileMetaData, error) {
	// Fast path: check the last-sync cache.
	if n.lastFileMetaData != nil {
		if md, ok := n.lastFileMetaData[fileName]; ok {
			return md, nil
		}
	}

	if n.Dir == nil {
		return nil, nil
	}

	in, err := n.Dir.OpenInput(fileName, store.IOContextDefault)
	if err != nil {
		// File does not exist or cannot be opened — must copy.
		if n.verboseFiles {
			n.message("file " + fileName + ": will copy [file does not exist]")
		}
		return nil, nil //nolint:nilerr
	}
	defer in.Close()

	length := in.Length()

	// Read the full index header from position 0.
	// Java: CodecUtil.readIndexHeader(in) rewinds to 0, reads codec magic,
	// codec name, version, then reads (16 + 1 + suffixLen) more bytes, and
	// returns the whole slice from [0 .. headerLen+16+1+suffixLen).
	header, readErr := readIndexHeaderBytes(in)
	if readErr != nil {
		if n.verboseFiles {
			n.message("file " + fileName + ": will copy [existing file is corrupt reading header: " + readErr.Error() + "]")
		}
		return nil, nil //nolint:nilerr
	}

	// Read the footer bytes from the end of the file.
	footer, readErr := readFooterBytes(in)
	if readErr != nil {
		if n.verboseFiles {
			n.message("file " + fileName + ": will copy [existing file is corrupt reading footer: " + readErr.Error() + "]")
		}
		return nil, nil //nolint:nilerr
	}

	// Extract the CRC32 from the footer (last 8 bytes of the footer are the checksum long).
	checksum, readErr := codecs.RetrieveChecksum(in)
	if readErr != nil {
		if n.verboseFiles {
			n.message("file " + fileName + ": will copy [existing file is corrupt retrieving checksum: " + readErr.Error() + "]")
		}
		return nil, nil //nolint:nilerr
	}

	if n.verboseFiles {
		n.message(fmt.Sprintf("file %s has length=%d", fileName, length))
	}

	return &FileMetaData{
		Header:   header,
		Footer:   footer,
		Length:   length,
		Checksum: checksum,
	}, nil
}

// readIndexHeaderBytes replicates CodecUtil.readIndexHeader(IndexInput):
// seeks to 0, validates the codec magic, reads the codec name, version,
// 16-byte segment ID and suffix length byte, then seeks back to 0 and reads
// the whole header as a raw byte slice.
//
// Returns an error for truncated or corrupt files.
func readIndexHeaderBytes(in store.IndexInput) ([]byte, error) {
	footerLen := int64(codecs.FooterLength())
	if in.Length() < footerLen {
		return nil, fmt.Errorf("file too short (%d bytes) to contain a codec header+footer", in.Length())
	}

	if err := in.SetPosition(0); err != nil {
		return nil, err
	}

	// Read big-endian int32 magic (4 bytes).
	magicBuf := make([]byte, 4)
	if err := in.ReadBytes(magicBuf); err != nil {
		return nil, fmt.Errorf("cannot read magic: %w", err)
	}
	magic := int32(magicBuf[0])<<24 | int32(magicBuf[1])<<16 | int32(magicBuf[2])<<8 | int32(magicBuf[3])
	if magic != codecs.CODEC_MAGIC {
		return nil, fmt.Errorf("codec header mismatch: actual header=%d vs expected=%d", magic, codecs.CODEC_MAGIC)
	}

	// Read codec name (vInt length + bytes, same as Java's DataInput.readString).
	codecName, err := store.ReadString(in)
	if err != nil {
		return nil, fmt.Errorf("cannot read codec name: %w", err)
	}

	// Skip version int32 (4 bytes big-endian in Lucene's writeBEInt).
	if err := skipBytesIn(in, 4); err != nil {
		return nil, fmt.Errorf("cannot skip version: %w", err)
	}

	// Skip 16-byte segment ID.
	if err := skipBytesIn(in, 16); err != nil {
		return nil, fmt.Errorf("cannot skip segment ID: %w", err)
	}

	// Read the 1-byte suffix length.
	suffixLenBuf := make([]byte, 1)
	if err := in.ReadBytes(suffixLenBuf); err != nil {
		return nil, fmt.Errorf("cannot read suffix length: %w", err)
	}
	suffixLen := int(suffixLenBuf[0])

	// Total header size = headerLength(codecName) + 16 (ID) + 1 (suffix len) + suffixLen.
	// headerLength = 9 + len(codecName) where 9 = 4 (magic) + 4 (version) + 1 (vint for short names).
	// For codec names ≤ 127 chars the vInt is a single byte, so:
	//   headerLength(codec) = 4 + 1 + len(codec) + 4 = 9 + len(codec)
	headerSize := codecs.IndexHeaderLength(codecName, "") + suffixLen

	// Seek back to 0 and read the whole header.
	if err := in.SetPosition(0); err != nil {
		return nil, err
	}
	bytes := make([]byte, headerSize)
	if err := in.ReadBytes(bytes); err != nil {
		return nil, fmt.Errorf("cannot read full index header: %w", err)
	}
	return bytes, nil
}

// readFooterBytes replicates CodecUtil.readFooter(IndexInput): seeks to
// length−footerLength(), validates the footer magic, then seeks back and
// reads the raw 16-byte footer.
func readFooterBytes(in store.IndexInput) ([]byte, error) {
	footerLen := int64(codecs.FooterLength())
	if in.Length() < footerLen {
		return nil, fmt.Errorf("file too short (%d bytes) to contain a codec footer", in.Length())
	}
	footerOffset := in.Length() - footerLen
	if err := in.SetPosition(footerOffset); err != nil {
		return nil, err
	}

	// Validate footer magic (big-endian int32).
	magicBuf := make([]byte, 4)
	if err := in.ReadBytes(magicBuf); err != nil {
		return nil, fmt.Errorf("cannot read footer magic: %w", err)
	}
	footerMagic := int32(magicBuf[0])<<24 | int32(magicBuf[1])<<16 | int32(magicBuf[2])<<8 | int32(magicBuf[3])
	if footerMagic != codecs.FOOTER_MAGIC {
		return nil, fmt.Errorf("codec footer mismatch: actual=%d vs expected=%d", footerMagic, codecs.FOOTER_MAGIC)
	}

	// Seek back to footer start and read raw bytes.
	if err := in.SetPosition(footerOffset); err != nil {
		return nil, err
	}
	bytes := make([]byte, footerLen)
	if err := in.ReadBytes(bytes); err != nil {
		return nil, fmt.Errorf("cannot read full footer: %w", err)
	}
	return bytes, nil
}

// skipBytesIn advances in.GetFilePointer() by n bytes.
func skipBytesIn(in store.IndexInput, n int) error {
	buf := make([]byte, n)
	return in.ReadBytes(buf)
}

// ---------------------------------------------------------------------------
// PrimaryNode — stub
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.
// ---------------------------------------------------------------------------

// PrimaryNode is the primary NRT node that owns an IndexWriter and serves
// CopyState snapshots to replica nodes.
//
// Deviations: setCurrentInfos (which serialises SegmentInfos to bytes and
// builds the per-file FileMetaData map) and flushAndRefresh require an
// IndexWriter, which is not yet wired. GetCopyState, ReadLocalFileMetaData,
// PreCopyMergedSegmentFiles and NRTReplicaNewInfosVersion are present.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.
type PrimaryNode struct {
	Node
	mu sync.Mutex

	// PrimaryGen increments each time a new primary is elected.
	PrimaryGen int64

	// FinishedMergedFiles holds filenames of merges that have finished and been
	// copied to all replicas (Java: finishedMergedFiles).
	FinishedMergedFiles map[string]struct{}

	// copyState is the latest snapshot published to replicas.
	copyState *CopyState
}

// NewPrimaryNode constructs a PrimaryNode.
//
// dir may be nil in test contexts. In production use, dir must be the
// store.Directory containing the index.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode(IndexWriter,int,long,long,SearcherFactory,PrintStream).
func NewPrimaryNode(id int, primaryGen int64, dir interface{}, out io.Writer) *PrimaryNode {
	return &PrimaryNode{
		Node:                *NewNode(id, dir, out),
		PrimaryGen:          primaryGen,
		FinishedMergedFiles: make(map[string]struct{}),
	}
}

// GetCopyState returns the current CopyState snapshot, or nil if none has
// been established yet (before the first flushAndRefresh).
//
// In Java the caller must later call releaseCopyState to decrement the
// SegmentInfos refcount held by the primary. In Gocene that refcount mechanism
// is not yet implemented.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.getCopyState.
func (p *PrimaryNode) GetCopyState() *CopyState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.copyState
}

// ReadLocalFileMetaData reads identity metadata for a local file.
//
// Delegates to Node.ReadLocalFileMetaData, which checks the last-sync cache
// first, then opens the file, reads its index header and footer bytes, and
// computes the CRC32. Returns nil when the file is absent or corrupt.
//
// Port of org.apache.lucene.replicator.nrt.Node.readLocalFileMetaData (called
// by PrimaryNode.setCurrentInfos).
func (p *PrimaryNode) ReadLocalFileMetaData(fileName string) *FileMetaData {
	md, _ := p.Node.ReadLocalFileMetaData(fileName)
	return md
}

// PreCopyMergedSegmentFiles notifies replicas of a freshly merged segment so
// they can pre-warm it before the primary cuts over. Subclasses implement the
// actual replica-notification transport.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.preCopyMergedSegmentFiles.
func (p *PrimaryNode) PreCopyMergedSegmentFiles(_ interface{}, _ map[string]*FileMetaData) {}

// NRTReplicaNewInfosVersion is called by the replica when it opens a new NRT
// reader, giving the primary a chance to advance its own version tracking.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.nrtReplicaNewInfosVersion.
func (p *PrimaryNode) NRTReplicaNewInfosVersion(_ int64) {}

// Close closes the primary node. When an IndexWriter is wired, this will
// roll back the writer and close the directory.
func (p *PrimaryNode) Close() error { return nil }

// ---------------------------------------------------------------------------
// ReplicaNode — stub
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.
// ---------------------------------------------------------------------------

// ReplicaNode is an NRT replica node that receives SegmentInfos and file
// copies from the primary.
//
// NewNRTPoint implements the version-guard and primary-gen cut-over logic
// from Java's ReplicaNode.newNRTPoint, but does not yet launch a background
// CopyJob (that requires the network transport and IndexWriter stack).
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.
type ReplicaNode struct {
	Node
	mu sync.Mutex

	// lastPrimaryGen is the primary generation from the last successful sync.
	lastPrimaryGen int64

	// CurrentCopyState is the last CopyState received from the primary.
	CurrentCopyState *CopyState

	// state tracks the lifecycle of the replica ("idle", "syncing", …).
	// Shadowed here so that NewNRTPoint can update it independently of Node.state.
	replicaState string
}

// NewReplicaNode constructs a ReplicaNode.
//
// dir may be nil in test contexts.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode(int,Directory,SearcherFactory,PrintStream).
func NewReplicaNode(id int, dir interface{}, out io.Writer) *ReplicaNode {
	return &ReplicaNode{
		Node:         *NewNode(id, dir, out),
		replicaState: "idle",
	}
}

// NewNRTPoint notifies this replica of a new NRT point available on the
// primary. It implements the version-guard checks from Java's
// ReplicaNode.newNRTPoint:
//
//  1. If newPrimaryGen differs from the last known primary, cut over and
//     discard any pre-copied merged segment files (maybeNewPrimary).
//  2. Skip if version == current searching version (no-op) or version <
//     current version (stale notification, possible due to thread scheduling).
//  3. Otherwise record the pending copy state.
//
// The actual background file-copy (CopyJob launch) is deferred until the
// network transport and IndexWriter layers are wired.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.newNRTPoint.
func (r *ReplicaNode) NewNRTPoint(newPrimaryGen int64, version int64, infosBytes []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Handle primary change: discard stale pre-copied merged segment references.
	r.maybeNewPrimaryLocked(newPrimaryGen)

	curVersion := r.currentVersionLocked()

	if version == curVersion {
		// No-op: we already have this version.
		r.message(fmt.Sprintf("top: new NRT point has same version as current (%d); skipping", version))
		return nil
	}
	if version < curVersion {
		// Stale notification — two syncs raced and the older one arrived late.
		r.message(fmt.Sprintf("top: new NRT point (version=%d) is older than current (version=%d); skipping", version, curVersion))
		return nil
	}

	// Record the new copy state reference; the actual file copy is deferred.
	r.replicaState = "syncing"
	r.CurrentCopyState = &CopyState{
		Version:    version,
		PrimaryGen: newPrimaryGen,
		InfosBytes: infosBytes,
	}
	r.message(fmt.Sprintf("top: new NRT point version=%d primaryGen=%d", version, newPrimaryGen))

	return nil
}

// maybeNewPrimaryLocked handles a primary generation change.
// Must be called with r.mu held.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.maybeNewPrimary.
func (r *ReplicaNode) maybeNewPrimaryLocked(newPrimaryGen int64) {
	if newPrimaryGen == r.lastPrimaryGen {
		return
	}
	r.message(fmt.Sprintf("top: now change lastPrimaryGen from %d to %d", r.lastPrimaryGen, newPrimaryGen))
	r.lastPrimaryGen = newPrimaryGen
}

// currentVersionLocked returns the version currently being searched, or -1
// if no NRT point has been established.
// Must be called with r.mu held.
func (r *ReplicaNode) currentVersionLocked() int64 {
	if r.CurrentCopyState == nil {
		return -1
	}
	return r.CurrentCopyState.Version
}

// GetCurrentVersion returns the version of the current NRT point, or -1 if
// no point has been established yet.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.getCurrentSearchingVersion.
func (r *ReplicaNode) GetCurrentVersion() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentVersionLocked()
}

// Close closes the replica node.
func (r *ReplicaNode) Close() error { return nil }

// ---------------------------------------------------------------------------
// SegmentInfosSearcherManager — stub
//
// Port of org.apache.lucene.replicator.nrt.SegmentInfosSearcherManager.
// ---------------------------------------------------------------------------

// SegmentInfosSearcherManager opens and manages IndexSearchers built from
// replicated SegmentInfos rather than from an IndexWriter.
//
// In Java this extends ReferenceManager<IndexSearcher> and opens readers
// directly from SegmentInfos via StandardDirectoryReader.open(). The full
// implementation requires the index-layer DirectoryReader infrastructure;
// the current stub stores the SegmentInfos reference for future wiring.
//
// Port of org.apache.lucene.replicator.nrt.SegmentInfosSearcherManager.
type SegmentInfosSearcherManager struct {
	mu sync.Mutex

	// dir is the replica's directory; used when opening new readers.
	dir store.Directory

	// node is the owning ReplicaNode, used for logging.
	node *ReplicaNode

	// currentInfos holds the latest SegmentInfos snapshot.
	// Typed as interface{} until SegmentInfos is fully wired.
	currentInfos interface{}
}

// NewSegmentInfosSearcherManager constructs a SegmentInfosSearcherManager.
//
// dir may be nil in test contexts. infos is the initial SegmentInfos (may be nil).
//
// Port of org.apache.lucene.replicator.nrt.SegmentInfosSearcherManager(Directory,Node,SegmentInfos,SearcherFactory).
func NewSegmentInfosSearcherManager(dir interface{}, node *ReplicaNode) *SegmentInfosSearcherManager {
	m := &SegmentInfosSearcherManager{node: node}
	if sd, ok := dir.(store.Directory); ok {
		m.dir = sd
	}
	return m
}

// SetCurrentInfos installs a new SegmentInfos snapshot, which will be used
// on the next Refresh call.
//
// Port of org.apache.lucene.replicator.nrt.SegmentInfosSearcherManager.setCurrentInfos.
func (s *SegmentInfosSearcherManager) SetCurrentInfos(infos interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentInfos = infos
}

// GetCurrentInfos returns the current SegmentInfos snapshot.
//
// Port of org.apache.lucene.replicator.nrt.SegmentInfosSearcherManager.getCurrentInfos.
func (s *SegmentInfosSearcherManager) GetCurrentInfos() interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentInfos
}

// Close releases all held resources.
func (s *SegmentInfosSearcherManager) Close() error { return nil }

// ---------------------------------------------------------------------------
// PreCopyMergedSegmentWarmer — stub
//
// Port of org.apache.lucene.replicator.nrt.PreCopyMergedSegmentWarmer.
// ---------------------------------------------------------------------------

// PreCopyMergedSegmentWarmer pre-copies merged segments to all replicas
// before the primary switches to the merged view, keeping replica NRT
// latency proportional to flushed segment sizes rather than merged sizes.
//
// Port of org.apache.lucene.replicator.nrt.PreCopyMergedSegmentWarmer.
type PreCopyMergedSegmentWarmer struct {
	primary *PrimaryNode
}

// NewPreCopyMergedSegmentWarmer constructs a PreCopyMergedSegmentWarmer.
//
// Port of org.apache.lucene.replicator.nrt.PreCopyMergedSegmentWarmer(PrimaryNode).
func NewPreCopyMergedSegmentWarmer(primary *PrimaryNode) *PreCopyMergedSegmentWarmer {
	return &PreCopyMergedSegmentWarmer{primary: primary}
}

// Warm triggers pre-copy of the merged segment files to all replicas by
// calling primary.PreCopyMergedSegmentFiles with the segment's file metadata.
//
// The info argument is a *spi.SegmentCommitInfo; typed as interface{} until
// the index layer is fully wired.
//
// Port of org.apache.lucene.replicator.nrt.PreCopyMergedSegmentWarmer.warm.
func (w *PreCopyMergedSegmentWarmer) Warm(info interface{}) error {
	// In Java, warm() calls primary.preCopyMergedSegmentFiles(info, files)
	// where files is built from the SegmentCommitInfo's file set. Because
	// SegmentCommitInfo is not yet wired, we pass an empty map so that the
	// call signature is satisfied without panicking.
	//
	// When SegmentCommitInfo is available, replace nil with the actual map:
	//   files := buildFilesFromInfo(info)
	//   w.primary.PreCopyMergedSegmentFiles(info, files)
	w.primary.PreCopyMergedSegmentFiles(info, nil)
	return nil
}
