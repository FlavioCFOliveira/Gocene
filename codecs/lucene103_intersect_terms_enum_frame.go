// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// IntersectTermsEnumFrame is the Go port of
// org.apache.lucene.codecs.lucene103.blocktree.IntersectTermsEnumFrame from
// Apache Lucene 10.4.0. It is the per-block cursor pushed onto
// [Lucene103IntersectTermsEnum].stack as the automaton-driven walk descends
// the .tim block file.
//
// A frame owns three families of state:
//
//  1. Disk geometry: fp / fpOrig / fpEnd / isLastInFloor / numFollowFloorBlocks
//     / nextFloorLabel mirror the .tim floor-block layout. Together they let
//     LoadNextFloorBlock seek to the next floor sub-block without re-reading
//     the parent's floor data.
//  2. Per-block buffers: suffixBytes / suffixLengthBytes / statBytes / bytes
//     are reused across blocks, growing through [util.Oversize] exactly like
//     Lucene's ArrayUtil.oversize. The four ByteArrayDataInput readers
//     (suffixesReader / suffixLengthsReader / statsReader / bytesReader) are
//     reset onto those buffers each Load().
//  3. Automaton position: state / lastState / transition / transitionIndex /
//     transitionCount mirror the same fields in Java. SetState rebinds the
//     transition iterator at frame entry; the parent enum reads transition.min
//     while walking floor blocks to prune sub-blocks beyond the active range.
//
// Sprint 56 promotes this type from a stub-only struct (the previous
// intersectTermsEnumFrame in [Lucene103IntersectTermsEnum]) to the full
// surface declared by Lucene's reference. Two pieces remain deferred to
// backlog task #2692:
//
//   - DecodeMetaData stops short of forwarding the per-term metadata bytes to
//     PostingsReaderBase.DecodeTerm. The SPI takes a [store.IndexInput] while
//     the Java frame holds a ByteArrayDataInput; bridging the two requires the
//     "wrap ByteArrayDataInput as IndexInput" shim that lands with #2692.
//     DocFreq / TotalTermFreq decoding is fully ported; only the postings
//     metadata pass is skipped.
//   - The frame does not allocate the larger 5-buffer set Lucene uses for
//     SIMD-aligned suffix decoding (suffixLengthBytes is allocated at the
//     base 32-byte default). The actual decompression already lands via
//     [CompressionAlgorithm.Read]; the in-memory shape of the auxiliary
//     buffers is identical to Lucene's defaults.
//
// IntersectTermsEnumFrame is single-threaded — the owning enum guards the
// transitions. The exported fields exist so the future deep-port can drive
// the cursor from the enum without going through accessors; treat them as
// package-internal until #2692 lands.
type IntersectTermsEnumFrame struct {
	// Ord is the frame's position in the owning enum's stack (0 == root).
	// Mirrors IntersectTermsEnumFrame.ord in Java.
	Ord int

	// FP is the file pointer of the current floor sub-block.
	// FPOrig is the file pointer of the parent floor block (the seek target
	// stored in the .tip trie). FPEnd is the end of the parent block.
	//
	// (FP == FPOrig) marks the parent's first emission; (FPEnd != 0)
	// signals "more sub-blocks follow inline".
	FP     int64
	FPOrig int64
	FPEnd  int64

	// LastSubFP is the file pointer of the most recently emitted sub-block
	// label, used by the parent enum to push a child frame.
	LastSubFP int64

	// State is the automaton state at the entry of the frame.
	// LastState is the state recorded just before the last suffix byte was
	// consumed; the parent uses it when popping the child frame to restore
	// the resume point.
	State     int
	LastState int

	// MetaDataUpto tracks how many term entries inside the block have had
	// their stats + postings metadata decoded.
	MetaDataUpto int

	// SuffixBytes holds the decoded (post-CompressionAlgorithm) suffix
	// bytes for every entry in the block. SuffixesReader is a
	// ByteArrayDataInput rebound to SuffixBytes after each Load.
	SuffixBytes    []byte
	SuffixesReader *store.ByteArrayDataInput

	// SuffixLengthBytes / SuffixLengthsReader hold the per-entry suffix
	// lengths (VInt-encoded for non-leaf blocks, possibly run-length
	// compressed when allEqual is set).
	SuffixLengthBytes   []byte
	SuffixLengthsReader *store.ByteArrayDataInput

	// StatBytes / StatsReader hold the per-term DocFreq / TotalTermFreq
	// codes consumed by DecodeMetaData. StatsSingletonRunLength tracks the
	// run-length compression of (DocFreq == 1, TotalTermFreq == 1) terms
	// emitted by the writer's encodeStats fast-path.
	StatBytes               []byte
	StatsSingletonRunLength int
	StatsReader             *store.ByteArrayDataInput

	// FloorDataPos / floorDataReader cache the floor-data IndexInput shared
	// with the owning TrieReader. The frame seeks back to FloorDataPos at
	// the top of LoadNextFloorBlock so several frames can share one reader
	// without trampling each other's file pointer.
	FloorDataPos    int64
	FloorDataReader store.IndexInput

	// Prefix is the depth in the trie (number of bytes shared by every
	// term in the block). Mirrors IntersectTermsEnumFrame.prefix.
	Prefix int

	// EntCount is the number of entries (term + sub-block markers) in the
	// current block. NextEnt is the index of the next entry to decode.
	EntCount int
	NextEnt  int

	// IsLastInFloor is true when this block is either not a floor block,
	// or, it is the last sub-block of a floor block.
	IsLastInFloor bool

	// IsLeafBlock is true when every entry in the block is a term (no
	// sub-block markers); leaf blocks compress the per-entry header to a
	// bare VInt suffix length.
	IsLeafBlock bool

	// NumFollowFloorBlocks counts the floor sub-blocks not yet visited.
	// NextFloorLabel is the label of the next floor sub-block, in 0..256
	// (256 == "no more sub-blocks").
	NumFollowFloorBlocks int
	NextFloorLabel       int

	// Transition is the per-frame Transition object the automaton walks
	// within this block. Lucene preallocates one per frame so the inner
	// loop avoids re-allocation; the Go port mirrors that.
	Transition *automaton.Transition

	// TransitionIndex / TransitionCount mirror the same names in Java.
	// SetState populates them from the automaton's getNumTransitions.
	TransitionIndex int
	TransitionCount int

	// Node is the cached TrieReader.Node this frame anchors at. The owning
	// enum populates it from its nodes[] cache on push.
	Node *TrieNode

	// TermState is the per-term postings metadata buffer. Sprint 56
	// allocates a fresh BlockTermState; the deferred DecodeTerm pass
	// (backlog #2692) will populate it via PostingsReaderBase.
	TermState *BlockTermState

	// Bytes / BytesReader hold the per-term postings metadata blob that
	// PostingsReaderBase.DecodeTerm consumes during DecodeMetaData.
	Bytes       []byte
	BytesReader *store.ByteArrayDataInput

	// StartBytePos / Suffix mirror the same names in Java. StartBytePos
	// is the offset into SuffixBytes where the current entry's suffix
	// starts; Suffix is the length of that suffix. NextLeaf / NextNonLeaf
	// populate both during enumeration.
	StartBytePos int
	Suffix       int

	// ite is the back-pointer to the owning enum. The Java implementation
	// uses the enclosing-instance reference for ite.fr (fieldReader),
	// ite.in (terms IndexInput), ite.automaton, ite.runAutomaton, and so
	// on. The Go port stores the pointer explicitly.
	ite *Lucene103IntersectTermsEnum
}

