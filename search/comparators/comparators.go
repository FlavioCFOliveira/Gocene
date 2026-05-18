// Package comparators hosts the Sprint 51 ports for
// org.apache.lucene.search.comparators.
package comparators

// The Sprint 51 search.comparators-module port surfaces these types as
// typed stubs so dependent packages keep compiling; concrete behaviour
// ports (BKD-aware skip-list comparator with hit-queue integration,
// double/float/int/long primitive specialisations, ord-based sorted-
// doc-values comparator) land progressively in follow-up deep-port
// sprints.

// DocComparator mirrors
// org.apache.lucene.search.comparators.DocComparator.
type DocComparator struct{}

// NewDocComparator builds a DocComparator.
func NewDocComparator() *DocComparator { return &DocComparator{} }

// DoubleComparator mirrors
// org.apache.lucene.search.comparators.DoubleComparator.
type DoubleComparator struct{}

// NewDoubleComparator builds a DoubleComparator.
func NewDoubleComparator() *DoubleComparator { return &DoubleComparator{} }

// FloatComparator mirrors
// org.apache.lucene.search.comparators.FloatComparator.
type FloatComparator struct{}

// NewFloatComparator builds a FloatComparator.
func NewFloatComparator() *FloatComparator { return &FloatComparator{} }

// IntComparator mirrors
// org.apache.lucene.search.comparators.IntComparator.
type IntComparator struct{}

// NewIntComparator builds an IntComparator.
func NewIntComparator() *IntComparator { return &IntComparator{} }

// LongComparator mirrors
// org.apache.lucene.search.comparators.LongComparator.
type LongComparator struct{}

// NewLongComparator builds a LongComparator.
func NewLongComparator() *LongComparator { return &LongComparator{} }

// NumericComparator mirrors
// org.apache.lucene.search.comparators.NumericComparator.
type NumericComparator struct{}

// NewNumericComparator builds a NumericComparator.
func NewNumericComparator() *NumericComparator { return &NumericComparator{} }

// TermOrdValComparator mirrors
// org.apache.lucene.search.comparators.TermOrdValComparator.
type TermOrdValComparator struct{}

// NewTermOrdValComparator builds a TermOrdValComparator.
func NewTermOrdValComparator() *TermOrdValComparator { return &TermOrdValComparator{} }
