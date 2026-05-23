// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"fmt"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene 8.0 norms format constants.
//
// Port of package-private fields from
// org.apache.lucene.backward_codecs.lucene80.Lucene80NormsFormat
// (Lucene 10.4.0).
const (
	lucene80NormsDataCodec      = "Lucene80NormsData"
	lucene80NormsDataExt        = "nvd"
	lucene80NormsMetaCodec      = "Lucene80NormsMetadata"
	lucene80NormsMetaExt        = "nvm"
	lucene80NormsVersionStart   = int32(0)
	lucene80NormsVersionCurrent = lucene80NormsVersionStart
)

// lucene80NormsEntry holds the per-field metadata for a norms field.
//
// Port of Lucene80NormsProducer.NormsEntry.
type lucene80NormsEntry struct {
	denseRankPower      byte
	bytesPerNorm        byte
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	numDocsWithField    int32
	normsOffset         int64
}

// Lucene80NormsProducer reads norms written by the Lucene 8.0 format.
//
// Port of org.apache.lucene.backward_codecs.lucene80.Lucene80NormsProducer
// (Lucene 10.4.0).
//
// DEFERRED: getNorms per-field decoding requires IndexedDISI and
// LegacyDirectReader to be fully ported. GetNorms returns nil until then.
type Lucene80NormsProducer struct {
	norms   map[int]*lucene80NormsEntry
	data    gstore.IndexInput
	maxDoc  int
	version int32
	closed  bool
}

// NewLucene80NormsProducer opens the .nvm/.nvd files for the given segment
// and validates their codec headers.
//
// Port of Lucene80NormsProducer(SegmentReadState, String, String, String, String).
func NewLucene80NormsProducer(
	state *index.SegmentReadState,
	dataCodec, dataExtension, metaCodec, metaExtension string,
) (*Lucene80NormsProducer, error) {
	p := &Lucene80NormsProducer{
		norms:   make(map[int]*lucene80NormsEntry),
		maxDoc:  state.SegmentInfo.DocCount(),
		version: -1,
	}

	// --- meta file -------------------------------------------------------
	metaName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, metaExtension)
	metaIn, err := bcstore.OpenChecksumInput(state.Directory, metaName, gstore.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene80 norms: open meta %q: %w", metaName, err)
	}
	defer func() { _ = metaIn.Close() }()

	var priorErr error
	p.version, priorErr = codecs.CheckIndexHeader(
		metaIn,
		metaCodec,
		lucene80NormsVersionStart,
		lucene80NormsVersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if priorErr == nil {
		priorErr = p.readFields(metaIn, state.SegmentInfo.Name(), state.FieldInfos)
	}
	if footerErr := checkLucene80NormsFooter(metaIn); footerErr != nil {
		if priorErr != nil {
			return nil, priorErr
		}
		return nil, footerErr
	}
	if priorErr != nil {
		return nil, priorErr
	}

	// --- data file -------------------------------------------------------
	dataName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, dataExtension)
	dataIn, err := bcstore.OpenInput(state.Directory, dataName, gstore.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene80 norms: open data %q: %w", dataName, err)
	}

	version2, err := codecs.CheckIndexHeader(
		dataIn,
		dataCodec,
		lucene80NormsVersionStart,
		lucene80NormsVersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene80 norms: check data header: %w", err)
	}
	if p.version != version2 {
		_ = dataIn.Close()
		return nil, fmt.Errorf(
			"lucene80 norms: format version mismatch: meta=%d data=%d",
			p.version, version2)
	}
	if _, err = codecs.RetrieveChecksum(dataIn); err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene80 norms: retrieve checksum: %w", err)
	}

	p.data = dataIn
	return p, nil
}