// NewIntersectTermsEnumFrame allocates a frame for the given enum at the
// given stack ordinal. Mirrors the Java constructor
// IntersectTermsEnumFrame(IntersectTermsEnum, int).
//
// The constructor allocates SuffixBytes / SuffixLengthBytes / StatBytes /
// Bytes at the same baseline sizes the Lucene reference uses (128 / 32 /
// 64 / 32). Each buffer grows via [util.Oversize] inside Load when the
// on-disk block exceeds the baseline.
func NewIntersectTermsEnumFrame(ite *Lucene103IntersectTermsEnum, ord int) (*IntersectTermsEnumFrame, error) {
	if ite == nil {
		return nil, errors.New("NewIntersectTermsEnumFrame: ite must not be nil")
	}

	f := &IntersectTermsEnumFrame{
		Ord:                 ord,
		SuffixBytes:         make([]byte, 128),
		SuffixesReader:      store.NewByteArrayDataInput(nil),
		SuffixLengthBytes:   make([]byte, 32),
		SuffixLengthsReader: store.NewByteArrayDataInput(nil),
		StatBytes:           make([]byte, 64),
		StatsReader:         store.NewByteArrayDataInput(nil),
		Bytes:               make([]byte, 32),
		BytesReader:         store.NewByteArrayDataInput(nil),
		Transition:          automaton.NewTransition(),
		ite:                 ite,
	}

	// Lucene asks the postings reader for a fresh BlockTermState. When the
	// owning enum has been constructed without a PostingsReaderBase (Sprint
	// 56 ships a stub Intersect path; see backlog #2692), fall back to a
	// vanilla BlockTermState so the frame is still safe to construct.
	if ite.fr != nil && ite.fr.parent != nil && ite.fr.parent.postingsReader != nil {
		f.TermState = ite.fr.parent.postingsReader.NewTermState()
	} else {
		f.TermState = NewBlockTermState()
	}
	f.TermState.TotalTermFreq = -1
	return f, nil
}

