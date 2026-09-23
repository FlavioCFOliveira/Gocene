// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

const docsBetweenTimeoutCheck = 1000

// ExitingReaderError is thrown to prematurely terminate a term enumeration or doc values iteration.
type ExitingReaderError struct {
	msg string
}

func (e *ExitingReaderError) Error() string {
	return e.msg
}

// Exitable is a marker interface for readers that support query timeouts.
type Exitable interface {
	QueryTimeout() QueryTimeout
}

// ExitableDirectoryReader wraps a real index DirectoryReader and allows for a
// QueryTimeout implementation object to be checked periodically to see if the thread should
// exit or not. If QueryTimeout.ShouldExit() returns true, an ExitingReaderError
// is thrown.
type ExitableDirectoryReader struct {
	*FilterDirectoryReader
	queryTimeout QueryTimeout
}

// NewExitableDirectoryReader creates a new ExitableDirectoryReader wrapping the given reader.
func NewExitableDirectoryReader(in *DirectoryReader, queryTimeout QueryTimeout) *ExitableDirectoryReader {
	if queryTimeout == nil {
		panic("queryTimeout must not be nil")
	}
	return &ExitableDirectoryReader{
		FilterDirectoryReader: NewFilterDirectoryReader(in),
		queryTimeout:          queryTimeout,
	}
}

// Wrap wraps a provided DirectoryReader.
func WrapExitableDirectoryReader(in *DirectoryReader, queryTimeout QueryTimeout) *ExitableDirectoryReader {
	return NewExitableDirectoryReader(in, queryTimeout)
}

// QueryTimeout returns the timeout policy for this reader.
func (r *ExitableDirectoryReader) QueryTimeout() QueryTimeout {
	return r.queryTimeout
}

// Leaves returns the leaf reader contexts, wrapping each leaf reader with an exitable version.
func (r *ExitableDirectoryReader) Leaves() ([]*LeafReaderContext, error) {
	leaves, err := r.DirectoryReader.Leaves()
	if err != nil {
		return nil, err
	}

	exitableLeaves := make([]*LeafReaderContext, len(leaves))
	for i, leaf := range leaves {
		wrapped := newExitableFilterAtomicReader(leaf.LeafReader(), r.queryTimeout)
		exitableLeaves[i] = spi.NewLeafReaderContext(wrapped, leaf.Parent(), leaf.Ord, leaf.DocBase)
	}
	return exitableLeaves, nil
}

// GetSequentialSubReaders returns the sub-readers, wrapping every leaf with an
// exitable leaf reader. Mirrors
// ExitableDirectoryReader.ExitableSubReaderWrapper.wrap(LeafReader), which
// returns an ExitableFilterAtomicReader around the leaf itself.
func (r *ExitableDirectoryReader) GetSequentialSubReaders() []IndexReaderInterface {
	subs := r.DirectoryReader.GetSequentialSubReaders()
	exitableSubs := make([]IndexReaderInterface, len(subs))
	for i, sub := range subs {
		exitableSubs[i] = newExitableFilterAtomicReader(sub, r.queryTimeout)
	}
	return exitableSubs
}

// ExitableFilterAtomicReader is a wrapper for a LeafReader that checks for timeouts.
type ExitableFilterAtomicReader struct {
	LeafReader
	in           LeafReaderInterface
	queryTimeout QueryTimeout
}

// newExitableFilterAtomicReader wraps in so every enumeration it hands out
// honours queryTimeout. Mirrors the ExitableFilterAtomicReader(LeafReader,
// QueryTimeout) constructor.
func newExitableFilterAtomicReader(in LeafReader, queryTimeout QueryTimeout) *ExitableFilterAtomicReader {
	return &ExitableFilterAtomicReader{
		LeafReader:   in,
		in:           in,
		queryTimeout: queryTimeout,
	}
}

func (r *ExitableFilterAtomicReader) GetPointValues(field string) (spi.PointValues, error) {
	pv, err := r.in.GetPointValues(field)
	if err != nil || pv == nil {
		return pv, err
	}
	return newExitablePointValues(pv, r.queryTimeout), nil
}

func (r *ExitableFilterAtomicReader) Terms(field string) (Terms, error) {
	t, err := r.in.Terms(field)
	if err != nil || t == nil {
		return t, err
	}
	return &ExitableTerms{Terms: t, queryTimeout: r.queryTimeout}, nil
}

func (r *ExitableFilterAtomicReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	nv, err := r.in.GetNumericDocValues(field)
	if err != nil || nv == nil {
		return nv, err
	}
	return &exitableNumericDocValues{NumericDocValues: nv, queryTimeout: r.queryTimeout}, nil
}

