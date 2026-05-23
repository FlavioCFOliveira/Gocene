// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
)

// SPINameOpenNLPLemmatizer is the SPI name for OpenNLPLemmatizerFilterFactory.
const SPINameOpenNLPLemmatizer = "openNlpLemmatizer"

// OpenNLPLemmatizerFilterFactory creates OpenNLPLemmatizerFilter instances.
//
// Go port of
// org.apache.lucene.analysis.opennlp.OpenNLPLemmatizerFilterFactory
// (Apache Lucene 10.4.0).
//
// Deviation: The Java factory implements ResourceLoaderAware and loads model
// files at inform() time. In Go, models are registered in the
// tools.OpenNLPOpsFactory cache before use.
//
// At least one of DictionaryName and LemmatizerModelName must be set.
type OpenNLPLemmatizerFilterFactory struct {
	DictionaryName      string
	LemmatizerModelName string
}

// NewOpenNLPLemmatizerFilterFactory creates a factory. At least one of
// dictionaryName and lemmatizerModelName must be non-empty.
func NewOpenNLPLemmatizerFilterFactory(dictionaryName, lemmatizerModelName string) *OpenNLPLemmatizerFilterFactory {
	if dictionaryName == "" && lemmatizerModelName == "" {
		panic("OpenNLPLemmatizerFilterFactory: at least one of DictionaryName and LemmatizerModelName must be set")
	}
	return &OpenNLPLemmatizerFilterFactory{
		DictionaryName:      dictionaryName,
		LemmatizerModelName: lemmatizerModelName,
	}
}

// Create creates an OpenNLPLemmatizerFilter wrapping input.
func (f *OpenNLPLemmatizerFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	op := tools.GetLemmatizer(f.DictionaryName, f.LemmatizerModelName)
	return NewOpenNLPLemmatizerFilter(input, op)
}

// Ensure OpenNLPLemmatizerFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*OpenNLPLemmatizerFilterFactory)(nil)