// LoadNextFloorBlock advances the frame to the next floor sub-block whose
// label lies inside the active transition. Mirrors
// IntersectTermsEnumFrame.loadNextFloorBlock.
//
// Pre-conditions: NumFollowFloorBlocks > 0 and FloorDataReader is the
// IndexInput returned by TrieReader.FloorData on the parent node.
func (f *IntersectTermsEnumFrame) LoadNextFloorBlock() error {
	if f.NumFollowFloorBlocks <= 0 {
		return fmt.Errorf("IntersectTermsEnumFrame.LoadNextFloorBlock: nextFloorLabel=%d", f.NextFloorLabel)
	}
	if f.FloorDataReader == nil {
		return errors.New("IntersectTermsEnumFrame.LoadNextFloorBlock: floorDataReader is nil")
	}
	if err := f.FloorDataReader.SetPosition(f.FloorDataPos); err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.LoadNextFloorBlock: seek floor data: %w", err)
	}
	for {
		delta, err := store.ReadVLong(f.FloorDataReader)
		if err != nil {
			return fmt.Errorf("IntersectTermsEnumFrame.LoadNextFloorBlock: read sub-block delta: %w", err)
		}
		// The writer encodes (delta << 1) | hasTerms; the read side only
		// needs the file-pointer delta, so the LSB is shifted out.
		f.FP = f.FPOrig + int64(uint64(delta)>>1)
		f.NumFollowFloorBlocks--
		if f.NumFollowFloorBlocks != 0 {
			b, err := f.FloorDataReader.ReadByte()
			if err != nil {
				return fmt.Errorf("IntersectTermsEnumFrame.LoadNextFloorBlock: read next floor label: %w", err)
			}
			f.NextFloorLabel = int(b) & 0xff
		} else {
			f.NextFloorLabel = 256
		}
		if f.NumFollowFloorBlocks == 0 || f.NextFloorLabel > f.Transition.Min {
			break
		}
	}
	if err := f.Load(nil); err != nil {
		return err
	}
	f.FloorDataPos = f.FloorDataReader.GetFilePointer()
	return nil
}

