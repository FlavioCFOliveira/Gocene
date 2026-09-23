// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DefaultTargetNumBlockLines is the default value for the target block size
// (number of terms per block).
//
// Mirrors UniformSplitTermsWriter.DEFAULT_TARGET_NUM_BLOCK_LINES
// (UniformSplitTermsWriter.java:102).
const DefaultTargetNumBlockLines = 32

// DefaultDeltaNumLines is the default value for the maximum allowed delta
// variation of the block size (delta of the number of terms per block). The
// block size will be [target block size]+-[allowed delta].
//
// Mirrors UniformSplitTermsWriter.DEFAULT_DELTA_NUM_LINES
// (UniformSplitTermsWriter.java:108), whose Java initialiser is
// `(int) (DEFAULT_TARGET_NUM_BLOCK_LINES * 0.1)`. Go rejects a truncating
// constant conversion — int(3.2) does not compile — so the truncation is
// spelled out on the value it produces: 32 * 0.1 = 3.2 -> 3.
const DefaultDeltaNumLines = 3

// MaxNumBlockLines is the upper limit of the block size (maximum number of
// terms per block).
//
// Mirrors the protected static UniformSplitTermsWriter.MAX_NUM_BLOCK_LINES
// (UniformSplitTermsWriter.java:111). Go has no protected access, so it is
// exported: Java's protected statics are reachable from subclasses in other
// packages.
const MaxNumBlockLines = 1_000

// UniformSplitTermsWriter is a block-based terms index and dictionary that
// assigns terms to nearly uniform length blocks. This technique is called
// Uniform Split.
//
// The block construction is driven by two parameters, targetNumBlockLines and
// deltaNumLines. Each block size (number of terms) is targetNumBlockLines +-
// deltaNumLines. The algorithm computes the minimal distinguishing prefix (MDP)
// between each term and its previous term (alphabetically ordered). Then it
// selects in the neighborhood of the targetNumBlockLines, and within the
// deltaNumLines, the term with the minimal MDP. This term becomes the first
// term of the next block and its MDP is the block key. This block key is added
// to the terms dictionary trie.
//
// We call dictionary the trie structure in memory, and block file the disk file
// containing the block lines, with one term and its corresponding term state
// details per line.
//
// When seeking a term, the dictionary seeks the floor leaf of the trie for the
// searched term and jumps to the corresponding file pointer in the block file.
// There, the block terms are scanned for the exact searched term.
//
// The terms inside a block do not need to share a prefix. Only the block key is
// used to find the block from the dictionary trie. And the block key is
// selected because it is the locally smallest MDP. This makes the dictionary
// trie very compact.
//
// An interesting property of the Uniform Split technique is the very linear
// balance between memory usage and lookup performance. By decreasing the target
// block size, the block scan becomes faster, and since there are more blocks,
// the dictionary trie memory usage increases. Additionally, small blocks are
// faster to read from disk. A good sweet spot for the target block size is 32
// with delta of 3 (10%) (default values). This can be tuned in the constructor.
//
// There are additional optimizations:
//
//   - Each block has a header that allows the lookup to jump directly to the
//     middle term with a fast comparison. This reduces the linear scan by 2 for
//     a small disk size increase.
//   - Each block term is incrementally encoded according to its previous term.
//     This both reduces the disk size and speeds up the block scan.
//   - All term line details (the terms states) are written after all terms.
//     This allows faster term scan without needing to decode the term states.
//   - All file pointers are base-encoded. Their value is relative to the block
//     base file pointer (not to the previous file pointer), this allows to read
//     the term state of any term independently.
//
// Blocks can be compressed or encrypted with an optional BlockEncoder provided
// in the constructor.
//
// The TermsBlocksExtension block file contains all the term blocks for each
// field sequentially. It also contains the fields metadata at the end of the
// file.
//
// The TermsDictionaryExtension dictionary file contains the trie (fst.FST
// bytes) for each field sequentially.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.UniformSplitTermsWriter from
// Apache Lucene 10.5.0, which extends FieldsConsumer.
//
// The fields below render the protected fields of the Java class
// (UniformSplitTermsWriter.java:113-123). Go has no protected access, so they
// are exported: Java's protected fields are reachable from a subclass in
// another package, which is what
// codecs/uniformsplit/sharedterms.STUniformSplitTermsWriter is.
type UniformSplitTermsWriter struct {
	// FieldsConsumerBase carries the concrete FieldsConsumer.merge that Java
	// inherits from the superclass (FieldsConsumer.java:72).
	*codecs.FieldsConsumerBase

	FieldInfos     *index.FieldInfos
	PostingsWriter codecs.PostingsWriterBase
	MaxDoc         int

	TargetNumBlockLines int
	DeltaNumLines       int

	BlockEncoder        BlockEncoder
	FieldMetadataWriter *FieldMetadataSerializer
	BlockOutput         store.IndexOutput
	DictionaryOutput    store.IndexOutput
}

