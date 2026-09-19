// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/join/src/java/org/apache/lucene/search/join/GenericTermsCollector.java

package join

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GenericTermsCollector mirrors the interface
// org.apache.lucene.search.join.GenericTermsCollector, which extends Collector.
type GenericTermsCollector interface {
	search.Collector

	// GetCollectedTerms mirrors `BytesRefHash getCollectedTerms()`.
	GetCollectedTerms() *util.BytesRefHash

	// GetScoresPerTerm mirrors `float[] getScoresPerTerm()`.
	GetScoresPerTerm() []float32
}

// CreateCollectorMV mirrors
// `static GenericTermsCollector createCollectorMV(Function<SortedSetDocValues> mvFunction, ScoreMode mode)`.
func CreateCollectorMV(mvFunction DocValuesTermsCollectorFunction[index.SortedSetDocValues], mode ScoreMode) GenericTermsCollector {
	switch mode {
	case None:
		return Wrap(NewTermsCollectorMV(mvFunction))
	case Avg:
		return NewTermsWithScoreCollectorMVAvg(mvFunction)
	case Max, Min, Total:
		fallthrough
	default:
		return NewTermsWithScoreCollectorMV(mvFunction, mode)
	}
}

// Verbose mirrors
// `static Function<SortedSetDocValues> verbose(PrintStream out, Function<SortedSetDocValues> mvFunction)`.
//
// Java's PrintStream is rendered as an io.Writer, the Go stream contract that
// carries the same responsibility.
func Verbose(out io.Writer, mvFunction DocValuesTermsCollectorFunction[index.SortedSetDocValues]) DocValuesTermsCollectorFunction[index.SortedSetDocValues] {
	return func(ctx index.LeafReader) (index.SortedSetDocValues, error) {
		target, err := mvFunction(ctx)
		if err != nil {
			return nil, err
		}
		return &verboseSortedSetDocValues{out: out, target: target}, nil
	}
}

// verboseSortedSetDocValues renders the anonymous SortedSetDocValues subclass
// returned by GenericTermsCollector.verbose, which traces every call it
// forwards to its target.
//
// Java's PrintStream.println swallows an I/O failure into an internal error
// flag instead of throwing, so the write results below are deliberately not
// propagated: that is exactly what the Java code does.
type verboseSortedSetDocValues struct {
	out    io.Writer
	target index.SortedSetDocValues
}

// DocID mirrors `public int docID()`.
func (v *verboseSortedSetDocValues) DocID() int { return v.target.DocID() }

// NextDoc mirrors `public int nextDoc()`.
func (v *verboseSortedSetDocValues) NextDoc() (int, error) {
	docID, err := v.target.NextDoc()
	if err != nil {
		return 0, err
	}
	fmt.Fprintln(v.out, "\nnextDoc doc# "+fmt.Sprint(docID))
	return docID, nil
}

// Advance mirrors `public int advance(int dest)`.
func (v *verboseSortedSetDocValues) Advance(dest int) (int, error) {
	docID, err := v.target.Advance(dest)
	if err != nil {
		return 0, err
	}
	fmt.Fprintln(v.out, "\nadvance("+fmt.Sprint(dest)+") -> doc# "+fmt.Sprint(docID))
	return docID, nil
}

// AdvanceExact mirrors `public boolean advanceExact(int dest)`.
func (v *verboseSortedSetDocValues) AdvanceExact(dest int) (bool, error) {
	exists, err := v.target.AdvanceExact(dest)
	if err != nil {
		return false, err
	}
	fmt.Fprintln(v.out, "\nadvanceExact("+fmt.Sprint(dest)+") -> exists# "+fmt.Sprint(exists))
	return exists, nil
}

// Cost mirrors `public long cost()`.
func (v *verboseSortedSetDocValues) Cost() int64 { return v.target.Cost() }

// NextOrd mirrors `public long nextOrd()`.
func (v *verboseSortedSetDocValues) NextOrd() (int, error) { return v.target.NextOrd() }

// DocValueCount mirrors `public int docValueCount()`.
func (v *verboseSortedSetDocValues) DocValueCount() int { return v.target.DocValueCount() }

// LookupOrd mirrors `public BytesRef lookupOrd(long ord)`.
func (v *verboseSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	val, err := v.target.LookupOrd(ord)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(v.out, util.WrapBytes(val).String()+", ")
	return val, nil
}

// GetValueCount mirrors `public long getValueCount()`.
func (v *verboseSortedSetDocValues) GetValueCount() int { return v.target.GetValueCount() }

// IntoBitSet carries the body DocIdSetIterator.intoBitSet(int, FixedBitSet, int)
// gives the anonymous Java subclass, which does not override it. The inherited
// body dispatches back to docID() and nextDoc(), so it observes the overrides
// above exactly as Java does.
func (v *verboseSortedSetDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(v, upTo, bitSet, offset)
}

// DocIDRunEnd carries the body DocIdSetIterator.docIDRunEnd() gives the
// anonymous Java subclass, which does not override it.
func (v *verboseSortedSetDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(v)
}

// CreateCollectorSV mirrors
// `static GenericTermsCollector createCollectorSV(Function<SortedDocValues> svFunction, ScoreMode mode)`.
func CreateCollectorSV(svFunction DocValuesTermsCollectorFunction[index.SortedDocValues], mode ScoreMode) GenericTermsCollector {
	switch mode {
	case None:
		return Wrap(NewTermsCollectorSV(svFunction))
	case Avg:
		return NewTermsWithScoreCollectorSVAvg(svFunction)
	case Max, Min, Total:
		fallthrough
	default:
		return NewTermsWithScoreCollectorSV(svFunction, mode)
	}
}

// Wrap mirrors `static GenericTermsCollector wrap(final TermsCollector<?> collector)`.
func Wrap(collector TermsCollector) GenericTermsCollector {
	return &wrappedTermsCollector{collector: collector}
}

// wrappedTermsCollector renders the anonymous GenericTermsCollector returned by
// GenericTermsCollector.wrap.
type wrappedTermsCollector struct {
	// BaseCollector carries the default body of Collector.setWeight(Weight),
	// which the anonymous Java class does not override.
	search.BaseCollector

	collector TermsCollector
}

// GetLeafCollector mirrors `public LeafCollector getLeafCollector(LeafReaderContext context)`.
func (w *wrappedTermsCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	return w.collector.GetLeafCollector(context)
}

// ScoreMode mirrors `public org.apache.lucene.search.ScoreMode scoreMode()`.
func (w *wrappedTermsCollector) ScoreMode() search.ScoreMode {
	return w.collector.ScoreMode()
}

// GetCollectedTerms mirrors `public BytesRefHash getCollectedTerms()`.
func (w *wrappedTermsCollector) GetCollectedTerms() *util.BytesRefHash {
	return w.collector.GetCollectorTerms()
}

// GetScoresPerTerm mirrors `public float[] getScoresPerTerm()`, whose body is
//
//	throw new UnsupportedOperationException("scores are not available for " + collector);
//
// The Java exception is unchecked and the method signature carries no way to
// report it, so the port raises it as a panic, as it does elsewhere for
// UnsupportedOperationException. Java interpolates Object.toString(), which has
// no Go counterpart; the collector's type is printed instead.
func (w *wrappedTermsCollector) GetScoresPerTerm() []float32 {
	panic(fmt.Sprintf("UnsupportedOperationException: scores are not available for %T", w.collector))
}

// interface compliance
var (
	_ GenericTermsCollector    = (*wrappedTermsCollector)(nil)
	_ index.SortedSetDocValues = (*verboseSortedSetDocValues)(nil)
)
