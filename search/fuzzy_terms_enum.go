// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/FuzzyTermsEnum.java

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// FuzzyTermsEnum is a subclass of TermsEnum for enumerating all terms that are
// similar to the specified filter term.
//
// Term enumerations are always ordered by BytesRef comparison. Each term in
// the enumeration is greater than all that precede it.
//
// Mirrors org.apache.lucene.search.FuzzyTermsEnum, declared as
// {@code public final class FuzzyTermsEnum extends BaseTermsEnum}
// (Lucene 10.5.0).
type FuzzyTermsEnum struct {
	// NOTE: we can't subclass FilteredTermsEnum here because we need to
	// sometimes change actualEnum:
	actualEnum index.TermsEnum

	atts *util.AttributeSource

	// We use this to communicate the score (boost) of the current matched term
	// we are on back to MultiTermQuery.TopTermsBlendedFreqScoringRewrite that
	// is collecting the best (default 50) matched terms:
	boostAtt BoostAttribute

	// MultiTermQuery.TopTermsBlendedFreqScoringRewrite tells us the worst boost
	// still in its queue using this att, which we use to know when we can
	// reduce the automaton from ed=2 to ed=1, or ed=0 if only single top term
	// is collected:
	maxBoostAtt MaxNonCompetitiveBoostAttribute

	automata   []*automaton.CompiledAutomaton
	terms      index.Terms
	termLength int
	term       *index.Term

	bottom     float32
	bottomTerm []byte

	queuedBottom *util.BytesRef

	// Maximum number of edits we will accept. This is either 2 or 1 (or,
	// degenerately, 0) passed by the user originally, but as we collect terms,
	// we can lower this (e.g. from 2 to 1) if we detect that the term queue is
	// full, and all collected terms are ed=1:
	maxEdits int
}

// Compile-time assertion that FuzzyTermsEnum is a TermsEnum.
var _ index.TermsEnum = (*FuzzyTermsEnum)(nil)

// NewFuzzyTermsEnum constructs an enumeration of all terms from the specified
// terms which share a prefix of length prefixLength with term and which have
// at most maxEdits edits.
//
// After calling the constructor the enumeration is already pointing to the
// first valid term if such a term exists.
//
// Mirrors the public constructor
// {@code FuzzyTermsEnum(Terms terms, Term term, int maxEdits, int prefixLength, boolean transpositions)},
// whose body delegates to the private constructor with a fresh AttributeSource.
func NewFuzzyTermsEnum(terms index.Terms, term *index.Term, maxEdits, prefixLength int, transpositions bool) (*FuzzyTermsEnum, error) {
	return newFuzzyTermsEnumWithAttributes(terms, util.NewAttributeSource(), term, maxEdits, prefixLength, transpositions)
}

// newFuzzyTermsEnumWithAttributes constructs an enumeration sharing automata
// between segments through atts.
//
// Mirrors the package-private constructor
// {@code FuzzyTermsEnum(Terms terms, AttributeSource atts, Term term, int maxEdits, int prefixLength, boolean transpositions)}.
func newFuzzyTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource, term *index.Term, maxEdits, prefixLength int, transpositions bool) (*FuzzyTermsEnum, error) {
	return newFuzzyTermsEnum(terms, atts, term, func() (*FuzzyAutomatonBuilder, error) {
		return NewFuzzyAutomatonBuilder(term.Text(), maxEdits, prefixLength, transpositions)
	})
}