// NewUniformSplitTermsWriter mirrors the public
// UniformSplitTermsWriter(PostingsWriterBase, SegmentWriteState, BlockEncoder)
// constructor (UniformSplitTermsWriter.java:129).
//
// blockEncoder is an optional block encoder, may be nil if none. It can be used
// for compression or encryption.
func NewUniformSplitTermsWriter(
	postingsWriter codecs.PostingsWriterBase,
	state *index.SegmentWriteState,
	blockEncoder BlockEncoder,
) (*UniformSplitTermsWriter, error) {
	return NewUniformSplitTermsWriterWithBlockSizes(
		postingsWriter,
		state,
		DefaultTargetNumBlockLines,
		DefaultDeltaNumLines,
		blockEncoder)
}

// NewUniformSplitTermsWriterWithBlockSizes mirrors the public
// UniformSplitTermsWriter(PostingsWriterBase, SegmentWriteState, int, int,
// BlockEncoder) constructor (UniformSplitTermsWriter.java:144). Go has no
// overloading, so the constructors are distinguished by name.
//
// blockEncoder is an optional block encoder, may be nil if none. It can be used
// for compression or encryption.
func NewUniformSplitTermsWriterWithBlockSizes(
	postingsWriter codecs.PostingsWriterBase,
	state *index.SegmentWriteState,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder BlockEncoder,
) (*UniformSplitTermsWriter, error) {
	return NewUniformSplitTermsWriterWithCodec(
		postingsWriter,
		state,
		targetNumBlockLines,
		deltaNumLines,
		blockEncoder,
		FieldMetadataSerializerInstance,
		Name,
		VersionCurrent,
		TermsBlocksExtension,
		TermsDictionaryExtension)
}

// NewUniformSplitTermsWriterWithCodec mirrors the protected
// UniformSplitTermsWriter constructor that takes the codec identity
// (UniformSplitTermsWriter.java:176).
//
// targetNumBlockLines is the target number of lines per block. It must be
// strictly greater than 0. The parameters can be pre-validated with
// ValidateSettings. There is one term per block line, with its corresponding
// details (index.TermState).
//
// deltaNumLines is the maximum allowed delta variation of the number of lines
// per block. It must be greater than or equal to 0 and strictly less than
// targetNumBlockLines. The block size will be targetNumBlockLines +-
// deltaNumLines. The block size must always be less than or equal to
// MaxNumBlockLines.
//
// blockEncoder is an optional block encoder, may be nil if none. It can be used
// for compression or encryption.
func NewUniformSplitTermsWriterWithCodec(
	postingsWriter codecs.PostingsWriterBase,
	state *index.SegmentWriteState,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder BlockEncoder,
	fieldMetadataWriter *FieldMetadataSerializer,
	codecName string,
	versionCurrent int32,
	termsBlocksExtension string,
	dictionaryExtension string,
) (*UniformSplitTermsWriter, error) {
	if err := ValidateSettings(targetNumBlockLines, deltaNumLines); err != nil {
		return nil, err
	}
	var blockOutput store.IndexOutput
	var dictionaryOutput store.IndexOutput
	success := false
	defer func() {
		if !success {
			util.CloseAllWhileHandlingException(blockOutput, dictionaryOutput)
		}
	}()

	w := &UniformSplitTermsWriter{
		FieldInfos:          state.FieldInfos,
		PostingsWriter:      postingsWriter,
		MaxDoc:              state.SegmentInfo.MaxDoc(),
		TargetNumBlockLines: targetNumBlockLines,
		DeltaNumLines:       deltaNumLines,
		BlockEncoder:        blockEncoder,
		FieldMetadataWriter: fieldMetadataWriter,
	}
	// Java's `this` inside FieldsConsumer.merge is the UniformSplitTermsWriter
	// (or its subclass) being constructed; see codecs.FieldsConsumerBase.
	w.FieldsConsumerBase = codecs.NewFieldsConsumerBase(w)

	termsName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, termsBlocksExtension)
	blockOutput, err := state.Directory.CreateOutput(termsName, state.Context)
	if err != nil {
		return nil, err
	}
	if err := codecs.WriteIndexHeader(
		blockOutput, codecName, versionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, err
	}

	indexName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, dictionaryExtension)
	dictionaryOutput, err = state.Directory.CreateOutput(indexName, state.Context)
	if err != nil {
		return nil, err
	}
	if err := codecs.WriteIndexHeader(
		dictionaryOutput, codecName, versionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, err
	}

	if err := postingsWriter.Init(blockOutput, state); err != nil {
		return nil, err
	}

	w.BlockOutput = blockOutput
	w.DictionaryOutput = dictionaryOutput
	success = true
	return w, nil
}

