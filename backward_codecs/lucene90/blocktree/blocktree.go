// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package blocktree implements org.apache.lucene.backward_codecs.lucene90.blocktree.
package blocktree

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// Block-tree output-encoding constants.
// These match the Java static final fields on Lucene90BlockTreeTermsReader.
const (
	// outputFlagsNumBits is the number of flag bits packed into the low
	// bits of each FST output vlong. Port of OUTPUT_FLAGS_NUM_BITS = 2.
	outputFlagsNumBits = 2

	// outputFlagIsFloor marks a block as a floor block.
	// Port of OUTPUT_FLAG_IS_FLOOR = 0x1.
	outputFlagIsFloor = 0x1

	// outputFlagHasTerms marks a block as containing real terms.
	// Port of OUTPUT_FLAG_HAS_TERMS = 0x2.
	outputFlagHasTerms = 0x2

	// versionMSBVLongOutput is the first format version that uses MSB-encoded
	// VLong outputs. Port of VERSION_MSB_VLONG_OUTPUT = 1.
	versionMSBVLongOutput = 1
)

// CompressionAlgorithm mirrors org.apache.lucene.backward_codecs.lucene90.blocktree.CompressionAlgorithm.
//
// For the stub (Name/Version) form the zero-value is used; for the real form
// the Code field is set via ByCode.
type CompressionAlgorithm struct {
	Name    string
	Version string
}

// NewCompressionAlgorithm builds a CompressionAlgorithm with the supplied version.
func NewCompressionAlgorithm(version string) *CompressionAlgorithm {
	return &CompressionAlgorithm{Name: "CompressionAlgorithm", Version: version}
}

// compressionAlg is the typed enum used inside the real frame implementation.
// It mirrors the Java enum, mapped onto the codecs package values.
type compressionAlg = codecs.CompressionAlgorithm

// noCompression is a convenience alias used in frame initialisation.
const noCompression = codecs.CompressionNoCompression

// compressionAlgByCode delegates to the codecs package lookup.
func compressionAlgByCode(code int) (compressionAlg, error) {
	alg, err := codecs.CompressionAlgorithmByCode(code)
	if err != nil {
		return 0, fmt.Errorf("lucene90 blocktree: %w", err)
	}
	return alg, nil
}

// Lucene90BlockTreeTermsReader is the parent reader that holds segment-level
// resources shared across all fields: the terms-file IndexInput (termsIn) and
// the PostingsReaderBase. It mirrors the package-private class of the same name.
//
// The stub constructor (NewLucene90BlockTreeTermsReader) is preserved for the
// codec registry. The real constructor is NewLucene90BlockTreeTermsReaderFull.
type Lucene90BlockTreeTermsReader struct {
	// Name/Version are kept for the stub registry path.
	Name    string
	Version string

	// termsIn is the clone-source for per-enum IndexInputs (.tim file).
	termsIn store.IndexInput

	// postingsReader decodes per-term metadata and creates PostingsEnums.
	postingsReader codecs.PostingsReaderBase

	// version is the format version read from the codec header.
	version int

	// segment is the segment name, used for error messages.
	segment string
}

// NewLucene90BlockTreeTermsReader builds a stub Lucene90BlockTreeTermsReader
// for the codec registry.
func NewLucene90BlockTreeTermsReader(version string) *Lucene90BlockTreeTermsReader {
	return &Lucene90BlockTreeTermsReader{Name: "Lucene90BlockTreeTermsReader", Version: version}
}

// NewLucene90BlockTreeTermsReaderFull constructs a fully-wired reader.
// segment is the segment name; version must be the version read from the
// codec header.
func NewLucene90BlockTreeTermsReaderFull(
	termsIn store.IndexInput,
	postingsReader codecs.PostingsReaderBase,
	version int,
	segment string,
) *Lucene90BlockTreeTermsReader {
	return &Lucene90BlockTreeTermsReader{
		Name:           "Lucene90BlockTreeTermsReader",
		Version:        fmt.Sprintf("%d", version),
		termsIn:        termsIn,
		postingsReader: postingsReader,
		version:        version,
		segment:        segment,
	}
}

// byteReader is a minimal interface that provides byte-by-byte reading.
// Both store.DataInput and outputAccumulator satisfy it.
type byteReader interface {
	ReadByte() (byte, error)
}

// readVLongOutput reads a VLong output from in using either the MSB-VLong
// encoding (version >= versionMSBVLongOutput) or the standard VLong.
// Mirrors FieldReader.readVLongOutput(DataInput).
func (r *Lucene90BlockTreeTermsReader) readVLongOutput(in byteReader) (int64, error) {
	if r.version >= versionMSBVLongOutput {
		return readMSBVLong(in)
	}
	return readVLongFromBytes(in)
}

