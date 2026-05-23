// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "strings"

// ToStringUtil provides English translations of Japanese morphological data.
// It is used only for debugging and reflection purposes.
//
// This is the Go port of org.apache.lucene.analysis.ja.dict.ToStringUtil from
// Apache Lucene 10.4.0.
var ToStringUtil = toStringUtil{}

type toStringUtil struct{}

// posTranslations maps Japanese POS tags to their English equivalents.
var posTranslations = map[string]string{
	"名詞":               "noun",
	"名詞-一般":            "noun-common",
	"名詞-固有名詞":          "noun-proper",
	"名詞-固有名詞-一般":       "noun-proper-misc",
	"名詞-固有名詞-人名":       "noun-proper-person",
	"名詞-固有名詞-人名-一般":    "noun-proper-person-misc",
	"名詞-固有名詞-人名-姓":     "noun-proper-person-surname",
	"名詞-固有名詞-人名-名":     "noun-proper-person-given_name",
	"名詞-固有名詞-組織":       "noun-proper-organization",
	"名詞-固有名詞-地域":       "noun-proper-place",
	"名詞-固有名詞-地域-一般":    "noun-proper-place-misc",
	"名詞-固有名詞-地域-国":     "noun-proper-place-country",
	"名詞-代名詞":           "noun-pronoun",
	"名詞-代名詞-一般":        "noun-pronoun-misc",
	"名詞-代名詞-縮約":        "noun-pronoun-contraction",
	"名詞-副詞可能":          "noun-adverbial",
	"名詞-サ変接続":          "noun-verbal",
	"名詞-形容動詞語幹":        "noun-adjective-base",
	"名詞-数":             "noun-numeric",
	"名詞-非自立":           "noun-affix",
	"名詞-非自立-一般":        "noun-affix-misc",
	"名詞-非自立-副詞可能":      "noun-affix-adverbial",
	"名詞-非自立-助動詞語幹":     "noun-affix-aux",
	"名詞-非自立-形容動詞語幹":    "noun-affix-adjective-base",
	"名詞-特殊":            "noun-special",
	"名詞-特殊-助動詞語幹":      "noun-special-aux",
	"名詞-接尾":            "noun-suffix",
	"名詞-接尾-一般":         "noun-suffix-misc",
	"名詞-接尾-人名":         "noun-suffix-person",
	"名詞-接尾-地域":         "noun-suffix-place",
	"名詞-接尾-サ変接続":       "noun-suffix-verbal",
	"名詞-接尾-助動詞語幹":      "noun-suffix-aux",
	"名詞-接尾-形容動詞語幹":     "noun-suffix-adjective-base",
	"名詞-接尾-副詞可能":       "noun-suffix-adverbial",
	"名詞-接尾-助数詞":        "noun-suffix-classifier",
	"名詞-接尾-特殊":         "noun-suffix-special",
	"名詞-接続詞的":          "noun-suffix-conjunctive",
	"名詞-動詞非自立的":        "noun-verbal_aux",
	"名詞-引用文字列":         "noun-quotation",
	"名詞-ナイ形容詞語幹":       "noun-nai_adjective",
	"接頭詞":              "prefix",
	"接頭詞-名詞接続":         "prefix-nominal",
	"接頭詞-動詞接続":         "prefix-verbal",
	"接頭詞-形容詞接続":        "prefix-adjectival",
	"接頭詞-数接続":          "prefix-numerical",
	"動詞":               "verb",
	"動詞-自立":            "verb-main",
	"動詞-非自立":           "verb-auxiliary",
	"動詞-接尾":            "verb-suffix",
	"形容詞":              "adjective",
	"形容詞-自立":           "adjective-main",
	"形容詞-非自立":          "adjective-auxiliary",
	"形容詞-接尾":           "adjective-suffix",
	"副詞":               "adverb",
	"副詞-一般":            "adverb-misc",
	"副詞-助詞類接続":         "adverb-particle_conjunction",
	"連体詞":              "adnominal",
	"接続詞":              "conjunction",
	"助詞":               "particle",
	"助詞-格助詞":           "particle-case",
	"助詞-格助詞-一般":        "particle-case-misc",
	"助詞-格助詞-引用":        "particle-case-quote",
	"助詞-格助詞-連語":        "particle-case-compound",
	"助詞-接続助詞":          "particle-conjunctive",
	"助詞-係助詞":           "particle-dependency",
	"助詞-副助詞":           "particle-adverbial",
	"助詞-間投助詞":          "particle-interjective",
	"助詞-並立助詞":          "particle-coordinate",
	"助詞-終助詞":           "particle-final",
	"助詞-副助詞／並立助詞／終助詞": "particle-adverbial/conjunctive/final",
	"助詞-連体化":           "particle-adnominalizer",
	"助詞-副詞化":           "particle-adnominalizer",
	"助詞-特殊":            "particle-special",
	"助動詞":              "auxiliary-verb",
	"感動詞":              "interjection",
	"記号":               "symbol",
	"記号-一般":            "symbol-misc",
	"記号-句点":            "symbol-period",
	"記号-読点":            "symbol-comma",
	"記号-空白":            "symbol-space",
	"記号-括弧開":           "symbol-open_bracket",
	"記号-括弧閉":           "symbol-close_bracket",
	"記号-アルファベット":       "symbol-alphabetic",
	"その他":              "other",
	"その他-間投":           "other-interjection",
	"フィラー":             "filler",
	"非言語音":             "non-verbal",
	"語断片":              "fragment",
	"未知語":              "unknown",
}