// readFields reads the field-level metadata entries from meta until the -1
// field-number sentinel.
//
// Port of Lucene80NormsProducer.readFields(IndexInput, FieldInfos).
func (p *Lucene80NormsProducer) readFields(
	meta gstore.DataInput,
	segmentName string,
	infos *index.FieldInfos,
) error {
	for {
		fieldNumber, err := meta.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene80 norms: readFields: read fieldNumber: %w", err)
		}
		if fieldNumber == -1 {
			break
		}
		info := infos.GetByNumber(int(fieldNumber))
		if info == nil {
			return fmt.Errorf(
				"lucene80 norms: invalid field number %d in segment %s",
				fieldNumber, segmentName)
		}
		if !info.HasNorms() {
			return fmt.Errorf(
				"lucene80 norms: field %s does not have norms in segment %s",
				info.Name(), segmentName)
		}
		e := &lucene80NormsEntry{}
		if e.docsWithFieldOffset, err = meta.ReadLong(); err != nil {
			return fmt.Errorf("lucene80 norms: docsWithFieldOffset: %w", err)
		}
		if e.docsWithFieldLength, err = meta.ReadLong(); err != nil {
			return fmt.Errorf("lucene80 norms: docsWithFieldLength: %w", err)
		}
		jt, err := meta.ReadShort()
		if err != nil {
			return fmt.Errorf("lucene80 norms: jumpTableEntryCount: %w", err)
		}
		e.jumpTableEntryCount = jt
		if e.denseRankPower, err = meta.ReadByte(); err != nil {
			return fmt.Errorf("lucene80 norms: denseRankPower: %w", err)
		}
		nd, err := meta.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene80 norms: numDocsWithField: %w", err)
		}
		e.numDocsWithField = nd
		if e.bytesPerNorm, err = meta.ReadByte(); err != nil {
			return fmt.Errorf("lucene80 norms: bytesPerNorm: %w", err)
		}
		switch e.bytesPerNorm {
		case 0, 1, 2, 4, 8:
			// valid values
		default:
			return fmt.Errorf(
				"lucene80 norms: invalid bytesPerNorm %d for field %s in segment %s",
				e.bytesPerNorm, info.Name(), segmentName)
		}
		if e.normsOffset, err = meta.ReadLong(); err != nil {
			return fmt.Errorf("lucene80 norms: normsOffset: %w", err)
		}
		p.norms[info.Number()] = e
	}
	return nil
}

// GetNorms returns norms for the given field.
//
// DEFERRED: returns nil until IndexedDISI and LegacyDirectReader are fully
// ported.
func (p *Lucene80NormsProducer) GetNorms(field *index.FieldInfo) (codecs.NumericDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("lucene80 norms: producer closed")
	}
	return nil, nil
}

// CheckIntegrity verifies the norms data file.
//
// DEFERRED: full checksum verification requires cloning the data input.
func (p *Lucene80NormsProducer) CheckIntegrity() error {
	if p.closed {
		return fmt.Errorf("lucene80 norms: producer closed")
	}
	return nil
}

// Close releases all resources.
//
// Port of Lucene80NormsProducer.close().
func (p *Lucene80NormsProducer) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	if p.data != nil {
		return p.data.Close()
	}
	return nil
}

// checkLucene80NormsFooter validates the codec footer on the meta stream.
func checkLucene80NormsFooter(in *bcstore.EndiannessReverserChecksumIndexInput) error {
	remaining := in.Length() - in.GetFilePointer()
	const footerLen = 16
	if remaining < footerLen {
		return fmt.Errorf("lucene80 norms: misplaced codec footer (too short): remaining=%d", remaining)
	}
	if remaining > footerLen {
		return fmt.Errorf("lucene80 norms: misplaced codec footer (too long): remaining=%d", remaining)
	}
	magic, err := gstore.ReadInt32(in)
	if err != nil {
		return fmt.Errorf("lucene80 norms: footer magic: %w", err)
	}
	const footerMagic = int32(^0x3FD76C17)
	if magic != footerMagic {
		return fmt.Errorf("lucene80 norms: footer magic mismatch: got %x want %x", magic, footerMagic)
	}
	algID, err := gstore.ReadInt32(in)
	if err != nil {
		return fmt.Errorf("lucene80 norms: footer algorithmID: %w", err)
	}
	if algID != 0 {
		return fmt.Errorf("lucene80 norms: unknown algorithmID: %d", algID)
	}
	actualChecksum := int64(in.GetChecksum())
	expected, err := gstore.ReadInt64(in)
	if err != nil {
		return fmt.Errorf("lucene80 norms: footer checksum: %w", err)
	}
	if actualChecksum != expected {
		return fmt.Errorf("lucene80 norms: checksum mismatch: actual=%x expected=%x",
			actualChecksum, expected)
	}
	return nil
}

// compile-time assertion
var _ codecs.NormsProducer = (*Lucene80NormsProducer)(nil)