// newFuzzyTermsEnum reproduces the private constructor
// {@code FuzzyTermsEnum(Terms terms, AttributeSource atts, Term term, Supplier<FuzzyAutomatonBuilder> automatonBuilder)}.
func newFuzzyTermsEnum(
	terms index.Terms,
	atts *util.AttributeSource,
	term *index.Term,
	automatonBuilder func() (*FuzzyAutomatonBuilder, error),
) (*FuzzyTermsEnum, error) {
	e := &FuzzyTermsEnum{
		terms: terms,
		atts:  atts,
		term:  term,
	}

	maxBoostAtt, ok := atts.AddAttribute(MaxNonCompetitiveBoostAttributeType).(MaxNonCompetitiveBoostAttribute)
	if !ok {
		return nil, errMaxNonCompetitiveBoostAttribute
	}
	e.maxBoostAtt = maxBoostAtt
	boostAtt, ok := atts.AddAttribute(BoostAttributeType).(BoostAttribute)
	if !ok {
		return nil, errBoostAttribute
	}
	e.boostAtt = boostAtt

	atts.AddAttributeImpl(newAutomatonAttributeImpl())
	aa, ok := atts.AddAttribute(automatonAttributeType).(automatonAttribute)
	if !ok {
		return nil, errAutomatonAttribute
	}
	if err := aa.Init(automatonBuilder); err != nil {
		return nil, err
	}

	e.automata = aa.GetAutomata()
	e.termLength = aa.GetTermLength()
	e.maxEdits = len(e.automata) - 1

	e.bottom = maxBoostAtt.GetMaxNonCompetitiveBoost()
	e.bottomTerm = maxBoostAtt.GetCompetitiveTerm()
	if err := e.bottomChanged(nil); err != nil {
		return nil, err
	}
	return e, nil
}

// SetMaxNonCompetitiveBoost sets the maximum non-competitive boost, which may
// allow switching to a lower max-edit automaton at run time.
func (e *FuzzyTermsEnum) SetMaxNonCompetitiveBoost(boost float32) {
	e.maxBoostAtt.SetMaxNonCompetitiveBoost(boost)
}

// GetBoost gets the boost of the current term.
func (e *FuzzyTermsEnum) GetBoost() float32 {
	return e.boostAtt.GetBoost()
}

// getAutomatonEnum returns an automata-based enum for matching up to
// editDistance from lastTerm, if possible.
func (e *FuzzyTermsEnum) getAutomatonEnum(editDistance int, lastTerm *util.BytesRef) (index.TermsEnum, error) {
	compiled := e.automata[editDistance]
	var initialSeekTerm *util.BytesRef
	if lastTerm == nil {
		// This is the first enum we are pulling:
		initialSeekTerm = nil
	} else {
		// We are pulling this enum (e.g., ed=1) after iterating for a while
		// already (e.g., ed=2):
		initialSeekTerm = compiled.Floor(lastTerm, util.NewBytesRefBuilder())
	}
	var startTerm *index.Term
	if initialSeekTerm != nil {
		startTerm = index.NewTermFromBytesRef(e.terms.Field(), initialSeekTerm)
	}
	return e.terms.Intersect(compiled, startTerm)
}

// bottomChanged is fired when the max non-competitive boost has changed. This
// is the hook to swap in a smarter actualEnum.
func (e *FuzzyTermsEnum) bottomChanged(lastTerm *util.BytesRef) error {
	oldMaxEdits := e.maxEdits

	// true if the last term encountered is lexicographically equal or after
	// the bottom term in the PQ
	termAfter := e.bottomTerm == nil ||
		(lastTerm != nil && util.BytesRefCompare(lastTerm, util.NewBytesRef(e.bottomTerm)) >= 0)

	// as long as the max non-competitive boost is >= the max boost
	// for some edit distance, keep dropping the max edit distance.
	for e.maxEdits > 0 {
		maxBoost := 1.0 - (float32(e.maxEdits) / float32(e.termLength))
		if e.bottom < maxBoost || (e.bottom == maxBoost && !termAfter) {
			break
		}
		e.maxEdits--
	}

	if oldMaxEdits != e.maxEdits || lastTerm == nil {
		// This is a very powerful optimization: the maximum edit distance has
		// changed. This happens because we collect only the top scoring N
		// (= 50, by default) terms, and if e.g. maxEdits=2, and the queue is
		// now full of matching terms, and we notice that the worst entry in
		// that queue is ed=1, then we can switch the automata here to ed=1
		// which is a big speedup.
		actualEnum, err := e.getAutomatonEnum(e.maxEdits, lastTerm)
		if err != nil {
			return err
		}
		e.actualEnum = actualEnum
	}
	return nil
}

