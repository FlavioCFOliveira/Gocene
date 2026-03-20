// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// PersianStopWords contains common Persian (Farsi) stop words.
// Source: Apache Lucene Persian stop words list
var PersianStopWords = []string{
	"آمد", "آمده", "آن", "آنها", "آنچه", "آنکه", "آورد", "آورده",
	"آی", "آیا", "آید", "آیند", "ابتدا", "اثر", "از", "است",
	"استفاده", "اش", "اغلب", "افتاد", "افتاده", "افزود", "افزوده",
	"اقلیت", "الا", "ام", "اما", "امر", "امروز", "ان", "اند",
	"انطور", "انقدر", "انکه", "انها", "اه", "اهم", "اور",
	"اورد", "اورده", "اولا", "اولین", "اون", "اکنون", "ای",
	"اید", "ایشان", "این", "ایند", "اینطور", "اینقدر", "اینکه",
	"اینه", "اینها", "با", "بار", "باره", "باش", "باشد",
	"باشم", "باشند", "باشی", "باشید", "بالا", "باید", "بایست",
	"بر", "برخی", "برعکس", "برم", "برن", "برو", "بس", "بسیار",
	"بسیاری", "بعد", "بقیه", "بله", "بلی", "بود", "بودم",
	"بودن", "بودند", "بوده", "بودی", "بودید", "بوی", "بی",
	"بیا", "بیاید", "بیاور", "بیاورد", "بیاورم", "بیاوری",
	"بیاورید", "بیاوریم", "بیاید", "بیایم", "بیایند", "بیایی",
	"بیایید", "بیاییم", "تازه", "تا", "تاکنون", "تر", "توی",
	"توله", "جای", "جلو", "جنگ", "جهت", "حال", "حالا", "حتما",
	"حتی", "خارج", "خدمت", "خواست", "خواستم", "خواستن",
	"خواستند", "خواسته", "خواستی", "خواستید", "خواستیم",
	"خواهد", "خواهم", "خواهند", "خواهی", "خواهید", "خواهیم",
	"خوب", "خود", "خوش", "خویش", "خیاه", "خیاهد", "خیاهم",
	"خیاهند", "خیاهی", "خیاهید", "خیاهیم", "در", "دارد",
	"دارم", "دارند", "داری", "دارید", "داریم", "داشت", "داشتم",
	"داشتن", "داشتند", "داشته", "داشتی", "داشتید", "داشتیم",
	"دایم", "دایما", "در", "درباره", "درکل", "درون", "دریغ",
	"دست", "دهد", "دهم", "دهند", "دهی", "دهید", "دهیم", "دو",
	"دوباره", "دیر", "دیروز", "دیگر", "دیگری", "را", "رسید",
	"رسیده", "رفت", "رفتار", "رفتم", "رفتن", "رفتند", "رفته",
	"رفتی", "رفتید", "رفتیم", "روند", "روی", "زد", "زدم",
	"زدن", "زدند", "زده", "زدی", "زدید", "زدیم", "زیاد",
	"زیادی", "زیر", "سابق", "ساخت", "ساختم", "ساختن", "ساختند",
	"ساخته", "ساختی", "ساختید", "ساختیم", "سپس", "شد", "شدم",
	"شدن", "شدند", "شده", "شدی", "شدید", "شدیم", "شما", "شود",
	"شوم", "شوند", "شوی", "شوید", "شویم", "صرف", "صورت", "ضد",
	"طبق", "طور", "طوری", "طی", "ظاهرا", "عدم", "عنوان", "فردا",
	"فقط", "فوق", "قاطع", "قبل", "لطفا", "ما", "مان", "ماند",
	"ماندم", "ماندن", "ماندند", "مانده", "ماندی", "ماندید",
	"ماندیم", "مبادا", "متاسفانه", "متن", "مثل", "مثلا", "مجددا",
	"مجموع", "مجموعا", "محکم", "مخالف", "مختلف", "مدت", "مذهب",
	"مرا", "مردم", "مردمی", "مستقیم", "مستقیما", "مسلما", "مشت",
	"مشکل", "مشکلات", "مطابق", "مطمعا", "مطمعنا", "معلوم",
	"معلومه", "معمولا", "مقابل", "ممکن", "ممکنه", "من", "مواد",
	"موضوع", "مورد", "موقع", "مکان", "می", "میان", "میزان",
	"میل", "میلیارد", "میلیون", "میکردم", "میکردند", "میکردی",
	"میکردید", "میکردیم", "میکن", "میکنم", "میکنند", "میکنی",
	"میکنید", "میکنیم", "میگو", "میگوید", "میگویم", "میگویند",
	"میگویی", "میگویید", "میگوییم", "نامه", "نباید", "نبش",
	"نبود", "نبودم", "نبودن", "نبودند", "نبوده", "نبودی",
	"نبودید", "نبودیم", "نخست", "نخستین", "ندارد", "ندارم",
	"ندارند", "نداری", "ندارید", "نداریم", "نداشت", "نداشتم",
	"نداشتن", "نداشتند", "نداشته", "نداشتی", "نداشتید",
	"نداشتیم", "نزد", "نزدیک", "نهایت", "نهایتا", "نشان",
	"نشد", "نشده", "نظیر", "نمی", "نکرد", "نکردم", "نکردن",
	"نکردند", "نکرده", "نکردی", "نکردید", "نکردیم", "نمود",
	"نمودم", "نمودن", "نمودند", "نموده", "نمودی", "نمودید",
	"نمودیم", "نمی", "نمیشود", "نه", "نوع", "نوعی", "نیست",
	"نیستم", "نیستند", "نیستی", "نیستید", "نیستیم", "نیمه",
	"هر", "هرچه", "هست", "هستم", "هستند", "هستی", "هستید",
	"هستیم", "همان", "همانطور", "همانطوری", "همانطورکه", "همه",
	"همیشه", "هنوز", "هنگام", "هیچ", "هیچکدام", "هیچکس",
	"و", "واقع", "واقعا", "واقعی", "وارد", "واقعا", "وظیفه",
	"وقت", "وقتش", "وقتیکه", "ولی", "ولی", "وگو", "وی", "ویا",
	"یا", "یاب", "یابد", "یابم", "یابند", "یابی", "یابید",
	"یابیم", "یافت", "یافتم", "یافتن", "یافتند", "یافته",
	"یافتی", "یافتید", "یافتیم", "یعنی", "یقینا", "یک", "یکی",
}