// SetState rebinds the per-frame Transition iterator to the new automaton
// state. Mirrors IntersectTermsEnumFrame.setState.
//
// When the state has no outgoing transitions, Lucene poisons transition.Min
// and transition.Max with -1 so the "label < min" check never falsely
// triggers and the parent's "label > max" check pops the frame
// immediately on the next step. The Go port preserves both sentinels.
func (f *IntersectTermsEnumFrame) SetState(state int) {
	f.State = state
	f.TransitionIndex = 0
	if f.ite == nil || f.ite.automaton == nil {
		// Defensive: with the stub enum (Sprint 56 ships no automaton
		// triple), there is no automaton to interrogate. Leave the
		// transition empty so the deferred deep-port surfaces the bug if
		// it ever forgets to wire the triple.
		f.TransitionCount = 0
		f.Transition.Min = -1
		f.Transition.Max = -1
		return
	}
	f.TransitionCount = f.ite.automaton.GetNumTransitions(state)
	if f.TransitionCount != 0 {
		f.ite.automaton.InitTransition(state, f.Transition)
		f.ite.automaton.GetNextTransition(f.Transition)
	} else {
		// Must set min to -1 so the "label < min" check never falsely
		// triggers, and max to -1 so we immediately realise we need to
		// step to the next transition and pop this frame.
		f.Transition.Min = -1
		f.Transition.Max = -1
	}
}

