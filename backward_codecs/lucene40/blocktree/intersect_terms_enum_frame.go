// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// intersectTermsEnumFrame holds the per-level state for one level of the
// block tree during IntersectTermsEnum traversal.
//
// Port of the package-private class
// org.apache.lucene.backward_codecs.lucene40.blocktree.IntersectTermsEnumFrame
// (Lucene 10.4.0).
//
// The full block-loading and navigation logic (load, next, decodeMetaData,
// etc.) is deferred to a later sprint; only the field declarations and a
// constructor are provided here.
type intersectTermsEnumFrame struct {
	ord int

	fp        int64
	fpOrig    int64
	fpEnd     int64
	lastSubFP int64

	// Automaton state at the start of this block.
	state     int
	lastState int

	metaDataUpto int

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

	// nextEnt is the index of the next entry to read.
	nextEnt int

	isLastInFloor bool
	isLeafBlock   bool

	numFollowFloorBlocks int
	nextFloorLabel       int

	// transition is the current automaton transition being matched.
	transition      automaton.Transition
	transitionIndex int
	transitionCount int

	arc *fst.Arc[*util.BytesRef]

	// termState holds decoded per-term metadata.
	termState *codecs.BlockTermState

	// bytes holds encoded per-term metadata (lazy-decoded).
	bytes       []byte
	bytesReader *store.ByteArrayDataInput

	// outputPrefix is the cumulative FST output up to this frame.
	outputPrefix *util.BytesRef

	startBytePos int
	suffix       int

	// ite is the owning IntersectTermsEnum.
	ite *IntersectTermsEnum

	version int32
}

// newIntersectTermsEnumFrame constructs a frame for the given
// IntersectTermsEnum at stack ordinal ord.
//
// Port of IntersectTermsEnumFrame(IntersectTermsEnum, int).
func newIntersectTermsEnumFrame(ite *IntersectTermsEnum, ord int) *intersectTermsEnumFrame {
	f := &intersectTermsEnumFrame{
		ite:             ite,
		ord:             ord,
		suffixBytes:     make([]byte, 128),
		suffixesReader:  store.NewByteArrayDataInput(nil),
		statBytes:       make([]byte, 64),
		statsReader:     store.NewByteArrayDataInput(nil),
		floorData:       make([]byte, 32),
		floorDataReader: store.NewByteArrayDataInput(nil),
		bytes:           make([]byte, 32),
		bytesReader:     store.NewByteArrayDataInput(nil),
		termState:       ite.fr.parent.postingsReader.NewTermState(),
		version:         ite.fr.parent.version,
	}
	if f.termState != nil {
		f.termState.TotalTermFreq = -1
	}

	if f.version >= VersionCompressedSuffixes {
		f.suffixLengthBytes = make([]byte, 32)
		f.suffixLengthsReader = store.NewByteArrayDataInput(nil)
	} else {
		f.suffixLengthsReader = f.suffixesReader
	}

	return f
}

// getTermBlockOrd returns the number of terms decoded so far in this block.
func (f *intersectTermsEnumFrame) getTermBlockOrd() int {
	if f.isLeafBlock {
		return f.nextEnt
	}
	if f.termState == nil {
		return 0
	}
	return f.termState.TermBlockOrd
}

// setState sets the automaton state for this frame and primes the first
// transition.
//
// Port of IntersectTermsEnumFrame.setState.
func (f *intersectTermsEnumFrame) setState(state int) {
	f.state = state
	f.transitionIndex = 0
	f.transitionCount = f.ite.automaton.GetNumTransitions(state)
	if f.transitionCount != 0 {
		f.ite.automaton.InitTransition(state, &f.transition)
		f.ite.automaton.GetNextTransition(&f.transition)
	} else {
		// No transitions: set sentinel values so label-range checks never
		// accidentally match.
		f.transition.Min = -1
		f.transition.Max = -1
	}
}

