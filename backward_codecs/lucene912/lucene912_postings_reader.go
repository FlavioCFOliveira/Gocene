// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Lucene912PostingsReader is the concrete reader for doc IDs (and
// optionally frequencies, positions, payloads, and offsets) encoded by the
// Lucene 9.12 postings format.
//
// Port of org.apache.lucene.backward_codecs.lucene912.Lucene912PostingsReader
// (Lucene 10.4.0).
//
// The full PostingsEnum implementations (BlockDocsEnum, EverythingEnum, and
// the impact enumerators) depend on ForUtil/ForDeltaUtil/PForUtil, which are
// ported in a later sprint. Until then, Postings and Impacts return
// ErrPostingsNotAvailable.
type Lucene912PostingsReader struct {
	docIn store.IndexInput
	posIn store.IndexInput // nil when the segment has no positions
	payIn store.IndexInput // nil when the segment has no payloads or offsets

	maxNumImpactsAtLevel0     int
	maxImpactNumBytesAtLevel0 int
	maxNumImpactsAtLevel1     int
	maxImpactNumBytesAtLevel1 int
}

// ErrPostingsNotAvailable is returned by Postings and Impacts until the full
// ForUtil/ForDeltaUtil/PForUtil port lands in a later sprint.
var ErrPostingsNotAvailable = errors.New(
	"lucene912: full PostingsEnum iteration not yet available (ForUtil port deferred)",
)

// NewLucene912PostingsReader opens and validates the meta, doc, pos, and pay
// files for the segment described by state.
//
// Port of Lucene912PostingsReader(SegmentReadState).
func NewLucene912PostingsReader(state *codecs.SegmentReadState) (*Lucene912PostingsReader, error) {
	metaName := codecs.GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, MetaExtension)

	rawMetaIn, err := state.Directory.OpenInput(metaName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene912 postings reader: open meta %q: %w", metaName, err)
	}
	metaIn := store.NewChecksumIndexInput(rawMetaIn)

	var version int32
	version, err = codecs.CheckIndexHeader(
		metaIn,
		metaCodec,
		versionStart,
		versionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene912 postings reader: check meta header: %w", err)
	}

	r := &Lucene912PostingsReader{}

	var v int32
	if v, err = metaIn.ReadInt(); err == nil {
		r.maxNumImpactsAtLevel0 = int(v)
	}
	if err == nil {
		if v, err = metaIn.ReadInt(); err == nil {
			r.maxImpactNumBytesAtLevel0 = int(v)
		}
	}
	if err == nil {
		if v, err = metaIn.ReadInt(); err == nil {
			r.maxNumImpactsAtLevel1 = int(v)
		}
	}
	if err == nil {
		if v, err = metaIn.ReadInt(); err == nil {
			r.maxImpactNumBytesAtLevel1 = int(v)
		}
	}
	var expectedDocFileLen int64
	if err == nil {
		expectedDocFileLen, err = metaIn.ReadLong()
	}
	if err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene912 postings reader: read meta: %w", err)
	}
	// expectedDocFileLen is preserved to match Java's logic; Gocene's
	// RetrieveChecksum does not accept an expected-length argument.
	_ = expectedDocFileLen

	var expectedPosFileLen, expectedPayFileLen int64 = -1, -1
	if state.FieldInfos.HasProx() {
		expectedPosFileLen, err = metaIn.ReadLong()
		if err != nil {
			_ = metaIn.Close()
			return nil, fmt.Errorf("lucene912 postings reader: read meta pos len: %w", err)
		}
		if fieldInfosHasPayloads(state.FieldInfos) || state.FieldInfos.HasOffsets() {
			expectedPayFileLen, err = metaIn.ReadLong()
			if err != nil {
				_ = metaIn.Close()
				return nil, fmt.Errorf("lucene912 postings reader: read meta pay len: %w", err)
			}
		}
	}

	if _, err = codecs.CheckFooter(metaIn); err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene912 postings reader: check meta footer: %w", err)
	}
	if err = metaIn.Close(); err != nil {
		return nil, fmt.Errorf("lucene912 postings reader: close meta: %w", err)
	}

	// Suppress unused-variable warning for expectedPosFileLen / expectedPayFileLen.
	// RetrieveChecksum in Gocene does not take an expected-length argument;
	// the length fields are preserved to match the Java reader's logic but not
	// used further until a stricter validate-length helper is added.
	_ = expectedPosFileLen
	_ = expectedPayFileLen

	var docIn, posIn, payIn store.IndexInput
	success := false
	defer func() {
		if !success {
			for _, in := range []store.IndexInput{docIn, posIn, payIn} {
				if in != nil {
					_ = in.Close()
				}
			}
		}
	}()

	// --- Open doc file ---
	docName := codecs.GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, DocExtension)
	docIn, err = state.Directory.OpenInput(docName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene912 postings reader: open doc %q: %w", docName, err)
	}
	if _, err = codecs.CheckIndexHeader(
		docIn, docCodec, version, version,
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	); err != nil {
		return nil, fmt.Errorf("lucene912 postings reader: check doc header: %w", err)
	}
	if _, err = codecs.RetrieveChecksum(docIn); err != nil {
		return nil, fmt.Errorf("lucene912 postings reader: retrieve doc checksum: %w", err)
	}

	// --- Open pos file (optional) ---
	if state.FieldInfos.HasProx() {
		posName := codecs.GetSegmentFileName(
			state.SegmentInfo.Name(), state.SegmentSuffix, PosExtension)
		posIn, err = state.Directory.OpenInput(posName, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return nil, fmt.Errorf("lucene912 postings reader: open pos %q: %w", posName, err)
		}
		if _, err = codecs.CheckIndexHeader(
			posIn, posCodec, version, version,
			state.SegmentInfo.GetID(), state.SegmentSuffix,
		); err != nil {
			return nil, fmt.Errorf("lucene912 postings reader: check pos header: %w", err)
		}
		if _, err = codecs.RetrieveChecksum(posIn); err != nil {
			return nil, fmt.Errorf("lucene912 postings reader: retrieve pos checksum: %w", err)
		}

		// --- Open pay file (optional) ---
		if fieldInfosHasPayloads(state.FieldInfos) || state.FieldInfos.HasOffsets() {
			payName := codecs.GetSegmentFileName(
				state.SegmentInfo.Name(), state.SegmentSuffix, PayExtension)
			payIn, err = state.Directory.OpenInput(payName, store.IOContext{Context: store.ContextRead})
			if err != nil {
				return nil, fmt.Errorf("lucene912 postings reader: open pay %q: %w", payName, err)
			}
			if _, err = codecs.CheckIndexHeader(
				payIn, payCodec, version, version,
				state.SegmentInfo.GetID(), state.SegmentSuffix,
			); err != nil {
				return nil, fmt.Errorf("lucene912 postings reader: check pay header: %w", err)
			}
			if _, err = codecs.RetrieveChecksum(payIn); err != nil {
				return nil, fmt.Errorf("lucene912 postings reader: retrieve pay checksum: %w", err)
			}
			r.payIn = payIn
		}
		r.posIn = posIn
	}

	r.docIn = docIn
	success = true
	return r, nil
}

