// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// segmentTermsEnumFrame holds the per-level state for one level of the block
// tree during SegmentTermsEnum traversal.
//
// Port of the package-private class
// org.apache.lucene.backward_codecs.lucene40.blocktree.SegmentTermsEnumFrame
// (Lucene 10.4.0).
//
// The full block-loading and navigation logic (loadBlock, nextEntry, etc.) is
// deferred to a later sprint; only the field declarations and a constructor
// are provided here.
type segmentTermsEnumFrame struct {
	// ord is the index of this frame in the SegmentTermsEnum.stack slice.
	ord int

	hasTerms     bool
	hasTermsOrig bool
	isFloor      bool

	arc *fst.Arc[*util.BytesRef]

	// fp is the file pointer of the start of this block.
	fp     int64
	fpOrig int64
	fpEnd  int64
	// totalSuffixBytes is used for Stats.
	totalSuffixBytes int64

	suffixBytes    []byte
	suffixesReader *store.ByteArrayDataInput

	suffixLengthBytes   []byte
	suffixLengthsReader *store.ByteArrayDataInput

	statBytes               []byte
	statsSingletonRunLength int
	statsReader             *store.ByteArrayDataInput

	floorData       []byte
	floorDataReader *store.ByteArrayDataInput

	// prefix is the length of the prefix shared by all terms in this block.
	prefix int

	// entCount is the number of entries (term or sub-block) in this block.
	entCount int

	// nextEnt is the index of the next entry to read, or -1 if not loaded.
	nextEnt int

	isLastInFloor bool
	isLeafBlock   bool

	lastSubFP int64

	nextFloorLabel       int
	numFollowFloorBlocks int

	metaDataUpto int

	// compressionAlg is the algorithm used for suffix compression in this block.
	compressionAlg CompressionAlgorithm

	// bytes holds encoded per-term metadata (lazy-decoded).
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// ste is the owning SegmentTermsEnum.
	ste *SegmentTermsEnum
}

// newSegmentTermsEnumFrame constructs a frame for the given SegmentTermsEnum at
// stack ordinal ord.
func newSegmentTermsEnumFrame(ste *SegmentTermsEnum, ord int) *segmentTermsEnumFrame {
	var version int32
	if ste.fr != nil && ste.fr.parent != nil {
		version = ste.fr.parent.version
	}

	var suffixLengthsReader *store.ByteArrayDataInput
	var suffixLengthBytes []byte
	if version >= versionCompressedSuffixes {
		suffixLengthBytes = make([]byte, 32)
		suffixLengthsReader = store.NewByteArrayDataInput(nil)
	}

	f := &segmentTermsEnumFrame{
		ste:                 ste,
		ord:                 ord,
		suffixBytes:         make([]byte, 128),
		suffixesReader:      store.NewByteArrayDataInput(nil),
		suffixLengthBytes:   suffixLengthBytes,
		suffixLengthsReader: suffixLengthsReader,
		statBytes:           make([]byte, 64),
		statsReader:         store.NewByteArrayDataInput(nil),
		floorData:           make([]byte, 32),
		floorDataReader:     store.NewByteArrayDataInput(nil),
		bytes:               make([]byte, 32),
		bytesReader:         store.NewByteArrayDataInput(nil),
		nextEnt:             -1,
	}
	// If suffix lengths share the same reader as suffixes (old format),
	// point suffixLengthsReader at suffixesReader.
	if version < versionCompressedSuffixes {
		f.suffixLengthsReader = f.suffixesReader
	}
	return f
}

// getTermBlockOrd returns the number of terms decoded so far in this block.
func (f *segmentTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	return 0 // state.termBlockOrd not available until full port
}
