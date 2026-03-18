// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// ChineseStopWords contains common Chinese stop words.
var ChineseStopWords = []string{
	"的", "了", "在", "是", "我", "有", "和", "就", "不", "人", "都", "一", "一个", "上", "也",
	"很", "到", "说", "要", "去", "你", "会", "着", "没有", "看", "好", "自己", "这", "那",
	"这些", "那些", "这个", "那个", "之", "与", "及", "等", "或", "但是", "而", "如果",
	"因为", "所以", "虽然", "但是", "可以", "需要", "进行", "通过", "对于", "关于",
	"以及", "其中", "其他", "已经", "开始", "现在", "当时", "这里", "那里", "什么",
	"怎么", "为什么", "谁", "哪", "个", "为", "以", "能", "可", "并", "把", "被", "让",
	"向", "从", "到", "给", "对", "关于", "比", "跟", "同", "和", "或", "但", "而", "因",
	"于", "则", "却", "还", "只", "最", "更", "太", "非常", "已经", "曾经", "正在",
	"将要", "过", "着", "了", "的", "地", "得", "着", "过", "们", "等", "第", "每",
}

// ChineseAnalyzer is an analyzer for Chinese language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.cn.ChineseAnalyzer.
//
// ChineseAnalyzer uses the StandardTokenizer with Chinese stop words removal.
// Note: For proper Chinese text segmentation, a specialized tokenizer like
// HMMChineseTokenizer or SmartChineseTokenizer would be needed. This implementation
// provides basic support using the StandardTokenizer.
type ChineseAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewChineseAnalyzer creates a new ChineseAnalyzer with default Chinese stop words.
func NewChineseAnalyzer() *ChineseAnalyzer {
	stopSet := GetWordSetFromStrings(ChineseStopWords, true)
	return NewChineseAnalyzerWithWords(stopSet)
}

// NewChineseAnalyzerWithWords creates a ChineseAnalyzer with custom stop words.
func NewChineseAnalyzerWithWords(stopWords *CharArraySet) *ChineseAnalyzer {
	a := &ChineseAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	// Note: For proper Chinese, a specialized tokenizer should be used
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *ChineseAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *ChineseAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *ChineseAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure ChineseAnalyzer implements Analyzer
var _ Analyzer = (*ChineseAnalyzer)(nil)
var _ AnalyzerInterface = (*ChineseAnalyzer)(nil)

// ChineseAnalyzerFactory creates ChineseAnalyzer instances.
type ChineseAnalyzerFactory struct {
	stopWords *CharArraySet
}

// NewChineseAnalyzerFactory creates a new ChineseAnalyzerFactory with default stop words.
func NewChineseAnalyzerFactory() *ChineseAnalyzerFactory {
	return &ChineseAnalyzerFactory{
		stopWords: GetWordSetFromStrings(ChineseStopWords, true),
	}
}

// NewChineseAnalyzerFactoryWithWords creates a new ChineseAnalyzerFactory with custom stop words.
func NewChineseAnalyzerFactoryWithWords(stopWords *CharArraySet) *ChineseAnalyzerFactory {
	return &ChineseAnalyzerFactory{
		stopWords: stopWords,
	}
}

// Create creates a new ChineseAnalyzer.
func (f *ChineseAnalyzerFactory) Create() AnalyzerInterface {
	return NewChineseAnalyzerWithWords(f.stopWords)
}

// Ensure ChineseAnalyzerFactory implements AnalyzerFactory
var _ AnalyzerFactory = (*ChineseAnalyzerFactory)(nil)