// ValidateSettings validates the constructor settings.
//
// targetNumBlockLines is the target number of lines per block; it must be
// strictly greater than 0. deltaNumLines is the maximum allowed delta variation
// of the number of lines per block; it must be greater than or equal to 0 and
// strictly less than targetNumBlockLines. Additionally, targetNumBlockLines +
// deltaNumLines must be less than or equal to MaxNumBlockLines.
//
// Mirrors the protected static UniformSplitTermsWriter.validateSettings(int,
// int) (UniformSplitTermsWriter.java:241), which throws
// IllegalArgumentException; Gocene reports it as an error.
func ValidateSettings(targetNumBlockLines, deltaNumLines int) error {
	if targetNumBlockLines <= 0 {
		return fmt.Errorf("invalid negative or nul targetNumBlockLines=%d", targetNumBlockLines)
	}
	if deltaNumLines < 0 {
		return fmt.Errorf("invalid negative deltaNumLines=%d", deltaNumLines)
	}
	if deltaNumLines >= targetNumBlockLines {
		return fmt.Errorf("invalid too large deltaNumLines=%d, it must be < targetNumBlockLines=%d",
			deltaNumLines, targetNumBlockLines)
	}
	if targetNumBlockLines+deltaNumLines > MaxNumBlockLines {
		return fmt.Errorf("invalid (targetNumBlockLines + deltaNumLines)=%d, it must be <= MAX_NUM_BLOCK_LINES=%d",
			targetNumBlockLines+deltaNumLines, MaxNumBlockLines)
	}
	return nil
}

// Write mirrors UniformSplitTermsWriter.write(Fields, NormsProducer)
// (UniformSplitTermsWriter.java:265). Java's `for (String field : fields)`
// walks Fields.iterator(); Gocene's spi.Fields exposes the same walk through a
// FieldIterator.
func (w *UniformSplitTermsWriter) Write(fields spi.Fields, normsProducer codecs.NormsProducer) error {
	blockWriter := NewBlockWriter(w.BlockOutput, w.TargetNumBlockLines, w.DeltaNumLines, w.BlockEncoder)
	fieldsOutput := store.NewByteBuffersDataOutput()
	fieldsNumber := int32(0)
	it, err := fields.Iterator()
	if err != nil {
		return err
	}
	for {
		field, err := it.Next()
		if err != nil {
			return err
		}
		if field == "" {
			break
		}
		terms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if terms != nil {
			termsEnum, err := terms.Iterator()
			if err != nil {
				return err
			}
			fieldInfo := w.FieldInfos.FieldInfo(field)
			written, err := w.WriteFieldTerms(blockWriter, fieldsOutput, termsEnum, fieldInfo, normsProducer)
			if err != nil {
				return err
			}
			fieldsNumber += written
		}
	}
	if err := w.WriteFieldsMetadata(fieldsNumber, fieldsOutput); err != nil {
		return err
	}
	return codecs.WriteFooter(w.DictionaryOutput)
}

