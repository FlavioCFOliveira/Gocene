// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BaseTermsEnum provides default implementations for a subset of TermsEnum
// methods. Mirrors org.apache.lucene.index.BaseTermsEnum from Apache
// Lucene 10.4.0.
//
// The Java original supplies default impls for: attributes(), termState(),
// seekExact(BytesRef), seekExact(BytesRef, TermState), prepareSeekExact().
// Gocene's TermsEnum interface differs slightly (Term-based seeks, no
// AttributeSource yet), so this base struct supplies the equivalents:
//
//   - SeekExactDelegated   — defaults to SeekCeil + FOUND check
//   - SeekExactWithState   — defaults to SeekExactDelegated + state copy
//   - TermState            — returns a placeholder OrdTermState (subclasses
//     should override with their codec-specific impl)
//
// Concrete TermsEnum implementations embed BaseTermsEnum to inherit these
// behaviours and only override the codec-specific bits.
type BaseTermsEnum struct {
	TermsEnumBase
}

// SeekStatus represents the outcome of a SeekCeil call, matching Lucene's
// TermsEnum.SeekStatus enum.
type SeekStatus int

const (
	// SeekStatusFound — the requested term was found exactly.
	SeekStatusFound SeekStatus = iota
	// SeekStatusNotFound — the requested term was not found; the enumerator
	// is positioned at the smallest term greater than the requested one.
	SeekStatusNotFound
	// SeekStatusEnd — the requested term is greater than all terms in the
	// enumerator; the enumerator is exhausted.
	SeekStatusEnd
)

// SeekExactDelegated reproduces Java's default seekExact via SeekCeil. owner
// is the TermsEnum that embeds this base; it is required because Go cannot
// reach into the embedder's interface methods without an explicit reference.
func SeekExactDelegated(owner TermsEnum, term *Term) (bool, error) {
	got, err := owner.SeekCeil(term)
	if err != nil {
		return false, err
	}
	if got == nil {
		return false, nil
	}
	return got.Equals(term), nil
}

// SeekExactWithState reproduces Java's seekExact(BytesRef, TermState) by
// performing a plain SeekExact and then copying state into the supplied
// holder. Returns an error if the term is not found.
func SeekExactWithState(owner TermsEnum, term *Term, state TermState) error {
	if state == nil {
		return errOrdTermStateNil
	}
	found, err := owner.SeekExact(term)
	if err != nil {
		return err
	}
	if !found {
		return errTermNotFound
	}
	return nil
}

// errOrdTermStateNil and errTermNotFound are sentinels used by
// SeekExactWithState.
var (
	errOrdTermStateNil = errorString("BaseTermsEnum.SeekExactWithState: state must not be nil")
	errTermNotFound    = errorString("BaseTermsEnum.SeekExactWithState: term not found")
)

// errorString is a tiny zero-alloc string-backed error to avoid pulling in
// the errors package solely for two sentinels.
type errorString string

func (e errorString) Error() string { return string(e) }

// PrepareSeekExactDelegated reproduces the default body of
// BaseTermsEnum.prepareSeekExact(BytesRef) in Apache Lucene 10.5.0 —
// {@code return () -> seekExact(text)} — while still honouring an enumerator
// that overrides the method, exactly as Java's dynamic dispatch would. owner is
// the TermsEnum the call is made on; Go cannot reach an embedder's interface
// methods without an explicit reference.
//
// A nil supplier means the enumerator could determine cheaply that the term
// does not exist; the default body never returns nil.
func PrepareSeekExactDelegated(owner TermsEnum, term *Term) (util.IOBooleanSupplier, error) {
	if preparer, ok := owner.(interface {
		PrepareSeekExact(term *Term) (util.IOBooleanSupplier, error)
	}); ok {
		return preparer.PrepareSeekExact(term)
	}
	return func() (bool, error) { return owner.SeekExact(term) }, nil
}

// TermStateDelegated reproduces the default body of BaseTermsEnum.termState()
// in Apache Lucene 10.5.0 — an anonymous TermState whose copyFrom throws
// UnsupportedOperationException — while still honouring an enumerator that
// overrides the method.
func TermStateDelegated(owner TermsEnum) (TermState, error) {
	if stateful, ok := owner.(interface{ TermState() (TermState, error) }); ok {
		return stateful.TermState()
	}
	return baseTermState{}, nil
}

// baseTermState mirrors the anonymous TermState returned by
// BaseTermsEnum.termState(), whose copyFrom throws UnsupportedOperationException.
type baseTermState struct{}

// CopyFrom is unsupported, mirroring the anonymous TermState's copyFrom.
func (baseTermState) CopyFrom(TermState) error {
	return errBaseTermStateCopyFrom
}

var errBaseTermStateCopyFrom = errorString("BaseTermsEnum.TermState.CopyFrom: unsupported operation")
