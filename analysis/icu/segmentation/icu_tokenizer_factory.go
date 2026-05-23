// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import "github.com/FlavioCFOliveira/Gocene/analysis"

// ICUTokenizerFactory creates ICUTokenizer instances.
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.ICUTokenizerFactory
// (Apache Lucene 10.4.0).
//
// Deviation: The Java factory accepts a "rulefiles" argument that loads
// per-script ICU RuleBasedBreakIterator rule files at runtime. This port
// does not support per-script rule files because there is no CGO-free
// equivalent of ICU4J's RuleBasedBreakIterator.getInstanceFromCompiledRules.
// Only the cjkAsWords and myanmarAsWords boolean options are supported.
type ICUTokenizerFactory struct {
	config ICUTokenizerConfig
}

// NewICUTokenizerFactory creates a factory using
// DefaultICUTokenizerConfig(cjkAsWords=true, myanmarAsWords=true).
func NewICUTokenizerFactory() *ICUTokenizerFactory {
	return NewICUTokenizerFactoryWith(true, true)
}

// NewICUTokenizerFactoryWith creates a factory with explicit cjkAsWords and
// myanmarAsWords options.
func NewICUTokenizerFactoryWith(cjkAsWords, myanmarAsWords bool) *ICUTokenizerFactory {
	return &ICUTokenizerFactory{
		config: NewDefaultICUTokenizerConfig(cjkAsWords, myanmarAsWords),
	}
}

// NewICUTokenizerFactoryWithConfig creates a factory using a custom config.
func NewICUTokenizerFactoryWithConfig(config ICUTokenizerConfig) *ICUTokenizerFactory {
	return &ICUTokenizerFactory{config: config}
}

// Create creates a new ICUTokenizer.
func (f *ICUTokenizerFactory) Create() analysis.Tokenizer {
	return NewICUTokenizerWith(f.config)
}

// Ensure ICUTokenizerFactory implements TokenizerFactory.
var _ analysis.TokenizerFactory = (*ICUTokenizerFactory)(nil)
