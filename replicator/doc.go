// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package replicator is the umbrella for Gocene's index replication
// support, the Go port of Apache Lucene's replicator module
// (org.apache.lucene.replicator).
//
// Index replication keeps a read-only copy of an index on one or more
// replica nodes in step with a primary node that performs the writes.
// Lucene offers two complementary replication models, and Gocene mirrors
// that split across two locations:
//
//   - Revision-based, commit-level replication. A primary publishes a
//     Revision (a named, immutable snapshot of the committed files);
//     replicas pull the missing files and atomically swap to the new
//     revision. In Gocene this model is implemented by LocalReplicator
//     and IndexRevision in the index/ package, which copy the segment
//     files that back a commit from a source Directory to a target
//     Directory.
//
//   - Near-real-time (NRT) segment replication. Instead of waiting for a
//     commit, the primary copies newly flushed and merged segments to the
//     replicas as they are produced, so replicas can refresh to recently
//     indexed documents with very low latency. In Gocene this model lives
//     in the replicator/nrt subpackage, which ports
//     org.apache.lucene.replicator.nrt: PrimaryNode and ReplicaNode, the
//     CopyState / CopyJob / CopyOneFile file-transfer primitives, the
//     FileMetaData checksum/length records, the ReplicaFileDeleter
//     reference counter, and the SegmentInfosSearcherManager that exposes
//     a replica's current point-in-time view.
//
// # Binary compatibility
//
// Both models persist or exchange byte sequences that Apache Lucene
// 10.4.0 defines: the on-disk segment files that are copied between
// directories, and the wire-level framing of CopyState / FileMetaData
// used to advertise a node's files. Those encodings are covered by the
// serialization helpers in replicator/nrt (see wire.go) and remain
// subject to the project's bidirectional byte-for-byte compatibility
// mandate.
//
// # Status
//
// The top-level replicator package is currently a thin namespace and
// test scaffold; the working primitives live in replicator/nrt and in
// the index/ package as described above. Several NRT types that depend on
// not-yet-ported index-layer infrastructure (IndexWriter wiring,
// ReferenceManager) are provided as stubs in replicator/nrt, with full
// implementations tracked in the project backlog. New replication code
// that does not belong to a more specific subpackage should be added
// here.
package replicator
