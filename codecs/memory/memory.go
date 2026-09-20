// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// DirectPostingsFormat is the codec that holds postings in flat in-memory
// arrays. Mirrors org.apache.lucene.codecs.memory.DirectPostingsFormat.
type DirectPostingsFormat struct {
	MinSkipCount  int
	LowFreqCutoff int
}

// NewDirectPostingsFormat builds the format.
func NewDirectPostingsFormat(minSkip, lowFreq int) *DirectPostingsFormat {
	if minSkip < 1 {
		minSkip = 8
	}
	if lowFreq < 1 {
		lowFreq = 32
	}
	return &DirectPostingsFormat{MinSkipCount: minSkip, LowFreqCutoff: lowFreq}
}

// FSTPostingsFormat is the FST-backed postings format. Mirrors
// org.apache.lucene.codecs.memory.FSTPostingsFormat.
type FSTPostingsFormat struct{}

// NewFSTPostingsFormat builds the format.
func NewFSTPostingsFormat() *FSTPostingsFormat { return &FSTPostingsFormat{} }

// Format constants of the FST terms dictionary, ported from
// org.apache.lucene.codecs.memory.FSTTermsWriter (FSTTermsWriter.java:110-113),
// where Java declares them and where FSTTermsReader reads them from.
const (
	// termsExtension renders `static final String TERMS_EXTENSION = "tfp"`
	// (FSTTermsWriter.java:110).
	termsExtension = "tfp"
	// termsCodecName renders `static final String TERMS_CODEC_NAME =
	// "FSTTerms"` (FSTTermsWriter.java:111).
	termsCodecName = "FSTTerms"
	// termsVersionStart renders `public static final int TERMS_VERSION_START =
	// 2` (FSTTermsWriter.java:112).
	termsVersionStart int32 = 2
	// termsVersionCurrent renders `public static final int
	// TERMS_VERSION_CURRENT = TERMS_VERSION_START` (FSTTermsWriter.java:113).
	termsVersionCurrent = termsVersionStart
)

// FSTTermsWriter writes FST-backed term dictionaries. Mirrors
// org.apache.lucene.codecs.memory.FSTTermsWriter.
type FSTTermsWriter struct {
	Format *FSTPostingsFormat
}

// NewFSTTermsWriter builds the writer.
func NewFSTTermsWriter(format *FSTPostingsFormat) *FSTTermsWriter {
	return &FSTTermsWriter{Format: format}
}

func (w *FSTTermsWriter) Write(state *index.SegmentWriteState, fields spi.Terms) error {
	// Implementation of writing FST terms...
	return fmt.Errorf("FSTTermsWriter: Write not yet implemented")
}