// Next advances to the next matching term, reproducing
// {@code public BytesRef next()}.
func (e *FuzzyTermsEnum) Next() (*index.Term, error) {
	if e.queuedBottom != nil {
		if err := e.bottomChanged(e.queuedBottom); err != nil {
			return nil, err
		}
		e.queuedBottom = nil
	}

	term, err := e.actualEnum.Next()
	if err != nil {
		return nil, err
	}
	if term == nil {
		// end
		return nil, nil
	}

	ed := e.maxEdits

	// we know the outer DFA always matches.
	// now compute exact edit distance
	for ed > 0 {
		if e.matches(term.Bytes, ed-1) {
			ed--
		} else {
			break
		}
	}

	if ed == 0 { // exact match
		e.boostAtt.SetBoost(1.0)
	} else {
		codePointCount := util.CodePointCount(term.Bytes)
		minTermLength := codePointCount
		if e.termLength < minTermLength {
			minTermLength = e.termLength
		}

		similarity := 1.0 - (float32(ed) / float32(minTermLength))
		e.boostAtt.SetBoost(similarity)
	}

	bottom := e.maxBoostAtt.GetMaxNonCompetitiveBoost()
	bottomTerm := e.maxBoostAtt.GetCompetitiveTerm()
	if bottom != e.bottom || !sameByteSlice(bottomTerm, e.bottomTerm) {
		e.bottom = bottom
		e.bottomTerm = bottomTerm
		// clone the term before potentially doing something with it
		// this is a rare but wonderful occurrence anyway

		// We must delay bottomChanged until the next next() call otherwise we
		// mess up docFreq(), etc., for the current term:
		e.queuedBottom = util.DeepCopyOfBytesRef(term.Bytes)
	}

	return term, nil
}

// sameByteSlice renders Java's reference comparison
// {@code bottomTerm != this.bottomTerm} on the BytesRef the
// MaxNonCompetitiveBoostAttribute hands back.
//
// The Java test is identity, not content: the attribute either still holds the
// very BytesRef seen last time (no change) or a different object (changed). Go
// cannot compare slice headers with ==, so the same distinction is drawn by
// comparing the bytes, which agrees with Java's outcome on every transition the
// attribute actually performs — the competitive term only ever changes to a
// different term.
func sameByteSlice(a, b []byte) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// matches returns true if term is within k edits of the query term.
func (e *FuzzyTermsEnum) matches(termIn *util.BytesRef, k int) bool {
	if k == 0 {
		return util.BytesRefEquals(termIn, e.term.Bytes)
	}
	return e.automata[k].RunAutomaton.Run(termIn.Bytes, termIn.Offset, termIn.Length)
}

// ─── proxy all other enum calls to the actual enum ──────────────────────────

// DocFreq proxies to the actual enum.
func (e *FuzzyTermsEnum) DocFreq() (int, error) { return e.actualEnum.DocFreq() }

// TotalTermFreq proxies to the actual enum.
func (e *FuzzyTermsEnum) TotalTermFreq() (int64, error) { return e.actualEnum.TotalTermFreq() }

// Postings proxies to the actual enum.
func (e *FuzzyTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return e.actualEnum.Postings(flags)
}

// PostingsWithLiveDocs proxies to the actual enum. Gocene's TermsEnum splits
// Lucene's single postings(PostingsEnum, int) into Postings and this form.
func (e *FuzzyTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.actualEnum.PostingsWithLiveDocs(liveDocs, flags)
}

// Impacts proxies to the actual enum. The return type is spi.ImpactsEnum
// because that is what the spi.TermsEnum contract declares.
func (e *FuzzyTermsEnum) Impacts(flags int) (spi.ImpactsEnum, error) {
	return e.actualEnum.Impacts(flags)
}

