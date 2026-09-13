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
//	lucene/core/src/java/org/apache/lucene/search/FuzzyQuery.java

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// DefaultMaxEdits mirrors
// {@code FuzzyQuery.defaultMaxEdits = LevenshteinAutomata.MAXIMUM_SUPPORTED_DISTANCE}.
const DefaultMaxEdits = automaton.MaximumSupportedLevenshteinDistance

// DefaultPrefixLength mirrors {@code FuzzyQuery.defaultPrefixLength = 0}.
const DefaultPrefixLength = 0

// DefaultMaxExpansions mirrors {@code FuzzyQuery.defaultMaxExpansions = 50}.
const DefaultMaxExpansions = 50

// DefaultTranspositions mirrors {@code FuzzyQuery.defaultTranspositions = true}.
const DefaultTranspositions = true

// FuzzyQueryDefaultRewriteMethod creates a default top-terms blended frequency
// scoring rewrite with the given max expansions.
//
// Mirrors the static
// {@code public static RewriteMethod defaultRewriteMethod(int maxExpansions)},
// whose body is
// {@code return new MultiTermQuery.TopTermsBlendedFreqScoringRewrite(maxExpansions);}.
// Go's flat package namespace would collide with the identically named static
// members other queries declare, so this one carries its declaring class.
func FuzzyQueryDefaultRewriteMethod(maxExpansions int) RewriteMethod {
	return NewTopTermsBlendedFreqScoringRewrite(maxExpansions)
}

// FuzzyQuery implements the fuzzy search query. The similarity measurement is
// based on the Damerau-Levenshtein (optimal string alignment) algorithm,
// though you can explicitly choose classic Levenshtein by passing false to the
// transpositions parameter.
//
// This query uses MultiTermQuery.TopTermsBlendedFreqScoringRewrite as default.
// So terms will be collected and scored according to their edit distance. Only
// the top terms are used for building the BooleanQuery. It is not recommended
// to change the rewrite mode for fuzzy queries.
//
// At most, this query will match terms up to
// automaton.MaximumSupportedLevenshteinDistance edits. Higher distances
// (especially with transpositions enabled) are generally not useful and will
// match a significant amount of the term dictionary.
//
// NOTE: terms of length 1 or 2 will sometimes not match because of how the
// scaled distance between two terms is computed. For a term to match, the edit
// distance between the terms must be less than the minimum length term (either
// the input term, or the candidate term). For example, FuzzyQuery on term
// "abcd" with maxEdits=2 will not match an indexed term "ab", and FuzzyQuery on
// term "a" with maxEdits=2 will not match an indexed term "abc".
//
// Mirrors org.apache.lucene.search.FuzzyQuery, declared as
// {@code public class FuzzyQuery extends MultiTermQuery} (Lucene 10.5.0).
type FuzzyQuery struct {
	MultiTermQuery

	maxEdits       int
	maxExpansions  int
	transpositions bool
	prefixLength   int
	term           *index.Term
}

// Compile-time assertions lock in the contracts this query participates in.
var (
	_ Query               = (*FuzzyQuery)(nil)
	_ MultiTermQueryOwner = (*FuzzyQuery)(nil)
)

