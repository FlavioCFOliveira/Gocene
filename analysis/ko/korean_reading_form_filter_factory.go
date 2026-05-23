// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// KoreanReadingFormFilterFactory creates KoreanReadingFormFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanReadingFormFilterFactory from Apache
// Lucene 10.4.0.
//
// Deviation: Gocene does not have a TokenFilterFactory SPI; this factory is a
// plain struct with a Create method.
type KoreanReadingFormFilterFactory struct{}

// NewKoreanReadingFormFilterFactory creates a KoreanReadingFormFilterFactory.
// args must be empty; any unrecognised key causes an error.
func NewKoreanReadingFormFilterFactory(args map[string]string) (*KoreanReadingFormFilterFactory, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("KoreanReadingFormFilterFactory: unknown parameters: %v", args)
	}
	return &KoreanReadingFormFilterFactory{}, nil
}

// Create wraps input in a KoreanReadingFormFilter.
func (f *KoreanReadingFormFilterFactory) Create(input analysis.TokenStream) analysis.TokenStream {
	return NewKoreanReadingFormFilter(input)
}
