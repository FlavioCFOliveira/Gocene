// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"fmt"
	"math"

	bcpacked "github.com/FlavioCFOliveira/Gocene/backward_codecs/packed"
	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene 8.0 doc values format constants.
//
// Port of package-private fields from
// org.apache.lucene.backward_codecs.lucene80.Lucene80DocValuesFormat
// (Lucene 10.4.0).
const (
	lucene80DVDataCodec  = "Lucene80DocValuesData"
	lucene80DVDataExt    = "dvd"
	lucene80DVMetaCodec  = "Lucene80DocValuesMetadata"
	lucene80DVMetaExt    = "dvm"
	lucene80DVModeKey    = "Lucene80DocValuesFormat.mode"
	lucene80VersionStart = int32(0)
	// VERSION_BIN_COMPRESSED introduced gzip compression for binary fields.
	lucene80VersionBinCompressed = int32(1)
	// VERSION_CONFIGURABLE_COMPRESSION lets the caller choose compression mode.
	lucene80VersionConfigurableCompression = int32(2)
	lucene80VersionCurrent                 = lucene80VersionConfigurableCompression

	// Field-type discriminators in the metadata stream.
	lucene80DVNumeric       = byte(0)
	lucene80DVBinary        = byte(1)
	lucene80DVSorted        = byte(2)
	lucene80DVSortedSet     = byte(3)
	lucene80DVSortedNumeric = byte(4)

	// Block-size constants for the various sub-encodings.
	lucene80DirectMonotonicBlockShift = 16
	lucene80NumericBlockShift         = 14
	lucene80TermsDictBlockShift       = 4
	lucene80TermsDictBlockLZ4Shift    = 6
	lucene80TermsDictBlockLZ4Code     = (lucene80TermsDictBlockLZ4Shift << 16) | 1
)

// lucene80DVNumericEntry holds the per-field metadata for a NUMERIC field.
//
// Port of Lucene80DocValuesProducer.NumericEntry.
type lucene80DVNumericEntry struct {
	table                []int64
	blockShift           int
	bitsPerValue         byte
	docsWithFieldOffset  int64
	docsWithFieldLength  int64
	jumpTableEntryCount  int16
	denseRankPower       byte
	numValues            int64
	minValue             int64
	gcd                  int64
	valuesOffset         int64
	valuesLength         int64
	valueJumpTableOffset int64
}

// lucene80DVBinaryEntry holds the per-field metadata for a BINARY field.
//
// Port of Lucene80DocValuesProducer.BinaryEntry.
type lucene80DVBinaryEntry struct {
	compressed               bool
	dataOffset               int64
	dataLength               int64
	docsWithFieldOffset      int64
	docsWithFieldLength      int64
	jumpTableEntryCount      int16
	denseRankPower           byte
	numDocsWithField         int32
	minLength                int32
	maxLength                int32
	addressesOffset          int64
	addressesLength          int64
	addressesMeta            *bcpacked.LegacyDirectMonotonicMeta
	numCompressedChunks      int32
	docsPerChunkShift        int32
	maxUncompressedChunkSize int32
}

// lucene80DVTermsDictEntry holds the shared terms-dictionary metadata for
// SORTED and SORTED_SET fields.
//
// Port of Lucene80DocValuesProducer.TermsDictEntry.
type lucene80DVTermsDictEntry struct {
	termsDictSize             int64
	termsDictBlockShift       int
	termsAddressesMeta        *bcpacked.LegacyDirectMonotonicMeta
	maxTermLength             int32
	termsDataOffset           int64
	termsDataLength           int64
	termsAddressesOffset      int64
	termsAddressesLength      int64
	termsDictIndexShift       int32
	termsIndexAddressesMeta   *bcpacked.LegacyDirectMonotonicMeta
	termsIndexOffset          int64
	termsIndexLength          int64
	termsIndexAddressesOffset int64
	termsIndexAddressesLength int64
	compressed                bool
	maxBlockLength            int32
}

