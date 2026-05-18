// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file holds the seven private IntersectVisitor factories
// SpatialQuery uses to drive its BKD tree walks. Each factory
// mirrors the analogously-named static helper on the Java reference
// org.apache.lucene.document.SpatialQuery (Lucene 10.4.0).
//
// All factories follow the same shape:
//
//   - capture the SpatialVisitor.innerFunction / leafPredicate
//     closures up front so per-leaf dispatch stays branch-only;
//   - return a struct that implements spatialIntersectVisitor and
//     forwards the seven hooks (Visit, VisitWithPackedValue,
//     VisitIterator, VisitIteratorWithPackedValue, Grow, Compare)
//     to the captured state;
//   - never panic — the Java reference panics with
//     UnsupportedOperationException in spots where the visitor
//     cannot meaningfully handle a hook; Gocene returns errSpatialNotApplicable
//     instead so the caller can decide what to do.
//
// The visitors that own a *util.DocIdSetBuilder or *util.FixedBitSet
// are not safe for concurrent use; SpatialQuery.getScorerSupplier
// hands one out per Get call (the bitsets are leaf-local) which
// matches the Java reference's contract.

// newEstimateVisitor mirrors SpatialQuery.getEstimateVisitor.
// Compare is the only hook the cost estimator actually uses; the
// two visit hooks panic in Java and return errSpatialNotApplicable
// here to stay safe for tests that drive the visitor in unusual
// orders.
func newEstimateVisitor(visitor SpatialVisitor, queryRelation document.QueryRelation) spatialIntersectVisitor {
	innerFn := visitor.GetInnerFunction(queryRelation)
	return &estimateVisitor{innerFn: innerFn}
}

type estimateVisitor struct {
	innerFn func(min, max []byte) spatialRelation
}

func (v *estimateVisitor) Visit(_ int) error                          { return errSpatialNotApplicable }
func (v *estimateVisitor) VisitWithPackedValue(_ int, _ []byte) error { return errSpatialNotApplicable }
func (v *estimateVisitor) VisitIterator(_ util.DocIdSetIterator) error {
	return errSpatialNotApplicable
}
func (v *estimateVisitor) VisitIteratorWithPackedValue(_ util.DocIdSetIterator, _ []byte) error {
	return errSpatialNotApplicable
}
func (v *estimateVisitor) Grow(_ int) {}
func (v *estimateVisitor) Compare(min, max []byte) spatialRelation {
	return v.innerFn(min, max)
}

// errSpatialNotApplicable is returned by visitor hooks that are
// invoked outside the contract the Java reference guarantees. The
// Java reference panics with UnsupportedOperationException in the
// same situation; the error variant keeps Gocene tests in control
// of the failure mode.
var errSpatialNotApplicable = errSpatialUnsupportedRelation

// newSparseVisitor mirrors SpatialQuery.getSparseVisitor. Used by
// INTERSECTS when the number of docs <= 4 * number of points, and
// the matcher pushes hits into a DocIdSetBuilder.
func newSparseVisitor(
	visitor SpatialVisitor,
	queryRelation document.QueryRelation,
	result *util.DocIdSetBuilder,
) spatialIntersectVisitor {
	return &sparseVisitor{
		innerFn:       visitor.GetInnerFunction(queryRelation),
		leafPredicate: visitor.GetLeafPredicate(queryRelation),
		result:        result,
	}
}

type sparseVisitor struct {
	innerFn       func(min, max []byte) spatialRelation
	leafPredicate func(packed []byte) bool
	result        *util.DocIdSetBuilder
	adder         util.BulkAdder
}

func (v *sparseVisitor) ensureAdder() {
	if v.adder == nil {
		v.adder = v.result.Grow(0)
	}
}

func (v *sparseVisitor) Grow(count int) { v.adder = v.result.Grow(count) }

func (v *sparseVisitor) Visit(docID int) error {
	v.ensureAdder()
	v.adder.Add(docID)
	return nil
}

func (v *sparseVisitor) VisitWithPackedValue(docID int, packed []byte) error {
	if v.leafPredicate(packed) {
		return v.Visit(docID)
	}
	return nil
}

func (v *sparseVisitor) VisitIterator(iter util.DocIdSetIterator) error {
	v.ensureAdder()
	return v.adder.AddIterator(iter)
}

func (v *sparseVisitor) VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packed []byte) error {
	if v.leafPredicate(packed) {
		return v.VisitIterator(iter)
	}
	return nil
}

func (v *sparseVisitor) Compare(min, max []byte) spatialRelation {
	return v.innerFn(min, max)
}

// newIntersectsDenseVisitor mirrors
// SpatialQuery.getIntersectsDenseVisitor. Used by INTERSECTS when
// the number of points > 4 * number of docs; the matcher pushes
// hits into a FixedBitSet so already-visited docs can be skipped
// on subsequent visits.
func newIntersectsDenseVisitor(
	visitor SpatialVisitor,
	queryRelation document.QueryRelation,
	result *util.FixedBitSet,
	cost []int64,
) spatialIntersectVisitor {
	return &intersectsDenseVisitor{
		innerFn:       visitor.GetInnerFunction(queryRelation),
		leafPredicate: visitor.GetLeafPredicate(queryRelation),
		result:        result,
		cost:          cost,
	}
}

