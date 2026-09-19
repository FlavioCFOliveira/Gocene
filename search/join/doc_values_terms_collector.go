// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/join/src/java/org/apache/lucene/search/join/DocValuesTermsCollector.java

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DocValuesTermsCollectorFunction mirrors the @FunctionalInterface
// org.apache.lucene.search.join.DocValuesTermsCollector.Function<R>, whose sole
// method is {@code R apply(LeafReader t) throws IOException}.
//
// Go has no nested types, so the Java nesting DocValuesTermsCollector.Function
// is flattened into this package-level name.
type DocValuesTermsCollectorFunction[R any] func(t index.LeafReader) (R, error)

// DocValuesTermsCollector mirrors the abstract class
// org.apache.lucene.search.join.DocValuesTermsCollector<DV>, which extends
// SimpleCollector and reloads a doc-values cursor at every segment switch.
type DocValuesTermsCollector[DV any] struct {
	// BaseSimpleCollector carries the concrete members of the Java superclass
	// org.apache.lucene.search.SimpleCollector: the empty setScorer(Scorable)
	// and setWeight(Weight) bodies, and Outer, which renders the `this` that
	// getLeafCollector returns.
	search.BaseSimpleCollector

	// BaseLeafCollector carries the two LeafCollector defaults that
	// SimpleCollector inherits without overriding: competitiveIterator() and
	// finish().
	search.BaseLeafCollector

	// docValues renders `protected DV docValues`.
	docValues DV

	// docValuesCall renders `private final Function<DV> docValuesCall`.
	docValuesCall DocValuesTermsCollectorFunction[DV]
}

// NewDocValuesTermsCollector mirrors
// `public DocValuesTermsCollector(Function<DV> docValuesCall)`.
func NewDocValuesTermsCollector[DV any](docValuesCall DocValuesTermsCollectorFunction[DV]) *DocValuesTermsCollector[DV] {
	return &DocValuesTermsCollector[DV]{docValuesCall: docValuesCall}
}

// GetLeafCollector mirrors
// `public final LeafCollector getLeafCollector(LeafReaderContext context)` of
// org.apache.lucene.search.SimpleCollector, whose body is
// `doSetNextReader(context); return this;`.
//
// Go has no virtual dispatch, so search.BaseSimpleCollector.GetLeafCollector
// would reach its own no-op DoSetNextReader rather than the override below.
// DocValuesTermsCollector.doSetNextReader is `final` in Java, so the pairing is
// fixed for every subclass and the Java body is reproduced here verbatim.
func (c *DocValuesTermsCollector[DV]) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c.Outer, nil
}

// DoSetNextReader mirrors
// `protected final void doSetNextReader(LeafReaderContext context)`.
func (c *DocValuesTermsCollector[DV]) DoSetNextReader(context *index.LeafReaderContext) error {
	dv, err := c.docValuesCall(context.LeafReader())
	if err != nil {
		return err
	}
	c.docValues = dv
	return nil
}

// SortedDocValues mirrors the static factory
// `static Function<SortedDocValues> sortedDocValues(String field)`.
func SortedDocValues(field string) DocValuesTermsCollectorFunction[index.SortedDocValues] {
	return func(ctx index.LeafReader) (index.SortedDocValues, error) {
		return index.GetSorted(ctx, field)
	}
}

// SortedSetDocValues mirrors the static factory
// `static Function<SortedSetDocValues> sortedSetDocValues(String field)`.
func SortedSetDocValues(field string) DocValuesTermsCollectorFunction[index.SortedSetDocValues] {
	return func(ctx index.LeafReader) (index.SortedSetDocValues, error) {
		return index.GetSortedSet(ctx, field)
	}
}