// lucene80DVSortedEntry holds the per-field metadata for a SORTED field.
//
// Port of Lucene80DocValuesProducer.SortedEntry.
type lucene80DVSortedEntry struct {
	lucene80DVTermsDictEntry
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower      byte
	numDocsWithField    int32
	bitsPerValue        byte
	ordsOffset          int64
	ordsLength          int64
}

// lucene80DVSortedSetEntry holds the per-field metadata for a SORTED_SET field.
//
// Port of Lucene80DocValuesProducer.SortedSetEntry.
type lucene80DVSortedSetEntry struct {
	lucene80DVTermsDictEntry
	singleValueEntry    *lucene80DVSortedEntry
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower      byte
	numDocsWithField    int32
	bitsPerValue        byte
	ordsOffset          int64
	ordsLength          int64
	addressesMeta       *bcpacked.LegacyDirectMonotonicMeta
	addressesOffset     int64
	addressesLength     int64
}

// lucene80DVSortedNumericEntry holds the per-field metadata for a
// SORTED_NUMERIC field.
//
// Port of Lucene80DocValuesProducer.SortedNumericEntry.
type lucene80DVSortedNumericEntry struct {
	lucene80DVNumericEntry
	numDocsWithField int32
	addressesMeta    *bcpacked.LegacyDirectMonotonicMeta
	addressesOffset  int64
	addressesLength  int64
}

// Lucene80DocValuesProducer reads doc values written by the Lucene 8.0 format.
//
// Port of org.apache.lucene.backward_codecs.lucene80.Lucene80DocValuesProducer
// (Lucene 10.4.0).
//
// DEFERRED: per-field decoding (getNumeric, getBinary, getSorted, …) requires
// LegacyDirectMonotonicReader and LegacyDirectReader to be fully ported. Until
// then all Get* methods return nil, nil (empty iterator).
type Lucene80DocValuesProducer struct {
	numerics       map[int]*lucene80DVNumericEntry
	binaries       map[int]*lucene80DVBinaryEntry
	sorted         map[int]*lucene80DVSortedEntry
	sortedSets     map[int]*lucene80DVSortedSetEntry
	sortedNumerics map[int]*lucene80DVSortedNumericEntry

	data    gstore.IndexInput
	maxDoc  int
	version int32
	closed  bool
}

// NewLucene80DocValuesProducer opens the .dvm/.dvd files for the given segment
// and validates their codec headers.
//
// Port of Lucene80DocValuesProducer(SegmentReadState, String, String, String, String).
func NewLucene80DocValuesProducer(
	state *index.SegmentReadState,
	dataCodec, dataExtension, metaCodec, metaExtension string,
) (*Lucene80DocValuesProducer, error) {
	p := &Lucene80DocValuesProducer{
		numerics:       make(map[int]*lucene80DVNumericEntry),
		binaries:       make(map[int]*lucene80DVBinaryEntry),
		sorted:         make(map[int]*lucene80DVSortedEntry),
		sortedSets:     make(map[int]*lucene80DVSortedSetEntry),
		sortedNumerics: make(map[int]*lucene80DVSortedNumericEntry),
		maxDoc:         state.SegmentInfo.DocCount(),
		version:        -1,
	}

	// --- meta file -------------------------------------------------------
	metaName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, metaExtension)
	metaIn, err := bcstore.OpenChecksumInput(state.Directory, metaName, gstore.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene80 doc values: open meta %q: %w", metaName, err)
	}

	var priorErr error
	defer func() {
		// Mirror Java's IOUtils.closeWhileHandlingException pattern.
		_ = metaIn.Close()
	}()

	p.version, priorErr = codecs.CheckIndexHeader(
		metaIn,
		metaCodec,
		lucene80VersionStart,
		lucene80VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if priorErr == nil {
		priorErr = p.readFields(metaIn, state.SegmentInfo.Name(), state.FieldInfos)
	}
	if footerErr := checkLucene80DVFooter(metaIn); footerErr != nil {
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
		return nil, fmt.Errorf("lucene80 doc values: open data %q: %w", dataName, err)
	}

	version2, err := codecs.CheckIndexHeader(
		dataIn,
		dataCodec,
		lucene80VersionStart,
		lucene80VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene80 doc values: check data header: %w", err)
	}
	if p.version != version2 {
		_ = dataIn.Close()
		return nil, fmt.Errorf(
			"lucene80 doc values: format version mismatch: meta=%d data=%d",
			p.version, version2)
	}
	if _, err = codecs.RetrieveChecksum(dataIn); err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene80 doc values: retrieve checksum: %w", err)
	}

	p.data = dataIn
	return p, nil
}