// readMSBVLong decodes a variable-length long encoded in MSB order.
// Port of FieldReader.readMSBVLong(DataInput).
func readMSBVLong(in byteReader) (int64, error) {
	var l int64
	for {
		b, err := in.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("readMSBVLong: %w", err)
		}
		l = (l << 7) | int64(b&0x7f)
		if b&0x80 == 0 {
			break
		}
	}
	return l, nil
}

// readVLongFromBytes reads a standard VLong (LSB-first) from a byteReader.
// This is used when the format version is < versionMSBVLongOutput.
func readVLongFromBytes(in byteReader) (int64, error) {
	b, err := in.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("readVLongFromBytes: %w", err)
	}
	result := int64(b & 0x7f)
	if b&0x80 == 0 {
		return result, nil
	}
	var shift uint = 7
	for {
		b, err = in.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("readVLongFromBytes: %w", err)
		}
		result |= int64(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("readVLongFromBytes: corrupted VLong")
		}
	}
	return result, nil
}

// FieldReader is the per-field read entry point for the Lucene 9.0 block-tree
// terms dictionary. It mirrors org.apache.lucene.backward_codecs.lucene90.blocktree.FieldReader.
//
// When constructed via NewFieldReader (stub path, codec registry) the index
// field is nil and all block traversal is deferred. When constructed via
// NewFieldReaderFull it holds a live FST index and a wired parent reader, and
// block traversal works correctly.
type FieldReader struct {
	// Name / Version are kept for the stub registry path.
	Name    string
	Version string

	// parent is the owning segment-level reader.
	parent *Lucene90BlockTreeTermsReader

	// fieldInfo is the schema descriptor for the field.
	fieldInfo *index.FieldInfo

	// numTerms is the total number of terms in this field.
	numTerms int64

	// rootCode is the BytesRef encoding of the root block pointer plus flags.
	rootCode *util.BytesRef

	// sumTotalTermFreq, sumDocFreq, docCount are field-level statistics.
	sumTotalTermFreq int64
	sumDocFreq       int64
	docCount         int

	// minTerm and maxTerm are the lexicographic bounds of the field.
	minTerm *util.BytesRef
	maxTerm *util.BytesRef

	// index is the FST (prefix trie) for seeking. Nil for a stub FieldReader.
	index *fst.FST[*util.BytesRef]
}

// NewFieldReader builds a stub FieldReader with the supplied version.
// This path is used by the codec registry; block traversal is not available.
func NewFieldReader(version string) *FieldReader {
	return &FieldReader{Name: "FieldReader", Version: version}
}

// NewFieldReaderFull constructs a fully-wired FieldReader.
// Callers must supply a non-nil parent reader and a non-nil FST index.
// The rootCode BytesRef is copied.
func NewFieldReaderFull(
	parent *Lucene90BlockTreeTermsReader,
	fieldInfo *index.FieldInfo,
	numTerms int64,
	rootCode *util.BytesRef,
	sumTotalTermFreq, sumDocFreq int64,
	docCount int,
	idx *fst.FST[*util.BytesRef],
	minTerm, maxTerm *util.BytesRef,
) (*FieldReader, error) {
	if parent == nil {
		return nil, fmt.Errorf("lucene90 blocktree: FieldReader parent must not be nil")
	}
	if fieldInfo == nil {
		return nil, fmt.Errorf("lucene90 blocktree: FieldReader fieldInfo must not be nil")
	}
	if numTerms <= 0 {
		return nil, fmt.Errorf("lucene90 blocktree: FieldReader numTerms must be > 0, got %d", numTerms)
	}
	if rootCode == nil {
		return nil, fmt.Errorf("lucene90 blocktree: FieldReader rootCode must not be nil")
	}
	if idx == nil {
		return nil, fmt.Errorf("lucene90 blocktree: FieldReader FST index must not be nil")
	}
	rc := rootCode.Clone()
	return &FieldReader{
		Name:             "FieldReader",
		parent:           parent,
		fieldInfo:        fieldInfo,
		numTerms:         numTerms,
		rootCode:         rc,
		sumTotalTermFreq: sumTotalTermFreq,
		sumDocFreq:       sumDocFreq,
		docCount:         docCount,
		index:            idx,
		minTerm:          minTerm,
		maxTerm:          maxTerm,
	}, nil
}

// Stats mirrors org.apache.lucene.backward_codecs.lucene90.blocktree.Stats.
type Stats struct{ Name, Version string }

// NewStats builds a Stats with the supplied version.
func NewStats(version string) *Stats { return &Stats{Name: "Stats", Version: version} }