// inflTypeTranslations maps Japanese inflection types to English.
var inflTypeTranslations = map[string]string{
	"*":          "*",
	"形容詞・アウオ段":   "adj-group-a-o-u",
	"形容詞・イ段":    "adj-group-i",
	"形容詞・イイ":    "adj-group-ii",
	"不変化型":      "non-inflectional",
	"特殊・タ":      "special-da",
	"特殊・ダ":      "special-ta",
	"文語・ゴトシ":    "classical-gotoshi",
	"特殊・ジャ":     "special-ja",
	"特殊・ナイ":     "special-nai",
	"五段・ラ行特殊":   "5-row-cons-r-special",
	"特殊・ヌ":      "special-nu",
	"文語・キ":      "classical-ki",
	"特殊・タイ":     "special-tai",
	"文語・ベシ":     "classical-beshi",
	"特殊・ヤ":      "special-ya",
	"文語・マジ":     "classical-maji",
	"下二・タ行":     "2-row-lower-cons-t",
	"特殊・デス":     "special-desu",
	"特殊・マス":     "special-masu",
	"五段・ラ行アル":   "5-row-aru",
	"文語・ナリ":     "classical-nari",
	"文語・リ":      "classical-ri",
	"文語・ケリ":     "classical-keri",
	"文語・ル":      "classical-ru",
	"五段・カ行イ音便":  "5-row-cons-k-i-onbin",
	"五段・サ行":     "5-row-cons-s",
	"一段":        "1-row",
	"五段・ワ行促音便":  "5-row-cons-w-cons-onbin",
	"五段・マ行":     "5-row-cons-m",
	"五段・タ行":     "5-row-cons-t",
	"五段・ラ行":     "5-row-cons-r",
	"サ変・−スル":    "irregular-suffix-suru",
	"五段・ガ行":     "5-row-cons-g",
	"サ変・−ズル":    "irregular-suffix-zuru",
	"五段・バ行":     "5-row-cons-b",
	"五段・ワ行ウ音便":  "5-row-cons-w-u-onbin",
	"下二・ダ行":     "2-row-lower-cons-d",
	"五段・カ行促音便ユク": "5-row-cons-k-cons-onbin-yuku",
	"上二・ダ行":     "2-row-upper-cons-d",
	"五段・カ行促音便":  "5-row-cons-k-cons-onbin",
	"一段・得ル":     "1-row-eru",
	"四段・タ行":     "4-row-cons-t",
	"五段・ナ行":     "5-row-cons-n",
	"下二・ハ行":     "2-row-lower-cons-h",
	"四段・ハ行":     "4-row-cons-h",
	"四段・バ行":     "4-row-cons-b",
	"サ変・スル":     "irregular-suru",
	"上二・ハ行":     "2-row-upper-cons-h",
	"下二・マ行":     "2-row-lower-cons-m",
	"四段・サ行":     "4-row-cons-s",
	"下二・ガ行":     "2-row-lower-cons-g",
	"カ変・来ル":     "kuru-kanji",
	"一段・クレル":    "1-row-kureru",
	"下二・得":      "2-row-lower-u",
	"カ変・クル":     "kuru-kana",
	"ラ変":        "irregular-cons-r",
	"下二・カ行":     "2-row-lower-cons-k",
}