// NewFuzzyQueryWithRewriteMethod creates a new FuzzyQuery that will match terms with an
// edit distance of at most maxEdits to term. If a prefixLength > 0 is
// specified, a common prefix of that length is also required.
//
//   - term: the term to search for
//   - maxEdits: must be >= 0 and <= automaton.MaximumSupportedLevenshteinDistance
//   - prefixLength: length of common (non-fuzzy) prefix
//   - maxExpansions: the maximum number of terms to match. If this number is
//     greater than GetMaxClauseCount() when the query is rewritten, then the
//     maxClauseCount will be used instead.
//   - transpositions: true if transpositions should be treated as a primitive
//     edit operation. If this is false, comparisons will implement the classic
//     Levenshtein algorithm.
//   - rewriteMethod: the rewrite method to use to build the final query
//
// Mirrors the six-argument constructor
// {@code FuzzyQuery(Term, int, int, int, boolean, RewriteMethod)}, including
// its three IllegalArgumentException checks and their messages.
func NewFuzzyQueryWithRewriteMethod(
	term *index.Term,
	maxEdits int,
	prefixLength int,
	maxExpansions int,
	transpositions bool,
	rewriteMethod RewriteMethod,
) *FuzzyQuery {
	if maxEdits < 0 || maxEdits > automaton.MaximumSupportedLevenshteinDistance {
		panic(fmt.Sprintf("maxEdits must be between 0 and %d", automaton.MaximumSupportedLevenshteinDistance))
	}
	if prefixLength < 0 {
		panic("prefixLength cannot be negative.")
	}
	if maxExpansions <= 0 {
		panic("maxExpansions must be positive.")
	}

	q := &FuzzyQuery{
		MultiTermQuery: *NewMultiTermQuery(term.Field, rewriteMethod),
		term:           term,
		maxEdits:       maxEdits,
		prefixLength:   prefixLength,
		transpositions: transpositions,
		maxExpansions:  maxExpansions,
	}
	q.MultiTermQuery.SetOwner(q)
	return q
}

// NewFuzzyQueryFull calls NewFuzzyQueryWithRewriteMethod with
// FuzzyQueryDefaultRewriteMethod(maxExpansions).
//
// Mirrors {@code FuzzyQuery(Term term, int maxEdits, int prefixLength, int maxExpansions, boolean transpositions)},
// the fullest constructor that does not take a RewriteMethod.
func NewFuzzyQueryFull(term *index.Term, maxEdits, prefixLength, maxExpansions int, transpositions bool) *FuzzyQuery {
	return NewFuzzyQueryWithRewriteMethod(
		term,
		maxEdits,
		prefixLength,
		maxExpansions,
		transpositions,
		FuzzyQueryDefaultRewriteMethod(maxExpansions),
	)
}

// NewFuzzyQueryWithParams calls NewFuzzyQueryFull with
// DefaultTranspositions.
//
// Mirrors {@code FuzzyQuery(Term term, int maxEdits, int prefixLength, int maxExpansions)}; Java
// reaches this arity through the five-argument constructor's default, which is
// spelled out here because Go has no overloading.
func NewFuzzyQueryWithParams(term *index.Term, maxEdits, prefixLength, maxExpansions int) *FuzzyQuery {
	return NewFuzzyQueryFull(term, maxEdits, prefixLength, maxExpansions, DefaultTranspositions)
}

// NewFuzzyQueryWithPrefix calls
// NewFuzzyQueryFull(term, maxEdits, prefixLength, DefaultMaxExpansions, DefaultTranspositions).
//
// Mirrors {@code FuzzyQuery(Term term, int maxEdits, int prefixLength)}.
func NewFuzzyQueryWithPrefix(term *index.Term, maxEdits, prefixLength int) *FuzzyQuery {
	return NewFuzzyQueryFull(term, maxEdits, prefixLength, DefaultMaxExpansions, DefaultTranspositions)
}

// NewFuzzyQueryWithMaxEdits calls
// NewFuzzyQueryWithPrefix(term, maxEdits, DefaultPrefixLength).
//
// Mirrors {@code FuzzyQuery(Term term, int maxEdits)}.
func NewFuzzyQueryWithMaxEdits(term *index.Term, maxEdits int) *FuzzyQuery {
	return NewFuzzyQueryWithPrefix(term, maxEdits, DefaultPrefixLength)
}

// NewFuzzyQuery calls NewFuzzyQueryWithMaxEdits(term, DefaultMaxEdits).
//
// Mirrors {@code FuzzyQuery(Term term)}.
func NewFuzzyQuery(term *index.Term) *FuzzyQuery {
	return NewFuzzyQueryWithMaxEdits(term, DefaultMaxEdits)
}

