// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.VersionFieldReader.
package idversion

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// VersionFieldReader is the Terms implementation for a single field in
// VersionBlockTreeTermsReader.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.VersionFieldReader
// (package-private in Java).
type VersionFieldReader struct {
	// NumTerms is the number of distinct terms in this field.
	NumTerms int64
	// FieldInfo describes the field.
	FieldInfo *index.FieldInfo
	// SumTotalTermFreq is the sum of all term frequencies.
	SumTotalTermFreq int64
	// SumDocFreq is the sum of document frequencies.
	SumDocFreq int64
	// DocCount is the number of documents that have at least one term.
	DocCount int
	// IndexStartFP is the file pointer of the FST index in the index file.
	IndexStartFP int64
	// RootBlockFP is the file pointer of the root block in the terms file.
	RootBlockFP int64
	// RootCode is the root FST output (blockFP | outputFlags, maxVersion).
	RootCode *fst.Pair[*util.BytesRef, int64]
	// MinTerm is the minimum term in this field (may be nil for older indexes).
	MinTerm *util.BytesRef
	// MaxTerm is the maximum term in this field (may be nil for older indexes).
	MaxTerm *util.BytesRef
	// Parent is the owning VersionBlockTreeTermsReader.
	Parent *VersionBlockTreeTermsReader

	// Index is the FST index for this field.
	Index *fst.FST[*fst.Pair[*util.BytesRef, int64]]
}

// NewVersionFieldReader constructs a VersionFieldReader, loading the FST index
// from indexIn if non-nil.
func NewVersionFieldReader(
	parent *VersionBlockTreeTermsReader,
	fieldInfo *index.FieldInfo,
	numTerms int64,
	rootCode *fst.Pair[*util.BytesRef, int64],
	sumTotalTermFreq, sumDocFreq int64,
	docCount int,
	indexStartFP int64,
	indexIn store.IndexInput,
	minTerm, maxTerm *util.BytesRef,
) (*VersionFieldReader, error) {
	if numTerms <= 0 {
		return nil, fmt.Errorf("VersionFieldReader: numTerms must be > 0 (got %d)", numTerms)
	}

	// Decode rootBlockFP from rootCode.output1 (the BytesRef sub-output).
	badi := store.NewByteArrayDataInput(rootCode.Output1.Bytes[rootCode.Output1.Offset : rootCode.Output1.Offset+rootCode.Output1.Length])
	rawFP, err := badi.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("VersionFieldReader: read rootBlockFP: %w", err)
	}
	rootBlockFP := rawFP >> versionBlockTreeOutputFlagsNumBits

	fr := &VersionFieldReader{
		NumTerms:         numTerms,
		FieldInfo:        fieldInfo,
		SumTotalTermFreq: sumTotalTermFreq,
		SumDocFreq:       sumDocFreq,
		DocCount:         docCount,
		IndexStartFP:     indexStartFP,
		RootBlockFP:      rootBlockFP,
		RootCode:         rootCode,
		MinTerm:          minTerm,
		MaxTerm:          maxTerm,
		Parent:           parent,
	}

	if indexIn != nil {
		clone := indexIn.Clone()
		if err := clone.SetPosition(indexStartFP); err != nil {
			return nil, fmt.Errorf("VersionFieldReader: seek to indexStartFP: %w", err)
		}
		meta, err := fst.ReadMetadata(clone, versionBlockTreeFSTOutputs())
		if err != nil {
			return nil, fmt.Errorf("VersionFieldReader: read FST metadata: %w", err)
		}
		var fstErr error
		fr.Index, fstErr = fst.NewFSTFromDataInput(meta, clone)
		if fstErr != nil {
			return nil, fmt.Errorf("VersionFieldReader: load FST: %w", fstErr)
		}
	}

	return fr, nil
}

// GetMin returns the minimum term for this field.
//
// Implements schema.Terms. Returns (*schema.Term, error).
func (f *VersionFieldReader) GetMin() (*schema.Term, error) {
	if f.MinTerm == nil {
		return nil, nil // Caller falls back to scanning.
	}
	return schema.NewTermFromBytesRef(f.FieldInfo.Name(), f.MinTerm), nil
}

