// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
)

// SPINameOpenNLPChunker is the SPI name for OpenNLPChunkerFilterFactory.
const SPINameOpenNLPChunker = "openNlpChunker"

// OpenNLPChunkerFilterFactory creates OpenNLPChunkerFilter instances.
//
// Go port of
// org.apache.lucene.analysis.opennlp.OpenNLPChunkerFilterFactory
// (Apache Lucene 10.4.0).
//
// Deviation: The Java factory implements ResourceLoaderAware and loads the
// chunker model from a file at inform() time. In Go, the model must be
// registered in the tools.OpenNLPOpsFactory cache before use.
type OpenNLPChunkerFilterFactory struct {
	ChunkerModelName string
}

// NewOpenNLPChunkerFilterFactory creates a factory for the named chunker
// model. If chunkerModelName is empty, the filter is created without a
// chunking model (no chunking performed).
func NewOpenNLPChunkerFilterFactory(chunkerModelName string) *OpenNLPChunkerFilterFactory {
	return &OpenNLPChunkerFilterFactory{ChunkerModelName: chunkerModelName}
}

// Create creates an OpenNLPChunkerFilter wrapping input.
func (f *OpenNLPChunkerFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	var chunkerOp *tools.NLPChunkerOp
	if f.ChunkerModelName != "" {
		chunkerOp = tools.GetChunker(f.ChunkerModelName)
	}
	return NewOpenNLPChunkerFilter(input, chunkerOp)
}

// Ensure OpenNLPChunkerFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*OpenNLPChunkerFilterFactory)(nil)
