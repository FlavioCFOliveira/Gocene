// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package nrt_test

// Tests for the NRT replication stubs.
//
// Deviation: The Java test peers (TestStressNRTReplication, TestNRTReplication,
// SimplePrimaryNode, SimpleReplicaNode, NodeProcess, Connection, Jobs,
// ThreadPumper, SimpleCopyJob, SimpleTransLog, TestSimpleServer) are large
// integration tests that depend on IndexWriter, network I/O, JVM process
// spawning and the full Lucene index stack. Those are deferred to backlog
// #2693. The tests here verify the self-contained types and their contracts.

import (
	"errors"
	"io"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/replicator/nrt"
)

// ---------------------------------------------------------------------------
// NodeCommunicationException
// ---------------------------------------------------------------------------

func TestNodeCommunicationException_Error(t *testing.T) {
	cause := errors.New("connection reset")
	err := nrt.NewNodeCommunicationException("send copy state", cause)
	if err.Error() == "" {
		t.Fatal("Error() must not be empty")
	}
}

func TestNodeCommunicationException_Unwrap(t *testing.T) {
	cause := errors.New("timeout")
	err := nrt.NewNodeCommunicationException("receive ack", cause)
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is must resolve to the wrapped cause")
	}
}

func TestNodeCommunicationException_NilCausePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("nil cause should panic")
		}
	}()
	nrt.NewNodeCommunicationException("whatever", nil)
}

// ---------------------------------------------------------------------------
// FileMetaData
// ---------------------------------------------------------------------------

func TestFileMetaData_String(t *testing.T) {
	f := &nrt.FileMetaData{Length: 1024, Checksum: 42}
	s := f.String()
	if s == "" {
		t.Fatal("String must not be empty")
	}
}

func TestFileMetaData_Fields(t *testing.T) {
	header := []byte{0, 1, 2, 3}
	footer := []byte{4, 5}
	f := &nrt.FileMetaData{Header: header, Footer: footer, Length: 100, Checksum: 0xDEAD}
	if f.Length != 100 {
		t.Fatalf("Length: want 100, got %d", f.Length)
	}
	if f.Checksum != 0xDEAD {
		t.Fatalf("Checksum: want 0xDEAD, got %x", f.Checksum)
	}
}

// ---------------------------------------------------------------------------
// CopyState
// ---------------------------------------------------------------------------

func TestCopyState_String(t *testing.T) {
	cs := &nrt.CopyState{Version: 7}
	if cs.String() == "" {
		t.Fatal("String must not be empty")
	}
}

func TestCopyState_Fields(t *testing.T) {
	files := map[string]*nrt.FileMetaData{
		"_0.si": {Length: 50, Checksum: 1},
	}
	cs := &nrt.CopyState{
		Files:      files,
		Version:    3,
		Gen:        2,
		InfosBytes: []byte{0xAB},
		PrimaryGen: 1,
	}
	if cs.Version != 3 {
		t.Fatalf("Version: want 3, got %d", cs.Version)
	}
	if _, ok := cs.Files["_0.si"]; !ok {
		t.Fatal("Files map must contain _0.si")
	}
}

// ---------------------------------------------------------------------------
// ReplicaFileDeleter
// ---------------------------------------------------------------------------

func TestReplicaFileDeleter_IncDecRef(t *testing.T) {
	d := nrt.NewReplicaFileDeleter(nil, nil)
	d.IncRef([]string{"a.txt", "b.txt"})
	if got := d.GetRefCount("a.txt"); got != 1 {
		t.Fatalf("refcount after IncRef: want 1, got %d", got)
	}
	d.IncRef([]string{"a.txt"})
	if got := d.GetRefCount("a.txt"); got != 2 {
		t.Fatalf("refcount after second IncRef: want 2, got %d", got)
	}
	d.DecRef([]string{"a.txt"})
	if got := d.GetRefCount("a.txt"); got != 1 {
		t.Fatalf("refcount after DecRef: want 1, got %d", got)
	}
	d.DecRef([]string{"a.txt"})
	if got := d.GetRefCount("a.txt"); got != 0 {
		t.Fatalf("refcount after final DecRef: want 0, got %d", got)
	}
}

