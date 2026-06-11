package hunspell

import (
	"testing"
)

func TestDebugCheckCompoundPattern2(t *testing.T) {
	dic := loadTestDictionary(t, false, "checkcompoundpattern2.aff", "checkcompoundpattern2.dic")
	t.Logf("compoundFlag=%c (%d), compoundBegin=%c (%d), compoundEnd=%c (%d)",
		dic.compoundFlag, dic.compoundFlag, dic.compoundBegin, dic.compoundBegin,
		dic.compoundEnd, dic.compoundEnd)
	t.Logf("compoundMin=%d, compoundMax=%d", dic.compoundMin, dic.compoundMax)
	t.Logf("checkCompoundPatterns=%d", len(dic.checkCompoundPatterns))
	for i, pat := range dic.checkCompoundPatterns {
		t.Logf("  pat[%d]: endChars=%q beginChars=%q replacement=%q",
			i, pat.endChars, pat.beginChars, pat.replacement)
	}

	h := NewHunspell(dic)
	s := NewStemmer(dic)

	// Check that "foo" and "bar" are findable in compound context
	runes := []rune("foobar")
	t.Logf("findStem foo in foobar(0,3,CompoundBegin):")
	stem := h.findStem(runes, 0, 3, WordCaseNeutral, WordContextCompoundBegin)
	if stem != nil {
		t.Logf("  found: %q (entryID=%d)", stem.Word, stem.EntryID)
	} else {
		t.Logf("  NOT FOUND")
	}

	// Check doStem directly
	t.Logf("doStem foobar(0,3,CompoundBegin):")
	s.doStem(runes, 0, 3, WordContextCompoundBegin, func(stem []rune, formID, morphDataID, _, _, _, _ int) bool {
		t.Logf("  stem=%q entryID=%d", string(stem), formID)
		return true
	})

	// Check bar in compound end context
	rem := []rune("bar")
	t.Logf("findStem bar(0,3,CompoundEnd):")
	stem2 := h.findStem(rem, 0, 3, WordCaseNeutral, WordContextCompoundEnd)
	if stem2 != nil {
		t.Logf("  found: %q (entryID=%d)", stem2.Word, stem2.EntryID)
	} else {
		t.Logf("  NOT FOUND")
	}

	// Check ExpandReplacement
	word := []rune("fozar")
	for breakPos := 1; breakPos < len(word); breakPos++ {
		for _, pat := range dic.checkCompoundPatterns {
			expanded := pat.ExpandReplacement(word, breakPos)
			if expanded != nil {
				t.Logf("breakPos=%d: ExpandReplacement returned %q", breakPos, string(expanded))
				stem := h.findStem(expanded, 0, breakPos+pat.EndLength(), WordCaseNeutral, WordContextCompoundBegin)
				if stem != nil {
					t.Logf("  stem found: %q", stem.Word)
					rem := expanded[breakPos+pat.EndLength():]
					stem2 := h.findStem(rem, 0, len(rem), WordCaseNeutral, WordContextCompoundEnd)
					if stem2 != nil {
						t.Logf("  second stem found: %q", stem2.Word)
						t.Logf("  mayCompound=%v", partMayCompound(dic, stem, stem2))
					} else {
						t.Logf("  second stem NOT FOUND")
					}
				} else {
					t.Logf("  stem NOT FOUND")
				}
			}
		}
	}

	// Test checkCompounds directly
	t.Logf("checkCompounds(\"fozar\",...): %v", h.checkCompounds([]rune("fozar"), 5, WordCaseNeutral))
	t.Logf("checkCompounds(\"barfoo\",...): %v", h.checkCompounds([]rune("barfoo"), 6, WordCaseNeutral))
	t.Logf("checkCompounds(\"foobar\",...): %v", h.checkCompounds([]rune("foobar"), 6, WordCaseNeutral))

	// Test checkCompoundsFull directly
	t.Logf("checkCompoundsFull(\"fozar\",...): %v", h.checkCompoundsFull([]rune("fozar"), WordCaseNeutral, nil))
	t.Logf("checkCompoundsFull(\"barfoo\",...): %v", h.checkCompoundsFull([]rune("barfoo"), WordCaseNeutral, nil))
	t.Logf("checkCompoundsFull(\"foobar\",...): %v", h.checkCompoundsFull([]rune("foobar"), WordCaseNeutral, nil))
}

func partMayCompound(d *Dictionary, first, second *Root) bool {
	// Copy of the compoundPart.mayCompound logic
	_ = d
	_ = first
	_ = second
	return true
}