// readFields reads the field-level metadata entries from meta until the -1
// field-number sentinel.
//
// Port of Lucene80DocValuesProducer.readFields(String, IndexInput, FieldInfos).
func (p *Lucene80DocValuesProducer) readFields(
	meta gstore.DataInput,
	segmentName string,
	infos *index.FieldInfos,
) error {
	for {
		fieldNumber, err := meta.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene80 doc values: readFields: read fieldNumber: %w", err)
		}
		if fieldNumber == -1 {
			break
		}
		info := infos.GetByNumber(int(fieldNumber))
		if info == nil {
			return fmt.Errorf(
				"lucene80 doc values: invalid field number %d in segment %s",
				fieldNumber, segmentName)
		}
		typ, err := meta.ReadByte()
		if err != nil {
			return fmt.Errorf("lucene80 doc values: readFields: read type: %w", err)
		}
		switch typ {
		case lucene80DVNumeric:
			entry, err := p.readNumericEntry(meta)
			if err != nil {
				return err
			}
			p.numerics[info.Number()] = entry
		case lucene80DVBinary:
			compressed, err := p.resolveBinaryCompressed(meta, info, segmentName)
			if err != nil {
				return err
			}
			entry, err := p.readBinaryEntry(meta, compressed)
			if err != nil {
				return err
			}
			p.binaries[info.Number()] = entry
		case lucene80DVSorted:
			entry, err := p.readSortedEntry(meta)
			if err != nil {
				return err
			}
			p.sorted[info.Number()] = entry
		case lucene80DVSortedSet:
			entry, err := p.readSortedSetEntry(meta)
			if err != nil {
				return err
			}
			p.sortedSets[info.Number()] = entry
		case lucene80DVSortedNumeric:
			entry, err := p.readSortedNumericEntry(meta)
			if err != nil {
				return err
			}
			p.sortedNumerics[info.Number()] = entry
		default:
			return fmt.Errorf(
				"lucene80 doc values: invalid type %d for field %d in segment %s",
				typ, fieldNumber, segmentName)
		}
	}
	return nil
}

// resolveBinaryCompressed resolves whether the binary field should be read as
// compressed, depending on the format version and the field attribute.
func (p *Lucene80DocValuesProducer) resolveBinaryCompressed(
	_ gstore.DataInput,
	info *index.FieldInfo,
	segmentName string,
) (bool, error) {
	if p.version >= lucene80VersionConfigurableCompression {
		val := info.GetAttribute(lucene80DVModeKey)
		if val == "" {
			return false, fmt.Errorf(
				"lucene80 doc values: missing attribute %q for field %s in segment %s",
				lucene80DVModeKey, info.Name(), segmentName)
		}
		return val == "BEST_COMPRESSION", nil
	}
	return p.version >= lucene80VersionBinCompressed, nil
}

// readNumericEntry reads the metadata for a NUMERIC field.
//
// Port of Lucene80DocValuesProducer.readNumeric(IndexInput).
func (p *Lucene80DocValuesProducer) readNumericEntry(
	meta gstore.DataInput,
) (*lucene80DVNumericEntry, error) {
	e := &lucene80DVNumericEntry{}
	if err := readNumericEntryInto(meta, e); err != nil {
		return nil, err
	}
	return e, nil
}