type intersectsDenseVisitor struct {
	innerFn       func(min, max []byte) spatialRelation
	leafPredicate func(packed []byte) bool
	result        *util.FixedBitSet
	cost          []int64
}

func (v *intersectsDenseVisitor) Grow(_ int) {}

func (v *intersectsDenseVisitor) Visit(docID int) error {
	v.result.Set(docID)
	v.cost[0]++
	return nil
}

func (v *intersectsDenseVisitor) VisitWithPackedValue(docID int, packed []byte) error {
	if !v.result.Get(docID) {
		if v.leafPredicate(packed) {
			return v.Visit(docID)
		}
	}
	return nil
}

func (v *intersectsDenseVisitor) VisitIterator(iter util.DocIdSetIterator) error {
	// Approximate the Java visit(DocIdSetIterator) bulk by draining
	// the iterator per-doc; util.FixedBitSet exposes Or only for
	// other bitsets, and the per-doc Or is correct (every doc that
	// matches gets its bit set and is counted).
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			return nil
		}
		if err := v.Visit(doc); err != nil {
			return err
		}
	}
}

func (v *intersectsDenseVisitor) VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packed []byte) error {
	if !v.leafPredicate(packed) {
		// Skip iterator entirely — the cell does not match the
		// predicate, so none of its docs do.
		return drainDocIDs(iter)
	}
	return v.VisitIterator(iter)
}

func (v *intersectsDenseVisitor) Compare(min, max []byte) spatialRelation {
	return v.innerFn(min, max)
}

// newDenseVisitor mirrors SpatialQuery.getDenseVisitor. Used by
// WITHIN and DISJOINT when there are multivalued docs; the visitor
// owns both a "result" bitset (docs that may match) and an
// "excluded" bitset (docs that definitely do not).
func newDenseVisitor(
	visitor SpatialVisitor,
	queryRelation document.QueryRelation,
	result *util.FixedBitSet,
	excluded *util.FixedBitSet,
	cost []int64,
) spatialIntersectVisitor {
	return &denseVisitor{
		innerFn:       visitor.GetInnerFunction(queryRelation),
		leafPredicate: visitor.GetLeafPredicate(queryRelation),
		result:        result,
		excluded:      excluded,
		cost:          cost,
	}
}

type denseVisitor struct {
	innerFn       func(min, max []byte) spatialRelation
	leafPredicate func(packed []byte) bool
	result        *util.FixedBitSet
	excluded      *util.FixedBitSet
	cost          []int64
}

func (v *denseVisitor) Grow(_ int) {}

func (v *denseVisitor) Visit(docID int) error {
	v.result.Set(docID)
	v.cost[0]++
	return nil
}

func (v *denseVisitor) VisitWithPackedValue(docID int, packed []byte) error {
	if !v.excluded.Get(docID) {
		if v.leafPredicate(packed) {
			return v.Visit(docID)
		}
		v.excluded.Set(docID)
	}
	return nil
}

func (v *denseVisitor) VisitIterator(iter util.DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			return nil
		}
		if err := v.Visit(doc); err != nil {
			return err
		}
	}
}

func (v *denseVisitor) VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packed []byte) error {
	if v.leafPredicate(packed) {
		return v.VisitIterator(iter)
	}
	// Exclude every doc the iterator yields — matches the Java
	// reference's excluded.or(iterator) bulk shortcut.
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			return nil
		}
		v.excluded.Set(doc)
	}
}

func (v *denseVisitor) Compare(min, max []byte) spatialRelation {
	return v.innerFn(min, max)
}

// newContainsDenseVisitor mirrors
// SpatialQuery.getContainsDenseVisitor. Used by CONTAINS; the
// visitor classifies each doc as CANDIDATE / NOTWITHIN / DISJOINT
// and routes the result through the result / excluded bitsets.
func newContainsDenseVisitor(
	visitor SpatialVisitor,
	queryRelation document.QueryRelation,
	result *util.FixedBitSet,
	excluded *util.FixedBitSet,
	cost []int64,
) spatialIntersectVisitor {
	return &containsDenseVisitor{
		innerFn:      visitor.GetInnerFunction(queryRelation),
		leafFunction: visitor.Contains(),
		result:       result,
		excluded:     excluded,
		cost:         cost,
	}
}

type containsDenseVisitor struct {
	innerFn      func(min, max []byte) spatialRelation
	leafFunction func(packed []byte) geo.WithinRelation
	result       *util.FixedBitSet
	excluded     *util.FixedBitSet
	cost         []int64
}

func (v *containsDenseVisitor) Grow(_ int) {}

func (v *containsDenseVisitor) Visit(docID int) error {
	v.excluded.Set(docID)
	return nil
}