// Load materialises the on-disk block this frame points to into the
// per-frame buffers. Mirrors IntersectTermsEnumFrame.load.
//
// node is non-nil on the first push of a floor block — Load then reads the
// floor-data header from the parent's TrieReader and (when the parent
// state is non-accepting with outgoing transitions) skips floor sub-blocks
// whose labels lie before the active transition. node is nil on subsequent
// pushes within the same floor sequence; LoadNextFloorBlock handles those.
func (f *IntersectTermsEnumFrame) Load(node *TrieNode) error {
	if f.ite == nil || f.ite.in == nil {
		return errors.New("IntersectTermsEnumFrame.Load: terms IndexInput is nil")
	}

	if node != nil && node.IsFloor() {
		// This block is the first one in a possible sequence of floor
		// blocks corresponding to a single seek point from the .tip trie.
		reader, err := f.ite.trieReader.FloorData(node)
		if err != nil {
			return fmt.Errorf("IntersectTermsEnumFrame.Load: open floor data: %w", err)
		}
		f.FloorDataReader = reader

		numFollow, err := store.ReadVInt(reader)
		if err != nil {
			return fmt.Errorf("IntersectTermsEnumFrame.Load: read numFollowFloorBlocks: %w", err)
		}
		f.NumFollowFloorBlocks = int(numFollow)
		b, err := reader.ReadByte()
		if err != nil {
			return fmt.Errorf("IntersectTermsEnumFrame.Load: read nextFloorLabel: %w", err)
		}
		f.NextFloorLabel = int(b) & 0xff

		// If current state is not accepting and has outgoing transitions,
		// we must process the first sub-block in case it has an empty
		// suffix: maybe skip floor blocks.
		if f.ite.runAutomaton != nil && !f.ite.runAutomaton.IsAccept(f.State) && f.TransitionCount != 0 {
			if f.TransitionIndex != 0 {
				return fmt.Errorf("IntersectTermsEnumFrame.Load: transitionIndex=%d", f.TransitionIndex)
			}
			for f.NumFollowFloorBlocks != 0 && f.NextFloorLabel <= f.Transition.Min {
				delta, err := store.ReadVLong(reader)
				if err != nil {
					return fmt.Errorf("IntersectTermsEnumFrame.Load: read sub-block delta: %w", err)
				}
				f.FP = f.FPOrig + int64(uint64(delta)>>1)
				f.NumFollowFloorBlocks--
				if f.NumFollowFloorBlocks != 0 {
					b, err := reader.ReadByte()
					if err != nil {
						return fmt.Errorf("IntersectTermsEnumFrame.Load: read next floor label: %w", err)
					}
					f.NextFloorLabel = int(b) & 0xff
				} else {
					f.NextFloorLabel = 256
				}
			}
		}
		f.FloorDataPos = reader.GetFilePointer()
	}

	if err := f.ite.in.SetPosition(f.FP); err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: seek to fp=%d: %w", f.FP, err)
	}
	code, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: read block code: %w", err)
	}
	f.EntCount = int(uint32(code) >> 1)
	if f.EntCount <= 0 {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: entCount=%d must be > 0", f.EntCount)
	}
	f.IsLastInFloor = (code & 1) != 0

	// Term suffixes: a VLong header packs (numSuffixBytes << 3) | (isLeaf << 2) | compressionAlgCode.
	codeL, err := store.ReadVLong(f.ite.in)
	if err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: read suffix code: %w", err)
	}
	f.IsLeafBlock = (codeL & 0x04) != 0
	numSuffixBytes := int(uint64(codeL) >> 3)
	if cap(f.SuffixBytes) < numSuffixBytes {
		f.SuffixBytes = make([]byte, util.Oversize(numSuffixBytes, 1))
	}
	alg, err := CompressionAlgorithmByCode(int(codeL) & 0x03)
	if err != nil {
		return index.NewCorruptIndexExceptionWithCause(err.Error(), "lucene103 terms", err)
	}
	// CompressionAlgorithm.Read needs both DataInput and VInt helpers;
	// IndexInput embeds DataInput but does not require VariableLengthInput
	// at the interface level. The production [store.BufferedIndexInput]
	// (and ByteBuffersIndexInput) both satisfy it directly, so the type
	// assertion succeeds on every real on-disk reader. The fallback path
	// would only trip in tests that pass a hand-rolled IndexInput stub.
	compressIn, ok := f.ite.in.(CompressionInput)
	if !ok {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: terms IndexInput %T does not support VInt reads", f.ite.in)
	}
	if err := alg.Read(compressIn, f.SuffixBytes, numSuffixBytes); err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: decompress suffixes: %w", err)
	}
	f.SuffixesReader.ResetWithSlice(f.SuffixBytes, 0, numSuffixBytes)

	// Suffix-length bytes: same VInt-packed (allEqual << 0) | (length << 1) header.
	numSuffixLengthBytes, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: read suffix-length code: %w", err)
	}
	allEqual := (numSuffixLengthBytes & 0x01) != 0
	numLenBytes := int(uint32(numSuffixLengthBytes) >> 1)
	if cap(f.SuffixLengthBytes) < numLenBytes {
		f.SuffixLengthBytes = make([]byte, util.Oversize(numLenBytes, 1))
	}
	if allEqual {
		b, err := f.ite.in.ReadByte()
		if err != nil {
			return fmt.Errorf("IntersectTermsEnumFrame.Load: read constant suffix length: %w", err)
		}
		buf := f.SuffixLengthBytes[:numLenBytes]
		for i := range buf {
			buf[i] = b
		}
	} else {
		if err := f.ite.in.ReadBytes(f.SuffixLengthBytes[:numLenBytes]); err != nil {
			return fmt.Errorf("IntersectTermsEnumFrame.Load: read suffix lengths: %w", err)
		}
	}
	f.SuffixLengthsReader.ResetWithSlice(f.SuffixLengthBytes, 0, numLenBytes)

	// Stats: per-term DocFreq / TotalTermFreq codes.
	numBytes, err := store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: read stats length: %w", err)
	}
	if cap(f.StatBytes) < int(numBytes) {
		f.StatBytes = make([]byte, util.Oversize(int(numBytes), 1))
	}
	if err := f.ite.in.ReadBytes(f.StatBytes[:numBytes]); err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: read stats: %w", err)
	}
	f.StatsReader.ResetWithSlice(f.StatBytes, 0, int(numBytes))
	f.StatsSingletonRunLength = 0
	f.MetaDataUpto = 0

	if f.TermState != nil {
		f.TermState.TermBlockOrd = 0
	}
	f.NextEnt = 0

	// Metadata: postings-side metadata blob (decoded lazily inside DecodeMetaData).
	numBytes, err = store.ReadVInt(f.ite.in)
	if err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: read metadata length: %w", err)
	}
	if cap(f.Bytes) < int(numBytes) {
		f.Bytes = make([]byte, util.Oversize(int(numBytes), 1))
	}
	if err := f.ite.in.ReadBytes(f.Bytes[:numBytes]); err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.Load: read metadata: %w", err)
	}
	f.BytesReader.ResetWithSlice(f.Bytes, 0, int(numBytes))

	if !f.IsLastInFloor {
		// Sub-blocks of a single floor block are written one after another;
		// tail-recurse by snapshotting the end pointer so the parent can
		// resume on next floor advance.
		f.FPEnd = f.ite.in.GetFilePointer()
	}
	return nil
}

