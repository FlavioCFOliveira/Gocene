// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Port of org.apache.lucene.index.MultiSorter from Apache Lucene 10.4.0.
//
// Gocene deviations vs. the Java reference (option-(c) gap closures):
//
//   - ComparableProvider is defined locally as a function type rather than
//     an inner interface of IndexSorter, because IndexSorter currently
//     ships without the comparable-provider machinery. The provider's
//     signature mirrors getAsComparableLong(int)long; an error channel is
//     added for parity with Go's I/O-fallible NumericDocValues iteration.
//   - SortField.GetIndexSorter returns a degenerate sorter whose providers
//     yield int64(docID) (i.e. natural-order). This preserves the
//     branch-coverage of MultiSorter while a real per-type sorter
//     (NumericIndexSorter, StringIndexSorter, ...) is not yet ported.
//   - FieldInfos.GetParentField returns "" (no parent field configured).
//     The parent-block branch and the LUCENE_10 corruption check are
//     skipped when GetParentField is empty, matching Lucene's "field is
//     null" path.
//   - CodecReader.GetLeafMetaData returns nil here (no per-leaf metadata
//     pipeline yet). The parent-block / created-version branches are
//     skipped when leaf metadata is nil; this is observationally
//     equivalent to a segment that has no blocks and never opts into the
//     LUCENE_10 corruption check.
//   - PackedLongValues.monotonicBuilder(PackedInts.COMPACT) is replaced by
//     packed.DeltaPackedBuilder with the page size = 256 and the COMPACT
//     overhead ratio. The mapped doc-ID sequence per reader is
//     monotonically non-decreasing, so delta-packing is semantically
//     correct; the on-heap footprint may diverge from JVM-Lucene because
//     monotonic-block packing is not yet available.
//   - BitSet.of(NumericDocValues, maxDoc) is provided by a file-local
//     helper bitSetOfParentDocs that materialises a util.FixedBitSet by
//     iterating the NumericDocValues stream.
//
// These deviations are scoped to this port and do not alter the
// observable contract of the existing IndexSorter / SortField / FieldInfos
// surfaces beyond adding the listed methods.

// ComparableProvider returns, for a single document of a given reader,
// the int64 comparable that participates in the per-field merge order.
// Mirrors org.apache.lucene.index.IndexSorter.ComparableProvider.
type ComparableProvider func(docID int) (int64, error)

// testComparableProvidersHook is a test-only seam that lets unit tests
// inject deterministic per-reader providers without standing up the
// per-type IndexSorter (Numeric / Sorted / ...) ports. When nil,
// GetComparableProviders falls back to the degenerate identity.
var testComparableProvidersHook func(s *IndexSorter, readers []*CodecReader) []ComparableProvider

// GetComparableProviders returns one ComparableProvider per reader. The
// degenerate Gocene implementation maps a docID to itself; replace with a
// per-type IndexSorter (numeric / sorted / ...) once those subclasses are
// ported.
func (s *IndexSorter) GetComparableProviders(readers []*CodecReader) []ComparableProvider {
	if testComparableProvidersHook != nil {
		return testComparableProvidersHook(s, readers)
	}
	providers := make([]ComparableProvider, len(readers))
	for i := range readers {
		providers[i] = func(docID int) (int64, error) {
			return int64(docID), nil
		}
	}
	return providers
}

// GetIndexSorter returns an IndexSorter capable of sorting the documents
// of a sub-reader by this SortField. The current Gocene implementation
// returns a degenerate sorter (see GetComparableProviders above). A nil
// return value mirrors Lucene's "this field cannot be used for index
// sorting" path that MultiSorter rejects as IllegalArgumentException.
func (sf *SortField) GetIndexSorter() *IndexSorter {
	return NewIndexSorter(nil)
}

// GetReverse reports whether this SortField sorts descending. Mirrors
// org.apache.lucene.search.SortField.getReverse.
func (sf *SortField) GetReverse() bool {
	return sf.descending
}

// GetFields returns the underlying SortField slice. Mirrors
// org.apache.lucene.search.Sort.getSort. The slice is returned by value
// (one element per field) to match the existing in-package Sort layout.
func (s *Sort) GetFields() []SortField {
	return s.fields
}