// loadNextFloorBlock advances to the next sub-block of a floor block.
//
// Port of IntersectTermsEnumFrame.loadNextFloorBlock.
func (f *intersectTermsEnumFrame) loadNextFloorBlock() error {
	for {
		v, err := f.floorDataReader.ReadVLong()
		if err != nil {
			return fmt.Errorf("blocktree intersect loadNextFloorBlock: %w", err)
		}
		f.fp = f.fpOrig + int64(uint64(v)>>1)
		f.numFollowFloorBlocks--
		if f.numFollowFloorBlocks != 0 {
			b, err2 := f.floorDataReader.ReadByte()
			if err2 != nil {
				return fmt.Errorf("blocktree intersect loadNextFloorBlock floor label: %w", err2)
			}
			f.nextFloorLabel = int(b) & 0xFF
		} else {
			f.nextFloorLabel = 256
		}
		if f.numFollowFloorBlocks == 0 || f.nextFloorLabel > f.transition.Min {
			break
		}
	}
	return f.load(nil)
}

// load reads a block from the terms file, optionally using frameIndexData to
// parse the floor-block structure.
//
// Port of IntersectTermsEnumFrame.load.
func (f *intersectTermsEnumFrame) load(frameIndexData *util.BytesRef) error {
	if frameIndexData != nil {
		f.floorDataReader.ResetWithSlice(
			frameIndexData.Bytes,
			frameIndexData.Offset,
			frameIndexData.Length,
		)
		// Skip first vlong — has redundant fp / hasTerms / isFloor flags.
		code, err := f.floorDataReader.ReadVLong()
		if err != nil {
			return fmt.Errorf("blocktree intersect load floor code: %w", err)
		}
		if (code & OutputFlagIsFloor) != 0 {
			// Floor frame.
			v, err2 := f.floorDataReader.ReadVInt()
			if err2 != nil {
				return fmt.Errorf("blocktree intersect load numFollowFloorBlocks: %w", err2)
			}
			f.numFollowFloorBlocks = int(v)
			b, err3 := f.floorDataReader.ReadByte()
			if err3 != nil {
				return fmt.Errorf("blocktree intersect load nextFloorLabel: %w", err3)
			}
			f.nextFloorLabel = int(b) & 0xFF

			// If current state is not accepting and has transitions, skip
			// floor blocks that precede the first applicable transition.
			if !f.ite.runAutomaton.IsAccept(f.state) && f.transitionCount != 0 {
				for f.numFollowFloorBlocks != 0 && f.nextFloorLabel <= f.transition.Min {
					v2, err4 := f.floorDataReader.ReadVLong()
					if err4 != nil {
						return fmt.Errorf("blocktree intersect load skip floor fp: %w", err4)
					}
					f.fp = f.fpOrig + int64(uint64(v2)>>1)
					f.numFollowFloorBlocks--
					if f.numFollowFloorBlocks != 0 {
						b2, err5 := f.floorDataReader.ReadByte()
						if err5 != nil {
							return fmt.Errorf("blocktree intersect load skip floor label: %w", err5)
						}
						f.nextFloorLabel = int(b2) & 0xFF
					} else {
						f.nextFloorLabel = 256
					}
				}
			}
		}
	}

	if err := f.ite.in.SetPosition(f.fp); err != nil {
		return fmt.Errorf("blocktree intersect load seek fp=%d: %w", f.fp, err)
	}
	code, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("blocktree intersect load entCount: %w", err)
	}
	f.entCount = int(code >> 1)
	f.isLastInFloor = (code & 1) != 0

	// Read suffix bytes.
	if f.version >= VersionCompressedSuffixes {
		codeL, err2 := store.ReadVLong(f.ite.in)
		if err2 != nil {
			return fmt.Errorf("blocktree intersect load suffix codeL: %w", err2)
		}
		f.isLeafBlock = (codeL & 0x04) != 0
		numSuffixBytes := int(codeL >> 3)
		if numSuffixBytes > len(f.suffixBytes) {
			f.suffixBytes = util.GrowExactByte(f.suffixBytes, util.Oversize(numSuffixBytes, 1))
		}
		algCode := int(codeL & 0x03)
		alg, err3 := CompressionAlgorithmByCode(algCode)
		if err3 != nil {
			return fmt.Errorf("blocktree intersect load: %w", err3)
		}
		if err4 := alg.Decompress(indexInputCompressAdapter{f.ite.in}, f.suffixBytes, numSuffixBytes); err4 != nil {
			return fmt.Errorf("blocktree intersect load suffix decompress: %w", err4)
		}
		f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numSuffixBytes)

		numSuffixLengthsRaw, err5 := store.ReadVInt(f.ite.in)
		if err5 != nil {
			return fmt.Errorf("blocktree intersect load suffix lengths header: %w", err5)
		}
		allEqual := (numSuffixLengthsRaw & 0x01) != 0
		numSuffixLengthBytes := int(numSuffixLengthsRaw >> 1)
		if numSuffixLengthBytes > len(f.suffixLengthBytes) {
			f.suffixLengthBytes = util.GrowExactByte(f.suffixLengthBytes, util.Oversize(numSuffixLengthBytes, 1))
		}
		if allEqual {
			b, err6 := f.ite.in.ReadByte()
			if err6 != nil {
				return fmt.Errorf("blocktree intersect load allEqual suffix byte: %w", err6)
			}
			for i := 0; i < numSuffixLengthBytes; i++ {
				f.suffixLengthBytes[i] = b
			}
		} else {
			if err6 := f.ite.in.ReadBytes(f.suffixLengthBytes[:numSuffixLengthBytes]); err6 != nil {
				return fmt.Errorf("blocktree intersect load suffix lengths: %w", err6)
			}
		}
		f.suffixLengthsReader.ResetWithSlice(f.suffixLengthBytes, 0, numSuffixLengthBytes)
	} else {
		code2, err2 := store.ReadVInt(f.ite.in)
		if err2 != nil {
			return fmt.Errorf("blocktree intersect load old suffix code: %w", err2)
		}
		f.isLeafBlock = (code2 & 1) != 0
		numBytes := int(code2 >> 1)
		if numBytes > len(f.suffixBytes) {
			f.suffixBytes = util.GrowExactByte(f.suffixBytes, util.Oversize(numBytes, 1))
		}
		if err3 := f.ite.in.ReadBytes(f.suffixBytes[:numBytes]); err3 != nil {
			return fmt.Errorf("blocktree intersect load suffix bytes (old): %w", err3)
		}
		f.suffixesReader.ResetWithSlice(f.suffixBytes, 0, numBytes)
		f.suffixLengthsReader = f.suffixesReader
	}

	// Read stats bytes.
	statLen, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("blocktree intersect load stats len: %w", err)
	}
	numStatBytes := int(statLen)
	if numStatBytes > len(f.statBytes) {
		f.statBytes = util.GrowExactByte(f.statBytes, util.Oversize(numStatBytes, 1))
	}
	if err2 := f.ite.in.ReadBytes(f.statBytes[:numStatBytes]); err2 != nil {
		return fmt.Errorf("blocktree intersect load stats: %w", err2)
	}
	f.statsReader.ResetWithSlice(f.statBytes, 0, numStatBytes)
	f.statsSingletonRunLength = 0
	f.metaDataUpto = 0

	if f.termState != nil {
		f.termState.TermBlockOrd = 0
	}
	f.nextEnt = 0

	// Read meta bytes.
	metaLen, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("blocktree intersect load meta len: %w", err)
	}
	numMetaBytes := int(metaLen)
	if numMetaBytes > len(f.bytes) {
		f.bytes = util.GrowExactByte(f.bytes, util.Oversize(numMetaBytes, 1))
	}
	if err2 := f.ite.in.ReadBytes(f.bytes[:numMetaBytes]); err2 != nil {
		return fmt.Errorf("blocktree intersect load meta: %w", err2)
	}
	f.bytesReader.ResetWithSlice(f.bytes, 0, numMetaBytes)

	if !f.isLastInFloor {
		f.fpEnd = f.ite.in.GetFilePointer()
	}
	return nil
}

