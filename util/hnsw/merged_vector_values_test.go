package hnsw

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// mergedKnnVectorValues combines multiple KnnVectorValues sources with
// docID remapping via DocMaps. It simulates the merge-time vector values
// view that KnnVectorsWriter would produce during segment merge.
type mergedKnnVectorValues struct {
	dim     int
	sources []KnnVectorValues
	docMaps []DocMap
}

func (m *mergedKnnVectorValues) Dimension() int { return m.dim }
func (m *mergedKnnVectorValues) Size() int {
	total := 0
	for _, s := range m.sources {
		total += s.Size()
	}
	return total
}
func (m *mergedKnnVectorValues) OrdToDoc(ord int) int { return ord }
func (m *mergedKnnVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return acceptDocs
}
func (m *mergedKnnVectorValues) Iterator() DocIndexIterator {
	return &mergedDocIndexIterator{
		sources: m.sources,
		docMaps: m.docMaps,
	}
}

type mergedDocIndexIterator struct {
	sources []KnnVectorValues
	docMaps []DocMap
	srcIdx  int
	sub     DocIndexIterator
	base    int
	idx     int
}

func (it *mergedDocIndexIterator) NextDoc() (int, error) {
	for {
		if it.sub == nil {
			if it.srcIdx >= len(it.sources) {
				return util.NO_MORE_DOCS, nil
			}
			it.sub = it.sources[it.srcIdx].Iterator()
		}
		docID, err := it.sub.NextDoc()
		if err != nil {
			return 0, err
		}
		if docID == util.NO_MORE_DOCS {
			it.base += it.sources[it.srcIdx].Size()
			it.srcIdx++
			it.sub = nil
			continue
		}
		// Apply docMap if present
		if it.srcIdx < len(it.docMaps) && it.docMaps[it.srcIdx] != nil {
			mapped := it.docMaps[it.srcIdx].Get(docID)
			if mapped < 0 {
				continue // deleted doc, skip
			}
			docID = mapped
		}
		it.idx = it.base + it.sub.Index()
		return docID, nil
	}
}

func (it *mergedDocIndexIterator) Index() int { return it.idx }

func TestMergedKnnVectorValues_SingleSource(t *testing.T) {
	src := &stubKnnVectorValues{dim: 4, n: 10}
	merged := &mergedKnnVectorValues{
		dim:     4,
		sources: []KnnVectorValues{src},
	}
	if merged.Size() != 10 {
		t.Fatalf("Size=%d, want 10", merged.Size())
	}
}

func TestMergedKnnVectorValues_MultipleSources(t *testing.T) {
	merged := &mergedKnnVectorValues{
		dim: 4,
		sources: []KnnVectorValues{
			&stubKnnVectorValues{dim: 4, n: 5},
			&stubKnnVectorValues{dim: 4, n: 7},
			&stubKnnVectorValues{dim: 4, n: 3},
		},
	}
	if merged.Size() != 15 {
		t.Fatalf("Size=%d, want 15", merged.Size())
	}
}

func TestMergedKnnVectorValues_IterateAcrossSources(t *testing.T) {
	merged := &mergedKnnVectorValues{
		dim: 4,
		sources: []KnnVectorValues{
			&stubKnnVectorValues{dim: 4, n: 3},
			&stubKnnVectorValues{dim: 4, n: 2},
		},
	}
	iter := merged.Iterator()
	count := 0
	var docs []int
	for {
		docID, err := iter.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if docID == util.NO_MORE_DOCS {
			break
		}
		count++
		docs = append(docs, docID)
	}
	if count != 5 {
		t.Fatalf("iterated %d docs, want 5: %v", count, docs)
	}
}

func TestMergedKnnVectorValues_WithDeletedDocs(t *testing.T) {
	src := &stubKnnVectorValues{dim: 4, n: 4}
	dm := &stubDocMap{mapping: []int{0, -1, 1, -1}} // docs 1,3 deleted
	merged := &mergedKnnVectorValues{
		dim:     4,
		sources: []KnnVectorValues{src},
		docMaps: []DocMap{dm},
	}
	iter := merged.Iterator()
	// Should see docs [0, 1] only
	docID, _ := iter.NextDoc()
	if docID != 0 {
		t.Fatalf("doc=%d, want 0", docID)
	}
	docID, _ = iter.NextDoc()
	if docID != 1 {
		t.Fatalf("doc=%d, want 1", docID)
	}
	docID, _ = iter.NextDoc()
	if docID != util.NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", docID)
	}
}

func TestMergedKnnVectorValues_IterateIndex(t *testing.T) {
	merged := &mergedKnnVectorValues{
		dim: 4,
		sources: []KnnVectorValues{
			&stubKnnVectorValues{dim: 4, n: 2},
			&stubKnnVectorValues{dim: 4, n: 2},
		},
	}
	iter := merged.Iterator()
	// First source: docs 0,1 -> indices 0,1
	iter.NextDoc()
	if iter.Index() != 0 {
		t.Fatalf("Index()=%d after doc 0, want 0", iter.Index())
	}
	iter.NextDoc()
	if iter.Index() != 1 {
		t.Fatalf("Index()=%d after doc 1, want 1", iter.Index())
	}
	// Second source: docs 0,1 -> indices 2,3
	iter.NextDoc()
	if iter.Index() != 2 {
		t.Fatalf("Index()=%d after first doc of src2, want 2", iter.Index())
	}
}
