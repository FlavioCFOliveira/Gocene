// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MutablePointTree is the codec-level mutable point tree used by BKD writers
// during partitioning and sorting. It is the Go port of
// org.apache.lucene.codecs.MutablePointTree from Apache Lucene 10.4.0
// (which itself is a nested interface inside PointValues used by the BKD
// writer when buffered points still live in RAM).
//
// Method contracts mirror the Java reference (Lucene 10.4.0):
//   - Swap(i, j):           exchange the entries at slots i and j
//   - GetValue(i, dst):     fill dst with the packed value at slot i
//   - GetByteAt(i, k):      return the k-th byte (as a Go byte) of slot i
//   - GetDocID(i):          return the docID associated with the point at slot i
//   - Save(i, j) / Restore(i, j): scratch-storage hooks used by
//     util.StableMSBRadixSorter
//
// Deviation: util/bkd contains an identically-shaped interface
// (bkd.MutablePointTree) because that package was developed before this
// codec-level alias landed and cannot import codecs without creating a
// cycle (bkd already depends on codecs.Relation). Both interfaces are
// structurally compatible: any concrete type that satisfies one satisfies
// the other. A future consolidation may pick one canonical definition;
// for now this codecs.MutablePointTree exists primarily to record the
// SPI surface that downstream Sprint 15 consumers reference, while
// bkd.MutablePointTree remains the implementation-side type.
type MutablePointTree interface {
	// Swap exchanges the entries at slots i and j.
	Swap(i, j int)

	// GetValue fills dst with the packed value at slot i. Implementations
	// are allowed to alias their underlying storage rather than copying.
	GetValue(i int, dst *util.BytesRef)

	// GetByteAt returns the k-th byte of the packed value at slot i.
	GetByteAt(i, k int) byte

	// GetDocID returns the docID associated with the point at slot i.
	GetDocID(i int) int

	// Save writes the value at slot i into the j-th scratch position.
	Save(i, j int)

	// Restore copies the scratch values back into slots [i, j) of the
	// primary storage.
	Restore(i, j int)
}