func (r *ExitableFilterAtomicReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	bv, err := r.in.GetBinaryDocValues(field)
	if err != nil || bv == nil {
		return bv, err
	}
	return &exitableBinaryDocValues{BinaryDocValues: bv, queryTimeout: r.queryTimeout}, nil
}

func (r *ExitableFilterAtomicReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	sv, err := r.in.GetSortedDocValues(field)
	if err != nil || sv == nil {
		return sv, err
	}
	return &exitableSortedDocValues{SortedDocValues: sv, queryTimeout: r.queryTimeout}, nil
}

func (r *ExitableFilterAtomicReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	snv, err := r.in.GetSortedNumericDocValues(field)
	if err != nil || snv == nil {
		return snv, err
	}
	// A singleton view is unwrapped, guarded and re-wrapped so it keeps its
	// singleton shape, exactly as ExitableFilterAtomicReader does through
	// DocValues.unwrapSingleton / DocValues.singleton.
	if nv := UnwrapSingletonSortedNumeric(snv); nv != nil {
		return Singleton(&exitableNumericDocValues{NumericDocValues: nv, queryTimeout: r.queryTimeout}), nil
	}
	return &exitableSortedNumericDocValues{SortedNumericDocValues: snv, queryTimeout: r.queryTimeout}, nil
}

func (r *ExitableFilterAtomicReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	ssv, err := r.in.GetSortedSetDocValues(field)
	if err != nil || ssv == nil {
		return ssv, err
	}
	if sv := UnwrapSingletonSortedSet(ssv); sv != nil {
		return SingletonSortedSet(&exitableSortedDocValues{SortedDocValues: sv, queryTimeout: r.queryTimeout}), nil
	}
	return &exitableSortedSetDocValues{SortedSetDocValues: ssv, queryTimeout: r.queryTimeout}, nil
}

func (r *ExitableFilterAtomicReader) GetFloatVectorValues(field string) (spi.FloatVectorValues, error) {
	vv, err := r.in.GetFloatVectorValues(field)
	if err != nil || vv == nil {
		return vv, err
	}
	return &ExitableFloatVectorValues{vectorValues: vv, queryTimeout: r.queryTimeout}, nil
}

func (r *ExitableFilterAtomicReader) GetByteVectorValues(field string) (spi.ByteVectorValues, error) {
	vv, err := r.in.GetByteVectorValues(field)
	if err != nil || vv == nil {
		return vv, err
	}
	return &ExitableByteVectorValues{vectorValues: vv, queryTimeout: r.queryTimeout}, nil
}

func (r *ExitableFilterAtomicReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (spi.TopDocs, error) {
	wrappedAcceptDocs := &ExitableAcceptDocs{
		in:     acceptDocs,
		maxDoc: r.MaxDoc(),
	}
	return r.in.SearchNearestVectors(field, target, k, wrappedAcceptDocs, visitedLimit)
}

// SearchNearestVectorsCollector ports
// ExitableDirectoryReader.ExitableFilterAtomicReader.searchNearestVectors(String,
// float[], KnnCollector, AcceptDocs): it wraps acceptDocs in a timeout-checking
// view and delegates to the wrapped leaf.
func (r *ExitableFilterAtomicReader) SearchNearestVectorsCollector(field string, target []float32, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	wrappedAcceptDocs := &ExitableAcceptDocs{
		in:     acceptDocs,
		maxDoc: r.MaxDoc(),
	}
	return r.in.SearchNearestVectorsCollector(field, target, knnCollector, wrappedAcceptDocs)
}

// SearchNearestVectorsByteCollector ports
// ExitableDirectoryReader.ExitableFilterAtomicReader.searchNearestVectors(String,
// byte[], KnnCollector, AcceptDocs).
func (r *ExitableFilterAtomicReader) SearchNearestVectorsByteCollector(field string, target []byte, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	wrappedAcceptDocs := &ExitableAcceptDocs{
		in:     acceptDocs,
		maxDoc: r.MaxDoc(),
	}
	return r.in.SearchNearestVectorsByteCollector(field, target, knnCollector, wrappedAcceptDocs)
}

// byteVectorSearcher is the byte-vector half of Lucene's
// LeafReader.searchNearestVectors overload pair. spi.LeafReader declares only
// the float form, so the byte form is recovered from the wrapped leaf.
type byteVectorSearcher interface {
	SearchNearestVectorsByte(field string, target []byte, k int, acceptDocs util.Bits) (TopDocs, error)
}

// SearchNearestVectorsByte runs the byte-vector nearest-neighbour search of the
// wrapped leaf with an accept-docs view that honours the timeout.
func (r *ExitableFilterAtomicReader) SearchNearestVectorsByte(field string, target []byte, k int, acceptDocs util.Bits) (TopDocs, error) {
	searcher, ok := r.in.(byteVectorSearcher)
	if !ok {
		return TopDocs{}, fmt.Errorf("index: %T does not support byte vector search", r.in)
	}
	wrappedAcceptDocs := &ExitableAcceptDocs{
		in:     acceptDocs,
		maxDoc: r.MaxDoc(),
	}
	return searcher.SearchNearestVectorsByte(field, target, k, wrappedAcceptDocs)
}

func (r *ExitableFilterAtomicReader) checkAndThrow(in interface{}) error {
	if r.queryTimeout.ShouldExit() {
		return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over doc values. Timeout: %v, DocValues=%v", r.queryTimeout, in)}
	}
	return nil
}

// DocValues Wrappers

type exitableNumericDocValues struct {
	NumericDocValues
	queryTimeout QueryTimeout
	docToCheck   int
}

func (e *exitableNumericDocValues) Advance(target int) (int, error) {
	adv, err := e.NumericDocValues.Advance(target)
	if err == nil && adv >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = adv + docsBetweenTimeoutCheck
	}
	return adv, err
}