// writeFieldsMetadata mirrors UniformSplitTermsWriter.writeFieldsMetadata
// (UniformSplitTermsWriter.java:284).
func (w *UniformSplitTermsWriter) WriteFieldsMetadata(fieldsNumber int32, fieldsOutput *store.ByteBuffersDataOutput) error {
	fieldsStartPosition := w.BlockOutput.GetFilePointer()
	if err := w.BlockOutput.WriteVInt(fieldsNumber); err != nil {
		return err
	}
	if w.BlockEncoder == nil {
		if err := w.WriteUnencodedFieldsMetadata(fieldsOutput); err != nil {
			return err
		}
	} else {
		if err := w.WriteEncodedFieldsMetadata(fieldsOutput); err != nil {
			return err
		}
	}
	// Must be a fixed length. Read by UniformSplitTermsReader when seeking
	// fields metadata.
	if err := w.BlockOutput.WriteLong(fieldsStartPosition); err != nil {
		return err
	}
	return codecs.WriteFooter(w.BlockOutput)
}

// writeUnencodedFieldsMetadata mirrors
// UniformSplitTermsWriter.writeUnencodedFieldsMetadata
// (UniformSplitTermsWriter.java:298).
func (w *UniformSplitTermsWriter) WriteUnencodedFieldsMetadata(fieldsOutput *store.ByteBuffersDataOutput) error {
	return fieldsOutput.CopyTo(w.BlockOutput)
}

// writeEncodedFieldsMetadata mirrors
// UniformSplitTermsWriter.writeEncodedFieldsMetadata
// (UniformSplitTermsWriter.java:303).
func (w *UniformSplitTermsWriter) WriteEncodedFieldsMetadata(fieldsOutput *store.ByteBuffersDataOutput) error {
	encodedBytes, err := w.BlockEncoder.Encode(fieldsOutput.ToDataInput(), fieldsOutput.Size())
	if err != nil {
		return err
	}
	if err := w.BlockOutput.WriteVLong(encodedBytes.Size()); err != nil {
		return err
	}
	return encodedBytes.WriteTo(w.BlockOutput)
}

// writeFieldTerms returns 1 if the field was written; 0 otherwise.
//
// Mirrors UniformSplitTermsWriter.writeFieldTerms
// (UniformSplitTermsWriter.java:313).
func (w *UniformSplitTermsWriter) WriteFieldTerms(
	blockWriter *BlockWriter,
	fieldsOutput store.DataOutput,
	termsEnum spi.TermsEnum,
	fieldInfo *index.FieldInfo,
	normsProducer codecs.NormsProducer,
) (int32, error) {
	fieldMetadata, err := NewFieldMetadata(fieldInfo, w.MaxDoc)
	if err != nil {
		return 0, err
	}
	fieldMetadata.SetDictionaryStartFP(w.DictionaryOutput.GetFilePointer())

	enumFlags, err := w.PostingsWriter.SetField(fieldInfo)
	if err != nil {
		return 0, err
	}
	blockWriter.SetField(fieldMetadata)
	dictionaryBuilder, err := NewFSTDictionaryBuilder()
	if err != nil {
		return 0, err
	}
	var lastTerm *util.BytesRef
	for {
		term, err := termsEnum.Next()
		if err != nil {
			return 0, err
		}
		if term == nil {
			break
		}
		blockTermState, err := w.WritePostingLine(termsEnum, fieldMetadata, normsProducer, enumFlags)
		if err != nil {
			return 0, err
		}
		if blockTermState != nil {
			lastTerm = util.BytesRefDeepCopyOf(termsEnum.Term().BytesValue())
			if err := blockWriter.AddLine(lastTerm, blockTermState, dictionaryBuilder); err != nil {
				return 0, err
			}
		}
	}

	// Flush remaining terms.
	if err := blockWriter.FinishLastBlock(dictionaryBuilder); err != nil {
		return 0, err
	}

	if fieldMetadata.GetNumTerms() > 0 {
		fieldMetadata.SetLastTerm(lastTerm)
		if err := w.FieldMetadataWriter.Write(fieldsOutput, fieldMetadata); err != nil {
			return 0, err
		}
		if err := w.WriteDictionary(dictionaryBuilder); err != nil {
			return 0, err
		}
		return 1, nil
	}
	return 0, nil
}

