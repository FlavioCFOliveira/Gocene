// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DeltaBaseTermStateSerializer is a TermState serializer which encodes each file
// pointer as a delta relative to a base file pointer. It differs from
// Lucene104PostingsWriter.encodeTerm which encodes each file pointer as a delta
// relative to the previous file pointer.
//
// It automatically sets the base file pointer to the first valid file pointer
// for doc start FP, pos start FP, pay start FP. These base file pointers have to
// be reset by the caller with ResetBaseStartFP before starting to write a new
// block.
//
// Mirrors
// org.apache.lucene.codecs.uniformsplit.DeltaBaseTermStateSerializer from
// Apache Lucene 10.5.0.
type DeltaBaseTermStateSerializer struct {
	baseDocStartFP int64
	basePosStartFP int64
	basePayStartFP int64
}

// NewDeltaBaseTermStateSerializer constructs a DeltaBaseTermStateSerializer.
// Mirrors the DeltaBaseTermStateSerializer() constructor
// (DeltaBaseTermStateSerializer.java:56), whose body calls resetBaseStartFP.
func NewDeltaBaseTermStateSerializer() *DeltaBaseTermStateSerializer {
	s := &DeltaBaseTermStateSerializer{}
	s.ResetBaseStartFP()
	return s
}

// ResetBaseStartFP resets the base file pointers to 0. This method has to be
// called before starting to write a new block. Mirrors
// DeltaBaseTermStateSerializer.resetBaseStartFP.
func (s *DeltaBaseTermStateSerializer) ResetBaseStartFP() {
	s.baseDocStartFP = 0
	s.basePosStartFP = 0
	s.basePayStartFP = 0
}

// GetBaseDocStartFP returns the base doc start file pointer. It is the file
// pointer of the first TermState written after ResetBaseStartFP is called.
func (s *DeltaBaseTermStateSerializer) GetBaseDocStartFP() int64 { return s.baseDocStartFP }

// GetBasePosStartFP returns the base position start file pointer. It is the
// file pointer of the first TermState written after ResetBaseStartFP is called.
func (s *DeltaBaseTermStateSerializer) GetBasePosStartFP() int64 { return s.basePosStartFP }

// GetBasePayStartFP returns the base payload start file pointer. It is the file
// pointer of the first TermState written after ResetBaseStartFP is called.
func (s *DeltaBaseTermStateSerializer) GetBasePayStartFP() int64 { return s.basePayStartFP }