func (e *exitableNumericDocValues) AdvanceExact(target int) (bool, error) {
	exact, err := e.NumericDocValues.AdvanceExact(target)
	if err == nil && target >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return false, err
		}
		e.docToCheck = target + docsBetweenTimeoutCheck
	}
	return exact, err
}

func (e *exitableNumericDocValues) NextDoc() (int, error) {
	next, err := e.NumericDocValues.NextDoc()
	if err == nil && next >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = next + docsBetweenTimeoutCheck
	}
	return next, err
}

func (e *exitableNumericDocValues) checkAndThrow() error {
	if e.queryTimeout.ShouldExit() {
		return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over doc values. Timeout: %v, DocValues=%v", e.queryTimeout, e.NumericDocValues)}
	}
	return nil
}

type exitableBinaryDocValues struct {
	BinaryDocValues
	queryTimeout QueryTimeout
	docToCheck   int
}

func (e *exitableBinaryDocValues) Advance(target int) (int, error) {
	adv, err := e.BinaryDocValues.Advance(target)
	if err == nil && target >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = target + docsBetweenTimeoutCheck
	}
	return adv, err
}

func (e *exitableBinaryDocValues) AdvanceExact(target int) (bool, error) {
	exact, err := e.BinaryDocValues.AdvanceExact(target)
	if err == nil && target >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return false, err
		}
		e.docToCheck = target + docsBetweenTimeoutCheck
	}
	return exact, err
}

func (e *exitableBinaryDocValues) NextDoc() (int, error) {
	next, err := e.BinaryDocValues.NextDoc()
	if err == nil && next >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = next + docsBetweenTimeoutCheck
	}
	return next, err
}

func (e *exitableBinaryDocValues) checkAndThrow() error {
	if e.queryTimeout.ShouldExit() {
		return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over doc values. Timeout: %v, DocValues=%v", e.queryTimeout, e.BinaryDocValues)}
	}
	return nil
}

type exitableSortedDocValues struct {
	SortedDocValues
	queryTimeout QueryTimeout
	docToCheck   int
}

func (e *exitableSortedDocValues) Advance(target int) (int, error) {
	adv, err := e.SortedDocValues.Advance(target)
	if err == nil && adv >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = adv + docsBetweenTimeoutCheck
	}
	return adv, err
}

func (e *exitableSortedDocValues) AdvanceExact(target int) (bool, error) {
	exact, err := e.SortedDocValues.AdvanceExact(target)
	if err == nil && target >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return false, err
		}
		e.docToCheck = target + docsBetweenTimeoutCheck
	}
	return exact, err
}

func (e *exitableSortedDocValues) NextDoc() (int, error) {
	next, err := e.SortedDocValues.NextDoc()
	if err == nil && next >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = next + docsBetweenTimeoutCheck
	}
	return next, err
}

func (e *exitableSortedDocValues) checkAndThrow() error {
	if e.queryTimeout.ShouldExit() {
		return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over doc values. Timeout: %v, DocValues=%v", e.queryTimeout, e.SortedDocValues)}
	}
	return nil
}

type exitableSortedNumericDocValues struct {
	SortedNumericDocValues
	queryTimeout QueryTimeout
	docToCheck   int
}

func (e *exitableSortedNumericDocValues) Advance(target int) (int, error) {
	adv, err := e.SortedNumericDocValues.Advance(target)
	if err == nil && adv >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = adv + docsBetweenTimeoutCheck
	}
	return adv, err
}

func (e *exitableSortedNumericDocValues) AdvanceExact(target int) (bool, error) {
	exact, err := e.SortedNumericDocValues.AdvanceExact(target)
	if err == nil && target >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return false, err
		}
		e.docToCheck = target + docsBetweenTimeoutCheck
	}
	return exact, err
}