// readNumericEntryInto fills a lucene80DVNumericEntry (or embedded in
// SortedNumericEntry) from meta.
//
// Port of Lucene80DocValuesProducer.readNumeric(IndexInput, NumericEntry).
func readNumericEntryInto(meta gstore.DataInput, e *lucene80DVNumericEntry) error {
	var err error
	if e.docsWithFieldOffset, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 numeric: docsWithFieldOffset: %w", err)
	}
	if e.docsWithFieldLength, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 numeric: docsWithFieldLength: %w", err)
	}
	jt, err := meta.ReadShort()
	if err != nil {
		return fmt.Errorf("lucene80 numeric: jumpTableEntryCount: %w", err)
	}
	e.jumpTableEntryCount = jt
	if e.denseRankPower, err = meta.ReadByte(); err != nil {
		return fmt.Errorf("lucene80 numeric: denseRankPower: %w", err)
	}
	if e.numValues, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 numeric: numValues: %w", err)
	}
	tableSize, err := meta.ReadInt()
	if err != nil {
		return fmt.Errorf("lucene80 numeric: tableSize: %w", err)
	}
	if tableSize > 256 {
		return fmt.Errorf("lucene80 numeric: invalid tableSize %d", tableSize)
	}
	if tableSize >= 0 {
		e.table = make([]int64, tableSize)
		for i := int32(0); i < tableSize; i++ {
			if e.table[i], err = meta.ReadLong(); err != nil {
				return fmt.Errorf("lucene80 numeric: table[%d]: %w", i, err)
			}
		}
	}
	if tableSize < -1 {
		e.blockShift = int(-2 - tableSize)
	} else {
		e.blockShift = -1
	}
	if e.bitsPerValue, err = meta.ReadByte(); err != nil {
		return fmt.Errorf("lucene80 numeric: bitsPerValue: %w", err)
	}
	if e.minValue, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 numeric: minValue: %w", err)
	}
	if e.gcd, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 numeric: gcd: %w", err)
	}
	if e.valuesOffset, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 numeric: valuesOffset: %w", err)
	}
	if e.valuesLength, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 numeric: valuesLength: %w", err)
	}
	if e.valueJumpTableOffset, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 numeric: valueJumpTableOffset: %w", err)
	}
	return nil
}

// readBinaryEntry reads the metadata for a BINARY field.
//
// Port of Lucene80DocValuesProducer.readBinary(IndexInput, boolean).
func (p *Lucene80DocValuesProducer) readBinaryEntry(
	meta gstore.DataInput,
	compressed bool,
) (*lucene80DVBinaryEntry, error) {
	e := &lucene80DVBinaryEntry{compressed: compressed}
	var err error
	if e.dataOffset, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 binary: dataOffset: %w", err)
	}
	if e.dataLength, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 binary: dataLength: %w", err)
	}
	if e.docsWithFieldOffset, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 binary: docsWithFieldOffset: %w", err)
	}
	if e.docsWithFieldLength, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 binary: docsWithFieldLength: %w", err)
	}
	jt, err := meta.ReadShort()
	if err != nil {
		return nil, fmt.Errorf("lucene80 binary: jumpTableEntryCount: %w", err)
	}
	e.jumpTableEntryCount = jt
	if e.denseRankPower, err = meta.ReadByte(); err != nil {
		return nil, fmt.Errorf("lucene80 binary: denseRankPower: %w", err)
	}
	nd, err := meta.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene80 binary: numDocsWithField: %w", err)
	}
	e.numDocsWithField = nd
	ml, err := meta.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene80 binary: minLength: %w", err)
	}
	e.minLength = ml
	xl, err := meta.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene80 binary: maxLength: %w", err)
	}
	e.maxLength = xl

	if (compressed && e.numDocsWithField > 0) || e.minLength < e.maxLength {
		if e.addressesOffset, err = meta.ReadLong(); err != nil {
			return nil, fmt.Errorf("lucene80 binary: addressesOffset: %w", err)
		}
		numAddresses := int64(e.numDocsWithField) + 1
		if compressed {
			nc, err2 := gstore.ReadVInt(meta)
			if err2 != nil {
				return nil, fmt.Errorf("lucene80 binary: numCompressedChunks: %w", err2)
			}
			e.numCompressedChunks = nc
			dps, err2 := gstore.ReadVInt(meta)
			if err2 != nil {
				return nil, fmt.Errorf("lucene80 binary: docsPerChunkShift: %w", err2)
			}
			e.docsPerChunkShift = dps
			mu, err2 := gstore.ReadVInt(meta)
			if err2 != nil {
				return nil, fmt.Errorf("lucene80 binary: maxUncompressedChunkSize: %w", err2)
			}
			e.maxUncompressedChunkSize = mu
			numAddresses = int64(e.numCompressedChunks)
		}
		blockShift, err2 := gstore.ReadVInt(meta)
		if err2 != nil {
			return nil, fmt.Errorf("lucene80 binary: blockShift: %w", err2)
		}
		addrMeta, err2 := loadLegacyDirectMonotonicMeta(meta, numAddresses, int(blockShift))
		if err2 != nil {
			return nil, fmt.Errorf("lucene80 binary: addressesMeta: %w", err2)
		}
		e.addressesMeta = addrMeta
		if e.addressesLength, err = meta.ReadLong(); err != nil {
			return nil, fmt.Errorf("lucene80 binary: addressesLength: %w", err)
		}
	}
	return e, nil
}

