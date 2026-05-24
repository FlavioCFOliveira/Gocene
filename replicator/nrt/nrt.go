// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package nrt provides Near-Real-Time replication primitives for Lucene
// segment-level replication between a primary and one or more replica nodes.
//
// Port of org.apache.lucene.replicator.nrt.
//
// Deviation: All classes that depend on IndexWriter, DirectoryReader,
// SegmentInfos, ReferenceManager, or other index-layer infrastructure not
// yet ported are provided as stubs. Full implementations are deferred to
// backlog #2693.
package nrt

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
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
// The Header and Footer bytes must be identical between primary and replica
// for the files to be considered equal.
//
// Port of org.apache.lucene.replicator.nrt.FileMetaData.
type FileMetaData struct {
	// Header is the first bytes of the file (codec header).
	Header []byte
	// Footer is the last bytes of the file (codec footer).
	Footer []byte
	// Length is the total byte length of the file.
	Length int64
	// Checksum is the CRC32 checksum of the file content.
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
	// Represented as interface{} because SegmentInfos is deferred to #2693.
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
// Full implementation deferred to #2693.
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

// NewCopyJob constructs a CopyJob. Deferred to #2693.
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

// Cancel marks this job as cancelled.
//
// Port of org.apache.lucene.replicator.nrt.CopyJob.cancel.
func (j *CopyJob) Cancel(reason string, exc error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cancelReason = reason
	j.exc = exc
}

// GetFailed reports whether this job encountered an error.
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

// CopyOneFile copies a single file from a source reader to the replica directory.
//
// Full implementation deferred to #2693.
//
// Port of org.apache.lucene.replicator.nrt.CopyOneFile.
type CopyOneFile struct {
	name     string
	tmpName  string
	metaData *FileMetaData

	// bytesCopied tracks progress — deferred to #2693.
	bytesCopied int64
}

// NewCopyOneFile constructs a CopyOneFile.
//
// Port of org.apache.lucene.replicator.nrt.CopyOneFile(DataInput,ReplicaNode,String,FileMetaData,byte[]).
// Deferred to #2693.
func NewCopyOneFile(name, tmpName string, metaData *FileMetaData) *CopyOneFile {
	return &CopyOneFile{name: name, tmpName: tmpName, metaData: metaData}
}

// BytesCopied returns the number of bytes copied so far.
func (c *CopyOneFile) BytesCopied() int64 { return c.bytesCopied }

// FileName returns the target file name.
func (c *CopyOneFile) FileName() string { return c.name }

// TmpFileName returns the temporary file name used during copy.
func (c *CopyOneFile) TmpFileName() string { return c.tmpName }

// Close is a no-op stub.
func (c *CopyOneFile) Close() error { return nil }

// ---------------------------------------------------------------------------
// ReplicaFileDeleter
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.
// ---------------------------------------------------------------------------

// ReplicaFileDeleter manages reference counts for files held by the replica.
// Files with a zero reference count are deleted from the directory.
//
// Full implementation (FileDeleter integration) deferred to #2693.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.
type ReplicaFileDeleter struct {
	mu        sync.Mutex
	refCounts map[string]int
	// dir and node are deferred to #2693.
	dir  interface{} // store.Directory
	node interface{} // Node
}

// NewReplicaFileDeleter constructs a ReplicaFileDeleter.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter(Node,Directory).
func NewReplicaFileDeleter(node, dir interface{}) *ReplicaFileDeleter {
	return &ReplicaFileDeleter{
		refCounts: make(map[string]int),
		dir:       dir,
		node:      node,
	}
}

// IncRef increments the reference count for each of the given file names.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.incRef.
func (d *ReplicaFileDeleter) IncRef(fileNames []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, f := range fileNames {
		d.refCounts[f]++
	}
}

// DecRef decrements the reference count for each file; files reaching zero
// are removed from the map (actual deletion deferred to #2693).
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.decRef.
func (d *ReplicaFileDeleter) DecRef(fileNames []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, f := range fileNames {
		if d.refCounts[f] <= 1 {
			delete(d.refCounts, f)
		} else {
			d.refCounts[f]--
		}
	}
}

// GetRefCount returns the current reference count for a file.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.getRefCount.
func (d *ReplicaFileDeleter) GetRefCount(fileName string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.refCounts[fileName]
}

// DeleteIfNoRef removes the file from the ref map if unreferenced.
// Actual directory deletion deferred to #2693.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaFileDeleter.deleteIfNoRef.
func (d *ReplicaFileDeleter) DeleteIfNoRef(fileName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.refCounts[fileName] == 0 {
		delete(d.refCounts, fileName)
	}
}

// ---------------------------------------------------------------------------
// Node — abstract base
//
// Port of org.apache.lucene.replicator.nrt.Node.
// ---------------------------------------------------------------------------

// Node is the abstract base for NRT primary and replica nodes.
//
// Full implementation deferred to #2693.
//
// Port of org.apache.lucene.replicator.nrt.Node.
type Node struct {
	// ID is the compact ordinal for this node.
	ID int
	// VerboseFiles controls whether file-level messages are emitted.
	verboseFiles bool
	// PrintStream is the log output — deferred to #2693.
	printStream io.Writer

	// Dir is the underlying Directory — typed as interface{} until store.Directory
	// is fully ported (#2693).
	Dir interface{}

	// Constants propagated from Java.
	PrimaryGenKey string
	VersionKey    string
}

const (
	// PrimaryGenKey is the commit user-data key for the primary generation.
	PrimaryGenKey = "__primaryGen"
	// VersionKey is the commit user-data key for the NRT version.
	VersionKey = "__version"
)

// NewNode constructs a Node stub.
func NewNode(id int, dir interface{}, out io.Writer) *Node {
	return &Node{
		ID:          id,
		Dir:         dir,
		printStream: out,
	}
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

// Close is a no-op stub.
func (n *Node) Close() error { return nil }

// ---------------------------------------------------------------------------
// PrimaryNode — stub
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.
// ---------------------------------------------------------------------------

// PrimaryNode is the primary NRT node that owns an IndexWriter and serves
// CopyState snapshots to replica nodes.
//
// Full implementation deferred to #2693.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.
type PrimaryNode struct {
	Node
	mu sync.Mutex

	// PrimaryGen increments each time a new primary is elected.
	PrimaryGen int64

	// FinishedMergedFiles tracks filenames of completed merges.
	FinishedMergedFiles map[string]struct{}

	// copyState is the current snapshot — deferred to #2693.
	copyState *CopyState
}

// NewPrimaryNode constructs a PrimaryNode stub.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode(IndexWriter,int,long,long,SearcherFactory,PrintStream).
// Deferred to #2693.
func NewPrimaryNode(id int, primaryGen int64, dir interface{}, out io.Writer) *PrimaryNode {
	return &PrimaryNode{
		Node:                *NewNode(id, dir, out),
		PrimaryGen:          primaryGen,
		FinishedMergedFiles: make(map[string]struct{}),
	}
}

// GetCopyState returns the current CopyState snapshot.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.getCopyState.
// Deferred to #2693.
func (p *PrimaryNode) GetCopyState() *CopyState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.copyState
}

// ReadLocalFileMetaData reads metadata for a local file.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.readLocalFileMetaData.
// Deferred to #2693.
func (p *PrimaryNode) ReadLocalFileMetaData(_ string) *FileMetaData { return nil }

// PreCopyMergedSegmentFiles initiates pre-copy of merged segment files to replicas.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.preCopyMergedSegmentFiles.
// Deferred to #2693.
func (p *PrimaryNode) PreCopyMergedSegmentFiles(_ interface{}, _ map[string]*FileMetaData) {}

// NRTReplicaNewInfosVersion is called when a new NRT point is opened.
//
// Port of org.apache.lucene.replicator.nrt.PrimaryNode.nrtReplicaNewInfosVersion.
// Deferred to #2693.
func (p *PrimaryNode) NRTReplicaNewInfosVersion(_ int64) {}

// Close closes the primary node.
func (p *PrimaryNode) Close() error { return nil }

// ---------------------------------------------------------------------------
// ReplicaNode — stub
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.
// ---------------------------------------------------------------------------

// ReplicaNode is an NRT replica node that receives SegmentInfos and file
// copies from the primary.
//
// Full implementation deferred to #2693.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.
type ReplicaNode struct {
	Node
	mu sync.Mutex

	// PrimaryGen is the generation of the primary this replica is tracking.
	PrimaryGen int64

	// CurrentCopyState is the last CopyState received from the primary.
	CurrentCopyState *CopyState
}

// NewReplicaNode constructs a ReplicaNode stub.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode(int,Directory,SearcherFactory,PrintStream).
// Deferred to #2693.
func NewReplicaNode(id int, dir interface{}, out io.Writer) *ReplicaNode {
	return &ReplicaNode{
		Node: *NewNode(id, dir, out),
	}
}

// NewNRTPoint installs a new NRT point received from the primary.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.newNRTPoint.
// Deferred to #2693.
func (r *ReplicaNode) NewNRTPoint(_ int64, _ int64, _ []byte) error { return nil }

// GetCurrentVersion returns the version of the current NRT point.
//
// Port of org.apache.lucene.replicator.nrt.ReplicaNode.getCurrentVersion.
// Deferred to #2693.
func (r *ReplicaNode) GetCurrentVersion() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.CurrentCopyState == nil {
		return -1
	}
	return r.CurrentCopyState.Version
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
// Full implementation deferred to #2693.
//
// Port of org.apache.lucene.replicator.nrt.SegmentInfosSearcherManager.
type SegmentInfosSearcherManager struct {
	mu sync.Mutex
	// dir and node — deferred to #2693.
	dir  interface{}
	node *ReplicaNode
}

// NewSegmentInfosSearcherManager constructs a SegmentInfosSearcherManager.
//
// Port of org.apache.lucene.replicator.nrt.SegmentInfosSearcherManager(Directory,Node,SegmentInfos,SearcherFactory).
// Deferred to #2693.
func NewSegmentInfosSearcherManager(dir interface{}, node *ReplicaNode) *SegmentInfosSearcherManager {
	return &SegmentInfosSearcherManager{dir: dir, node: node}
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
// Full implementation deferred to #2693.
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

// Warm triggers pre-copy of the merged segment files to all replicas.
//
// Port of org.apache.lucene.replicator.nrt.PreCopyMergedSegmentWarmer.warm.
// Deferred to #2693.
func (w *PreCopyMergedSegmentWarmer) Warm(_ interface{}) error { return nil }
