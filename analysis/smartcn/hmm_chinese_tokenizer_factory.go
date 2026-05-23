// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// HMMChineseTokenizerFactory creates HMMChineseTokenizer instances.
//
// Go port of org.apache.lucene.analysis.cn.smart.HMMChineseTokenizerFactory
// (Apache Lucene 10.4.0).
//
// SPI name: "hmmChinese".
type HMMChineseTokenizerFactory struct{}

// Name is the SPI name of this factory.
const HMMChineseTokenizerFactoryName = "hmmChinese"

// NewHMMChineseTokenizerFactory creates a new HMMChineseTokenizerFactory.
// Returns an error if any unknown args are provided (matching Java behaviour).
func NewHMMChineseTokenizerFactory(args map[string]string) (*HMMChineseTokenizerFactory, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("HMMChineseTokenizerFactory: unknown parameters: %v", args)
	}
	return &HMMChineseTokenizerFactory{}, nil
}

// Create returns a new HMMChineseTokenizer.
func (f *HMMChineseTokenizerFactory) Create() analysis.Tokenizer {
	tok, err := NewHMMChineseTokenizer()
	if err != nil {
		// The factory contract does not return an error; panic is acceptable
		// for init failures in tokenizer factories (mirrors Java RuntimeException).
		panic(fmt.Sprintf("HMMChineseTokenizerFactory.Create: %v", err))
	}
	return tok
}

// Ensure HMMChineseTokenizerFactory implements analysis.TokenizerFactory.
var _ analysis.TokenizerFactory = (*HMMChineseTokenizerFactory)(nil)
