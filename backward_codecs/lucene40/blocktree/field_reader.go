// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// ErrBlockTraversalNotAvailable is returned by TermsEnum methods that require
// full block-tree traversal logic (loadBlock / nextEntry), which is deferred
// to a later sprint.
var ErrBlockTraversalNotAvailable = errors.New(
	"blocktree: full block traversal not yet implemented",
)

// FieldReader implements index.Terms for a single field in the Lucene 4.0
// block-tree terms dictionary.
//
// Port of org.apache.lucene.backward_codecs.lucene40.blocktree.FieldReader
// (Lucene 10.4.0).
type FieldReader struct {
	index.TermsBase

	parent *Lucene40BlockTreeTermsReader

	fieldInfo        *index.FieldInfo
	numTerms         int64
	sumTotalTermFreq int64
	sumDocFreq       int64
	docCount         int
	rootBlockFP      int64
	rootCode         *util.BytesRef
	minTerm          *util.BytesRef
	maxTerm          *util.BytesRef

	// index is the FST terms index; nil for very old formats.
	fstIndex *fst.FST[*util.BytesRef]
}

// newFieldReader constructs a FieldReader and loads the FST index.
//
// Port of FieldReader(Lucene40BlockTreeTermsReader, FieldInfo, long,
// BytesRef, long, long, int, long, IndexInput, IndexInput, BytesRef, BytesRef).
func newFieldReader(
	parent *Lucene40BlockTreeTermsReader,
	fieldInfo *index.FieldInfo,
	numTerms int64,
	rootCode *util.BytesRef,
	sumTotalTermFreq, sumDocFreq int64,
	docCount int,
	indexStartFP int64,
	metaIn, indexIn store.IndexInput,
	minTerm, maxTerm *util.BytesRef,
) (*FieldReader, error) {
	br := store.NewByteArrayDataInput(rootCode.Bytes[rootCode.Offset : rootCode.Offset+rootCode.Length])
	rootVL, err := br.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("blocktree field reader: read rootBlockFP: %w", err)
	}

	fr := &FieldReader{
		parent:           parent,
		fieldInfo:        fieldInfo,
		numTerms:         numTerms,
		sumTotalTermFreq: sumTotalTermFreq,
		sumDocFreq:       sumDocFreq,
		docCount:         docCount,
		rootCode:         rootCode,
		minTerm:          minTerm,
		maxTerm:          maxTerm,
		rootBlockFP:      rootVL >> OutputFlagsNumBits,
	}

	// Load FST index.
	// When metaIn == indexIn (pre-Lucene 8.6 format), seek the index input
	// directly to indexStartFP and read metadata from a clone.
	var fstMetadata *fst.FSTMetadata[*util.BytesRef]
	outputs := fst.ByteSequenceOutputs()

	if metaIn == indexIn {
		// Old format: metadata is inline in the index file.
		cloneIn := indexIn.Clone()
		if cloneIn == nil {
			return nil, fmt.Errorf("blocktree field reader: cannot clone index input")
		}
		if err = cloneIn.SetPosition(indexStartFP); err != nil {
			_ = cloneIn.Close()
			return nil, fmt.Errorf("blocktree field reader: seek index clone: %w", err)
		}
		fstMetadata, err = fst.ReadMetadata(cloneIn, outputs)
		if err != nil {
			_ = cloneIn.Close()
			return nil, fmt.Errorf("blocktree field reader: read FST metadata (old format): %w", err)
		}
		// cloneIn is now positioned directly after the metadata; FST bytes follow.
		// Load FST bytes from the clone.
		fr.fstIndex, err = fst.NewFSTFromDataInput(fstMetadata, cloneIn)
		if err != nil {
			_ = cloneIn.Close()
			return nil, fmt.Errorf("blocktree field reader: load FST (old format): %w", err)
		}
		_ = cloneIn.Close()
	} else {
		// New format: metadata read from metaIn, bytes read from indexIn.
		fstMetadata, err = fst.ReadMetadata(metaIn, outputs)
		if err != nil {
			return nil, fmt.Errorf("blocktree field reader: read FST metadata: %w", err)
		}
		// Try off-heap loading via RandomAccessInput; fall back to on-heap.
		if rai, ok := indexIn.(store.RandomAccessInput); ok {
			offHeap, err2 := fst.NewOffHeapFSTStore(rai, indexStartFP, fstMetadata.NumBytes())
			if err2 == nil {
				fr.fstIndex, err = fst.FromFSTReader(fstMetadata, offHeap)
				if err != nil {
					return nil, fmt.Errorf("blocktree field reader: load off-heap FST: %w", err)
				}
				return fr, nil
			}
		}
		// Fall back to on-heap: seek a clone and load.
		cloneIn := indexIn.Clone()
		if cloneIn == nil {
			return nil, fmt.Errorf("blocktree field reader: cannot clone index input for FST")
		}
		if err = cloneIn.SetPosition(indexStartFP); err != nil {
			_ = cloneIn.Close()
			return nil, fmt.Errorf("blocktree field reader: seek FST bytes: %w", err)
		}
		fr.fstIndex, err = fst.NewFSTFromDataInput(fstMetadata, cloneIn)
		if err != nil {
			_ = cloneIn.Close()
			return nil, fmt.Errorf("blocktree field reader: load FST: %w", err)
		}
		_ = cloneIn.Close()
	}

	return fr, nil
}