// WriteTermState writes a BlockTermState to the provided output.
//
// Simpler variant of Lucene104PostingsWriter.encodeTerm(DataOutput, FieldInfo,
// BlockTermState, boolean). Mirrors
// DeltaBaseTermStateSerializer.writeTermState(DataOutput, FieldInfo,
// BlockTermState).
func (s *DeltaBaseTermStateSerializer) WriteTermState(out store.DataOutput, fieldInfo *index.FieldInfo, termState index.TermState) error {
	opts := fieldInfo.IndexOptions()
	hasFreqs := opts != index.IndexOptionsDocs
	hasPositions := opts >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	hasPayloads := fieldInfo.HasPayloads()

	// Mirrors "IntBlockTermState intTermState = (IntBlockTermState) termState".
	ts, ok := termState.(*codecs.IntBlockTermState)
	if !ok {
		return fmt.Errorf("DeltaBaseTermStateSerializer.WriteTermState: term state is %T, want *codecs.IntBlockTermState", termState)
	}

	if err := out.WriteVInt(int32(ts.DocFreq)); err != nil {
		return err
	}
	if hasFreqs {
		if err := out.WriteVLong(ts.TotalTermFreq - int64(ts.DocFreq)); err != nil {
			return err
		}
	}

	if ts.SingletonDocID != -1 {
		if err := out.WriteVInt(int32(ts.SingletonDocID)); err != nil {
			return err
		}
	} else {
		if s.baseDocStartFP == 0 {
			s.baseDocStartFP = ts.DocStartFP
		}
		if err := out.WriteVLong(ts.DocStartFP - s.baseDocStartFP); err != nil {
			return err
		}
	}

	if hasPositions {
		if s.basePosStartFP == 0 {
			s.basePosStartFP = ts.PosStartFP
		}
		if err := out.WriteVLong(ts.PosStartFP - s.basePosStartFP); err != nil {
			return err
		}
		if hasPayloads || hasOffsets {
			if s.basePayStartFP == 0 {
				s.basePayStartFP = ts.PayStartFP
			}
			if err := out.WriteVLong(ts.PayStartFP - s.basePayStartFP); err != nil {
				return err
			}
		}
		if ts.LastPosBlockOffset != -1 {
			if err := out.WriteVLong(ts.LastPosBlockOffset); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReadTermState reads a BlockTermState from the provided input.
//
// Simpler variant of Lucene104PostingsReader.decodeTerm(DataInput, FieldInfo,
// BlockTermState, boolean). reuse is a BlockTermState to reuse, or nil to
// create a new one. Mirrors DeltaBaseTermStateSerializer.readTermState.
func (s *DeltaBaseTermStateSerializer) ReadTermState(
	baseDocStartFP, basePosStartFP, basePayStartFP int64,
	in store.DataInput,
	fieldInfo *index.FieldInfo,
	reuse index.TermState,
) (index.TermState, error) {
	opts := fieldInfo.IndexOptions()
	hasFreqs := opts != index.IndexOptionsDocs
	hasPositions := opts >= index.IndexOptionsDocsAndFreqsAndPositions

	// Mirrors "reuse != null ? reset((IntBlockTermState) reuse) : new IntBlockTermState()".
	var ts *codecs.IntBlockTermState
	if reuse == nil {
		ts = codecs.NewIntBlockTermState()
	} else {
		r, ok := reuse.(*codecs.IntBlockTermState)
		if !ok {
			return nil, fmt.Errorf("DeltaBaseTermStateSerializer.ReadTermState: reuse is %T, want *codecs.IntBlockTermState", reuse)
		}
		ts = s.reset(r)
	}

	docFreq, err := in.ReadVInt()
	if err != nil {
		return nil, err
	}
	ts.DocFreq = int(docFreq)

	if hasFreqs {
		deltaTF, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.TotalTermFreq = int64(docFreq) + deltaTF
	} else {
		ts.TotalTermFreq = int64(docFreq)
	}

	if ts.DocFreq == 1 {
		singletonID, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		ts.SingletonDocID = int(singletonID)
	} else {
		deltaDocFP, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.DocStartFP = baseDocStartFP + deltaDocFP
	}

	if hasPositions {
		deltaPosFP, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.PosStartFP = basePosStartFP + deltaPosFP

		hasOffsets := opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
		if hasOffsets || fieldInfo.HasPayloads() {
			deltaPayFP, err := in.ReadVLong()
			if err != nil {
				return nil, err
			}
			ts.PayStartFP = basePayStartFP + deltaPayFP
		}

		// Lucene104PostingsFormat.BLOCK_SIZE, which is ForUtil.BLOCK_SIZE.
		if ts.TotalTermFreq > int64(codecs.ForUtilBlockSize) {
			lastPosOff, err := in.ReadVLong()
			if err != nil {
				return nil, err
			}
			ts.LastPosBlockOffset = lastPosOff
		}
	}

	return ts, nil
}

// reset clears every field of termState back to its constructor value.
//
// Mirrors DeltaBaseTermStateSerializer.reset(IntBlockTermState).
func (s *DeltaBaseTermStateSerializer) reset(termState *codecs.IntBlockTermState) *codecs.IntBlockTermState {
	// OrdTermState.
	termState.Ord = 0

	// BlockTermState.
	termState.DocFreq = 0
	termState.TotalTermFreq = 0
	termState.TermBlockOrd = 0
	termState.BlockFilePointer = 0

	// IntBlockTermState.
	termState.DocStartFP = 0
	termState.PosStartFP = 0
	termState.PayStartFP = 0
	termState.LastPosBlockOffset = -1
	termState.SingletonDocID = -1

	return termState
}

// deltaBaseTermStateSerializerRAMUsage renders the private static
// RAM_USAGE = RamUsageEstimator.shallowSizeOfInstance(
// DeltaBaseTermStateSerializer.class) (DeltaBaseTermStateSerializer.java:48).
var deltaBaseTermStateSerializerRAMUsage = util.ShallowSizeOf(DeltaBaseTermStateSerializer{})

// intBlockTermStateRAMUsage renders the private static
// INT_BLOCK_TERM_STATE_RAM_USAGE =
// RamUsageEstimator.shallowSizeOfInstance(IntBlockTermState.class)
// (DeltaBaseTermStateSerializer.java:50).
var intBlockTermStateRAMUsage = util.ShallowSizeOf(codecs.IntBlockTermState{})

// RamBytesUsed mirrors DeltaBaseTermStateSerializer.ramBytesUsed()
// (DeltaBaseTermStateSerializer.java:213), the Accountable method.
func (s *DeltaBaseTermStateSerializer) RamBytesUsed() int64 {
	return deltaBaseTermStateSerializerRAMUsage
}

// DeltaBaseTermStateSerializerRamBytesUsed returns the estimated RAM usage of
// the given TermState.
//
// Mirrors the static DeltaBaseTermStateSerializer.ramBytesUsed(TermState)
// (DeltaBaseTermStateSerializer.java:221). Java scopes the two ramBytesUsed
// overloads by receiver kind (instance versus static); Go has no static
// members, so the static one becomes this package-level function.
func DeltaBaseTermStateSerializerRamBytesUsed(termState index.TermState) int64 {
	if _, ok := termState.(*codecs.IntBlockTermState); ok {
		return intBlockTermStateRAMUsage
	}
	return util.ShallowSizeOf(termState)
}

// DeltaBaseTermStateSerializer implements Accountable
// (DeltaBaseTermStateSerializer.java:46).
var _ util.Accountable = (*DeltaBaseTermStateSerializer)(nil)