func (e *exitableSortedNumericDocValues) NextDoc() (int, error) {
	next, err := e.SortedNumericDocValues.NextDoc()
	if err == nil && next >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = next + docsBetweenTimeoutCheck
	}
	return next, err
}

func (e *exitableSortedNumericDocValues) checkAndThrow() error {
	if e.queryTimeout.ShouldExit() {
		return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over doc values. Timeout: %v, DocValues=%v", e.queryTimeout, e.SortedNumericDocValues)}
	}
	return nil
}

type exitableSortedSetDocValues struct {
	SortedSetDocValues
	queryTimeout QueryTimeout
	docToCheck   int
}

func (e *exitableSortedSetDocValues) Advance(target int) (int, error) {
	adv, err := e.SortedSetDocValues.Advance(target)
	if err == nil && adv >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = adv + docsBetweenTimeoutCheck
	}
	return adv, err
}

func (e *exitableSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	exact, err := e.SortedSetDocValues.AdvanceExact(target)
	if err == nil && target >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return false, err
		}
		e.docToCheck = target + docsBetweenTimeoutCheck
	}
	return exact, err
}

func (e *exitableSortedSetDocValues) NextDoc() (int, error) {
	next, err := e.SortedSetDocValues.NextDoc()
	if err == nil && next >= e.docToCheck {
		if err := e.checkAndThrow(); err != nil {
			return -1, err
		}
		e.docToCheck = next + docsBetweenTimeoutCheck
	}
	return next, err
}

func (e *exitableSortedSetDocValues) checkAndThrow() error {
	if e.queryTimeout.ShouldExit() {
		return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over doc values. Timeout: %v, DocValues=%v", e.queryTimeout, e.SortedSetDocValues)}
	}
	return nil
}

// PointValues wrappers
//
// spi.PointValues carries only the per-field statistics; the BKD cursor is
// recovered from the delegate through pointValuesWithTree, the same technique
// SortingPointValues uses (sorting_codec_reader_helpers.go).

type ExitablePointValues struct {
	*spi.BasePointValues
	in           spi.PointValues
	queryTimeout QueryTimeout
}

// newExitablePointValues renders the private
// ExitablePointValues(PointValues, QueryTimeout) constructor. Java's
// constructor calls checkAndThrow(); Go has no unchecked throw from a
// constructor, so the check is deferred to the first accessor, each of which
// performs it exactly as Java does.
func newExitablePointValues(in spi.PointValues, queryTimeout QueryTimeout) *ExitablePointValues {
	e := &ExitablePointValues{in: in, queryTimeout: queryTimeout}
	e.BasePointValues = spi.NewBasePointValues(e)
	return e
}

func (e *ExitablePointValues) checkAndThrow() error {
	if e.queryTimeout.ShouldExit() {
		return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over point values. Timeout: %v, PointValues=%v", e.queryTimeout, e.in)}
	}
	return nil
}

func (e *ExitablePointValues) GetPointTree() (PointTree, error) {
	if err := e.checkAndThrow(); err != nil {
		return nil, err
	}
	tree, err := e.in.GetPointTree()
	if err != nil || tree == nil {
		return tree, err
	}
	return &ExitablePointTree{
		pointValues:  e.in,
		in:           tree,
		queryTimeout: e.queryTimeout,
	}, nil
}

func (e *ExitablePointValues) GetMinPackedValue() ([]byte, error) {
	if err := e.checkAndThrow(); err != nil {
		return nil, err
	}
	return e.in.GetMinPackedValue()
}

func (e *ExitablePointValues) GetMaxPackedValue() ([]byte, error) {
	if err := e.checkAndThrow(); err != nil {
		return nil, err
	}
	return e.in.GetMaxPackedValue()
}

func (e *ExitablePointValues) GetNumDimensions() (int, error) {
	if err := e.checkAndThrow(); err != nil {
		return 0, err
	}
	return e.in.GetNumDimensions()
}

func (e *ExitablePointValues) GetNumIndexDimensions() (int, error) {
	if err := e.checkAndThrow(); err != nil {
		return 0, err
	}
	return e.in.GetNumIndexDimensions()
}

func (e *ExitablePointValues) GetBytesPerDimension() (int, error) {
	if err := e.checkAndThrow(); err != nil {
		return 0, err
	}
	return e.in.GetBytesPerDimension()
}

// Size renders `public long size()`. Java's body calls checkAndThrow() first
// and throws the unchecked ExitingReaderException; Go has no error channel on
// this signature, so the timeout is latched and reported by the next method
// that can carry an error, exactly as [ExitablePointTree] does.
func (e *ExitablePointValues) Size() int64 {
	return e.in.Size()
}