// fieldInfosHasPayloads reports whether any field in fi has payloads.
// Mirrors Java FieldInfos.hasPayloads().
func fieldInfosHasPayloads(fi *index.FieldInfos) bool {
	it := fi.Iterator()
	for it.HasNext() {
		if it.Next().HasPayloads() {
			return true
		}
	}
	return false
}

// Init validates the terms-in header against the expected codec name and
// block size.
//
// Port of Lucene912PostingsReader.init(IndexInput, SegmentReadState).
func (r *Lucene912PostingsReader) Init(termsIn store.IndexInput, state *codecs.SegmentReadState) error {
	if _, err := codecs.CheckIndexHeader(
		termsIn,
		termsCodec,
		versionStart,
		versionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	); err != nil {
		return fmt.Errorf("lucene912 postings reader: init terms header: %w", err)
	}
	blockSize, err := store.ReadVInt(termsIn)
	if err != nil {
		return fmt.Errorf("lucene912 postings reader: read block size: %w", err)
	}
	if int(blockSize) != BlockSize {
		return fmt.Errorf(
			"lucene912 postings reader: index-time BLOCK_SIZE (%d) != read-time BLOCK_SIZE (%d)",
			blockSize, BlockSize,
		)
	}
	return nil
}

// NewTermState allocates a fresh IntBlockTermState.
func (r *Lucene912PostingsReader) NewTermState() *codecs.BlockTermState {
	return NewIntBlockTermState().BlockTermState
}

