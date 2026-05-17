// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "errors"

// MultiDocValues is a stateless container of static helpers that flatten
// composite-reader doc values into a single virtual iterator that spans all
// leaves. Mirrors org.apache.lucene.index.MultiDocValues from Apache
// Lucene 10.4.0.
//
// Gocene skeleton: only the API surface is in place; the multi-segment
// flattening (which needs ord-map composition and per-leaf delegation) is
// deferred to a follow-up task — see backlog #2703. Each helper currently
// returns ErrMultiDocValuesNotImplemented so callers can detect the gap at
// the call site.

// ErrMultiDocValuesNotImplemented is returned by every MultiDocValues helper
// until the full Lucene parity port lands.
var ErrMultiDocValuesNotImplemented = errors.New("MultiDocValues helpers are not implemented yet (Sprint 22 follow-up #2703)")

// MultiDocValuesGetNormValues returns the norm values for field, flattened
// across all leaves of r. Currently returns ErrMultiDocValuesNotImplemented.
func MultiDocValuesGetNormValues(_ IndexReaderInterface, _ string) (NumericDocValues, error) {
	return nil, ErrMultiDocValuesNotImplemented
}

// MultiDocValuesGetNumericValues returns the numeric doc values for field,
// flattened across all leaves of r.
func MultiDocValuesGetNumericValues(_ IndexReaderInterface, _ string) (NumericDocValues, error) {
	return nil, ErrMultiDocValuesNotImplemented
}

// MultiDocValuesGetBinaryValues returns the binary doc values for field,
// flattened across all leaves of r.
func MultiDocValuesGetBinaryValues(_ IndexReaderInterface, _ string) (BinaryDocValues, error) {
	return nil, ErrMultiDocValuesNotImplemented
}

// MultiDocValuesGetSortedNumericValues returns the sorted numeric doc values
// for field, flattened across all leaves of r.
func MultiDocValuesGetSortedNumericValues(_ IndexReaderInterface, _ string) (SortedNumericDocValues, error) {
	return nil, ErrMultiDocValuesNotImplemented
}

// MultiDocValuesGetSortedValues returns the sorted doc values for field,
// flattened across all leaves of r.
func MultiDocValuesGetSortedValues(_ IndexReaderInterface, _ string) (SortedDocValues, error) {
	return nil, ErrMultiDocValuesNotImplemented
}

// MultiDocValuesGetSortedSetValues returns the sorted-set doc values for
// field, flattened across all leaves of r.
func MultiDocValuesGetSortedSetValues(_ IndexReaderInterface, _ string) (SortedSetDocValues, error) {
	return nil, ErrMultiDocValuesNotImplemented
}