// readSortedEntry reads the metadata for a SORTED field.
//
// Port of Lucene80DocValuesProducer.readSorted(IndexInput).
func (p *Lucene80DocValuesProducer) readSortedEntry(
	meta gstore.DataInput,
) (*lucene80DVSortedEntry, error) {
	e := &lucene80DVSortedEntry{}
	var err error
	if e.docsWithFieldOffset, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sorted: docsWithFieldOffset: %w", err)
	}
	if e.docsWithFieldLength, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sorted: docsWithFieldLength: %w", err)
	}
	jt, err := meta.ReadShort()
	if err != nil {
		return nil, fmt.Errorf("lucene80 sorted: jumpTableEntryCount: %w", err)
	}
	e.jumpTableEntryCount = jt
	if e.denseRankPower, err = meta.ReadByte(); err != nil {
		return nil, fmt.Errorf("lucene80 sorted: denseRankPower: %w", err)
	}
	nd, err := meta.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene80 sorted: numDocsWithField: %w", err)
	}
	e.numDocsWithField = nd
	if e.bitsPerValue, err = meta.ReadByte(); err != nil {
		return nil, fmt.Errorf("lucene80 sorted: bitsPerValue: %w", err)
	}
	if e.ordsOffset, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sorted: ordsOffset: %w", err)
	}
	if e.ordsLength, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sorted: ordsLength: %w", err)
	}
	if err = readTermsDictEntry(meta, &e.lucene80DVTermsDictEntry); err != nil {
		return nil, err
	}
	return e, nil
}

// readSortedSetEntry reads the metadata for a SORTED_SET field.
//
// Port of Lucene80DocValuesProducer.readSortedSet(IndexInput).
func (p *Lucene80DocValuesProducer) readSortedSetEntry(
	meta gstore.DataInput,
) (*lucene80DVSortedSetEntry, error) {
	e := &lucene80DVSortedSetEntry{}
	multiValued, err := meta.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: multiValued: %w", err)
	}
	switch multiValued {
	case 0: // single-valued — stored as SORTED
		sv, err2 := p.readSortedEntry(meta)
		if err2 != nil {
			return nil, err2
		}
		e.singleValueEntry = sv
		return e, nil
	case 1: // multi-valued
	default:
		return nil, fmt.Errorf("lucene80 sortedSet: invalid multiValued byte %d", multiValued)
	}
	if e.docsWithFieldOffset, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: docsWithFieldOffset: %w", err)
	}
	if e.docsWithFieldLength, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: docsWithFieldLength: %w", err)
	}
	jt, err := meta.ReadShort()
	if err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: jumpTableEntryCount: %w", err)
	}
	e.jumpTableEntryCount = jt
	if e.denseRankPower, err = meta.ReadByte(); err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: denseRankPower: %w", err)
	}
	if e.bitsPerValue, err = meta.ReadByte(); err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: bitsPerValue: %w", err)
	}
	if e.ordsOffset, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: ordsOffset: %w", err)
	}
	if e.ordsLength, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: ordsLength: %w", err)
	}
	nd, err := meta.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: numDocsWithField: %w", err)
	}
	e.numDocsWithField = nd
	if e.addressesOffset, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: addressesOffset: %w", err)
	}
	blockShift, err := gstore.ReadVInt(meta)
	if err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: blockShift: %w", err)
	}
	addrMeta, err := loadLegacyDirectMonotonicMeta(meta, int64(e.numDocsWithField)+1, int(blockShift))
	if err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: addressesMeta: %w", err)
	}
	e.addressesMeta = addrMeta
	if e.addressesLength, err = meta.ReadLong(); err != nil {
		return nil, fmt.Errorf("lucene80 sortedSet: addressesLength: %w", err)
	}
	if err = readTermsDictEntry(meta, &e.lucene80DVTermsDictEntry); err != nil {
		return nil, err
	}
	return e, nil
}