// SeekExactWithState proxies {@code seekExact(BytesRef term, TermState state)}
// to the actual enum. Gocene spells the two-argument seekExact this way; the
// default body lives in index.SeekExactWithState.
func (e *FuzzyTermsEnum) SeekExactWithState(term *index.Term, state index.TermState) error {
	if stateful, ok := e.actualEnum.(interface {
		SeekExactWithState(*index.Term, index.TermState) error
	}); ok {
		return stateful.SeekExactWithState(term, state)
	}
	return index.SeekExactWithState(e.actualEnum, term, state)
}

// TermState proxies to the actual enum.
func (e *FuzzyTermsEnum) TermState() (index.TermState, error) {
	return index.TermStateDelegated(e.actualEnum)
}

// Ord proxies to the actual enum.
func (e *FuzzyTermsEnum) Ord() int64 { return e.actualEnum.Ord() }

// Attributes returns the AttributeSource this enum was built with, reproducing
// {@code public AttributeSource attributes() { return atts; }} — the override
// that lets a RewriteMethod share the automata and the boost cursor across
// segments.
func (e *FuzzyTermsEnum) Attributes() *util.AttributeSource { return e.atts }

// SeekExact proxies to the actual enum.
func (e *FuzzyTermsEnum) SeekExact(text *index.Term) (bool, error) {
	return e.actualEnum.SeekExact(text)
}

// PrepareSeekExact proxies to the actual enum.
func (e *FuzzyTermsEnum) PrepareSeekExact(text *index.Term) (util.IOBooleanSupplier, error) {
	return index.PrepareSeekExactDelegated(e.actualEnum, text)
}

// SeekCeil proxies to the actual enum.
func (e *FuzzyTermsEnum) SeekCeil(text *index.Term) (*index.Term, error) {
	return e.actualEnum.SeekCeil(text)
}

// SeekExactOrd proxies {@code seekExact(long ord)} to the actual enum.
func (e *FuzzyTermsEnum) SeekExactOrd(ord int64) error {
	if seeker, ok := e.actualEnum.(interface{ SeekExactOrd(int64) error }); ok {
		return seeker.SeekExactOrd(ord)
	}
	return errFuzzySeekExactOrdUnsupported
}

// Term proxies to the actual enum.
func (e *FuzzyTermsEnum) Term() *index.Term { return e.actualEnum.Term() }

// errFuzzySeekExactOrdUnsupported reports a delegate TermsEnum with no
// ord-based seek. Lucene's TermsEnum#seekExact(long) is an optional method and
// the codec may throw UnsupportedOperationException.
var errFuzzySeekExactOrdUnsupported = errors.New(
	"FuzzyTermsEnum.seekExact(long): the delegate TermsEnum does not support ord seeks")

// FuzzyTermsException indicates that there was an issue creating a fuzzy query
// for a given term. Typically occurs with terms longer than 220 UTF-8
// characters, but also possible with shorter terms consisting of UTF-32 code
// points.
//
// Mirrors the nested class FuzzyTermsEnum.FuzzyTermsException, declared as
// {@code public static class FuzzyTermsException extends RuntimeException}
// with the message "Term too complex: " + term.
type FuzzyTermsException struct {
	Term  string
	Cause error
}

// Error renders {@code super("Term too complex: " + term, cause)}.
func (e *FuzzyTermsException) Error() string {
	return "Term too complex: " + e.Term
}

// Unwrap exposes the cause, mirroring RuntimeException#getCause().
func (e *FuzzyTermsException) Unwrap() error { return e.Cause }

// newFuzzyTermsException mirrors
// {@code FuzzyTermsException(String term, Throwable cause)}.
func newFuzzyTermsException(term string, cause error) *FuzzyTermsException {
	return &FuzzyTermsException{Term: term, Cause: cause}
}

// ─── AutomatonAttribute ─────────────────────────────────────────────────────

