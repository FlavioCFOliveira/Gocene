// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package completion

import "strings"

// KatakanaRomanizer converts katakana strings to romanized (rōmaji) forms.
// Multiple romanization outputs are possible for a single input because some
// keystrokes map to multiple romaji sequences.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.completion.KatakanaRomanizer from Apache
// Lucene 10.4.0.
//
// Deviation: the Java original loads a romaji_map.txt resource file at
// class-load time and uses binary search over sorted CharsRef arrays for
// prefix lookup. This Go port uses an inline map of the standard Modified
// Hepburn / Kunrei / Wāpuro romaji mappings rather than an external file,
// deferring resource-based loading to the codec sprint.
type KatakanaRomanizer struct {
	// romajiMap maps katakana sequences to their possible romaji outputs.
	romajiMap map[string][]string
}

var defaultRomanizer = buildDefaultRomanizer()

// GetInstance returns the singleton KatakanaRomanizer.
func GetInstance() *KatakanaRomanizer { return defaultRomanizer }

// buildDefaultRomanizer constructs the inline romaji map.
func buildDefaultRomanizer() *KatakanaRomanizer {
	m := map[string][]string{
		"ア": {"a"}, "イ": {"i"}, "ウ": {"u"}, "エ": {"e"}, "オ": {"o"},
		"カ": {"ka"}, "キ": {"ki"}, "ク": {"ku"}, "ケ": {"ke"}, "コ": {"ko"},
		"サ": {"sa"}, "シ": {"shi", "si"}, "ス": {"su"}, "セ": {"se"}, "ソ": {"so"},
		"タ": {"ta"}, "チ": {"chi", "ti"}, "ツ": {"tsu", "tu"}, "テ": {"te"}, "ト": {"to"},
		"ナ": {"na"}, "ニ": {"ni"}, "ヌ": {"nu"}, "ネ": {"ne"}, "ノ": {"no"},
		"ハ": {"ha"}, "ヒ": {"hi"}, "フ": {"fu", "hu"}, "ヘ": {"he"}, "ホ": {"ho"},
		"マ": {"ma"}, "ミ": {"mi"}, "ム": {"mu"}, "メ": {"me"}, "モ": {"mo"},
		"ヤ": {"ya"}, "ユ": {"yu"}, "ヨ": {"yo"},
		"ラ": {"ra"}, "リ": {"ri"}, "ル": {"ru"}, "レ": {"re"}, "ロ": {"ro"},
		"ワ": {"wa"}, "ヲ": {"wo", "o"}, "ン": {"n", "nn"},
		"ガ": {"ga"}, "ギ": {"gi"}, "グ": {"gu"}, "ゲ": {"ge"}, "ゴ": {"go"},
		"ザ": {"za"}, "ジ": {"ji", "zi"}, "ズ": {"zu"}, "ゼ": {"ze"}, "ゾ": {"zo"},
		"ダ": {"da"}, "ヂ": {"di", "ji"}, "ヅ": {"du", "zu"}, "デ": {"de"}, "ド": {"do"},
		"バ": {"ba"}, "ビ": {"bi"}, "ブ": {"bu"}, "ベ": {"be"}, "ボ": {"bo"},
		"パ": {"pa"}, "ピ": {"pi"}, "プ": {"pu"}, "ペ": {"pe"}, "ポ": {"po"},
		"ァ": {"a"}, "ィ": {"i"}, "ゥ": {"u"}, "ェ": {"e"}, "ォ": {"o"},
		"ッ": {"tsu", "tu", "xtu"},
		"ャ": {"ya"}, "ュ": {"yu"}, "ョ": {"yo"},
		"キャ": {"kya"}, "キュ": {"kyu"}, "キョ": {"kyo"},
		"シャ": {"sha", "sya"}, "シュ": {"shu", "syu"}, "ショ": {"sho", "syo"},
		"チャ": {"cha", "tya"}, "チュ": {"chu", "tyu"}, "チョ": {"cho", "tyo"},
		"ニャ": {"nya"}, "ニュ": {"nyu"}, "ニョ": {"nyo"},
		"ヒャ": {"hya"}, "ヒュ": {"hyu"}, "ヒョ": {"hyo"},
		"ミャ": {"mya"}, "ミュ": {"myu"}, "ミョ": {"myo"},
		"リャ": {"rya"}, "リュ": {"ryu"}, "リョ": {"ryo"},
		"ギャ": {"gya"}, "ギュ": {"gyu"}, "ギョ": {"gyo"},
		"ジャ": {"ja", "jya", "zya"}, "ジュ": {"ju", "jyu", "zyu"}, "ジョ": {"jo", "jyo", "zyo"},
		"ビャ": {"bya"}, "ビュ": {"byu"}, "ビョ": {"byo"},
		"ピャ": {"pya"}, "ピュ": {"pyu"}, "ピョ": {"pyo"},
		"ファ": {"fa"}, "フィ": {"fi"}, "フェ": {"fe"}, "フォ": {"fo"},
		"ウィ": {"wi"}, "ウェ": {"we"}, "ウォ": {"wo"},
		"ー": {"-"},
		"　": {" "}, // full-width space
	}
	return &KatakanaRomanizer{romajiMap: m}
}

// Romanize converts a katakana string into all possible romaji sequences.
// Returns nil if no mapping is possible.
func (r *KatakanaRomanizer) Romanize(input string) []string {
	if input == "" {
		return []string{""}
	}
	runes := []rune(input)
	return r.romanizeRunes(runes, 0)
}

func (r *KatakanaRomanizer) romanizeRunes(runes []rune, pos int) []string {
	if pos >= len(runes) {
		return []string{""}
	}
	// Try longest match first (up to 2 runes for digraphs like キョ).
	maxLen := 2
	if pos+maxLen > len(runes) {
		maxLen = len(runes) - pos
	}
	for l := maxLen; l >= 1; l-- {
		key := string(runes[pos : pos+l])
		candidates, ok := r.romajiMap[key]
		if !ok {
			continue
		}
		rest := r.romanizeRunes(runes, pos+l)
		if rest == nil {
			continue
		}
		var results []string
		for _, c := range candidates {
			for _, tail := range rest {
				results = append(results, c+tail)
			}
		}
		return results
	}
	// No mapping: pass through the rune as-is.
	rest := r.romanizeRunes(runes, pos+1)
	if rest == nil {
		return nil
	}
	var results []string
	ch := string(runes[pos])
	for _, tail := range rest {
		results = append(results, ch+tail)
	}
	return results
}

// RomanizeFirst returns the first (canonical) romanization of input.
func (r *KatakanaRomanizer) RomanizeFirst(input string) string {
	results := r.Romanize(input)
	if len(results) == 0 {
		return input
	}
	return results[0]
}

// RomanizeToLower returns all lowercase romanizations of input.
func (r *KatakanaRomanizer) RomanizeToLower(input string) []string {
	results := r.Romanize(input)
	for i, s := range results {
		results[i] = strings.ToLower(s)
	}
	return results
}