// GetMaxEdits returns the maximum number of edit distances allowed for this
// query to match.
func (q *FuzzyQuery) GetMaxEdits() int { return q.maxEdits }

// GetPrefixLength returns the non-fuzzy prefix length. This is the number of
// characters at the start of a term that must be identical (not fuzzy) to the
// query term if the query is to match that term.
func (q *FuzzyQuery) GetPrefixLength() int { return q.prefixLength }

// GetTranspositions returns true if transpositions should be treated as a
// primitive edit operation. If this is false, comparisons will implement the
// classic Levenshtein algorithm.
func (q *FuzzyQuery) GetTranspositions() bool { return q.transpositions }

// GetMaxExpansions returns the maximum number of terms this query will match.
//
// Java keeps maxExpansions private and exposes it only through hashCode/equals
// and the default rewrite method; the accessor is the Go rendering of that
// same field, which the surrounding package already needs.
func (q *FuzzyQuery) GetMaxExpansions() int { return q.maxExpansions }

// GetAutomata returns the compiled automata used to match terms.
//
// Mirrors {@code public CompiledAutomaton getAutomata()}, whose body is
// {@code return getFuzzyAutomaton(term.text(), maxEdits, prefixLength, transpositions);}.
func (q *FuzzyQuery) GetAutomata() *automaton.CompiledAutomaton {
	return FuzzyQueryGetFuzzyAutomaton(q.term.Text(), q.maxEdits, q.prefixLength, q.transpositions)
}

// FuzzyQueryGetFuzzyAutomaton returns the CompiledAutomaton internally used by
// FuzzyQuery to match terms. This is a very low-level method and may no longer
// exist in case the implementation of fuzzy-matching changes in the future.
//
// Mirrors the static
// {@code public static CompiledAutomaton getFuzzyAutomaton(String term, int maxEdits, int prefixLength, boolean transpositions)}.
// The declaring class is carried in the name because Go's flat package
// namespace has no room for a bare getFuzzyAutomaton.
//
// A term the FuzzyAutomatonBuilder cannot compile raises a
// [FuzzyTermsException], mirroring the exception FuzzyTermsEnum declares for
// exactly this failure.
func FuzzyQueryGetFuzzyAutomaton(term string, maxEdits, prefixLength int, transpositions bool) *automaton.CompiledAutomaton {
	builder, err := NewFuzzyAutomatonBuilder(term, maxEdits, prefixLength, transpositions)
	if err != nil {
		panic(newFuzzyTermsException(term, err))
	}
	return builder.BuildMaxEditAutomaton()
}

// Visit reproduces
//
//	if (visitor.acceptField(field)) {
//	  visitor.consumeTermsMatching(this, term.field(), () -> getAutomata().runAutomaton);
//	}
func (q *FuzzyQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.GetField()) {
		visitor.ConsumeTermsMatching(q, q.term.Field, func() ByteRunAutomaton {
			return byteRunAutomatonAdapter{a: q.GetAutomata().RunAutomaton}
		})
	}
}

// GetTermsEnumWithAttributes reproduces
//
//	if (maxEdits == 0) { // can only match if it's exact
//	  return new SingleTermsEnum(terms.iterator(), term.bytes());
//	}
//	return new FuzzyTermsEnum(terms, atts, getTerm(), maxEdits, prefixLength, transpositions);
//
// index.NewSingleTermFilteredEnum is the Go port of
// org.apache.lucene.index.SingleTermsEnum; see its doc comment for why it does
// not carry the Lucene name.
func (q *FuzzyQuery) GetTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	if q.maxEdits == 0 { // can only match if it's exact
		it, err := terms.GetIterator()
		if err != nil {
			return nil, err
		}
		return index.NewSingleTermFilteredEnum(it, q.term), nil
	}
	return newFuzzyTermsEnumWithAttributes(terms, atts, q.GetTerm(), q.maxEdits, q.prefixLength, q.transpositions)
}

// GetTerm returns the pattern term.
func (q *FuzzyQuery) GetTerm() *index.Term { return q.term }

