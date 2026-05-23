// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ne

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// DefaultStopWordFile is the name of the bundled Nepali stopword file.
// (Lucene loads this from the classpath; Gocene embeds it as a string literal.)
const DefaultStopWordFile = "stopwords.txt"

// NepaliStopWords is the default Nepali stop-word list.
// Source: Apache Lucene 10.4.0,
// analysis/common/src/resources/org/apache/lucene/analysis/ne/stopwords.txt
// (derived from NLTK https://github.com/nltk/nltk_data under Apache 2.0)
var NepaliStopWords = []string{
	"छ", "र", "पनि", "छन्", "लागि", "भएको", "गरेको", "भने", "गर्न", "गर्ने",
	"हो", "तथा", "यो", "रहेको", "उनले", "थियो", "हुने", "गरेका", "थिए", "गर्दै",
	"तर", "नै", "को", "मा", "हुन्", "भन्ने", "हुन", "गरी", "त", "हुन्छ",
	"अब", "के", "रहेका", "गरेर", "छैन", "दिए", "भए", "यस", "ले", "गर्नु",
	"औं", "सो", "त्यो", "कि", "जुन", "यी", "का", "गरि", "ती", "न",
	"छु", "छौं", "लाई", "नि", "उप", "अक्सर", "आदि", "कसरी", "क्रमशः", "चाले",
	"अगाडी", "अझै", "अनुसार", "अन्तर्गत", "अन्य", "अन्यत्र", "अन्यथा", "अरु",
	"अरुलाई", "अर्को", "अर्थात", "अर्थात्", "अलग", "आए", "आजको", "ओठ", "आत्म",
	"आफू", "आफूलाई", "आफ्नै", "आफ्नो", "आयो", "उदाहरण", "उनको", "उहालाई",
	"एउटै", "एक", "एकदम", "कतै", "कसै", "कसैले", "कहाँबाट", "कहिलेकाहीं",
	"किन", "किनभने", "कुनै", "कुरा", "कृपया", "केही", "कोही", "गए", "गरौं",
	"गर्छ", "गर्छु", "गर्नुपर्छ", "गयौ", "गैर", "चार", "चाहनुहुन्छ", "चाहन्छु",
	"चाहिए", "छू", "जताततै", "जब", "जबकि", "जसको", "जसबाट", "जसमा", "जसलाई",
	"जसले", "जस्तै", "जस्तो", "जस्तोसुकै", "जहाँ", "जान", "जाहिर", "जे", "जो",
	"ठीक", "तत्काल", "तदनुसार", "तपाईको", "तपाई", "पर्याप्त", "पहिले", "पहिलो",
	"पहिल्यै", "पाँच", "पाँचौं", "तल", "तापनी", "तिनी", "तिनीहरू", "तिनीहरुको",
	"तिनिहरुलाई", "तिमी", "तिर", "तीन", "त्यहाँ", "त्यस", "त्यसको", "त्यसमा",
	"त्यसैले", "त्यहाँबाट", "त्यहाँसम्म", "त्यही", "दिन", "दुई", "दुवै", "दोस्रो",
	"नगर्नुहोस्", "नभएसम्म", "नभए", "नहुन", "निम्न", "नीचे", "नै", "पछि", "पछाडि",
	"पनि", "पहिले", "परन्तु", "पुनः", "पूर्ण", "प्रत्येक", "प्रत्येकको", "प्रथम",
	"प्रभाव", "प्रायः", "प्रस्तुत", "प्राय", "बारे", "बाट", "बाहेक", "बाहिर",
	"बिना", "भन्दा", "भयो", "भित्र", "भित्रै", "भैन", "मध्ये", "माथि", "माथिका",
	"मात्र", "मात्रै", "मुनि", "यसको", "यसले", "यसमा", "यसरी", "यसैले", "यसो",
	"यहाँ", "यहाँबाट", "यहाँसम्म", "याे", "यो", "रहनेछ", "लगायत", "लगातार",
	"वा", "विपरित", "विभिन्न", "विरुद्ध", "विशेष", "सक्छ", "सक्छु", "सक्दैन",
	"सन्", "सबभन्दा", "सम्म", "सम्पूर्ण", "सहित", "हामी", "हामीलाई", "हाम्रो",
	"हुँदैन", "हुँदा", "हुनेछ", "हुनेछन्", "हुनु", "हुनुपर्छ", "हुन्छन्",
	"ह्वाँ", "उनलाई", "उनी", "उनको", "उनकै", "छन्", "बाट", "अझ",
}

// NepaliAnalyzer is an analyzer for Nepali.
//
// Pipeline: StandardTokenizer → LowerCaseFilter → DecimalDigitFilter →
// StopFilter.
//
// This is the Go port of
// org.apache.lucene.analysis.ne.NepaliAnalyzer from Apache Lucene 10.4.0.
//
// Deviation: IndicNormalizationFilter and SnowballFilter(NepaliStemmer) are
// omitted because IndicNormalizationFilter is a Sprint-28 stub and the
// Tartarus NepaliStemmer is not yet available. These will be wired once those
// dependencies land.
type NepaliAnalyzer struct {
	*analysis.BaseAnalyzer

	stopWords        *analysis.CharArraySet
	stemExclusionSet *analysis.CharArraySet
}

// GetDefaultStopSet returns the default Nepali stop-word set.
func GetDefaultStopSet() *analysis.CharArraySet {
	return analysis.GetWordSetFromStrings(NepaliStopWords, false)
}

// NewNepaliAnalyzer creates a NepaliAnalyzer with default stop words and no
// stem-exclusion set.
func NewNepaliAnalyzer() *NepaliAnalyzer {
	return NewNepaliAnalyzerFull(GetDefaultStopSet(), analysis.NewCharArraySet(0, false))
}

// NewNepaliAnalyzerWithStopWords creates a NepaliAnalyzer with a custom
// stop-word set.
func NewNepaliAnalyzerWithStopWords(stopWords *analysis.CharArraySet) *NepaliAnalyzer {
	return NewNepaliAnalyzerFull(stopWords, analysis.NewCharArraySet(0, false))
}

// NewNepaliAnalyzerFull creates a NepaliAnalyzer with explicit stop words and
// stem-exclusion set.
func NewNepaliAnalyzerFull(stopWords, stemExclusionSet *analysis.CharArraySet) *NepaliAnalyzer {
	a := &NepaliAnalyzer{
		BaseAnalyzer:     analysis.NewAnalyzer(),
		stopWords:        stopWords,
		stemExclusionSet: stemExclusionSet,
	}
	a.TokenizerFactory = analysis.NewStandardTokenizerFactory()
	a.AddTokenFilter(analysis.NewLowerCaseFilterFactory())
	a.AddTokenFilter(analysis.NewDecimalDigitFilterFactory())
	a.AddTokenFilter(analysis.NewStopFilterFactoryWithWords(stopWords))
	return a
}

// GetStopWords returns the stop-word set.
func (a *NepaliAnalyzer) GetStopWords() *analysis.CharArraySet { return a.stopWords }

// GetStemExclusionSet returns the stem-exclusion set.
func (a *NepaliAnalyzer) GetStemExclusionSet() *analysis.CharArraySet { return a.stemExclusionSet }

// TokenStream creates a TokenStream for the given reader.
func (a *NepaliAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Ensure NepaliAnalyzer implements Analyzer.
var _ analysis.Analyzer = (*NepaliAnalyzer)(nil)
var _ analysis.AnalyzerInterface = (*NepaliAnalyzer)(nil)