// Next decodes the next entry in the block, returning true when the entry
// is a sub-block (the parent enum then pushes a new frame at LastSubFP)
// and false when the entry is a regular term.
//
// Mirrors IntersectTermsEnumFrame.next.
func (f *IntersectTermsEnumFrame) Next() (bool, error) {
	if f.IsLeafBlock {
		return false, f.NextLeaf()
	}
	return f.NextNonLeaf()
}

// NextLeaf decodes the next term entry from a leaf block. Mirrors
// IntersectTermsEnumFrame.nextLeaf — leaf blocks store one VInt suffix
// length per entry with no sub-block markers, so the decoder is a tight
// VInt + skip pair.
func (f *IntersectTermsEnumFrame) NextLeaf() error {
	if f.NextEnt < 0 || f.NextEnt >= f.EntCount {
		return fmt.Errorf("IntersectTermsEnumFrame.NextLeaf: nextEnt=%d entCount=%d fp=%d", f.NextEnt, f.EntCount, f.FP)
	}
	f.NextEnt++
	suffix, err := f.SuffixLengthsReader.ReadVInt()
	if err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.NextLeaf: read suffix length: %w", err)
	}
	f.Suffix = int(suffix)
	f.StartBytePos = f.SuffixesReader.GetPosition()
	if err := f.SuffixesReader.SetPosition(f.StartBytePos + f.Suffix); err != nil {
		return fmt.Errorf("IntersectTermsEnumFrame.NextLeaf: skip suffix bytes: %w", err)
	}
	return nil
}

// NextNonLeaf decodes the next entry from a non-leaf block. The entry's
// header packs (subBlock << 0) | (suffixLength << 1); when the LSB is
// clear the entry is a regular term and the parent enum can apply the
// automaton to the suffix, when it is set the entry is a sub-block marker
// and the parent enum pushes a child frame at LastSubFP.
//
// Mirrors IntersectTermsEnumFrame.nextNonLeaf.
func (f *IntersectTermsEnumFrame) NextNonLeaf() (bool, error) {
	if f.NextEnt < 0 || f.NextEnt >= f.EntCount {
		return false, fmt.Errorf("IntersectTermsEnumFrame.NextNonLeaf: nextEnt=%d entCount=%d fp=%d", f.NextEnt, f.EntCount, f.FP)
	}
	f.NextEnt++
	code, err := f.SuffixLengthsReader.ReadVInt()
	if err != nil {
		return false, fmt.Errorf("IntersectTermsEnumFrame.NextNonLeaf: read suffix code: %w", err)
	}
	f.Suffix = int(uint32(code) >> 1)
	f.StartBytePos = f.SuffixesReader.GetPosition()
	if err := f.SuffixesReader.SetPosition(f.StartBytePos + f.Suffix); err != nil {
		return false, fmt.Errorf("IntersectTermsEnumFrame.NextNonLeaf: skip suffix bytes: %w", err)
	}
	if code&1 == 0 {
		// A normal term.
		if f.TermState != nil {
			f.TermState.TermBlockOrd++
		}
		return false, nil
	}
	// A sub-block marker: make sub-FP absolute.
	delta, err := f.SuffixLengthsReader.ReadVLong()
	if err != nil {
		return false, fmt.Errorf("IntersectTermsEnumFrame.NextNonLeaf: read sub-block FP delta: %w", err)
	}
	f.LastSubFP = f.FP - delta
	return true, nil
}

