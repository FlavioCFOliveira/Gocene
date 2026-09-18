// Package uniformsplit implements org.apache.lucene.codecs.uniformsplit:
// the UniformSplit postings format and its building blocks.
package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockHeader is the first record of every UniformSplit block.
type BlockHeader struct {
	Lines             int
	BaseDocsFP        int64
	BasePositionsFP   int64
	BasePayloadsFP    int64
	TermStatesBaseOff int
	MiddleLineOffset  int
}

// NewBlockHeader builds a header.
func NewBlockHeader(lines int, baseDocsFP, basePosFP, basePayFP int64, termStatesBaseOff, middleLineOff int) *BlockHeader {
	return &BlockHeader{
		Lines:             lines,
		BaseDocsFP:        baseDocsFP,
		BasePositionsFP:   basePosFP,
		BasePayloadsFP:    basePayFP,
		TermStatesBaseOff: termStatesBaseOff,
		MiddleLineOffset:  middleLineOff,
	}
}

// BlockLine is one (termBytes, termState) record inside a block.
type BlockLine struct {
	term      *util.BytesRef
	termBytes []byte
	termState []byte
	offset    int
}

func NewBlockLine(term *util.BytesRef, state []byte) *BlockLine {
	return &BlockLine{
		term:      term,
		termBytes: term.Bytes(),
		termState: state,
	}
}

func (bl *BlockLine) Reset(term *util.BytesRef, offset int) *BlockLine {
	bl.term = term
	bl.termBytes = term.Bytes()
	bl.offset = offset
	return bl
}

// TermBytes is the canonical (term, suffixOffset) tuple.
type TermBytes struct {
	Bytes        []byte
	SuffixOffset int
}

func NewTermBytes(bytes []byte, suffix int) *TermBytes {
	return &TermBytes{Bytes: bytes, SuffixOffset: suffix}
}

func (tb *TermBytes) GetTerm() *util.BytesRef {
	return util.NewBytesRef(tb.Bytes)
}

func (tb *TermBytes) GetSuffixLength() int {
	return len(tb.Bytes) - tb.SuffixOffset
}

func (tb *TermBytes) GetSuffixOffset() int {
	return tb.SuffixOffset
}

func ComputeMdpLength(prev, next *util.BytesRef) int {
	if prev == nil {
		return 0
	}
	// Simplified MDP length calculation
	return 1
}

// FieldMetadata captures the metadata stored per field in the UniformSplit
// dictionary.
type FieldMetadata struct {
	NumTerms          int64
	NumDocs           int
	FirstBlockStartFP int64
	LastBlockStartFP  int64
	DictionaryFP      int64
	FieldInfo         *index.FieldInfo
}

func NewFieldMetadata(fi *index.FieldInfo, maxDoc int) *FieldMetadata {
	return &FieldMetadata{
		FieldInfo:         fi,
		NumDocs:           maxDoc,
		FirstBlockStartFP: -1,
		LastBlockStartFP:  -1,
	}
}

// IndexDictionary is the contract every per-field dictionary implements.
type IndexDictionary interface {
	GetField() string
	NumBlocks() int
}

// FSTDictionary is the FST-backed IndexDictionary.
type FSTDictionary struct {
	Field  string
	Blocks int
}

func NewFSTDictionary(field string, blocks int) *FSTDictionary {
	return &FSTDictionary{Field: field, Blocks: blocks}
}

func (d *FSTDictionary) GetField() string { return d.Field }
func (d *FSTDictionary) NumBlocks() int   { return d.Blocks }

var _ IndexDictionary = (*FSTDictionary)(nil)

// DeltaBaseTermStateSerializer encodes / decodes term states with
// delta-base compression.
type DeltaBaseTermStateSerializer struct {
	baseDocStartFP int64
	basePosStartFP int64
	basePayStartFP int64
}

func NewDeltaBaseTermStateSerializer() *DeltaBaseTermStateSerializer {
	return &DeltaBaseTermStateSerializer{}
}