// GetParentField returns the parent field name registered on these
// FieldInfos, or "" if none. Until parent-document blocks are wired into
// the indexing chain, this returns "" unconditionally. Mirrors
// org.apache.lucene.index.FieldInfos.getParentField.
func (fi *FieldInfos) GetParentField() string {
	return ""
}

// GetLeafMetaData returns the per-leaf metadata (created-version, sort,
// has-blocks). Until the leaf-metadata pipeline is wired through the
// codec readers, this returns nil; MultiSorter treats nil as "no blocks,
// no parent-field bookkeeping". Mirrors
// org.apache.lucene.index.LeafReader.getMetaData restricted to the
// LeafMetaData payload used by MultiSorter.
func (r *CodecReader) GetLeafMetaData() *LeafMetaData {
	return nil
}

// multiSorterSort does a merge sort of the leaves of the incoming
// readers, returning one DocMap per reader to map each leaf's documents
// into the merged segment. The documents in each incoming leaf must
// already be sorted by the same sort. Returns nil if the merge sort is
// not needed (segments are already in index sort order).
//
// Mirrors org.apache.lucene.index.MultiSorter#sort.
func multiSorterSort(sort *Sort, readers []*CodecReader) ([]DocMap, error) {
	// TODO: optimize if only 1 reader is incoming, though that's a rare case
	if sort == nil {
		return nil, fmt.Errorf("MultiSorter: sort is nil")
	}

	fields := sort.GetFields()
	comparables := make([][]ComparableProvider, len(fields))
	reverseMuls := make([]int, len(fields))

	for i := range fields {
		field := &fields[i]
		sorter := field.GetIndexSorter()
		if sorter == nil {
			return nil, fmt.Errorf("cannot use sort field %v for index sorting", field)
		}
		comparables[i] = sorter.GetComparableProviders(readers)
		for j, codecReader := range readers {
			fieldInfos := codecReader.GetFieldInfos()
			metaData := codecReader.GetLeafMetaData()
			parentField := ""
			if fieldInfos != nil {
				parentField = fieldInfos.GetParentField()
			}
			if metaData != nil && metaData.HasBlocks() && parentField != "" {
				parentDocs, err := codecReader.GetNumericDocValues(parentField)
				if err != nil {
					return nil, fmt.Errorf("MultiSorter: reading parent docs of %q: %w", parentField, err)
				}
				if parentDocs == nil {
					return nil, fmt.Errorf("MultiSorter: parent field %q must be present if index sorting is used with blocks", parentField)
				}
				parents, err := bitSetOfParentDocs(parentDocs, codecReader.MaxDoc())
				if err != nil {
					return nil, fmt.Errorf("MultiSorter: materialising parent bitset of %q: %w", parentField, err)
				}
				providers := comparables[i]
				inner := providers[j]
				providers[j] = func(docID int) (int64, error) {
					next := parents.NextSetBit(docID)
					return inner(next)
				}
			}
			if metaData != nil && metaData.HasBlocks() && parentField == "" &&
				metaData.CreatedVersionMajor() >= util.LuceneVersionMajor {
				return nil, NewCorruptIndexException(
					fmt.Sprintf("parent field is not set but the index has blocks and uses index sorting. indexCreatedVersionMajor: %d",
						metaData.CreatedVersionMajor()),
					"IndexingChain",
				)
			}
		}
		if field.GetReverse() {
			reverseMuls[i] = -1
		} else {
			reverseMuls[i] = 1
		}
	}

	leafCount := len(readers)

	// Snapshot per-leaf state. The priority queue mutates the value
	// fields in-place; using a pointer element keeps the queue's view
	// consistent with the iteration cursor.
	leaves := make([]*leafAndDocID, leafCount)
	for i := 0; i < leafCount; i++ {
		reader := readers[i]
		leaf := newLeafAndDocID(i, reader.GetLiveDocs(), reader.MaxDoc(), len(comparables))
		for j := 0; j < len(comparables); j++ {
			v, err := comparables[j][i](leaf.docID)
			if err != nil {
				return nil, fmt.Errorf("MultiSorter: seeding leaf %d field %d: %w", i, j, err)
			}
			leaf.valuesAsComparableLongs[j] = v
		}
		leaves[i] = leaf
	}

	queue, err := util.NewPriorityQueue[*leafAndDocID](leafCount, func(a, b *leafAndDocID) bool {
		for i := 0; i < len(comparables); i++ {
			av := a.valuesAsComparableLongs[i]
			bv := b.valuesAsComparableLongs[i]
			if av < bv {
				return reverseMuls[i]*-1 < 0
			}
			if av > bv {
				return reverseMuls[i]*1 < 0
			}
		}
		// tie-break by readerIndex, then docID
		if a.readerIndex != b.readerIndex {
			return a.readerIndex < b.readerIndex
		}
		return a.docID < b.docID
	})
	if err != nil {
		return nil, fmt.Errorf("MultiSorter: building priority queue: %w", err)
	}

	builders := make([]*packed.PackedLongValuesBuilder, leafCount)
	for i := 0; i < leafCount; i++ {
		queue.Add(leaves[i])
		b, err := packed.DeltaPackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
		if err != nil {
			return nil, fmt.Errorf("MultiSorter: builder %d: %w", i, err)
		}
		builders[i] = b
	}

	// Merge sort.
	mappedDocID := 0
	lastReaderIndex := 0
	isSorted := true
	for queue.Size() != 0 {
		top := queue.Top()
		if lastReaderIndex > top.readerIndex {
			isSorted = false
		}
		lastReaderIndex = top.readerIndex
		if err := builders[top.readerIndex].Add(int64(mappedDocID)); err != nil {
			return nil, fmt.Errorf("MultiSorter: appending mapped docID: %w", err)
		}
		if top.liveDocs == nil || top.liveDocs.Get(top.docID) {
			mappedDocID++
		}
		top.docID++
		if top.docID < top.maxDoc {
			for j := 0; j < len(comparables); j++ {
				v, err := comparables[j][top.readerIndex](top.docID)
				if err != nil {
					return nil, fmt.Errorf("MultiSorter: advancing leaf %d field %d: %w", top.readerIndex, j, err)
				}
				top.valuesAsComparableLongs[j] = v
			}
			queue.UpdateTop()
		} else {
			queue.Pop()
		}
	}
	if isSorted {
		return nil, nil
	}

	docMaps := make([]DocMap, leafCount)
	for i := 0; i < leafCount; i++ {
		remapped := builders[i].Build()
		liveDocs := readers[i].GetLiveDocs()
		docMaps[i] = &packedDocMap{remapped: remapped, liveDocs: liveDocs}
	}
	return docMaps, nil
}