// inflFormTranslations maps Japanese inflection forms to English.
var inflFormTranslations = map[string]string{
	"*":          "*",
	"基本形":        "base",
	"文語基本形":      "classical-base",
	"未然ヌ接続":      "imperfective-nu-connection",
	"未然ウ接続":      "imperfective-u-connection",
	"連用タ接続":      "conjunctive-ta-connection",
	"連用テ接続":      "conjunctive-te-connection",
	"連用ゴザイ接続":    "conjunctive-gozai-connection",
	"体言接続":       "uninflected-connection",
	"仮定形":        "subjunctive",
	"命令ｅ":        "imperative-e",
	"仮定縮約１":      "conditional-contracted-1",
	"仮定縮約２":      "conditional-contracted-2",
	"ガル接続":       "garu-connection",
	"未然形":        "imperfective",
	"連用形":        "conjunctive",
	"音便基本形":      "onbin-base",
	"連用デ接続":      "conjunctive-de-connection",
	"未然特殊":       "imperfective-special",
	"命令ｉ":        "imperative-i",
	"連用ニ接続":      "conjunctive-ni-connection",
	"命令ｙｏ":       "imperative-yo",
	"体言接続特殊":     "adnominal-special",
	"命令ｒｏ":       "imperative-ro",
	"体言接続特殊２":    "uninflected-special-connection-2",
	"未然レル接続":     "imperfective-reru-connection",
	"現代基本形":      "modern-base",
	"基本形-促音便":    "base-onbin",
}

// GetPOSTranslation returns the English translation of the Japanese POS tag s,
// or empty string if unknown.
func GetPOSTranslation(s string) string {
	return posTranslations[s]
}

// GetInflectionTypeTranslation returns the English translation of the Japanese
// inflection type s, or empty string if unknown.
func GetInflectionTypeTranslation(s string) string {
	return inflTypeTranslations[s]
}

// GetInflectedFormTranslation returns the English translation of the Japanese
// inflected form s, or empty string if unknown.
func GetInflectedFormTranslation(s string) string {
	return inflFormTranslations[s]
}

// GetRomanization romanizes a katakana string using modified Hepburn romanization.
func GetRomanization(s string) string {
	var sb strings.Builder
	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n; i++ {
		ch := runes[i]
		var ch2 rune
		var ch3 rune
		if i < n-1 {
			ch2 = runes[i+1]
		}
		if i < n-2 {
			ch3 = runes[i+2]
		}
		advance := romanizeChar(ch, ch2, ch3, &sb)
		i += advance
	}
	return sb.String()
}

// GetRomanizationInto writes the romanization of s into builder.
func GetRomanizationInto(builder *strings.Builder, s string) {
	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n; i++ {
		ch := runes[i]
		var ch2, ch3 rune
		if i < n-1 {
			ch2 = runes[i+1]
		}
		if i < n-2 {
			ch3 = runes[i+2]
		}
		advance := romanizeChar(ch, ch2, ch3, builder)
		i += advance
	}
}

