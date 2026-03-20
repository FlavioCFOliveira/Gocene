// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// BulgarianStopWords contains common Bulgarian stop words.
// Source: Apache Lucene Bulgarian stop words list
var BulgarianStopWords = []string{
	// Articles and determiners
	"а", "аз", "ако", "ала", "бе", "без", "беше", "би", "бил", "била",
	"били", "било", "благодаря", "близо", "бъдат", "бъде", "бяха", "в",
	"вас", "ваш", "ваша", "вероятно", "вече", "взема", "ви", "вие", "винаги",
	"вместо", "все", "всеки", "всички", "всичко", "всяка", "във", "въпреки",
	"върху", "г", "ги", "главно", "го", "д", "да", "дали", "два", "двама",
	"двамата", "две", "двете", "днес", "добра", "добре", "добро", "докато",
	"докога", "дори", "досега", "доста", "друг", "друга", "други", "е",
	"евтин", "едва", "един", "една", "еднаква", "еднакви", "еднакъв", "едно",
	"екип", "ето", "живот", "за", "зад", "заедно", "заради", "засега",
	"заспал", "затова", "защо", "защото", "и", "из", "или", "им", "има",
	"имат", "иска", "й", "каза", "как", "каква", "какво", "както", "какъв",
	"като", "кога", "когато", "кое", "което", "кои", "които", "кой",
	"който", "колко", "която", "къде", "където", "към", "лесен", "лесно",
	"ли", "м", "май", "малко", "ме", "между", "мек", "мен", "месец",
	"ми", "много", "мнозина", "мога", "могат", "може", "мокър", "моля",
	"момента", "му", "н", "на", "над", "назад", "най", "направи", "нас",
	"не", "него", "нещо", "нея", "ни", "ние", "никой", "нито", "нищо",
	"но", "нов", "нова", "нови", "новина", "някои", "някой", "няколко",
	"няма", "обаче", "около", "освен", "особено", "от", "отгоре", "отново",
	"още", "пак", "по", "повече", "повечето", "под", "поне", "поради",
	"после", "почти", "прави", "пред", "преди", "през", "при", "пък",
	"първата", "първи", "първо", "пъти", "равен", "равна", "с", "са",
	"сам", "само", "се", "сега", "си", "син", "скоро", "след", "следващ",
	"сме", "смях", "според", "сред", "срещу", "сте", "съм", "със", "също",
	"т", "тази", "така", "такива", "такъв", "там", "твой", "те", "тези",
	"ти", "тн", "то", "това", "тогава", "този", "той", "толкова", "точно",
	"три", "трябва", "тук", "тъй", "тя", "тях", "у", "утре", "харесва",
	"хиляди", "ч", "часа", "че", "често", "чрез", "ще", "щом", "юмрук",
	"я", "як",
}

// BulgarianAnalyzer is an analyzer for Bulgarian language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.bg.BulgarianAnalyzer.
//
// BulgarianAnalyzer uses the StandardTokenizer with Bulgarian stop words removal.
type BulgarianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewBulgarianAnalyzer creates a new BulgarianAnalyzer with default Bulgarian stop words.
func NewBulgarianAnalyzer() *BulgarianAnalyzer {
	stopSet := GetWordSetFromStrings(BulgarianStopWords, true)
	return NewBulgarianAnalyzerWithWords(stopSet)
}

// NewBulgarianAnalyzerWithWords creates a BulgarianAnalyzer with custom stop words.
func NewBulgarianAnalyzerWithWords(stopWords *CharArraySet) *BulgarianAnalyzer {
	a := &BulgarianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *BulgarianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *BulgarianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *BulgarianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*BulgarianAnalyzer)(nil)
var _ AnalyzerInterface = (*BulgarianAnalyzer)(nil)