// leafAndDocID tracks the merge cursor for one input reader. Mirrors
// MultiSorter.LeafAndDocID.
type leafAndDocID struct {
	readerIndex             int
	liveDocs                util.Bits
	maxDoc                  int
	valuesAsComparableLongs []int64
	docID                   int
}

func newLeafAndDocID(readerIndex int, liveDocs util.Bits, maxDoc, numComparables int) *leafAndDocID {
	return &leafAndDocID{
		readerIndex:             readerIndex,
		liveDocs:                liveDocs,
		maxDoc:                  maxDoc,
		valuesAsComparableLongs: make([]int64, numComparables),
	}
}

// packedDocMap is the per-reader DocMap returned by multiSorterSort. It
// answers Get(oldDocID) with the delta-packed mapped position, or -1 if
// the source document is not live.
type packedDocMap struct {
	remapped *packed.PackedLongValues
	liveDocs util.Bits
}

func (m *packedDocMap) Get(oldDocID int) int {
	if m.liveDocs != nil && !m.liveDocs.Get(oldDocID) {
		return -1
	}
	return int(m.remapped.Get(int64(oldDocID)))
}

// bitSetOfParentDocs materialises a util.FixedBitSet from a
// NumericDocValues stream. Mirrors org.apache.lucene.util.BitSet.of for
// the specific NumericDocValues / maxDoc overload that MultiSorter uses
// to mark parent docs.
func bitSetOfParentDocs(nd NumericDocValues, maxDoc int) (*util.FixedBitSet, error) {
	bs, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, fmt.Errorf("bitSetOfParentDocs: %w", err)
	}
	for {
		doc, err := nd.NextDoc()
		if err != nil {
			return nil, fmt.Errorf("bitSetOfParentDocs: NextDoc: %w", err)
		}
		if doc < 0 || doc >= maxDoc {
			break
		}
		bs.Set(doc)
	}
	return bs, nil
}
