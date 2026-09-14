// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// DocMap maps old doc IDs to new doc IDs during a merge. It is the Go port of
// org.apache.lucene.index.MergeState.DocMap from Apache Lucene 10.5.0, whose
// single member is `int get(int docID)`. A docID is mapped to the sentinel -1
// when the corresponding document was deleted in the source segment.
//
// Lucene declares it in org.apache.lucene.index, nested in MergeState. It is
// declared here because org.apache.lucene.util.bkd.BKDWriter.merge takes a
// List<MergeState.DocMap>, and Gocene's index package imports util/bkd, so
// util/bkd cannot import index. index.DocMap is an alias of this type, so the
// Lucene name stays available in the package Lucene declares it in.
type DocMap interface {
	// Get returns the new docID for the given old docID, or -1 if the
	// document was deleted.
	Get(oldDocID int) int
}