// writePostingLine writes the posting values for the current term in the given
// TermsEnum and updates the FieldMetadata stats.
//
// Returns the written BlockTermState; or nil if none.
//
// Mirrors UniformSplitTermsWriter.writePostingLine
// (UniformSplitTermsWriter.java:354), whose body is a call to
// PostingsWriterBase.writeTerm(BytesRef, TermsEnum, FixedBitSet,
// NormsProducer) followed by FieldMetadata.updateStats.
//
// PORT NOTE: Gocene's codecs.PostingsWriterBase declares no WriteTerm member,
// so the body of PushPostingsWriterBase.writeTerm
// (PushPostingsWriterBase.java:115) is driven here through codecs.WriteTerm,
// as every other Gocene terms dictionary writer does. That Java method reads
// two pieces of state the Gocene SPI does not hold on the writer: fieldInfo,
// which is the FieldInfo last passed to setField and is therefore
// fieldMetadata.GetFieldInfo(); and enumFlags, which Gocene returns from
// SetField instead of storing, so the caller carries it in as enumFlags.
func (w *UniformSplitTermsWriter) WritePostingLine(
	termsEnum spi.TermsEnum,
	fieldMetadata *FieldMetadata,
	normsProducer codecs.NormsProducer,
	enumFlags int,
) (index.TermState, error) {
	pusher, ok := w.PostingsWriter.(codecs.PushPostingsWriterBase)
	if !ok {
		return nil, fmt.Errorf("UniformSplitTermsWriter: postingsWriter %T does not implement PushPostingsWriterBase", w.PostingsWriter)
	}
	fieldInfo := fieldMetadata.GetFieldInfo()

	var normValues index.NumericDocValues
	if fieldInfo.HasNorms() {
		var err error
		normValues, err = normsProducer.GetNorms(fieldInfo)
		if err != nil {
			return nil, err
		}
	}
	if err := w.PostingsWriter.StartTerm(normValues); err != nil {
		return nil, err
	}

	postingsEnum, err := termsEnum.Postings(enumFlags)
	if err != nil {
		return nil, err
	}
	// assert postingsEnum != null;

	writeFreqs := fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
	writePositions := fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
	writeOffsets := fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	writePayloads := fieldInfo.HasPayloads()

	docFreq, totalTermFreq, err := codecs.WriteTerm(
		pusher, postingsEnum, writeFreqs, writePositions, writeOffsets, writePayloads, fieldMetadata.GetDocsSeen())
	if err != nil {
		return nil, err
	}
	if docFreq == 0 {
		// No doc for this term.
		return nil, nil
	}

	state := w.PostingsWriter.NewTermState()
	base := codecs.BaseState(state)
	base.DocFreq = docFreq
	// PushPostingsWriterBase.writeTerm: state.totalTermFreq = writeFreqs ? totalTermFreq : -1
	if writeFreqs {
		base.TotalTermFreq = totalTermFreq
	} else {
		base.TotalTermFreq = -1
	}
	if err := w.PostingsWriter.FinishTerm(state); err != nil {
		return nil, err
	}

	fieldMetadata.UpdateStats(state)
	return state, nil
}

// writeDictionary writes the dictionary index (FST) to disk.
//
// Mirrors UniformSplitTermsWriter.writeDictionary
// (UniformSplitTermsWriter.java:369).
func (w *UniformSplitTermsWriter) WriteDictionary(dictionaryBuilder IndexDictionaryBuilder) error {
	dictionary, err := dictionaryBuilder.Build()
	if err != nil {
		return err
	}
	return dictionary.Write(w.DictionaryOutput, w.BlockEncoder)
}

// Close mirrors UniformSplitTermsWriter.close
// (UniformSplitTermsWriter.java:373).
func (w *UniformSplitTermsWriter) Close() error {
	return util.CloseAll(w.BlockOutput, w.DictionaryOutput, w.PostingsWriter)
}

var _ spi.FieldsConsumer = (*UniformSplitTermsWriter)(nil)