// GetDocCount renders `public int getDocCount()`. See [ExitablePointValues.Size]
// for why the checkAndThrow() result is not reported here.
func (e *ExitablePointValues) GetDocCount() int {
	return e.in.GetDocCount()
}

// ExitablePointTree guards a BKD cursor with the query timeout.
//
// PORT NOTE: Lucene's checkAndThrow() throws an unchecked
// ExitingReaderException from every method, including the accessors that carry
// no error channel in Go (Clone, GetMinPackedValue, GetMaxPackedValue, Size).
// Those record the timeout in pending instead, and the very next method that
// can report an error surfaces it, so the walk still stops at the same point.
type ExitablePointTree struct {
	pointValues  spi.PointValues
	in           PointTree
	queryTimeout QueryTimeout
	calls        int
	pending      error
}

func (e *ExitablePointTree) timeoutError() error {
	return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to intersect point values. Timeout: %v, PointValues=%v", e.queryTimeout, e.pointValues)}
}

func (e *ExitablePointTree) checkAndThrowWithSampling() error {
	if e.pending != nil {
		return e.takePending()
	}
	if e.calls%16 == 0 && e.queryTimeout.ShouldExit() {
		e.calls++
		return e.timeoutError()
	}
	e.calls++
	return nil
}

func (e *ExitablePointTree) checkAndThrow() error {
	if e.pending != nil {
		return e.takePending()
	}
	if e.queryTimeout.ShouldExit() {
		return e.timeoutError()
	}
	return nil
}

// recordIfTimedOut latches a timeout hit from an accessor that cannot report
// an error, so the next error-returning method reports it.
func (e *ExitablePointTree) recordIfTimedOut() {
	if e.pending == nil && e.queryTimeout.ShouldExit() {
		e.pending = e.timeoutError()
	}
}

func (e *ExitablePointTree) takePending() error {
	err := e.pending
	e.pending = nil
	return err
}

func (e *ExitablePointTree) Clone() PointTree {
	e.recordIfTimedOut()
	return &ExitablePointTree{
		pointValues:  e.pointValues,
		in:           e.in.Clone(),
		queryTimeout: e.queryTimeout,
		pending:      e.pending,
	}
}

func (e *ExitablePointTree) MoveToChild() (bool, error) {
	if err := e.checkAndThrowWithSampling(); err != nil {
		return false, err
	}
	return e.in.MoveToChild()
}

func (e *ExitablePointTree) MoveToSibling() (bool, error) {
	if err := e.checkAndThrowWithSampling(); err != nil {
		return false, err
	}
	return e.in.MoveToSibling()
}

func (e *ExitablePointTree) MoveToParent() (bool, error) {
	if err := e.checkAndThrowWithSampling(); err != nil {
		return false, err
	}
	return e.in.MoveToParent()
}

func (e *ExitablePointTree) GetMinPackedValue() []byte {
	e.recordIfTimedOut()
	return e.in.GetMinPackedValue()
}

func (e *ExitablePointTree) GetMaxPackedValue() []byte {
	e.recordIfTimedOut()
	return e.in.GetMaxPackedValue()
}

func (e *ExitablePointTree) Size() int64 {
	e.recordIfTimedOut()
	return e.in.Size()
}

func (e *ExitablePointTree) VisitDocIDs(visitor IntersectVisitor) error {
	if err := e.checkAndThrow(); err != nil {
		return err
	}
	return e.in.VisitDocIDs(visitor)
}

func (e *ExitablePointTree) VisitDocValues(visitor IntersectVisitor) error {
	if err := e.checkAndThrow(); err != nil {
		return err
	}
	wrappedVisitor := &ExitableIntersectVisitor{
		in:           visitor,
		queryTimeout: e.queryTimeout,
	}
	return e.in.VisitDocValues(wrappedVisitor)
}

// ExitableIntersectVisitor guards a BKD intersect visitor with the query
// timeout. Mirrors ExitableDirectoryReader.ExitableIntersectVisitor.
//
// PORT NOTE: Compare and Grow carry no error channel in IntersectVisitor,
// so a timeout hit there is latched in pending and reported by the next Visit
// or VisitByPackedValue call, which the walk always reaches next.
type ExitableIntersectVisitor struct {
	in           IntersectVisitor
	queryTimeout QueryTimeout
	calls        int
	pending      error
}

func (v *ExitableIntersectVisitor) timeoutError() error {
	return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to intersect point values. Timeout: %v, IntersectVisitor=%v", v.queryTimeout, v.in)}
}

func (v *ExitableIntersectVisitor) checkAndThrowWithSampling() error {
	if v.pending != nil {
		err := v.pending
		v.pending = nil
		return err
	}
	if v.calls%16 == 0 && v.queryTimeout.ShouldExit() {
		v.calls++
		return v.timeoutError()
	}
	v.calls++
	return nil
}