// automatonAttributeType is the reflect.Type of the [automatonAttribute]
// interface, the Go stand-in for {@code AutomatonAttribute.class}.
var automatonAttributeType = reflect.TypeOf((*automatonAttribute)(nil)).Elem()

// errAutomatonAttribute reports an AttributeSource that handed back an impl
// which does not satisfy the automaton attribute. It cannot happen because the
// impl is installed with AddAttributeImpl immediately before the lookup.
var errAutomatonAttribute = errors.New(
	"AttributeSource returned an impl that is not a FuzzyTermsEnum automaton attribute")

// automatonAttribute is used for sharing automata between segments.
//
// Levenshtein automata are large and expensive to build; we don't want to
// build them directly on the query because this can blow up caches that use
// queries as keys; we also don't want to rebuild them for every segment. This
// attribute allows the FuzzyTermsEnum to build the automata once for its first
// segment and then share them for subsequent segment calls.
//
// Mirrors the private nested interface FuzzyTermsEnum.AutomatonAttribute.
type automatonAttribute interface {
	util.AttributeImpl

	GetAutomata() []*automaton.CompiledAutomaton
	GetTermLength() int
	Init(builder func() (*FuzzyAutomatonBuilder, error)) error
}

// automatonAttributeImpl mirrors the private nested class
// FuzzyTermsEnum.AutomatonAttributeImpl.
type automatonAttributeImpl struct {
	util.BaseAttributeImpl
	automata   []*automaton.CompiledAutomaton
	termLength int
}

var (
	_ automatonAttribute              = (*automatonAttributeImpl)(nil)
	_ util.AttributeImpl              = (*automatonAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*automatonAttributeImpl)(nil)
)

// newAutomatonAttributeImpl mirrors {@code new AutomatonAttributeImpl()}.
func newAutomatonAttributeImpl() *automatonAttributeImpl { return &automatonAttributeImpl{} }

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider] so that
// AddAttributeImpl registers this impl under the automaton attribute type,
// mirroring Java's reflective discovery of the implemented Attribute
// interfaces.
func (a *automatonAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{automatonAttributeType}
}

// GetAutomata returns the shared automata set.
func (a *automatonAttributeImpl) GetAutomata() []*automaton.CompiledAutomaton { return a.automata }

// GetTermLength returns the shared term length in code points.
func (a *automatonAttributeImpl) GetTermLength() int { return a.termLength }

// Init reproduces
//
//	if (automata != null) return;
//	FuzzyAutomatonBuilder builder = supplier.get();
//	this.termLength = builder.getTermLength();
//	this.automata = builder.buildAutomatonSet();
func (a *automatonAttributeImpl) Init(supplier func() (*FuzzyAutomatonBuilder, error)) error {
	if a.automata != nil {
		return nil
	}
	builder, err := supplier()
	if err != nil {
		return err
	}
	a.termLength = builder.GetTermLength()
	a.automata = builder.BuildAutomatonSet()
	return nil
}

// Clear reproduces {@code public void clear() { this.automata = null; }}.
func (a *automatonAttributeImpl) Clear() { a.automata = nil }

// ReflectWith reproduces
// {@code public void reflectWith(AttributeReflector reflector) { throw new UnsupportedOperationException(); }}.
func (a *automatonAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	panic(fmt.Sprintf("%T.ReflectWith: unsupported operation", a))
}

// CopyTo reproduces
// {@code public void copyTo(AttributeImpl target) { throw new UnsupportedOperationException(); }}.
func (a *automatonAttributeImpl) CopyTo(target util.AttributeImpl) {
	panic(fmt.Sprintf("%T.CopyTo: unsupported operation", a))
}

// CloneAttribute has no Java counterpart on AutomatonAttributeImpl, which
// inherits AttributeImpl#clone(). Java's clone is a shallow field copy, which
// is what this reproduces.
func (a *automatonAttributeImpl) CloneAttribute() util.AttributeImpl {
	clone := *a
	return &clone
}