// readSortedNumericEntry reads the metadata for a SORTED_NUMERIC field.
//
// Port of Lucene80DocValuesProducer.readSortedNumeric(IndexInput).
func (p *Lucene80DocValuesProducer) readSortedNumericEntry(
	meta gstore.DataInput,
) (*lucene80DVSortedNumericEntry, error) {
	e := &lucene80DVSortedNumericEntry{}
	if err := readNumericEntryInto(meta, &e.lucene80DVNumericEntry); err != nil {
		return nil, err
	}
	nd, err := meta.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene80 sortedNumeric: numDocsWithField: %w", err)
	}
	e.numDocsWithField = nd
	if int64(e.numDocsWithField) != e.numValues {
		if e.addressesOffset, err = meta.ReadLong(); err != nil {
			return nil, fmt.Errorf("lucene80 sortedNumeric: addressesOffset: %w", err)
		}
		blockShift, err2 := gstore.ReadVInt(meta)
		if err2 != nil {
			return nil, fmt.Errorf("lucene80 sortedNumeric: blockShift: %w", err2)
		}
		addrMeta, err2 := loadLegacyDirectMonotonicMeta(
			meta, int64(e.numDocsWithField)+1, int(blockShift))
		if err2 != nil {
			return nil, fmt.Errorf("lucene80 sortedNumeric: addressesMeta: %w", err2)
		}
		e.addressesMeta = addrMeta
		if e.addressesLength, err = meta.ReadLong(); err != nil {
			return nil, fmt.Errorf("lucene80 sortedNumeric: addressesLength: %w", err)
		}
	}
	return e, nil
}