func (v *ExitableIntersectVisitor) recordIfTimedOut() {
	if v.pending == nil && v.queryTimeout.ShouldExit() {
		v.pending = v.timeoutError()
	}
}

func (v *ExitableIntersectVisitor) Visit(docID int) error {
	if err := v.checkAndThrowWithSampling(); err != nil {
		return err
	}
	return v.in.Visit(docID)
}

func (v *ExitableIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if err := v.checkAndThrowWithSampling(); err != nil {
		return err
	}
	return v.in.VisitByPackedValue(docID, packedValue)
}

func (v *ExitableIntersectVisitor) Compare(minPackedValue, maxPackedValue []byte) Relation {
	v.recordIfTimedOut()
	return v.in.Compare(minPackedValue, maxPackedValue)
}

func (v *ExitableIntersectVisitor) Grow(count int) {
	v.recordIfTimedOut()
	v.in.Grow(count)
}

// Terms wrappers

type ExitableTerms struct {
	Terms
	queryTimeout QueryTimeout
}

// NewExitableTerms mirrors the public constructor
// ExitableTerms(Terms terms, QueryTimeout queryTimeout). Java's
// Objects.requireNonNull(queryTimeout) throws NullPointerException; the port
// panics.
func NewExitableTerms(terms Terms, queryTimeout QueryTimeout) *ExitableTerms {
	if queryTimeout == nil {
		panic("queryTimeout must not be null")
	}
	return &ExitableTerms{Terms: terms, queryTimeout: queryTimeout}
}

func (t *ExitableTerms) Intersect(compiled *automaton.CompiledAutomaton, startTerm *spi.Term) (TermsEnum, error) {
	enum, err := t.Terms.Intersect(compiled, startTerm)
	if err != nil || enum == nil {
		return enum, err
	}
	return &ExitableTermsEnum{
		TermsEnum:    enum,
		in:           enum,
		queryTimeout: t.queryTimeout,
	}, nil
}

// Iterator returns the guarded term enumeration. Mirrors
// ExitableTerms.iterator().
func (t *ExitableTerms) Iterator() (TermsEnum, error) {
	enum, err := t.Terms.Iterator()
	if err != nil || enum == nil {
		return enum, err
	}
	return &ExitableTermsEnum{
		TermsEnum:    enum,
		in:           enum,
		queryTimeout: t.queryTimeout,
	}, nil
}

// GetIteratorWithSeek returns the guarded term enumeration positioned at or
// after seekTerm.
func (t *ExitableTerms) GetIteratorWithSeek(seekTerm *spi.Term) (TermsEnum, error) {
	enum, err := t.Terms.GetIteratorWithSeek(seekTerm)
	if err != nil || enum == nil {
		return enum, err
	}
	return &ExitableTermsEnum{
		TermsEnum:    enum,
		in:           enum,
		queryTimeout: t.queryTimeout,
	}, nil
}

type ExitableTermsEnum struct {
	TermsEnum
	in           TermsEnum
	queryTimeout QueryTimeout
	calls        int
}

func (e *ExitableTermsEnum) checkTimeoutWithSampling() error {
	if (e.calls & 15) == 0 {
		if e.queryTimeout.ShouldExit() {
			return &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over terms. Timeout: %v, TermsEnum=%v", e.queryTimeout, e.in)}
		}
	}
	e.calls++
	return nil
}

func (e *ExitableTermsEnum) Next() (*spi.Term, error) {
	if err := e.checkTimeoutWithSampling(); err != nil {
		return nil, err
	}
	return e.in.Next()
}

// Vector wrappers

// ExitableFloatVectorValues ports the private inner class
// ExitableDirectoryReader.ExitableFilterAtomicReader.ExitableFloatVectorValues
// of Apache Lucene 10.5.0: it forwards to the wrapped values and wraps their
// iterator so the query timeout is honoured while walking it. Members Java
// does not override carry the FloatVectorValues / KnnVectorValues defaults.
type ExitableFloatVectorValues struct {
	vectorValues FloatVectorValues
	queryTimeout QueryTimeout
}

// Dimension forwards to the wrapped values.
func (v *ExitableFloatVectorValues) Dimension() int { return v.vectorValues.Dimension() }

// VectorValue forwards to the wrapped values.
func (v *ExitableFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	return v.vectorValues.VectorValue(ord)
}

// OrdToDoc forwards to the wrapped values.
func (v *ExitableFloatVectorValues) OrdToDoc(ord int) int { return v.vectorValues.OrdToDoc(ord) }

// Size forwards to the wrapped values.
func (v *ExitableFloatVectorValues) Size() int { return v.vectorValues.Size() }