// GetMax returns the maximum term for this field.
//
// Implements schema.Terms. Returns (*schema.Term, error).
func (f *VersionFieldReader) GetMax() (*schema.Term, error) {
	if f.MaxTerm == nil {
		return nil, nil
	}
	return schema.NewTermFromBytesRef(f.FieldInfo.Name(), f.MaxTerm), nil
}

// HasFreqs reports whether this field has term frequencies.
func (f *VersionFieldReader) HasFreqs() bool {
	return f.FieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
}

// HasOffsets reports whether this field has term offsets.
func (f *VersionFieldReader) HasOffsets() bool {
	return f.FieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

// HasPositions reports whether this field has term positions.
func (f *VersionFieldReader) HasPositions() bool {
	return f.FieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
}

// HasPayloads reports whether this field has payloads.
func (f *VersionFieldReader) HasPayloads() bool {
	return f.FieldInfo.HasPayloads()
}

// Iterator returns a TermsEnum over this field.
func (f *VersionFieldReader) Iterator() (*IDVersionSegmentTermsEnum, error) {
	return newIDVersionSegmentTermsEnum(f)
}

// Size returns the number of terms.
func (f *VersionFieldReader) Size() int64 { return f.NumTerms }

// GetSumTotalTermFreq returns the sum of all term frequencies.
//
// Implements schema.Terms.
func (f *VersionFieldReader) GetSumTotalTermFreq() (int64, error) {
	return f.SumTotalTermFreq, nil
}

// GetSumDocFreq returns the sum of document frequencies.
//
// Implements schema.Terms.
func (f *VersionFieldReader) GetSumDocFreq() (int64, error) {
	return f.SumDocFreq, nil
}

// GetDocCount returns the number of documents with at least one term.
//
// Implements schema.Terms.
func (f *VersionFieldReader) GetDocCount() (int, error) { return f.DocCount, nil }

// GetIterator returns a TermsEnum positioned before the first term.
//
// Implements schema.Terms.
func (f *VersionFieldReader) GetIterator() (schema.TermsEnum, error) {
	return f.Iterator()
}

// GetIteratorWithSeek returns a TermsEnum positioned at or after seekTerm.
//
// Implements schema.Terms.
func (f *VersionFieldReader) GetIteratorWithSeek(seekTerm *schema.Term) (schema.TermsEnum, error) {
	te, err := f.Iterator()
	if err != nil {
		return nil, err
	}
	if seekTerm != nil {
		if _, serr := te.SeekCeil(seekTerm); serr != nil {
			return nil, serr
		}
	}
	return te, nil
}

// GetPostingsReader returns a PostingsEnum for the given term text.
//
// Implements schema.Terms.
func (f *VersionFieldReader) GetPostingsReader(termText string, flags int) (schema.PostingsEnum, error) {
	te, err := f.Iterator()
	if err != nil {
		return nil, err
	}
	t := schema.NewTerm(f.FieldInfo.Name(), termText)
	found, err := te.SeekExact(t)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return te.Postings(flags)
}

// String returns a human-readable summary.
func (f *VersionFieldReader) String() string {
	return fmt.Sprintf("IDVersionTerms(terms=%d,postings=%d,positions=%d,docs=%d)",
		f.NumTerms, f.SumDocFreq, f.SumTotalTermFreq, f.DocCount)
}

// versionBlockTreeOutputFlagsNumBits is the number of flag bits stored in the
// output1 root-code long. Mirrors VersionBlockTreeTermsWriter.OUTPUT_FLAGS_NUM_BITS.
const versionBlockTreeOutputFlagsNumBits = 2

// versionBlockTreeFSTOutputs returns the PairOutputs used by VersionBlockTree
// for its FST index. The outputs pair is (BytesRef, Long).
// This mirrors VersionBlockTreeTermsWriter.FST_OUTPUTS.
func versionBlockTreeFSTOutputs() fst.Outputs[*fst.Pair[*util.BytesRef, int64]] {
	return fst.NewPairOutputs(
		fst.ByteSequenceOutputs(),
		fst.PositiveIntOutputs(),
	)
}