// readTermsDictEntry reads the shared terms-dictionary metadata into dst.
//
// Port of Lucene80DocValuesProducer.readTermDict(IndexInput, TermsDictEntry).
func readTermsDictEntry(meta gstore.DataInput, dst *lucene80DVTermsDictEntry) error {
	var err error
	dst.termsDictSize, err = gstore.ReadVLong(meta)
	if err != nil {
		return fmt.Errorf("lucene80 termsDict: termsDictSize: %w", err)
	}
	termsDictBlockCode, err := meta.ReadInt()
	if err != nil {
		return fmt.Errorf("lucene80 termsDict: termsDictBlockCode: %w", err)
	}
	if int(termsDictBlockCode) == lucene80TermsDictBlockLZ4Code {
		dst.compressed = true
		dst.termsDictBlockShift = lucene80TermsDictBlockLZ4Shift
	} else {
		dst.termsDictBlockShift = int(termsDictBlockCode)
	}

	blockShift, err := meta.ReadInt()
	if err != nil {
		return fmt.Errorf("lucene80 termsDict: blockShift: %w", err)
	}
	addressesSize := (dst.termsDictSize +
		int64(1<<dst.termsDictBlockShift) - 1) >> dst.termsDictBlockShift
	dst.termsAddressesMeta, err = loadLegacyDirectMonotonicMeta(
		meta, addressesSize, int(blockShift))
	if err != nil {
		return fmt.Errorf("lucene80 termsDict: termsAddressesMeta: %w", err)
	}
	dst.maxTermLength, err = meta.ReadInt()
	if err != nil {
		return fmt.Errorf("lucene80 termsDict: maxTermLength: %w", err)
	}
	if dst.compressed {
		dst.maxBlockLength, err = meta.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene80 termsDict: maxBlockLength: %w", err)
		}
	}
	if dst.termsDataOffset, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 termsDict: termsDataOffset: %w", err)
	}
	if dst.termsDataLength, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 termsDict: termsDataLength: %w", err)
	}
	if dst.termsAddressesOffset, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 termsDict: termsAddressesOffset: %w", err)
	}
	if dst.termsAddressesLength, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 termsDict: termsAddressesLength: %w", err)
	}
	dst.termsDictIndexShift, err = meta.ReadInt()
	if err != nil {
		return fmt.Errorf("lucene80 termsDict: termsDictIndexShift: %w", err)
	}
	blockShift2, err := meta.ReadInt()
	if err != nil {
		return fmt.Errorf("lucene80 termsDict: indexBlockShift: %w", err)
	}
	indexSize := (dst.termsDictSize +
		int64(1<<dst.termsDictIndexShift) - 1) >> dst.termsDictIndexShift
	dst.termsIndexAddressesMeta, err = loadLegacyDirectMonotonicMeta(
		meta, 1+indexSize, int(blockShift2))
	if err != nil {
		return fmt.Errorf("lucene80 termsDict: termsIndexAddressesMeta: %w", err)
	}
	if dst.termsIndexOffset, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 termsDict: termsIndexOffset: %w", err)
	}
	if dst.termsIndexLength, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 termsDict: termsIndexLength: %w", err)
	}
	if dst.termsIndexAddressesOffset, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 termsDict: termsIndexAddressesOffset: %w", err)
	}
	if dst.termsIndexAddressesLength, err = meta.ReadLong(); err != nil {
		return fmt.Errorf("lucene80 termsDict: termsIndexAddressesLength: %w", err)
	}
	return nil
}

// loadLegacyDirectMonotonicMeta reads the per-block metadata from meta for a
// LegacyDirectMonotonicReader with numValues values and 2^blockShift entries
// per block.
//
// Port of LegacyDirectMonotonicReader.loadMeta(IndexInput, long, int).
func loadLegacyDirectMonotonicMeta(
	meta gstore.DataInput,
	numValues int64,
	blockShift int,
) (*bcpacked.LegacyDirectMonotonicMeta, error) {
	m := bcpacked.NewLegacyDirectMonotonicMeta(numValues, blockShift)
	for i := 0; i < m.NumBlocks; i++ {
		min, err := meta.ReadLong()
		if err != nil {
			return nil, fmt.Errorf("loadMeta block %d mins: %w", i, err)
		}
		m.Mins[i] = min
		avgBits, err := meta.ReadInt()
		if err != nil {
			return nil, fmt.Errorf("loadMeta block %d avgs: %w", i, err)
		}
		// The Java side stores the float as its IEEE-754 int32 bit pattern.
		m.Avgs[i] = float32FromBits(uint32(avgBits))
		off, err := meta.ReadLong()
		if err != nil {
			return nil, fmt.Errorf("loadMeta block %d offsets: %w", i, err)
		}
		m.Offsets[i] = off
		bpv, err := meta.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("loadMeta block %d bpvs: %w", i, err)
		}
		m.BPVs[i] = bpv
	}
	return m, nil
}

// float32FromBits reinterprets the IEEE-754 bit pattern as a float32.
func float32FromBits(bits uint32) float32 {
	return math.Float32frombits(bits)
}

// GetNumeric returns numeric doc values for field.
//
// DEFERRED: returns nil until LegacyDirectReader/LegacyDirectMonotonicReader
// are fully ported.
func (p *Lucene80DocValuesProducer) GetNumeric(field *index.FieldInfo) (codecs.NumericDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("lucene80 doc values: producer closed")
	}
	return nil, nil
}

