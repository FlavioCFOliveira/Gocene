// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsReader.
package idversion

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IDVersionPostingsReader decodes IDVersion-format postings.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsReader.
type IDVersionPostingsReader struct{}

// Init validates the codec header written by IDVersionPostingsWriter.
func (r *IDVersionPostingsReader) Init(termsIn store.IndexInput, state *codecs.SegmentReadState) error {
	_, err := codecs.CheckIndexHeader(
		termsIn,
		TermsCodec,
		VersionStart,
		VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		return fmt.Errorf("IDVersionPostingsReader.Init: %w", err)
	}
	return nil
}

// NewTermState allocates an IDVersionTermState and registers it in the global
// sidecar registry so that EncodeTerm / DecodeTerm can access the IDVersion /
// DocID fields by pointer.
func (r *IDVersionPostingsReader) NewTermState() *codecs.BlockTermState {
	s := NewIDVersionTermState()
	globalTermStateRegistry.register(&s.BlockTermState)
	return &s.BlockTermState
}

// DecodeTerm reads per-term metadata from in, populating the IDVersion and
// DocID extra fields stored in the global sidecar registry.
//
// in must also implement store.VariableLengthInput (ReadVInt / ReadVLong).
// All real IndexInput implementations in Gocene embed VariableLengthInput, so
// the type assertion below succeeds in practice.
//
// Mirrors IDVersionPostingsReader.decodeTerm(DataInput, FieldInfo,
// BlockTermState, boolean).
func (r *IDVersionPostingsReader) DecodeTerm(
	in store.DataInput,
	_ *index.FieldInfo,
	termState *codecs.BlockTermState,
	absolute bool,
) error {
	vli, ok := in.(vLongReader)
	if !ok {
		return fmt.Errorf("IDVersionPostingsReader.DecodeTerm: IndexInput does not implement VariableLengthInput")
	}
	return r.decodeTermFromByteInput(vli, termState, absolute)
}

// decodeTermFromByteInput reads VInt/VLong from a DataInput that also
// satisfies store.VariableLengthInput. This is the common path called by both
// DecodeTerm (which receives a store.IndexInput wrapper) and
// DecodeTermFromBytesReader (called directly from the frame with a
// *store.ByteArrayDataInput to avoid the IndexInput wrapper overhead).
func (r *IDVersionPostingsReader) decodeTermFromByteInput(
	in vLongReader,
	termState *codecs.BlockTermState,
	absolute bool,
) error {
	extra := globalTermStateRegistry.lookup(termState)
	if extra == nil {
		return errors.New("IDVersionPostingsReader.DecodeTerm: unregistered BlockTermState")
	}

	docID, err := in.ReadVInt()
	if err != nil {
		return fmt.Errorf("IDVersionPostingsReader.DecodeTerm: read docID: %w", err)
	}
	extra.DocID = int(docID)

	if absolute {
		ver, err := in.ReadVLong()
		if err != nil {
			return fmt.Errorf("IDVersionPostingsReader.DecodeTerm: read absolute version: %w", err)
		}
		extra.IDVersion = ver
	} else {
		delta, err := in.ReadVLong()
		if err != nil {
			return fmt.Errorf("IDVersionPostingsReader.DecodeTerm: read version delta: %w", err)
		}
		extra.IDVersion += util.ZigZagDecodeInt64(delta)
	}
	return nil
}

// DecodeTermFromBytesReader is the package-internal variant called directly by
// IDVersionSegmentTermsEnumFrame to avoid wrapping *store.ByteArrayDataInput
// in a full store.IndexInput adapter (backlog #2692 defers that bridge).
func (r *IDVersionPostingsReader) DecodeTermFromBytesReader(
	in *store.ByteArrayDataInput,
	termState *codecs.BlockTermState,
	absolute bool,
) error {
	return r.decodeTermFromByteInput(in, termState, absolute)
}

// vLongReader is the minimal read surface required by decodeTermFromByteInput.
// Both *store.ByteArrayDataInput and any store.IndexInput satisfy this, so the
// helper works for both callers without requiring a full IndexInput.
type vLongReader interface {
	ReadVInt() (int32, error)
	ReadVLong() (int64, error)
}

// Postings returns a PostingsEnum for the given term state.
// If POSITIONS is requested, a SinglePostingsEnum is returned; otherwise a
// SingleDocsEnum.
//
// Mirrors IDVersionPostingsReader.postings(FieldInfo, BlockTermState,
// PostingsEnum, int).
func (r *IDVersionPostingsReader) Postings(
	_ *index.FieldInfo,
	termState *codecs.BlockTermState,
	reuse index.PostingsEnum,
	flags int,
) (index.PostingsEnum, error) {
	extra := globalTermStateRegistry.lookup(termState)
	if extra == nil {
		return nil, errors.New("IDVersionPostingsReader.Postings: unregistered BlockTermState")
	}

	const postingsFlagPositions = (1 << 3) | (1 << 4) // same as index package constants
	if flags&postingsFlagPositions != 0 {
		var posEnum *SinglePostingsEnum
		if p, ok := reuse.(*SinglePostingsEnum); ok {
			posEnum = p
		} else {
			posEnum = &SinglePostingsEnum{}
		}
		posEnum.Reset(extra.DocID, extra.IDVersion)
		return posEnum, nil
	}

	var docsEnum *SingleDocsEnum
	if d, ok := reuse.(*SingleDocsEnum); ok {
		docsEnum = d
	} else {
		docsEnum = &SingleDocsEnum{}
	}
	docsEnum.Reset(extra.DocID)
	return docsEnum, nil
}

// Impacts is not supported for IDVersion: callers use
// IDVersionSegmentTermsEnum.impacts instead.
func (r *IDVersionPostingsReader) Impacts(
	_ *index.FieldInfo,
	_ *codecs.BlockTermState,
	_ int,
) (any, error) {
	return nil, errors.New("IDVersionPostingsReader.Impacts: should never be called; IDVersionSegmentTermsEnum implements impacts directly")
}

// CheckIntegrity is a no-op (no separate postings file).
func (r *IDVersionPostingsReader) CheckIntegrity() error { return nil }

// Close is a no-op (no file handles to release).
func (r *IDVersionPostingsReader) Close() error { return nil }

// String returns a human-readable description.
func (r *IDVersionPostingsReader) String() string { return "IDVersionPostingsReader" }

var _ codecs.PostingsReaderBase = (*IDVersionPostingsReader)(nil)
