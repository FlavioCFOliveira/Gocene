// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "sync"

// Temp-format registry for the sorting consumers.
//
// SortingStoredFieldsConsumer and SortingTermVectorsConsumer require a
// dedicated "temporary" StoredFieldsFormat / TermVectorsFormat — a
// no-compression instance of Lucene90CompressingStoredFieldsFormat /
// Lucene90CompressingTermVectorsFormat in Apache Lucene 10.4.0 — to
// buffer per-document state in document-write order before reordering
// into the codec writer at flush time.
//
// Lucene reaches those formats directly:
//
//	private static final StoredFieldsFormat TEMP_STORED_FIELDS_FORMAT =
//	    new Lucene90CompressingStoredFieldsFormat(
//	        "TempStoredFields", NO_COMPRESSION, 128 * 1024, 1, 10);
//
// In Gocene the codecs/lucene90/compressing package already imports
// "index", so a direct import the other way would form a cycle. We
// break the cycle with a small registry: the compressing package
// publishes the configured temp formats from init(), and the sorting
// consumers consume them via DefaultTempStoredFieldsFormat /
// DefaultTempTermVectorsFormat at construction time.
//
// The registry is process-global, thread-safe, and idempotent under
// repeated registration (last registration wins). It is intentionally
// package-private write side: only the codec layer is expected to
// publish, and only the sorting consumers are expected to consume.

var (
	tempStoredFieldsFormatMu sync.RWMutex
	tempStoredFieldsFormat   StoredFieldsFormat

	tempTermVectorsFormatMu sync.RWMutex
	tempTermVectorsFormat   TermVectorsFormat
)

// RegisterDefaultTempStoredFieldsFormat publishes the StoredFieldsFormat
// used by SortingStoredFieldsConsumer when no explicit per-instance
// override is supplied. It is intended to be called from init() of the
// codec package that owns the canonical temp format.
//
// Passing nil clears the registration.
func RegisterDefaultTempStoredFieldsFormat(format StoredFieldsFormat) {
	tempStoredFieldsFormatMu.Lock()
	defer tempStoredFieldsFormatMu.Unlock()
	tempStoredFieldsFormat = format
}

// DefaultTempStoredFieldsFormat returns the currently registered default
// temporary StoredFieldsFormat, or nil when no codec has registered one.
func DefaultTempStoredFieldsFormat() StoredFieldsFormat {
	tempStoredFieldsFormatMu.RLock()
	defer tempStoredFieldsFormatMu.RUnlock()
	return tempStoredFieldsFormat
}

// RegisterDefaultTempTermVectorsFormat publishes the TermVectorsFormat
// used by SortingTermVectorsConsumer when no explicit per-instance
// override is supplied. It is intended to be called from init() of the
// codec package that owns the canonical temp format.
//
// Passing nil clears the registration.
func RegisterDefaultTempTermVectorsFormat(format TermVectorsFormat) {
	tempTermVectorsFormatMu.Lock()
	defer tempTermVectorsFormatMu.Unlock()
	tempTermVectorsFormat = format
}

// DefaultTempTermVectorsFormat returns the currently registered default
// temporary TermVectorsFormat, or nil when no codec has registered one.
func DefaultTempTermVectorsFormat() TermVectorsFormat {
	tempTermVectorsFormatMu.RLock()
	defer tempTermVectorsFormatMu.RUnlock()
	return tempTermVectorsFormat
}