func TestReplicaFileDeleter_DeleteIfNoRef(t *testing.T) {
	d := nrt.NewReplicaFileDeleter(nil, nil)
	d.IncRef([]string{"c.txt"})
	d.DeleteIfNoRef("c.txt") // should not delete — still referenced
	if got := d.GetRefCount("c.txt"); got != 1 {
		t.Fatalf("should still be referenced, got %d", got)
	}
	d.DecRef([]string{"c.txt"})
	d.DeleteIfNoRef("c.txt") // now unreferenced
	if got := d.GetRefCount("c.txt"); got != 0 {
		t.Fatalf("should be 0, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// CopyJob
// ---------------------------------------------------------------------------

func TestCopyJob_Ordinals(t *testing.T) {
	j1 := nrt.NewCopyJob("flush", nil, false, nil)
	j2 := nrt.NewCopyJob("merge", nil, true, nil)
	if j1.Ord >= j2.Ord {
		t.Fatalf("ordinals must be strictly increasing: j1=%d j2=%d", j1.Ord, j2.Ord)
	}
}

func TestCopyJob_Cancel(t *testing.T) {
	j := nrt.NewCopyJob("test", nil, false, nil)
	if j.GetFailed() {
		t.Fatal("fresh job must not be failed")
	}
	j.Cancel("shutdown", errors.New("node gone"))
	if !j.GetFailed() {
		t.Fatal("cancelled job must report failed")
	}
}

// ---------------------------------------------------------------------------
// CopyOneFile
// ---------------------------------------------------------------------------

func TestCopyOneFile_Fields(t *testing.T) {
	meta := &nrt.FileMetaData{Length: 256, Checksum: 7}
	c := nrt.NewCopyOneFile("_0.cfs", "_0.cfs.tmp", meta)
	if c.FileName() != "_0.cfs" {
		t.Fatalf("FileName: want _0.cfs, got %s", c.FileName())
	}
	if c.TmpFileName() != "_0.cfs.tmp" {
		t.Fatalf("TmpFileName: want _0.cfs.tmp, got %s", c.TmpFileName())
	}
	if c.BytesCopied() != 0 {
		t.Fatalf("BytesCopied initial: want 0, got %d", c.BytesCopied())
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Node
// ---------------------------------------------------------------------------

func TestNode_Message(t *testing.T) {
	node := nrt.NewNode(0, nil, io.Discard)
	node.Message("test message") // must not panic
}

func TestNode_VerboseFiles(t *testing.T) {
	node := nrt.NewNode(1, nil, io.Discard)
	node.SetVerboseFiles(true)
	if !node.IsVerboseFiles() {
		t.Fatal("IsVerboseFiles must reflect SetVerboseFiles(true)")
	}
	node.SetVerboseFiles(false)
	if node.IsVerboseFiles() {
		t.Fatal("IsVerboseFiles must reflect SetVerboseFiles(false)")
	}
}

func TestNodeConstants(t *testing.T) {
	if nrt.PrimaryGenKey != "__primaryGen" {
		t.Fatalf("PrimaryGenKey: want __primaryGen, got %s", nrt.PrimaryGenKey)
	}
	if nrt.VersionKey != "__version" {
		t.Fatalf("VersionKey: want __version, got %s", nrt.VersionKey)
	}
}

// ---------------------------------------------------------------------------
// PrimaryNode
// ---------------------------------------------------------------------------

func TestPrimaryNode_GetCopyState_InitiallyNil(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	if p.GetCopyState() != nil {
		t.Fatal("initial CopyState must be nil")
	}
}

func TestPrimaryNode_ReadLocalFileMetaData_ReturnsNilStub(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	if p.ReadLocalFileMetaData("_0.si") != nil {
		t.Fatal("stub must return nil")
	}
}

func TestPrimaryNode_Close(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReplicaNode
// ---------------------------------------------------------------------------

func TestReplicaNode_GetCurrentVersion_Initial(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)
	if got := r.GetCurrentVersion(); got != -1 {
		t.Fatalf("initial version: want -1, got %d", got)
	}
}

func TestReplicaNode_NewNRTPointStub(t *testing.T) {
	r := nrt.NewReplicaNode(1, nil, io.Discard)
	if err := r.NewNRTPoint(1, 1, nil); err != nil {
		t.Fatalf("NewNRTPoint stub: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SegmentInfosSearcherManager
// ---------------------------------------------------------------------------

func TestSegmentInfosSearcherManager_Close(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)
	m := nrt.NewSegmentInfosSearcherManager(nil, r)
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PreCopyMergedSegmentWarmer
// ---------------------------------------------------------------------------

func TestPreCopyMergedSegmentWarmer_WarmStub(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	w := nrt.NewPreCopyMergedSegmentWarmer(p)
	if err := w.Warm(nil); err != nil {
		t.Fatalf("Warm stub: %v", err)
	}
}