// PersianAnalyzer is an analyzer for Persian (Farsi) language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.fa.PersianAnalyzer.
//
// PersianAnalyzer uses the StandardTokenizer with Persian normalization and stop words removal.
type PersianAnalyzer struct {
	*BaseAnalyzer
	stopWords *CharArraySet
}

// NewPersianAnalyzer creates a new PersianAnalyzer with default Persian stop words.
func NewPersianAnalyzer() *PersianAnalyzer {
	stopSet := GetWordSetFromStrings(PersianStopWords, true)
	return NewPersianAnalyzerWithWords(stopSet)
}

// NewPersianAnalyzerWithWords creates a PersianAnalyzer with custom stop words.
func NewPersianAnalyzerWithWords(stopWords *CharArraySet) *PersianAnalyzer {
	a := &PersianAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewPersianNormalizationFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *PersianAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *PersianAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *PersianAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

var _ Analyzer = (*PersianAnalyzer)(nil)
var _ AnalyzerInterface = (*PersianAnalyzer)(nil)

// PersianNormalizer normalizes Persian text.
//
// This normalizes various representations of Arabic/Persian characters
type PersianNormalizer struct{}

// NewPersianNormalizer creates a new PersianNormalizer.
func NewPersianNormalizer() *PersianNormalizer {
	return &PersianNormalizer{}
}

// Normalize normalizes Persian text.
func (n *PersianNormalizer) Normalize(input string) string {
	if input == "" {
		return ""
	}

	runes := []rune(input)
	result := make([]rune, 0, len(runes))

	for _, r := range runes {
		normalized := n.normalizeRune(r)
		result = append(result, normalized)
	}

	return string(result)
}

// normalizeRune normalizes a single Persian rune.
func (n *PersianNormalizer) normalizeRune(r rune) rune {
	switch r {
	// Normalize Arabic Yeh to Persian Yeh
	case 0x0649: // ARABIC LETTER ALEF MAKSURA
		return 0x06CC // PERSIAN LETTER YEH
	// Normalize Arabic Kaf to Persian Kaf
	case 0x0643: // ARABIC LETTER KAF
		return 0x06A9 // PERSIAN LETTER KEHEH
	// Remove Kashida/Tatweel
	case 0x0640: // ARABIC TATWEEL
		return 0
	// Normalize different types of spaces
	case 0x200C: // ZERO WIDTH NON-JOINER (ZWNJ)
		return 0x200C // Keep it
	case 0x200D: // ZERO WIDTH JOINER (ZWJ)
		return 0x200D // Keep it
	}
	return r
}

// PersianNormalizationFilter normalizes Persian text.
type PersianNormalizationFilter struct {
	*BaseTokenFilter
	normalizer *PersianNormalizer
}

// NewPersianNormalizationFilter creates a new PersianNormalizationFilter.
func NewPersianNormalizationFilter(input TokenStream) *PersianNormalizationFilter {
	return &PersianNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewPersianNormalizer(),
	}
}

// IncrementToken processes the next token and applies Persian normalization.
func (f *PersianNormalizationFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				normalized := f.normalizer.Normalize(term)
				if normalized != term {
					termAttr.SetEmpty()
					termAttr.AppendString(normalized)
				}
			}
		}
	}

	return hasToken, nil
}

// PersianNormalizationFilterFactory creates PersianNormalizationFilter instances.
type PersianNormalizationFilterFactory struct{}

// NewPersianNormalizationFilterFactory creates a new PersianNormalizationFilterFactory.
func NewPersianNormalizationFilterFactory() *PersianNormalizationFilterFactory {
	return &PersianNormalizationFilterFactory{}
}

// Create creates a new PersianNormalizationFilter.
func (f *PersianNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewPersianNormalizationFilter(input)
}

var _ TokenFilterFactory = (*PersianNormalizationFilterFactory)(nil)