func (v *containsDenseVisitor) VisitWithPackedValue(docID int, packed []byte) error {
	if v.excluded.Get(docID) {
		return nil
	}
	switch v.leafFunction(packed) {
	case geo.WithinCandidate:
		v.cost[0]++
		v.result.Set(docID)
	case geo.WithinNotWithin:
		v.excluded.Set(docID)
	}
	return nil
}

func (v *containsDenseVisitor) VisitIterator(iter util.DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			return nil
		}
		v.excluded.Set(doc)
	}
}

func (v *containsDenseVisitor) VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packed []byte) error {
	within := v.leafFunction(packed)
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			return nil
		}
		switch within {
		case geo.WithinCandidate:
			v.cost[0]++
			v.result.Set(doc)
		case geo.WithinNotWithin:
			v.excluded.Set(doc)
		}
	}
}

func (v *containsDenseVisitor) Compare(min, max []byte) spatialRelation {
	return v.innerFn(min, max)
}

// newInverseDenseVisitor mirrors
// SpatialQuery.getInverseDenseVisitor. Used by WITHIN and DISJOINT
// when every doc has a value; the visitor walks the tree and
// clears the bitset for every doc whose cell does not match.
func newInverseDenseVisitor(
	visitor SpatialVisitor,
	queryRelation document.QueryRelation,
	result *util.FixedBitSet,
	cost []int64,
) spatialIntersectVisitor {
	innerFn := visitor.GetInnerFunction(queryRelation)
	return &inverseDenseVisitor{
		innerFn:       innerFn,
		leafPredicate: visitor.GetLeafPredicate(queryRelation),
		result:        result,
		cost:          cost,
	}
}

type inverseDenseVisitor struct {
	innerFn       func(min, max []byte) spatialRelation
	leafPredicate func(packed []byte) bool
	result        *util.FixedBitSet
	cost          []int64
}

func (v *inverseDenseVisitor) Grow(_ int) {}

func (v *inverseDenseVisitor) Visit(docID int) error {
	v.result.Clear(docID)
	v.cost[0]--
	return nil
}

func (v *inverseDenseVisitor) VisitWithPackedValue(docID int, packed []byte) error {
	if v.result.Get(docID) {
		if !v.leafPredicate(packed) {
			return v.Visit(docID)
		}
	}
	return nil
}

func (v *inverseDenseVisitor) VisitIterator(iter util.DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			return nil
		}
		v.result.Clear(doc)
		if v.cost[0] > 0 {
			v.cost[0]--
		}
	}
}

func (v *inverseDenseVisitor) VisitIteratorWithPackedValue(iter util.DocIdSetIterator, packed []byte) error {
	if !v.leafPredicate(packed) {
		return v.VisitIterator(iter)
	}
	return drainDocIDs(iter)
}

func (v *inverseDenseVisitor) Compare(min, max []byte) spatialRelation {
	// Transpose: this visitor's compare is invoked with the
	// already-transposed relation by callers that own the
	// QueryRelation transposition; the Java reference applies the
	// transpose here for symmetry.
	return transposeSpatialRelation(v.innerFn(min, max))
}

// newShallowInverseDenseVisitor mirrors
// SpatialQuery.getShallowInverseDenseVisitor. Used by WITHIN and
// DISJOINT to take inner-node compare decisions into account
// without touching the leaf-level visit hooks (leaf cells are
// already handled by the dense visitor's first pass).
func newShallowInverseDenseVisitor(
	visitor SpatialVisitor,
	queryRelation document.QueryRelation,
	result *util.FixedBitSet,
) spatialIntersectVisitor {
	return &shallowInverseDenseVisitor{
		innerFn: visitor.GetInnerFunction(queryRelation),
		result:  result,
	}
}

type shallowInverseDenseVisitor struct {
	innerFn func(min, max []byte) spatialRelation
	result  *util.FixedBitSet
}

func (v *shallowInverseDenseVisitor) Grow(_ int) {}

func (v *shallowInverseDenseVisitor) Visit(docID int) error {
	v.result.Clear(docID)
	return nil
}

func (v *shallowInverseDenseVisitor) VisitWithPackedValue(_ int, _ []byte) error {
	// Leaf-level hits are explicitly ignored by this visitor.
	return nil
}

func (v *shallowInverseDenseVisitor) VisitIterator(iter util.DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			return nil
		}
		v.result.Clear(doc)
	}
}

func (v *shallowInverseDenseVisitor) VisitIteratorWithPackedValue(_ util.DocIdSetIterator, _ []byte) error {
	// Leaf-level hits are explicitly ignored by this visitor.
	return nil
}

func (v *shallowInverseDenseVisitor) Compare(min, max []byte) spatialRelation {
	return transposeSpatialRelation(v.innerFn(min, max))
}

// drainDocIDs walks iter to completion without dispatching any
// per-doc hook. Used by visitors that want to skip every doc in a
// bulk iterator without accumulating them into a bitset.
func drainDocIDs(iter util.DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == util.NO_MORE_DOCS {
			return nil
		}
	}
}
