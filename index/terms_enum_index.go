// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsEnumIndex wraps a TermsEnum together with an integer identifier and
// caches the comparable big-endian prefix of the current term so that
// multi-way merges can compare terms with a single 64-bit comparison in the
// common case. All cursor-moving operations on the wrapped TermsEnum must go
// through this wrapper so the cached prefix stays in sync.
//
// Port of Apache Lucene 10.4.0 org.apache.lucene.index.TermsEnumIndex
// (package-private). Lucene exposes the type to a handful of index-internal
// callers (e.g. MultiTermsEnum, MergedPostingsEnum); Gocene matches the
// surface API and exports the names so they can be reused across sub-packages
// during the rest of the port.
//
// Two divergences from the Java original are intentional and documented
// inline:
//
//   - SeekCeil. Gocene's TermsEnum.SeekCeil returns *Term, not SeekStatus.
//     This wrapper reconstructs SeekStatus by comparing the returned term to
//     the requested one, preserving the Lucene contract for callers.
//   - SeekExactOrd. Gocene's TermsEnum interface omits seekExact(long ord);
//     the wrapper uses an optional ordinalSeeker interface and returns an
//     error if the underlying enumerator does not implement it.
type TermsEnumIndex struct {
	// SubIndex is the stable identifier supplied at construction time.
	// Equivalent to the package-private final field subIndex in Java.
	SubIndex int

	// TermsEnum is the wrapped enumerator. Exposed (like Java's
	// package-private field) so the rare caller that needs to query
	// non-cursor methods (DocFreq, Postings, ...) can do so directly.
	// Callers MUST NOT advance it without going through the wrapper.
	TermsEnum TermsEnum

	currentTerm        *util.BytesRef
	currentTermPrefix8 uint64
}

// ordinalSeeker is implemented by concrete TermsEnums that support
// seekExact(long ord) (e.g. doc-values backed enumerators). The wrapper
// performs a type assertion on demand instead of forcing every TermsEnum to
// satisfy this contract.
type ordinalSeeker interface {
	SeekExactOrd(ord int64) error
}

// errOrdinalSeekUnsupported is returned by SeekExactOrd when the wrapped
// TermsEnum does not implement ordinalSeeker.
var errOrdinalSeekUnsupported = errors.New(
	"TermsEnumIndex: wrapped TermsEnum does not support SeekExactOrd")

// EmptyTermsEnumIndexArray is the canonical empty slice, matching Java's
// EMPTY_ARRAY constant. Used by callers that need a sentinel zero-length
// allocation.
var EmptyTermsEnumIndexArray = []*TermsEnumIndex{}

// Prefix8ToComparableUnsignedLong copies the first 8 bytes of term as a
// comparable big-endian unsigned 64-bit integer. Missing bytes are padded
// with zeros, which means two distinct terms can map to the same value
// (e.g. [1,0] and [1] both map to 0x0100000000000000). Callers must fall
// back to a full byte-wise compare when the prefixes are equal.
//
// Port of TermsEnumIndex.prefix8ToComparableUnsignedLong.
func Prefix8ToComparableUnsignedLong(term *util.BytesRef) uint64 {
	if term == nil || term.Length == 0 {
		return 0
	}

	const (
		longBytes  = 8
		intBytes   = 4
		shortBytes = 2
	)

	buf := term.Bytes[term.Offset : term.Offset+term.Length]
	if term.Length >= longBytes {
		return binary.BigEndian.Uint64(buf[:longBytes])
	}

	var l uint64
	var o int
	if term.Length >= intBytes {
		l = uint64(binary.BigEndian.Uint32(buf[:intBytes]))
		o = intBytes
	}
	if o+shortBytes <= term.Length {
		l = (l << 16) | uint64(binary.BigEndian.Uint16(buf[o:o+shortBytes]))
		o += shortBytes
	}
	if o < term.Length {
		l = (l << 8) | uint64(buf[o])
	}
	// Left-shift the missing tail bytes so the value is right-padded with
	// zeros, matching Java's "l <<= (Long.BYTES - term.length) << 3".
	l <<= uint((longBytes - term.Length) * 8)
	return l
}

// NewTermsEnumIndex wraps termsEnum with the supplied subIndex identifier.
// The wrapper starts positioned before the first term; Term returns nil until
// Next or one of the Seek* methods is called.
func NewTermsEnumIndex(termsEnum TermsEnum, subIndex int) *TermsEnumIndex {
	return &TermsEnumIndex{
		SubIndex:  subIndex,
		TermsEnum: termsEnum,
	}
}

// Term returns the current term, or nil if the enumerator has not yet been
// positioned or is exhausted.
func (t *TermsEnumIndex) Term() *util.BytesRef {
	return t.currentTerm
}

// setTerm updates the cached current term and refreshes the comparable
// prefix. Mirrors the private setTerm helper in Java.
func (t *TermsEnumIndex) setTerm(term *util.BytesRef) {
	t.currentTerm = term
	if term == nil {
		t.currentTermPrefix8 = 0
	} else {
		t.currentTermPrefix8 = Prefix8ToComparableUnsignedLong(term)
	}
}

// Next advances the wrapped TermsEnum and returns the new term, or nil at
// end-of-stream.
func (t *TermsEnumIndex) Next() (*util.BytesRef, error) {
	term, err := t.TermsEnum.Next()
	if err != nil {
		return nil, err
	}
	br := termsEnumIndexBytes(term)
	t.setTerm(br)
	return br, nil
}