// Iterator wraps the delegate's iterator so the timeout is honoured while
// walking it.
func (v *ExitableFloatVectorValues) Iterator() DocIndexIterator {
	return createExitableIterator(v.vectorValues.Iterator(), v.queryTimeout)
}

// Scorer forwards to the wrapped values.
func (v *ExitableFloatVectorValues) Scorer(target []float32) (util.VectorScorer, error) {
	return v.vectorValues.Scorer(target)
}

// Copy is unsupported: Java's copy() throws UnsupportedOperationException.
func (v *ExitableFloatVectorValues) Copy() (KnnVectorValues, error) {
	return nil, fmt.Errorf("index: ExitableFloatVectorValues: Copy is not supported")
}

// CopyFloatVectorValues is unsupported for the same reason as Copy.
func (v *ExitableFloatVectorValues) CopyFloatVectorValues() (FloatVectorValues, error) {
	return nil, fmt.Errorf("index: ExitableFloatVectorValues: Copy is not supported")
}

// Rescorer carries the FloatVectorValues.rescorer default.
func (v *ExitableFloatVectorValues) Rescorer(target []float32) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// GetEncoding carries the FloatVectorValues.getEncoding override.
func (v *ExitableFloatVectorValues) GetEncoding() VectorEncoding { return VectorEncodingFloat32 }