// GetMin returns the smallest term in this field, or nil if unknown.
func (fr *FieldReader) GetMin() (*index.Term, error) {
	if fr.minTerm == nil {
		// Older index without stored min term; fall back to full scan.
		return nil, nil
	}
	return index.NewTermFromBytesRef(fr.fieldInfo.Name(), fr.minTerm), nil
}

// GetMax returns the largest term in this field, or nil if unknown.
func (fr *FieldReader) GetMax() (*index.Term, error) {
	if fr.maxTerm == nil {
		// Older index without stored max term; fall back to full scan.
		return nil, nil
	}
	return index.NewTermFromBytesRef(fr.fieldInfo.Name(), fr.maxTerm), nil
}

// GetStats returns block-tree statistics by running a full block scan.
//
// Deferred: requires full block traversal logic.
func (fr *FieldReader) GetStats() (*Stats, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// HasFreqs reports whether term frequencies are indexed for this field.
func (fr *FieldReader) HasFreqs() bool {
	return fr.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
}

// HasOffsets reports whether term offsets are indexed for this field.
func (fr *FieldReader) HasOffsets() bool {
	return fr.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

// HasPositions reports whether term positions are indexed for this field.
func (fr *FieldReader) HasPositions() bool {
	return fr.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
}

// HasPayloads reports whether payloads are indexed for this field.
func (fr *FieldReader) HasPayloads() bool {
	return fr.fieldInfo.HasPayloads()
}

// Size returns the number of unique terms in this field.
func (fr *FieldReader) Size() int64 {
	return fr.numTerms
}

// GetDocCount returns the number of documents that have at least one term in
// this field.
func (fr *FieldReader) GetDocCount() (int, error) {
	return fr.docCount, nil
}

// GetSumDocFreq returns the sum of docFreq across all terms.
func (fr *FieldReader) GetSumDocFreq() (int64, error) {
	return fr.sumDocFreq, nil
}

// GetSumTotalTermFreq returns the sum of totalTermFreq across all terms.
func (fr *FieldReader) GetSumTotalTermFreq() (int64, error) {
	return fr.sumTotalTermFreq, nil
}

// GetIterator returns a TermsEnum over all terms in this field.
func (fr *FieldReader) GetIterator() (index.TermsEnum, error) {
	return newSegmentTermsEnum(fr)
}

// GetIteratorWithSeek returns a TermsEnum positioned at or after seekTerm.
func (fr *FieldReader) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		return nil, err
	}
	if seekTerm != nil {
		if _, err = e.SeekCeil(seekTerm); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// GetPostingsReader returns a PostingsEnum for the given term, or nil if not found.
func (fr *FieldReader) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	te, err := fr.GetIterator()
	if err != nil {
		return nil, err
	}
	ok, err := te.SeekExact(index.NewTerm(fr.fieldInfo.Name(), termText))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return te.Postings(flags)
}

// Intersect returns a TermsEnum filtered by the given compiled automaton.
//
// Port of FieldReader.intersect(CompiledAutomaton, BytesRef).
func (fr *FieldReader) Intersect(
	compiled *automaton.CompiledAutomaton,
	startTerm *util.BytesRef,
) (index.TermsEnum, error) {
	return newIntersectTermsEnum(fr, compiled, startTerm)
}

// String returns a debug representation of this FieldReader.
func (fr *FieldReader) String() string {
	return fmt.Sprintf(
		"BlockTreeTerms(seg=%s terms=%d postings=%d positions=%d docs=%d)",
		fr.parent.segment,
		fr.numTerms,
		fr.sumDocFreq,
		fr.sumTotalTermFreq,
		fr.docCount,
	)
}

// compile-time assertion
var _ index.Terms = (*FieldReader)(nil)