// SeekCeil seeks to term or, if absent, to the next term, and reports the
// outcome via SeekStatus.
//
// Divergence from Lucene: Gocene's TermsEnum.SeekCeil returns *Term (or nil
// at end). We reconstruct SeekStatusFound vs SeekStatusNotFound by comparing
// the returned term to the requested one.
func (t *TermsEnumIndex) SeekCeil(target *util.BytesRef) (SeekStatus, error) {
	field := termFieldFromCurrent(t.TermsEnum)
	got, err := t.TermsEnum.SeekCeil(NewTermFromBytesRef(field, target))
	if err != nil {
		return SeekStatusEnd, err
	}
	if got == nil {
		t.setTerm(nil)
		return SeekStatusEnd, nil
	}
	t.setTerm(got.GetBytesRef())
	if bytes.Equal(t.currentTerm.ValidBytes(), target.ValidBytes()) {
		return SeekStatusFound, nil
	}
	return SeekStatusNotFound, nil
}

// SeekExact seeks to term exactly. Returns true if the term was found.
func (t *TermsEnumIndex) SeekExact(target *util.BytesRef) (bool, error) {
	field := termFieldFromCurrent(t.TermsEnum)
	found, err := t.TermsEnum.SeekExact(NewTermFromBytesRef(field, target))
	if err != nil {
		return false, err
	}
	if found {
		t.setTerm(t.TermsEnum.Term().GetBytesRef())
	} else {
		t.setTerm(nil)
	}
	return found, nil
}

// SeekExactOrd seeks to the term at ord. The wrapped TermsEnum must
// implement ordinalSeeker; otherwise an error is returned (Gocene's TermsEnum
// interface omits seekExact(long ord), see type-level docs).
func (t *TermsEnumIndex) SeekExactOrd(ord int64) error {
	seeker, ok := t.TermsEnum.(ordinalSeeker)
	if !ok {
		return fmt.Errorf("%w: subIndex=%d", errOrdinalSeekUnsupported, t.SubIndex)
	}
	if err := seeker.SeekExactOrd(ord); err != nil {
		return err
	}
	t.setTerm(t.TermsEnum.Term().GetBytesRef())
	return nil
}

// Reset copies the cursor state from other into t. The wrapped TermsEnum
// reference is also reassigned, matching Java's behaviour.
func (t *TermsEnumIndex) Reset(other *TermsEnumIndex) {
	t.TermsEnum = other.TermsEnum
	t.currentTerm = other.currentTerm
	t.currentTermPrefix8 = other.currentTermPrefix8
}

// CompareTermTo orders this wrapper's current term against that's. The
// comparison is unsigned lexicographic, matching Java's
// Arrays.compareUnsigned. The cached 8-byte prefixes are compared first to
// short-circuit the common case.
func (t *TermsEnumIndex) CompareTermTo(that *TermsEnumIndex) int {
	if t.currentTermPrefix8 != that.currentTermPrefix8 {
		if t.currentTermPrefix8 < that.currentTermPrefix8 {
			return -1
		}
		return 1
	}
	return bytes.Compare(t.currentTerm.ValidBytes(), that.currentTerm.ValidBytes())
}

// String returns the wrapped TermsEnum's string form, matching Java's
// Objects.toString(termsEnum).
func (t *TermsEnumIndex) String() string {
	if t.TermsEnum == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", t.TermsEnum)
}

// TermEquals reports whether this wrapper's current term equals the snapshot
// in state. Uses the cached prefix to short-circuit unequal terms cheaply.
func (t *TermsEnumIndex) TermEquals(state *TermsEnumTermState) bool {
	if state == nil || t.currentTerm == nil {
		return false
	}
	if t.currentTermPrefix8 != state.termPrefix8 {
		return false
	}
	return bytes.Equal(t.currentTerm.ValidBytes(), state.term.Get().ValidBytes())
}

// TermsEnumTermState is a snapshot of a TermsEnumIndex's current term that
// supports cheap equality checks. Port of TermsEnumIndex.TermState
// (renamed to avoid colliding with index.TermState).
type TermsEnumTermState struct {
	term        *util.BytesRefBuilder
	termPrefix8 uint64
}

// NewTermsEnumTermState returns an empty TermState snapshot.
func NewTermsEnumTermState() *TermsEnumTermState {
	return &TermsEnumTermState{
		term: util.NewBytesRefBuilder(),
	}
}

// CopyFrom captures the current term and cached prefix from t.
func (s *TermsEnumTermState) CopyFrom(t *TermsEnumIndex) {
	if t.currentTerm == nil {
		s.term.SetLength(0)
		s.termPrefix8 = 0
		return
	}
	s.term.CopyBytesRef(t.currentTerm)
	s.termPrefix8 = t.currentTermPrefix8
}

// termsEnumIndexBytes extracts the underlying BytesRef from a Term, or nil when term
// itself is nil. Centralises the small adapter between Gocene's Term-based
// TermsEnum and TermsEnumIndex's BytesRef-based surface.
func termsEnumIndexBytes(t *Term) *util.BytesRef {
	if t == nil {
		return nil
	}
	return t.GetBytesRef()
}

// termFieldFromCurrent returns the field name of the TermsEnum's current
// term, defaulting to the empty string if the cursor is not positioned. The
// field is needed to construct *Term values for SeekCeil/SeekExact because
// Gocene's TermsEnum surface is Term-based.
func termFieldFromCurrent(te TermsEnum) string {
	if current := te.Term(); current != nil {
		return current.Field
	}
	return ""
}