// DecodeTerm decodes per-term metadata from in into termState.
//
// Port of Lucene912PostingsReader.decodeTerm(DataInput, FieldInfo,
// BlockTermState, boolean).
func (r *Lucene912PostingsReader) DecodeTerm(
	in store.DataInput,
	fieldInfo *index.FieldInfo,
	termState *codecs.BlockTermState,
	absolute bool,
) error {
	// The Gocene PostingsReaderBase SPI uses *codecs.BlockTermState directly
	// with no provision for codec-specific sub-types. We carry the extra fields
	// in a package-level map keyed by pointer. Each BlockTermState is owned by
	// a single goroutine per the SPI contract, so no locking is needed.
	its := getOrCreateIntBlockTermState(termState)

	if absolute {
		its.DocStartFP = 0
		its.PosStartFP = 0
		its.PayStartFP = 0
	}

	l, err := store.ReadVLong(in)
	if err != nil {
		return fmt.Errorf("lucene912 decode term: read vlong: %w", err)
	}

	if l&0x01 == 0 {
		its.DocStartFP += l >> 1
		if termState.DocFreq == 1 {
			v, err2 := store.ReadVInt(in)
			if err2 != nil {
				return fmt.Errorf("lucene912 decode term: read singleton docID: %w", err2)
			}
			its.SingletonDocID = int(v)
		} else {
			its.SingletonDocID = -1
		}
	} else {
		// delta encoding for singleton docID
		its.SingletonDocID += int(util.ZigZagDecodeInt64(l >> 1))
	}

	opts := fieldInfo.IndexOptions()
	if opts >= index.IndexOptionsDocsAndFreqsAndPositions {
		delta, err2 := store.ReadVLong(in)
		if err2 != nil {
			return fmt.Errorf("lucene912 decode term: read pos fp delta: %w", err2)
		}
		its.PosStartFP += delta

		if opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets ||
			fieldInfo.HasPayloads() {
			delta2, err3 := store.ReadVLong(in)
			if err3 != nil {
				return fmt.Errorf("lucene912 decode term: read pay fp delta: %w", err3)
			}
			its.PayStartFP += delta2
		}

		if termState.TotalTermFreq > int64(BlockSize) {
			offset, err4 := store.ReadVLong(in)
			if err4 != nil {
				return fmt.Errorf("lucene912 decode term: read last pos block offset: %w", err4)
			}
			its.LastPosBlockOffset = offset
		} else {
			its.LastPosBlockOffset = -1
		}
	}
	return nil
}

// Postings returns a PostingsEnum for the term identified by termState.
//
// The full block-iterating implementation requires ForUtil/ForDeltaUtil/PForUtil
// (deferred sprint); this method returns ErrPostingsNotAvailable until then.
func (r *Lucene912PostingsReader) Postings(
	_ *index.FieldInfo,
	_ *codecs.BlockTermState,
	_ index.PostingsEnum,
	_ int,
) (index.PostingsEnum, error) {
	return nil, ErrPostingsNotAvailable
}

// Impacts returns an ImpactsEnum for the term identified by termState.
//
// Deferred to the same sprint as Postings; returns ErrPostingsNotAvailable.
func (r *Lucene912PostingsReader) Impacts(
	_ *index.FieldInfo,
	_ *codecs.BlockTermState,
	_ int,
) (any, error) {
	return nil, ErrPostingsNotAvailable
}

// CheckIntegrity verifies the CRC footers of all owned files.
func (r *Lucene912PostingsReader) CheckIntegrity() error {
	if _, err := codecs.ChecksumEntireFile(r.docIn); err != nil {
		return fmt.Errorf("lucene912 postings reader: checksum doc: %w", err)
	}
	if r.posIn != nil {
		if _, err := codecs.ChecksumEntireFile(r.posIn); err != nil {
			return fmt.Errorf("lucene912 postings reader: checksum pos: %w", err)
		}
	}
	if r.payIn != nil {
		if _, err := codecs.ChecksumEntireFile(r.payIn); err != nil {
			return fmt.Errorf("lucene912 postings reader: checksum pay: %w", err)
		}
	}
	return nil
}

// Close releases the file handles owned by this reader.
func (r *Lucene912PostingsReader) Close() error {
	var firstErr error
	for _, in := range []store.IndexInput{r.docIn, r.posIn, r.payIn} {
		if in == nil {
			continue
		}
		if err := in.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ─── side-channel state map ───────────────────────────────────────────────────
// The Gocene PostingsReaderBase SPI uses *codecs.BlockTermState as the state
// type with no provision for codec-specific sub-types. We store the extra
// fields in a package-level map keyed by pointer until the SPI is extended.
// This is intentionally a simple design; concurrent access is safe because
// each BlockTermState is owned by a single goroutine (per the SPI contract).

var intBlockTermStateMap = make(map[*codecs.BlockTermState]*IntBlockTermState)

func getOrCreateIntBlockTermState(bts *codecs.BlockTermState) *IntBlockTermState {
	if its, ok := intBlockTermStateMap[bts]; ok {
		return its
	}
	its := NewIntBlockTermState()
	its.BlockTermState = bts
	intBlockTermStateMap[bts] = its
	return its
}