// next advances to the next entry in this block.
// Returns true if the entry is a sub-block, false if it's a term.
//
// Port of IntersectTermsEnumFrame.next.
func (f *intersectTermsEnumFrame) next() bool {
	if f.isLeafBlock {
		f.nextLeaf()
		return false
	}
	return f.nextNonLeaf()
}

// nextLeaf advances within a leaf block (all entries are terms).
//
// Port of IntersectTermsEnumFrame.nextLeaf.
func (f *intersectTermsEnumFrame) nextLeaf() {
	f.nextEnt++
	v, _ := f.suffixLengthsReader.ReadVInt()
	f.suffix = int(v)
	f.startBytePos = f.suffixesReader.GetPosition()
	f.suffixesReader.SetPosition(f.startBytePos + f.suffix) //nolint:errcheck
}

// nextNonLeaf advances within a non-leaf block (entries are terms or sub-blocks).
// Returns true if the current entry is a sub-block.
//
// Port of IntersectTermsEnumFrame.nextNonLeaf.
func (f *intersectTermsEnumFrame) nextNonLeaf() bool {
	f.nextEnt++
	code, _ := f.suffixLengthsReader.ReadVInt()
	f.suffix = int(code >> 1)
	f.startBytePos = f.suffixesReader.GetPosition()
	f.suffixesReader.SetPosition(f.startBytePos + f.suffix) //nolint:errcheck
	if (code & 1) == 0 {
		// A normal term.
		if f.termState != nil {
			f.termState.TermBlockOrd++
		}
		return false
	}
	// A sub-block: make sub-FP absolute.
	v, _ := f.suffixLengthsReader.ReadVLong()
	f.lastSubFP = f.fp - int64(v)
	return true
}

