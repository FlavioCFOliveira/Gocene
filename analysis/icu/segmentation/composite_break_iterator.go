// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

// CompositeBreakIterator is an internal break iterator for multilingual text,
// following recommendations from UAX #29: Unicode Text Segmentation
// (https://unicode.org/reports/tr29/).
//
// Text is first divided into script boundaries via ScriptIterator. Processing
// is then delegated to the appropriate RuleBasedBreakIterator for each script.
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.CompositeBreakIterator
// (Apache Lucene 10.4.0).
//
// Deviation: Java indexes the wordBreakers array by UScript.getIntPropertyMaxValue
// (UProperty.SCRIPT) = 255. We allocate a fixed-size array of 256 entries, which
// covers all currently defined ISO 15924 numeric codes. UScriptJapanese (105) is
// the highest synthetic code used by Gocene.
type CompositeBreakIterator struct {
	config         ICUTokenizerConfig
	wordBreakers   [256]*BreakIteratorWrapper
	rbbi           *BreakIteratorWrapper
	scriptIterator *ScriptIterator
	text           []rune
}

// NewCompositeBreakIterator creates a CompositeBreakIterator backed by config.
func NewCompositeBreakIterator(config ICUTokenizerConfig) *CompositeBreakIterator {
	return &CompositeBreakIterator{
		config:         config,
		scriptIterator: NewScriptIterator(config.CombineCJ()),
	}
}

// Next retrieves the next break position.
//
// If the current RBBI range is exhausted within the script boundary, the next
// script boundary is examined. Returns Done when all text has been consumed.
func (c *CompositeBreakIterator) Next() int {
	next := c.rbbi.Next()
	for next == Done && c.scriptIterator.Next() {
		c.rbbi = c.getBreakIterator(c.scriptIterator.GetScriptCode())
		start := c.scriptIterator.GetScriptStart()
		length := c.scriptIterator.GetScriptLimit() - start
		c.rbbi.SetText(c.text, start, length)
		next = c.rbbi.Next()
	}
	if next == Done {
		return Done
	}
	return next + c.scriptIterator.GetScriptStart()
}

// Current returns the current break position, or Done.
func (c *CompositeBreakIterator) Current() int {
	current := c.rbbi.Current()
	if current == Done {
		return Done
	}
	return current + c.scriptIterator.GetScriptStart()
}

// GetRuleStatus returns the rule-status code from the active break iterator.
func (c *CompositeBreakIterator) GetRuleStatus() int {
	return c.rbbi.GetRuleStatus()
}

// GetScriptCode returns the UScript code for the current script run.
func (c *CompositeBreakIterator) GetScriptCode() int {
	return c.scriptIterator.GetScriptCode()
}

// SetText configures the iterator to analyse text[start : start+length].
func (c *CompositeBreakIterator) SetText(text []rune, start, length int) {
	c.text = text
	c.scriptIterator.SetText(text, start, length)
	if c.scriptIterator.Next() {
		c.rbbi = c.getBreakIterator(c.scriptIterator.GetScriptCode())
		s := c.scriptIterator.GetScriptStart()
		l := c.scriptIterator.GetScriptLimit() - s
		c.rbbi.SetText(text, s, l)
	} else {
		c.rbbi = c.getBreakIterator(UScriptCommon)
		c.rbbi.SetText(text, 0, 0)
	}
}

// getBreakIterator returns the cached BreakIteratorWrapper for scriptCode,
// creating it on first access.
func (c *CompositeBreakIterator) getBreakIterator(scriptCode int) *BreakIteratorWrapper {
	if scriptCode < 0 || scriptCode >= len(c.wordBreakers) {
		scriptCode = UScriptCommon
	}
	if c.wordBreakers[scriptCode] == nil {
		c.wordBreakers[scriptCode] = NewBreakIteratorWrapper(c.config.GetBreakIterator(scriptCode))
	}
	return c.wordBreakers[scriptCode]
}