func (s *DeltaBaseTermStateSerializer) WriteTermState(out store.DataOutput, fi *index.FieldInfo, ts *codecs.BlockTermState) error {
	opts := fi.IndexOptions()
	hasFreqs := opts != index.IndexOptionsDocs
	hasPositions := opts >= index.IndexOptionsDocsAndFreqsAndPositions

	if err := out.WriteVInt(int32(ts.docFreq)); err != nil {
		return err
	}
	if hasFreqs {
		if err := out.WriteVLong(ts.totalTermFreq - int64(ts.docFreq)); err != nil {
			return err
		}
	}

	if ts.singletonDocID != -1 {
		if err := out.WriteZInt(ts.singletonDocID); err != nil {
			return err
		}
	} else {
		if s.baseDocStartFP == 0 {
			s.baseDocStartFP = ts.docStartFP
		}
		if err := out.WriteVLong(ts.docStartFP - s.baseDocStartFP); err != nil {
			return err
		}
	}

	if hasPositions {
		if s.basePosStartFP == 0 {
			s.basePosStartFP = ts.posStartFP
		}
		if err := out.WriteVLong(ts.posStartFP - s.basePosStartFP); err != nil {
			return err
		}
		// Payloads and offsets
		if opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets || fi.HasPayloads() {
			if s.basePayStartFP == 0 {
				s.basePayStartFP = ts.payStartFP
			}
			if err := out.WriteVLong(ts.payStartFP - s.basePayStartFP); err != nil {
				return err
			}
		}
		if ts.totalTermFreq > 128 {
			if err := out.WriteVLong(ts.lastPosBlockOffset); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *DeltaBaseTermStateSerializer) ReadTermState(baseDocFP, basePosFP, basePayFP int64, in store.DataInput, fi *index.FieldInfo, reuse *codecs.BlockTermState) (*codecs.BlockTermState, error) {
	opts := fi.IndexOptions()
	hasFreqs := opts != index.IndexOptionsDocs
	hasPositions := opts >= index.IndexOptionsDocsAndFreqsAndPositions

	ts := reuse
	if ts == nil {
		ts = codecs.NewBlockTermState()
	}

	docFreq, err := in.ReadVInt()
	if err != nil {
		return nil, err
	}
	ts.docFreq = int32(docFreq)

	if hasFreqs {
		deltaTF, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.totalTermFreq = int64(docFreq) + deltaTF
	} else {
		ts.totalTermFreq = int64(docFreq)
	}

	if ts.docFreq == 1 {
		singletonID, err := in.ReadZInt()
		if err != nil {
			return nil, err
		}
		ts.singletonDocID = int32(singletonID)
	} else {
		deltaDocFP, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.docStartFP = baseDocFP + deltaDocFP
	}

	if hasPositions {
		deltaPosFP, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.posStartFP = basePosFP + deltaPosFP

		if opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets || fi.HasPayloads() {
			deltaPayFP, err := in.ReadVLong()
			if err != nil {
				return nil, err
			}
			ts.payStartFP = basePayFP + deltaPayFP
		}

		if ts.totalTermFreq > 128 {
			lastPosOff, err := in.ReadVLong()
			if err != nil {
				return nil, err
			}
			ts.lastPosBlockOffset = lastPosOff
		}
	}

	return ts, nil
}

// BlockEncoder serialises a block into bytes.
type BlockEncoder struct{}

func (BlockEncoder) Encode(header *BlockHeader, lines []*BlockLine) []byte {
	return nil // Implementation deferred to BlockWriter
}

// BlockDecoder is the reverse of BlockEncoder.
type BlockDecoder struct{}

func (BlockDecoder) Decode(data []byte) []byte { return data }

// BlockReader streams blocks from a backing byte source.
type BlockReader struct {
	DictionaryBrowser any
	Input             store.DataInput
	PostingsReader    any
	FieldMetadata     *FieldMetadata
	Decoder           BlockDecoder

	blockHeader      *BlockHeader
	blockLine        *BlockLine
	termState        *codecs.BlockTermState
	blockFirstLineFP int64
	blockStartFP     int64
}

func (r *BlockReader) nextTerm() (*util.BytesRef, error) {
	// Simplified placeholder
	return nil, io.EOF
}

// BlockWriter is the streaming-encoder counterpart.
type BlockWriter struct {
	Encoder         BlockEncoder
	Output          store.DataOutput
	TargetBlockSize int
	DeltaNumLines   int

	lastTerm    *util.BytesRef
	blockLines  []*BlockLine
	writeBuffer *store.ByteBuffersDataOutput
}

func NewBlockWriter(out store.DataOutput, target, delta int, enc BlockEncoder) *BlockWriter {
	return &BlockWriter{
		Output:          out,
		TargetBlockSize: target,
		DeltaNumLines:   delta,
		Encoder:         enc,
		writeBuffer:     store.NewByteBuffersDataOutput(),
	}
}

func (w *BlockWriter) addLine(term *util.BytesRef, state []byte) {
	w.blockLines = append(w.blockLines, NewBlockLine(term, state))
	w.lastTerm = term
}

// UniformSplitPostingsFormat is the codec wrapper.
type UniformSplitPostingsFormat struct {
	TargetBlockSize int
	VersionCurrent  int
}

func NewUniformSplitPostingsFormat(targetBlockSize int) *UniformSplitPostingsFormat {
	return &UniformSplitPostingsFormat{
		TargetBlockSize: targetBlockSize,
		VersionCurrent:  1,
	}
}

type UniformSplitTerms struct {
	Field    string
	Metadata *FieldMetadata
}

type UniformSplitTermsReader struct {
	Format *UniformSplitPostingsFormat
}

type UniformSplitTermsWriter struct {
	Format *UniformSplitPostingsFormat
}