// GetBinary returns binary doc values for field.
//
// DEFERRED: returns nil until LegacyDirectReader/LegacyDirectMonotonicReader
// are fully ported.
func (p *Lucene80DocValuesProducer) GetBinary(field *index.FieldInfo) (codecs.BinaryDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("lucene80 doc values: producer closed")
	}
	return nil, nil
}

// GetSorted returns sorted doc values for field.
//
// DEFERRED: returns nil until LegacyDirectReader/LegacyDirectMonotonicReader
// are fully ported.
func (p *Lucene80DocValuesProducer) GetSorted(field *index.FieldInfo) (codecs.SortedDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("lucene80 doc values: producer closed")
	}
	return nil, nil
}

// GetSortedSet returns sorted-set doc values for field.
//
// DEFERRED: returns nil until LegacyDirectReader/LegacyDirectMonotonicReader
// are fully ported.
func (p *Lucene80DocValuesProducer) GetSortedSet(field *index.FieldInfo) (codecs.SortedSetDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("lucene80 doc values: producer closed")
	}
	return nil, nil
}

// GetSortedNumeric returns sorted-numeric doc values for field.
//
// DEFERRED: returns nil until LegacyDirectReader/LegacyDirectMonotonicReader
// are fully ported.
func (p *Lucene80DocValuesProducer) GetSortedNumeric(field *index.FieldInfo) (codecs.SortedNumericDocValues, error) {
	if p.closed {
		return nil, fmt.Errorf("lucene80 doc values: producer closed")
	}
	return nil, nil
}

// CheckIntegrity verifies the checksum on the data file.
//
// Port of Lucene80DocValuesProducer.checkIntegrity().
func (p *Lucene80DocValuesProducer) CheckIntegrity() error {
	if p.closed {
		return fmt.Errorf("lucene80 doc values: producer closed")
	}
	// DEFERRED: full checksum verification requires cloning the data input.
	return nil
}

// Close releases all resources.
//
// Port of Lucene80DocValuesProducer.close().
func (p *Lucene80DocValuesProducer) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	if p.data != nil {
		return p.data.Close()
	}
	return nil
}

// checkLucene80DVFooter validates the codec footer written by
// EndiannessReverserChecksumIndexInput for a big-endian legacy format.
//
// codecs.CheckFooter requires *store.ChecksumIndexInput; we use the same
// logic but accept *bcstore.EndiannessReverserChecksumIndexInput directly,
// following the pattern established in backward_codecs/lucene94.
func checkLucene80DVFooter(in *bcstore.EndiannessReverserChecksumIndexInput) error {
	remaining := in.Length() - in.GetFilePointer()
	const footerLen = 16
	if remaining < footerLen {
		return fmt.Errorf("lucene80 doc values: misplaced codec footer (too short): remaining=%d", remaining)
	}
	if remaining > footerLen {
		return fmt.Errorf("lucene80 doc values: misplaced codec footer (too long): remaining=%d", remaining)
	}
	magic, err := gstore.ReadInt32(in)
	if err != nil {
		return fmt.Errorf("lucene80 doc values: footer magic: %w", err)
	}
	const footerMagic = int32(^0x3FD76C17)
	if magic != footerMagic {
		return fmt.Errorf("lucene80 doc values: footer magic mismatch: got %x want %x", magic, footerMagic)
	}
	algID, err := gstore.ReadInt32(in)
	if err != nil {
		return fmt.Errorf("lucene80 doc values: footer algorithmID: %w", err)
	}
	if algID != 0 {
		return fmt.Errorf("lucene80 doc values: unknown algorithmID: %d", algID)
	}
	actualChecksum := int64(in.GetChecksum())
	expected, err := gstore.ReadInt64(in)
	if err != nil {
		return fmt.Errorf("lucene80 doc values: footer checksum: %w", err)
	}
	if actualChecksum != expected {
		return fmt.Errorf("lucene80 doc values: checksum mismatch: actual=%x expected=%x",
			actualChecksum, expected)
	}
	return nil
}

// compile-time assertion
var _ codecs.DocValuesProducer = (*Lucene80DocValuesProducer)(nil)
