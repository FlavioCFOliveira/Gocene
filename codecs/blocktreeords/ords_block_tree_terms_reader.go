// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ordsBlockTreeTermsReaderFileExtensions lists the file extensions used by
// OrdsBlockTreeTermsReader.
const (
	// termsExtension is the primary postings dict file suffix.
	termsExtension = "tio"
	// termsIndexExtension is the FST index file suffix.
	termsIndexExtension = "tipo"
	// termsCodecName is written in the codec header.
	termsCodecName = "OrdsBlockTreeTerms"
	// termsIndexCodecName is written in the index header.
	termsIndexCodecName = "OrdsBlockTreeTermsIndex"
	// versionStart is the first supported format version.
	versionStart = int32(1)
	// versionCurrent is the current format version.
	versionCurrent = versionStart
)

// OrdsBlockTreeTermsReader is the FieldsProducer for the BlockTreeOrds
// postings format.  It opens the .tio (terms) and .tipo (index) files,
// validates codec headers, and constructs one OrdsFieldReader per field.
//
// Port of org.apache.lucene.codecs.blocktreeords.OrdsBlockTreeTermsReader
// (Lucene 10.4.0).
type OrdsBlockTreeTermsReader struct {
	// in is the primary terms file (.tio).
	in store.IndexInput
	// postingsReader provides the postings (doc/freq/pos) decoder.
	postingsReader codecs.PostingsReaderBase
	// fields is the ordered map of field name → OrdsFieldReader.
	fields map[string]*OrdsFieldReader
	// segment is the segment name for error messages.
	segment string
}

// fields returns the sorted field names.
func (r *OrdsBlockTreeTermsReader) fieldNames() []string {
	names := make([]string, 0, len(r.fields))
	for n := range r.fields {
		names = append(names, n)
	}
	return names
}
