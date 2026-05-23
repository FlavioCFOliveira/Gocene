// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
)

// SPINameOpenNLPPOS is the SPI name for OpenNLPPOSFilterFactory.
const SPINameOpenNLPPOS = "openNlppos"

// OpenNLPPOSFilterFactory creates OpenNLPPOSFilter instances.
//
// Go port of org.apache.lucene.analysis.opennlp.OpenNLPPOSFilterFactory
// (Apache Lucene 10.4.0).
//
// Deviation: The Java factory implements ResourceLoaderAware and loads the
// POS model from a file at inform() time. In Go, the model must be registered
// in the tools.OpenNLPOpsFactory cache before use.
//
// PosTaggerModelName is required.
type OpenNLPPOSFilterFactory struct {
	PosTaggerModelName string
}

// NewOpenNLPPOSFilterFactory creates a factory for the named POS tagger
// model. posTaggerModelName must not be empty.
func NewOpenNLPPOSFilterFactory(posTaggerModelName string) *OpenNLPPOSFilterFactory {
	if posTaggerModelName == "" {
		panic("OpenNLPPOSFilterFactory: posTaggerModelName must not be empty")
	}
	return &OpenNLPPOSFilterFactory{PosTaggerModelName: posTaggerModelName}
}

// Create creates an OpenNLPPOSFilter wrapping input.
func (f *OpenNLPPOSFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	op := tools.GetPOSTagger(f.PosTaggerModelName)
	return NewOpenNLPPOSFilter(input, op)
}

// Ensure OpenNLPPOSFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*OpenNLPPOSFilterFactory)(nil)