// romanizeChar writes the romanization of ch into b, and returns the number of
// additional characters consumed (lookahead).
//
//nolint:gocyclo // faithfully ported from Lucene's ToStringUtil.getRomanization
func romanizeChar(ch, ch2, ch3 rune, b *strings.Builder) int {
	switch ch {
	case 'ッ':
		switch ch2 {
		case 'カ', 'キ', 'ク', 'ケ', 'コ':
			b.WriteByte('k')
		case 'サ', 'シ', 'ス', 'セ', 'ソ':
			b.WriteByte('s')
		case 'タ', 'チ', 'ツ', 'テ', 'ト':
			b.WriteByte('t')
		case 'パ', 'ピ', 'プ', 'ペ', 'ポ':
			b.WriteByte('p')
		}
	case 'ア':
		b.WriteByte('a')
	case 'イ':
		if ch2 == 'ィ' {
			b.WriteString("yi")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("ye")
			return 1
		}
		b.WriteByte('i')
	case 'ウ':
		switch ch2 {
		case 'ァ':
			b.WriteString("wa")
			return 1
		case 'ィ':
			b.WriteString("wi")
			return 1
		case 'ゥ':
			b.WriteString("wu")
			return 1
		case 'ェ':
			b.WriteString("we")
			return 1
		case 'ォ':
			b.WriteString("wo")
			return 1
		case 'ュ':
			b.WriteString("wyu")
			return 1
		default:
			b.WriteByte('u')
		}
	case 'エ':
		b.WriteByte('e')
	case 'オ':
		if ch2 == 'ウ' {
			b.WriteString("ō")
			return 1
		}
		b.WriteByte('o')
	case 'カ':
		b.WriteString("ka")
	case 'キ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("kyō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("kyū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("kya")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("kyo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("kyu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("kye")
			return 1
		}
		b.WriteString("ki")
	case 'ク':
		switch ch2 {
		case 'ァ':
			b.WriteString("kwa")
			return 1
		case 'ィ':
			b.WriteString("kwi")
			return 1
		case 'ェ':
			b.WriteString("kwe")
			return 1
		case 'ォ':
			b.WriteString("kwo")
			return 1
		case 'ヮ':
			b.WriteString("kwa")
			return 1
		default:
			b.WriteString("ku")
		}
	case 'ケ':
		b.WriteString("ke")
	case 'コ':
		if ch2 == 'ウ' {
			b.WriteString("kō")
			return 1
		}
		b.WriteString("ko")
	case 'サ':
		b.WriteString("sa")
	case 'シ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("shō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("shū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("sha")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("sho")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("shu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("she")
			return 1
		}
		b.WriteString("shi")
	case 'ス':
		if ch2 == 'ィ' {
			b.WriteString("si")
			return 1
		}
		b.WriteString("su")
	case 'セ':
		b.WriteString("se")
	case 'ソ':
		if ch2 == 'ウ' {
			b.WriteString("sō")
			return 1
		}
		b.WriteString("so")
	case 'タ':
		b.WriteString("ta")
	case 'チ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("chō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("chū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("cha")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("cho")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("chu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("che")
			return 1
		}
		b.WriteString("chi")
	case 'ツ':
		if ch2 == 'ァ' {
			b.WriteString("tsa")
			return 1
		} else if ch2 == 'ィ' {
			b.WriteString("tsi")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("tse")
			return 1
		} else if ch2 == 'ォ' {
			b.WriteString("tso")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("tsyu")
			return 1
		}
		b.WriteString("tsu")
	case 'テ':
		if ch2 == 'ィ' {
			b.WriteString("ti")
			return 1
		} else if ch2 == 'ゥ' {
			b.WriteString("tu")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("tyu")
			return 1
		}
		b.WriteString("te")
	case 'ト':
		if ch2 == 'ウ' {
			b.WriteString("tō")
			return 1
		} else if ch2 == 'ゥ' {
			b.WriteString("tu")
			return 1
		}
		b.WriteString("to")
	case 'ナ':
		b.WriteString("na")
	case 'ニ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("nyō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("nyū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("nya")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("nyo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("nyu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("nye")
			return 1
		}
		b.WriteString("ni")
	case 'ヌ':
		b.WriteString("nu")
	case 'ネ':
		b.WriteString("ne")
	case 'ノ':
		if ch2 == 'ウ' {
			b.WriteString("nō")
			return 1
		}
		b.WriteString("no")
	case 'ハ':
		b.WriteString("ha")
	case 'ヒ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("hyō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("hyū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("hya")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("hyo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("hyu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("hye")
			return 1
		}
		b.WriteString("hi")
	case 'フ':
		if ch2 == 'ャ' {
			b.WriteString("fya")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("fyu")
			return 1
		} else if ch2 == 'ィ' && ch3 == 'ェ' {
			b.WriteString("fye")
			return 2
		} else if ch2 == 'ョ' {
			b.WriteString("fyo")
			return 1
		} else if ch2 == 'ァ' {
			b.WriteString("fa")
			return 1
		} else if ch2 == 'ィ' {
			b.WriteString("fi")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("fe")
			return 1
		} else if ch2 == 'ォ' {
			b.WriteString("fo")
			return 1
		}
		b.WriteString("fu")
	case 'ヘ':
		b.WriteString("he")
	case 'ホ':
		if ch2 == 'ウ' {
			b.WriteString("hō")
			return 1
		} else if ch2 == 'ゥ' {
			b.WriteString("hu")
			return 1
		}
		b.WriteString("ho")
	case 'マ':
		b.WriteString("ma")
	case 'ミ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("myō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("myū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("mya")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("myo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("myu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("mye")
			return 1
		}
		b.WriteString("mi")
	case 'ム':
		b.WriteString("mu")
	case 'メ':
		b.WriteString("me")
	case 'モ':
		if ch2 == 'ウ' {
			b.WriteString("mō")
			return 1
		}
		b.WriteString("mo")
	case 'ヤ':
		b.WriteString("ya")
	case 'ユ':
		b.WriteString("yu")
	case 'ヨ':
		if ch2 == 'ウ' {
			b.WriteString("yō")
			return 1
		}
		b.WriteString("yo")
	case 'ラ':
		if ch2 == '゜' {
			b.WriteString("la")
			return 1
		}
		b.WriteString("ra")
	case 'リ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("ryō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("ryū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("rya")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("ryo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("ryu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("rye")
			return 1
		} else if ch2 == '゜' {
			b.WriteString("li")
			return 1
		}
		b.WriteString("ri")
	case 'ル':
		if ch2 == '゜' {
			b.WriteString("lu")
			return 1
		}
		b.WriteString("ru")
	case 'レ':
		if ch2 == '゜' {
			b.WriteString("le")
			return 1
		}
		b.WriteString("re")
	case 'ロ':
		if ch2 == 'ウ' {
			b.WriteString("rō")
			return 1
		} else if ch2 == '゜' {
			b.WriteString("lo")
			return 1
		}
		b.WriteString("ro")
	case 'ワ':
		b.WriteString("wa")
	case 'ヰ':
		b.WriteByte('i')
	case 'ヱ':
		b.WriteByte('e')
	case 'ヲ':
		b.WriteByte('o')
	case 'ン':
		switch ch2 {
		case 'バ', 'ビ', 'ブ', 'ベ', 'ボ',
			'パ', 'ピ', 'プ', 'ペ', 'ポ',
			'マ', 'ミ', 'ム', 'メ', 'モ':
			b.WriteByte('m')
		case 'ヤ', 'ユ', 'ヨ', 'ア', 'イ', 'ウ', 'エ', 'オ':
			b.WriteString("n'")
		default:
			b.WriteByte('n')
		}
	case 'ガ':
		b.WriteString("ga")
	case 'ギ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("gyō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("gyū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("gya")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("gyo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("gyu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("gye")
			return 1
		}
		b.WriteString("gi")
	case 'グ':
		switch ch2 {
		case 'ァ':
			b.WriteString("gwa")
			return 1
		case 'ィ':
			b.WriteString("gwi")
			return 1
		case 'ェ':
			b.WriteString("gwe")
			return 1
		case 'ォ':
			b.WriteString("gwo")
			return 1
		case 'ヮ':
			b.WriteString("gwa")
			return 1
		default:
			b.WriteString("gu")
		}
	case 'ゲ':
		b.WriteString("ge")
	case 'ゴ':
		if ch2 == 'ウ' {
			b.WriteString("gō")
			return 1
		}
		b.WriteString("go")
	case 'ザ':
		b.WriteString("za")
	case 'ジ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("jō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("jū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("ja")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("jo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("ju")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("je")
			return 1
		}
		b.WriteString("ji")
	case 'ズ':
		if ch2 == 'ィ' {
			b.WriteString("zi")
			return 1
		}
		b.WriteString("zu")
	case 'ゼ':
		b.WriteString("ze")
	case 'ゾ':
		if ch2 == 'ウ' {
			b.WriteString("zō")
			return 1
		}
		b.WriteString("zo")
	case 'ダ':
		b.WriteString("da")
	case 'ヂ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("jō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("jū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("ja")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("jo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("ju")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("je")
			return 1
		}
		b.WriteString("ji")
	case 'ヅ':
		b.WriteString("zu")
	case 'デ':
		if ch2 == 'ィ' {
			b.WriteString("di")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("dyu")
			return 1
		}
		b.WriteString("de")
	case 'ド':
		if ch2 == 'ウ' {
			b.WriteString("dō")
			return 1
		} else if ch2 == 'ゥ' {
			b.WriteString("du")
			return 1
		}
		b.WriteString("do")
	case 'バ':
		b.WriteString("ba")
	case 'ビ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("byō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("byū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("bya")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("byo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("byu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("bye")
			return 1
		}
		b.WriteString("bi")
	case 'ブ':
		b.WriteString("bu")
	case 'ベ':
		b.WriteString("be")
	case 'ボ':
		if ch2 == 'ウ' {
			b.WriteString("bō")
			return 1
		}
		b.WriteString("bo")
	case 'パ':
		b.WriteString("pa")
	case 'ピ':
		if ch2 == 'ョ' && ch3 == 'ウ' {
			b.WriteString("pyō")
			return 2
		} else if ch2 == 'ュ' && ch3 == 'ウ' {
			b.WriteString("pyū")
			return 2
		} else if ch2 == 'ャ' {
			b.WriteString("pya")
			return 1
		} else if ch2 == 'ョ' {
			b.WriteString("pyo")
			return 1
		} else if ch2 == 'ュ' {
			b.WriteString("pyu")
			return 1
		} else if ch2 == 'ェ' {
			b.WriteString("pye")
			return 1
		}
		b.WriteString("pi")
	case 'プ':
		b.WriteString("pu")
	case 'ペ':
		b.WriteString("pe")
	case 'ポ':
		if ch2 == 'ウ' {
			b.WriteString("pō")
			return 1
		}
		b.WriteString("po")
	case 'ヷ':
		b.WriteString("va")
	case 'ヸ':
		b.WriteString("vi")
	case 'ヹ':
		b.WriteString("ve")
	case 'ヺ':
		b.WriteString("vo")
	case 'ヴ':
		if ch2 == 'ィ' && ch3 == 'ェ' {
			b.WriteString("vye")
			return 2
		}
		b.WriteByte('v')
	case 'ァ':
		b.WriteByte('a')
	case 'ィ':
		b.WriteByte('i')
	case 'ゥ':
		b.WriteByte('u')
	case 'ェ':
		b.WriteByte('e')
	case 'ォ':
		b.WriteByte('o')
	case 'ヮ':
		b.WriteString("wa")
	case 'ャ':
		b.WriteString("ya")
	case 'ュ':
		b.WriteString("yu")
	case 'ョ':
		b.WriteString("yo")
	case 'ー':
		// prolonged sound mark — emit nothing
	default:
		b.WriteRune(ch)
	}
	return 0
}