// GetTermBlockOrd returns the term's ordinal inside the block, suitable
// for use as the metadata-decode high-water mark. Leaf blocks rely on
// NextEnt directly (every entry is a term); non-leaf blocks rely on the
// per-term TermBlockOrd counter that NextNonLeaf maintains.
//
// Mirrors IntersectTermsEnumFrame.getTermBlockOrd.
func (f *IntersectTermsEnumFrame) GetTermBlockOrd() int {
	if f.IsLeafBlock {
		return f.NextEnt
	}
	if f.TermState == nil {
		return 0
	}
	return f.TermState.TermBlockOrd
}

// DecodeMetaData catches up on metadata decode for every term up to (and
// including) the current cursor position. Mirrors
// IntersectTermsEnumFrame.decodeMetaData.
//
// Stats decoding is fully ported: the (DocFreq, TotalTermFreq) pair is
// decoded inline, with the writer's run-length compression of singleton
// terms (DocFreq == 1, TotalTermFreq == 1) honoured via
// StatsSingletonRunLength. Postings-side metadata decoding (the call to
// PostingsReaderBase.DecodeTerm) is deferred to backlog #2692 because the
// SPI requires a [store.IndexInput] while the frame holds a
// ByteArrayDataInput; the bridging shim lands with the deep port.
func (f *IntersectTermsEnumFrame) DecodeMetaData() error {
	if f.TermState == nil {
		return errors.New("IntersectTermsEnumFrame.DecodeMetaData: termState is nil")
	}
	limit := f.GetTermBlockOrd()
	if limit <= 0 {
		return fmt.Errorf("IntersectTermsEnumFrame.DecodeMetaData: limit=%d must be > 0", limit)
	}
	absolute := f.MetaDataUpto == 0
	for f.MetaDataUpto < limit {
		// stats
		if f.StatsSingletonRunLength > 0 {
			f.TermState.DocFreq = 1
			f.TermState.TotalTermFreq = 1
			f.StatsSingletonRunLength--
		} else {
			token, err := f.StatsReader.ReadVInt()
			if err != nil {
				return fmt.Errorf("IntersectTermsEnumFrame.DecodeMetaData: read stats token: %w", err)
			}
			if token&1 == 1 {
				f.TermState.DocFreq = 1
				f.TermState.TotalTermFreq = 1
				f.StatsSingletonRunLength = int(uint32(token) >> 1)
			} else {
				f.TermState.DocFreq = int(uint32(token) >> 1)
				if f.ite != nil && f.ite.fr != nil && f.ite.fr.fieldInfo != nil &&
					f.ite.fr.fieldInfo.IndexOptions() == index.IndexOptionsDocs {
					f.TermState.TotalTermFreq = int64(f.TermState.DocFreq)
				} else {
					delta, err := f.StatsReader.ReadVLong()
					if err != nil {
						return fmt.Errorf("IntersectTermsEnumFrame.DecodeMetaData: read totalTermFreq delta: %w", err)
					}
					f.TermState.TotalTermFreq = int64(f.TermState.DocFreq) + delta
				}
			}
		}
		// metadata (postings-side): deferred to backlog #2692. See type
		// docstring for the SPI mismatch that blocks the call here.
		// _ = absolute below keeps the variable live until the bridging
		// shim lands so the deep port does not need to re-thread it.
		_ = absolute

		f.MetaDataUpto++
		absolute = false
	}
	f.TermState.TermBlockOrd = f.MetaDataUpto
	return nil
}