// Prefetch carries the KnnVectorValues.prefetch default, which does nothing.
func (v *ExitableFloatVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength default.
func (v *ExitableFloatVectorValues) GetVectorByteLength() int {
	return v.Dimension() * VectorEncodingByteSize(v.GetEncoding())
}

// GetAcceptOrds carries the KnnVectorValues.getAcceptOrds default.
func (v *ExitableFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(v, acceptDocs)
}

// ExitableByteVectorValues ports the private inner class
// ExitableDirectoryReader.ExitableFilterAtomicReader.ExitableByteVectorValues
// of Apache Lucene 10.5.0. See [ExitableFloatVectorValues].
type ExitableByteVectorValues struct {
	vectorValues ByteVectorValues
	queryTimeout QueryTimeout
}

// Dimension forwards to the wrapped values.
func (v *ExitableByteVectorValues) Dimension() int { return v.vectorValues.Dimension() }

// Size forwards to the wrapped values.
func (v *ExitableByteVectorValues) Size() int { return v.vectorValues.Size() }

// VectorValue forwards to the wrapped values.
func (v *ExitableByteVectorValues) VectorValue(ord int) ([]byte, error) {
	return v.vectorValues.VectorValue(ord)
}

// OrdToDoc forwards to the wrapped values.
func (v *ExitableByteVectorValues) OrdToDoc(ord int) int { return v.vectorValues.OrdToDoc(ord) }

// Iterator wraps the delegate's iterator so the timeout is honoured while
// walking it.
func (v *ExitableByteVectorValues) Iterator() DocIndexIterator {
	return createExitableIterator(v.vectorValues.Iterator(), v.queryTimeout)
}

// Scorer forwards to the wrapped values.
func (v *ExitableByteVectorValues) Scorer(target []byte) (util.VectorScorer, error) {
	return v.vectorValues.Scorer(target)
}

// Copy is unsupported: Java's copy() throws UnsupportedOperationException.
func (v *ExitableByteVectorValues) Copy() (KnnVectorValues, error) {
	return nil, fmt.Errorf("index: ExitableByteVectorValues: Copy is not supported")
}

// CopyByteVectorValues is unsupported for the same reason as Copy.
func (v *ExitableByteVectorValues) CopyByteVectorValues() (ByteVectorValues, error) {
	return nil, fmt.Errorf("index: ExitableByteVectorValues: Copy is not supported")
}

// Rescorer carries the ByteVectorValues.rescorer default.
func (v *ExitableByteVectorValues) Rescorer(target []byte) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// GetEncoding carries the ByteVectorValues.getEncoding override.
func (v *ExitableByteVectorValues) GetEncoding() VectorEncoding { return VectorEncodingByte }

// Prefetch carries the KnnVectorValues.prefetch default, which does nothing.
func (v *ExitableByteVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength default.
func (v *ExitableByteVectorValues) GetVectorByteLength() int {
	return v.Dimension() * VectorEncodingByteSize(v.GetEncoding())
}

// GetAcceptOrds carries the KnnVectorValues.getAcceptOrds default.
func (v *ExitableByteVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(v, acceptDocs)
}

func createExitableIterator(delegate DocIndexIterator, queryTimeout QueryTimeout) DocIndexIterator {
	return &exitableDocIndexIterator{
		delegate:     delegate,
		queryTimeout: queryTimeout,
	}
}

type exitableDocIndexIterator struct {
	delegate     DocIndexIterator
	queryTimeout QueryTimeout
	nextCheck    int
}

func (i *exitableDocIndexIterator) Index() int {
	return i.delegate.Index()
}

func (i *exitableDocIndexIterator) DocID() int {
	return i.delegate.DocID()
}

func (i *exitableDocIndexIterator) DocIDRunEnd() (int, error) {
	return i.delegate.DocIDRunEnd()
}

func (i *exitableDocIndexIterator) NextDoc() (int, error) {
	doc, err := i.delegate.NextDoc()
	if err == nil && doc >= i.nextCheck {
		if i.queryTimeout.ShouldExit() {
			return -1, &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over knn vector values. Timeout: %v, KnnVectorValues=%v", i.queryTimeout, i.delegate)}
		}
		i.nextCheck = doc + docsBetweenTimeoutCheck
	}
	return doc, err
}

func (i *exitableDocIndexIterator) Cost() int64 {
	return i.delegate.Cost()
}

func (i *exitableDocIndexIterator) Advance(target int) (int, error) {
	doc, err := i.delegate.Advance(target)
	if err == nil && doc >= i.nextCheck {
		if i.queryTimeout.ShouldExit() {
			return -1, &ExitingReaderError{msg: fmt.Sprintf("The request took too long to iterate over knn vector values. Timeout: %v, KnnVectorValues=%v", i.queryTimeout, i.delegate)}
		}
		i.nextCheck = doc + docsBetweenTimeoutCheck
	}
	return doc, err
}

// AcceptDocs wrapper

// ExitableAcceptDocs exposes the query's accepted documents as util.Bits,
// treating a nil delegate as "every document is accepted" over maxDoc.
type ExitableAcceptDocs struct {
	in     util.Bits
	maxDoc int
}

func (a *ExitableAcceptDocs) Get(index int) bool {
	if a.in == nil {
		return true
	}
	return a.in.Get(index)
}

func (a *ExitableAcceptDocs) Length() int {
	if a.in == nil {
		return a.maxDoc
	}
	return a.in.Length()
}

// bitsWithIterator is the doc-id cursor Lucene 10.5.0's AcceptDocs exposes
// through iterator(). util.Bits does not declare it, so it is recovered from
// the delegate.
type bitsWithIterator interface {
	Iterator() (util.DocIdSetIterator, error)
}

// Iterator returns the delegate's doc-id cursor over the accepted documents.
func (a *ExitableAcceptDocs) Iterator() (util.DocIdSetIterator, error) {
	if a.in == nil {
		return nil, fmt.Errorf("index: ExitableAcceptDocs: no bits available")
	}
	it, ok := a.in.(bitsWithIterator)
	if !ok {
		return nil, fmt.Errorf("index: ExitableAcceptDocs: %T exposes no doc-id iterator", a.in)
	}
	return it.Iterator()
}

// Cost is the cost of iterating the accepted documents.
//
// Java's ExitableAcceptDocs.cost() reads `return in.cost()` — it delegates to
// the AcceptDocs it wraps (ExitableDirectoryReader.java:416-419). The delegate
// held here is a Bits rather than an AcceptDocs, so the cost is the one
// AcceptDocs.fromLiveDocs gives a Bits: BitsAcceptDocs stores
// `bits instanceof BitSet ? bitSet.cardinality() : maxDoc` and returns it from
// cost(), with the comment "We have no better estimate. This should be ok in
// practice since background merges should keep the number of deletes under
// control (< 20% by default)" (AcceptDocs.java:129-133, 151-156).
func (a *ExitableAcceptDocs) Cost() int {
	if bitSet, ok := a.in.(util.BitSet); ok {
		return bitSet.Cardinality()
	}
	return a.maxDoc
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (i *exitableDocIndexIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(i, upTo, bitSet, offset)
}

// VisitByDocIDSetIterator renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator), which v does not
// override.
func (v *ExitableIntersectVisitor) VisitByDocIDSetIterator(iterator spi.DocIdSetIterator) error {
	return spi.DefaultVisitByDocIDSetIterator(v, iterator)
}

// VisitByIntsRef renders the default body of
// PointValues.IntersectVisitor.visit(IntsRef), which v does not override.
func (v *ExitableIntersectVisitor) VisitByIntsRef(ref *util.IntsRef) error {
	return spi.DefaultVisitByIntsRef(v, ref)
}

// VisitByDocIDSetIteratorAndPackedValue renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator, byte[]), which v
// does not override.
func (v *ExitableIntersectVisitor) VisitByDocIDSetIteratorAndPackedValue(iterator spi.DocIdSetIterator, packedValue []byte) error {
	return spi.DefaultVisitByDocIDSetIteratorAndPackedValue(v, iterator, packedValue)
}