// decodeMetaData lazily decodes per-term statistics and postings metadata for
// all terms up to getTermBlockOrd().
//
// Port of IntersectTermsEnumFrame.decodeMetaData.
func (f *intersectTermsEnumFrame) decodeMetaData() error {
	limit := f.getTermBlockOrd()
	absolute := f.metaDataUpto == 0

	for f.metaDataUpto < limit {
		if f.version >= VersionCompressedSuffixes {
			if f.statsSingletonRunLength > 0 {
				if f.termState != nil {
					f.termState.DocFreq = 1
					f.termState.TotalTermFreq = 1
				}
				f.statsSingletonRunLength--
			} else {
				token, _ := f.statsReader.ReadVInt()
				if (token & 1) == 1 {
					if f.termState != nil {
						f.termState.DocFreq = 1
						f.termState.TotalTermFreq = 1
					}
					f.statsSingletonRunLength = int(token >> 1)
				} else {
					if f.termState != nil {
						f.termState.DocFreq = int(token >> 1)
					}
					if f.ite.fr.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
						if f.termState != nil {
							f.termState.TotalTermFreq = int64(f.termState.DocFreq)
						}
					} else {
						v, _ := f.statsReader.ReadVLong()
						if f.termState != nil {
							f.termState.TotalTermFreq = int64(f.termState.DocFreq) + v
						}
					}
				}
			}
		} else {
			df, _ := f.statsReader.ReadVInt()
			if f.termState != nil {
				f.termState.DocFreq = int(df)
			}
			if f.ite.fr.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
				if f.termState != nil {
					f.termState.TotalTermFreq = int64(f.termState.DocFreq)
				}
			} else {
				v, _ := f.statsReader.ReadVLong()
				if f.termState != nil {
					f.termState.TotalTermFreq = int64(f.termState.DocFreq) + v
				}
			}
		}

		// Decode postings metadata.
		if f.termState != nil && f.ite.fr.parent != nil && f.ite.fr.parent.postingsReader != nil {
			if err := f.ite.fr.parent.postingsReader.DecodeTerm(
				f.bytesReader,
				f.ite.fr.fieldInfo,
				f.termState,
				absolute,
			); err != nil {
				return fmt.Errorf("blocktree intersect decodeMetaData: %w", err)
			}
		}

		f.metaDataUpto++
		absolute = false
	}
	if f.termState != nil {
		f.termState.TermBlockOrd = f.metaDataUpto
	}
	return nil
}
