// Package index implements org.apache.lucene.misc.index.
package index

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BinaryDocValueSelector picks a primary value from a multi-valued
// BinaryDocValues field. Mirrors
// org.apache.lucene.misc.index.BinaryDocValueSelector.
type BinaryDocValueSelector interface {
	Select(values [][]byte) []byte
}

// FirstBinaryDocValueSelector picks the first value.
type FirstBinaryDocValueSelector struct{}

// Select returns the first value or nil.
func (FirstBinaryDocValueSelector) Select(values [][]byte) []byte {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

// BPIndexReorderer is the bipartite-partitioning index reorderer used to
// improve compression after merge. Mirrors
// org.apache.lucene.misc.index.BPIndexReorderer.
type BPIndexReorderer struct {
	MaxIter int
}

// NewBPIndexReorderer builds the reorderer.
func NewBPIndexReorderer(maxIter int) *BPIndexReorderer {
	if maxIter < 1 {
		maxIter = 20
	}
	return &BPIndexReorderer{MaxIter: maxIter}
}

// BPReorderingMergePolicy applies BP reordering after merges. Mirrors
// org.apache.lucene.misc.index.BPReorderingMergePolicy.
type BPReorderingMergePolicy struct {
	Reorderer *BPIndexReorderer
}

// NewBPReorderingMergePolicy builds the policy.
func NewBPReorderingMergePolicy(reorderer *BPIndexReorderer) *BPReorderingMergePolicy {
	return &BPReorderingMergePolicy{Reorderer: reorderer}
}

// IndexRearranger reorders documents in an index based on a per-doc
// comparator. Mirrors org.apache.lucene.misc.index.IndexRearranger.
type IndexRearranger struct {
	Compare func(a, b int) int
}

// NewIndexRearranger builds the rearranger.
func NewIndexRearranger(compare func(a, b int) int) *IndexRearranger {
	return &IndexRearranger{Compare: compare}
}

// IndexSplitter splits an index into N segments based on a doc-id partition
// function. Mirrors org.apache.lucene.misc.index.IndexSplitter.
type IndexSplitter struct {
	Partition func(docID int) int
}

// NewIndexSplitter builds the splitter.
func NewIndexSplitter(partition func(docID int) int) *IndexSplitter {
	return &IndexSplitter{Partition: partition}
}

// MultiPassIndexSplitter is the multi-pass variant. Mirrors
// org.apache.lucene.misc.index.MultiPassIndexSplitter.
type MultiPassIndexSplitter struct {
	NumPasses int
}

// NewMultiPassIndexSplitter builds the splitter.
func NewMultiPassIndexSplitter(numPasses int) *MultiPassIndexSplitter {
	if numPasses < 1 {
		numPasses = 1
	}
	return &MultiPassIndexSplitter{NumPasses: numPasses}
}

// PKIndexSplitter splits an index by primary-key range using a Query that
// matches the "keep" subset. Mirrors
// org.apache.lucene.misc.index.PKIndexSplitter.
type PKIndexSplitter struct {
	KeepQuery search.Query
	Field     string
}

// NewPKIndexSplitter builds the splitter.
func NewPKIndexSplitter(field string, keep search.Query) *PKIndexSplitter {
	return &PKIndexSplitter{Field: field, KeepQuery: keep}
}

// IndexHelper exposes the (index.IndexReader)-typed callbacks the splitters
// rely on; kept here so callers don't need to depend on the index package
// directly when building higher-level orchestration.
type IndexHelper interface {
	NumDocs() int
	Term(docID int, field string) (index.Term, error)
}