// ToString reproduces {@code public String toString(String field)}:
//
//	if (!term.field().equals(field)) { buffer.append(term.field()); buffer.append(":"); }
//	buffer.append(term.text());
//	buffer.append('~');
//	buffer.append(maxEdits);
func (q *FuzzyQuery) ToString(field string) string {
	var buffer strings.Builder
	if q.term.Field != field {
		buffer.WriteString(q.term.Field)
		buffer.WriteString(":")
	}
	buffer.WriteString(q.term.Text())
	buffer.WriteByte('~')
	fmt.Fprintf(&buffer, "%d", q.maxEdits)
	return buffer.String()
}

// HashCode reproduces
//
//	final int prime = 31;
//	int result = super.hashCode();
//	result = prime * result + maxEdits;
//	result = prime * result + prefixLength;
//	result = prime * result + maxExpansions;
//	result = prime * result + (transpositions ? 0 : 1);
//	result = prime * result + ((term == null) ? 0 : term.hashCode());
func (q *FuzzyQuery) HashCode() int {
	const prime = 31
	result := q.MultiTermQuery.HashCode()
	result = prime*result + q.maxEdits
	result = prime*result + q.prefixLength
	result = prime*result + q.maxExpansions
	if q.transpositions {
		result = prime*result + 0
	} else {
		result = prime*result + 1
	}
	if q.term == nil {
		result = prime*result + 0
	} else {
		result = prime*result + q.term.HashCode()
	}
	return result
}

// Equals reproduces
//
//	if (this == obj) return true;
//	if (!super.equals(obj)) return false;
//	if (getClass() != obj.getClass()) return false;
//	FuzzyQuery other = (FuzzyQuery) obj;
//	return maxEdits == other.maxEdits && prefixLength == other.prefixLength
//	    && maxExpansions == other.maxExpansions && transpositions == other.transpositions
//	    && Objects.equals(term, other.term);
func (q *FuzzyQuery) Equals(obj spi.Query) bool {
	if q == obj {
		return true
	}
	if !q.MultiTermQuery.Equals(obj) {
		return false
	}
	other, ok := obj.(*FuzzyQuery)
	if !ok {
		return false
	}
	return q.maxEdits == other.maxEdits &&
		q.prefixLength == other.prefixLength &&
		q.maxExpansions == other.maxExpansions &&
		q.transpositions == other.transpositions &&
		termsEqual(q.term, other.term)
}

// termsEqual renders {@code Objects.equals(term, other.term)}.
func termsEqual(a, b *index.Term) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equals(b)
}

// FuzzyQueryFloatToEdits converts from "minimumSimilarity" fractions to raw
// edit distances.
//
//   - minimumSimilarity: scaled similarity
//   - termLen: length (in unicode codepoints) of the term
//
// Mirrors the static
// {@code public static int floatToEdits(float minimumSimilarity, int termLen)};
// the declaring class is carried in the name because Go's flat package
// namespace has no room for a bare floatToEdits.
func FuzzyQueryFloatToEdits(minimumSimilarity float32, termLen int) int {
	if minimumSimilarity >= 1.0 {
		// (int) Math.min(minimumSimilarity, MAXIMUM_SUPPORTED_DISTANCE)
		if minimumSimilarity < float32(automaton.MaximumSupportedLevenshteinDistance) {
			return int(minimumSimilarity)
		}
		return automaton.MaximumSupportedLevenshteinDistance
	} else if minimumSimilarity == 0.0 {
		return 0 // 0 means exact, not infinite # of edits!
	}
	// Math.min((int) ((1D - minimumSimilarity) * termLen), MAXIMUM_SUPPORTED_DISTANCE)
	// Java widens minimumSimilarity to double before the subtraction.
	edits := int((1.0 - float64(minimumSimilarity)) * float64(termLen))
	if edits < automaton.MaximumSupportedLevenshteinDistance {
		return edits
	}
	return automaton.MaximumSupportedLevenshteinDistance
}
