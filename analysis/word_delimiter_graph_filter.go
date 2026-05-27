// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"unicode"
)

// WordDelimiterGraphFilter produces a token graph by splitting words at delimiters,
// case changes, and number transitions.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.miscellaneous.WordDelimiterGraphFilter.
//
// The filter creates a graph with multiple paths for each input token. For example,
// "PowerShot" produces a graph where you can match "PowerShot" (the full token),
// "Power" (first subword), "Shot" (second subword), or both "Power" and "Shot".
//
// The filter handles:
//   - camelCase and PascalCase transitions (e.g., "PowerShot" -> "Power", "Shot")
//   - Delimiters (e.g., "Power-Shot" -> "Power", "Shot")
//   - Number transitions (e.g., "j2se" -> "j", "2", "se")
//   - Possessive removal (e.g., "O'Neil's" -> "O", "Neil")
//
// Configuration flags:
//   - splitOnCaseChange: split on case transitions (default: true)
//   - splitOnNumerics: split on letter-number transitions (default: true)
//   - stemEnglishPossessive: remove trailing "'s" (default: true)
//   - catenateWords: concatenate word parts (default: false)
//   - catenateNumbers: concatenate number parts (default: false)
//   - catenateAll: concatenate all parts (default: false)
//   - preserveOriginal: emit original token (default: false)
//   - generateWordParts: emit word subwords (default: true)
//   - generateNumberParts: emit number subwords (default: true)
//
// Example with default settings:
//
//	Input: "PowerShot12Mpx"
//	Output graph:
//	  Position 0: "Power" (posLen=1)
//	  Position 0: "PowerShot" (posLen=2, if catenateWords=true)
//	  Position 1: "Shot" (posLen=1)
//	  Position 2: "12" (posLen=1)
//	  Position 3: "Mpx" (posLen=1)
type WordDelimiterGraphFilter struct {
	*BaseTokenFilter

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// posLenAttr holds the PositionLengthAttribute from the shared attribute source
	posLenAttr PositionLengthAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// typeAttr holds the TypeAttribute from the shared attribute source
	typeAttr TypeAttribute

	// iterator is used to find word boundaries
	iterator *WordDelimiterIterator

	// Configuration flags
	splitOnCaseChange     bool
	splitOnNumerics       bool
	stemEnglishPossessive bool
	catenateWords         bool
	catenateNumbers       bool
	catenateAll           bool
	preserveOriginal      bool
	generateWordParts     bool
	generateNumberParts   bool

	// State for token generation
	text               []rune
	textLength         int
	subWords           []subWord
	subWordCount       int
	currentSubWord     int
	catenationBuffer   strings.Builder
	emitOriginal       bool
	emitCatenation     bool
	catenationPosition int
	catenationLength   int
	firstToken         bool
	inputExhausted     bool
	newInputToken      bool // true when starting a new input token

	// inputStartOffset and inputEndOffset capture the offsets reported by
	// the upstream tokenizer for the current input token. They are
	// snapshotted in processInputToken so that subword emission can rebase
	// per-rune positions against the original source text.
	inputStartOffset int
	inputEndOffset   int

	// utf16Prefix[i] holds the cumulative UTF-16 code-unit count that
	// precedes rune index i in the current input token text. utf16Prefix
	// has length textLength+1. Used to compute Lucene-compatible
	// (UTF-16-indexed) per-subword start/end offsets when surrogate pairs
	// are present in the input.
	utf16Prefix []int
}

// subWord represents a single subword (part) extracted from the input token
type subWord struct {
	start    int
	end      int
	wordType int
	position int
}

// WordDelimiterGraphFilter configuration constants
const (
	// Default flags
	DEFAULT_SPLIT_ON_CASE_CHANGE    = true
	DEFAULT_SPLIT_ON_NUMERICS       = true
	DEFAULT_STEM_ENGLISH_POSSESSIVE = true
	DEFAULT_CATENATE_WORDS          = false
	DEFAULT_CATENATE_NUMBERS        = false
	DEFAULT_CATENATE_ALL            = false
	DEFAULT_PRESERVE_ORIGINAL       = false
	DEFAULT_GENERATE_WORD_PARTS     = true
	DEFAULT_GENERATE_NUMBER_PARTS   = true
)

// NewWordDelimiterGraphFilter creates a new WordDelimiterGraphFilter with default settings.
func NewWordDelimiterGraphFilter(input TokenStream) *WordDelimiterGraphFilter {
	return NewWordDelimiterGraphFilterWithFlags(input,
		DEFAULT_SPLIT_ON_CASE_CHANGE,
		DEFAULT_SPLIT_ON_NUMERICS,
		DEFAULT_STEM_ENGLISH_POSSESSIVE,
		DEFAULT_CATENATE_WORDS,
		DEFAULT_CATENATE_NUMBERS,
		DEFAULT_CATENATE_ALL,
		DEFAULT_PRESERVE_ORIGINAL,
		DEFAULT_GENERATE_WORD_PARTS,
		DEFAULT_GENERATE_NUMBER_PARTS,
	)
}

// NewWordDelimiterGraphFilterWithFlags creates a new WordDelimiterGraphFilter with custom flags.
func NewWordDelimiterGraphFilterWithFlags(
	input TokenStream,
	splitOnCaseChange bool,
	splitOnNumerics bool,
	stemEnglishPossessive bool,
	catenateWords bool,
	catenateNumbers bool,
	catenateAll bool,
	preserveOriginal bool,
	generateWordParts bool,
	generateNumberParts bool,
) *WordDelimiterGraphFilter {
	filter := &WordDelimiterGraphFilter{
		BaseTokenFilter:       NewBaseTokenFilter(input),
		splitOnCaseChange:     splitOnCaseChange,
		splitOnNumerics:       splitOnNumerics,
		stemEnglishPossessive: stemEnglishPossessive,
		catenateWords:         catenateWords,
		catenateNumbers:       catenateNumbers,
		catenateAll:           catenateAll,
		preserveOriginal:      preserveOriginal,
		generateWordParts:     generateWordParts,
		generateNumberParts:   generateNumberParts,
		iterator: NewWordDelimiterIterator(
			splitOnCaseChange,
			splitOnNumerics,
			stemEnglishPossessive,
		),
		subWords:       make([]subWord, 0, 16),
		firstToken:     true,
		inputExhausted: false,
		newInputToken:  true,
	}

	filter.initAttributes()
	return filter
}

// initAttributes retrieves attributes from the shared AttributeSource.
func (f *WordDelimiterGraphFilter) initAttributes() {
	attrSource := f.GetAttributeSource()
	if attrSource == nil {
		return
	}

	// Get CharTermAttribute
	attr := attrSource.GetAttribute(CharTermAttributeType)
	if attr != nil {
		f.termAttr = attr.(CharTermAttribute)
	}

	// Get PositionIncrementAttribute
	attr = attrSource.GetAttribute(PositionIncrementAttributeType)
	if attr != nil {
		f.posIncrAttr = attr.(PositionIncrementAttribute)
	}

	// Get or add PositionLengthAttribute
	attr = attrSource.GetAttribute(PositionLengthAttributeType)
	if attr != nil {
		f.posLenAttr = attr.(PositionLengthAttribute)
	} else {
		f.posLenAttr = NewPositionLengthAttribute()
		attrSource.AddAttributeImpl(f.posLenAttr)
	}

	// Get OffsetAttribute
	attr = attrSource.GetAttribute(OffsetAttributeType)
	if attr != nil {
		f.offsetAttr = attr.(OffsetAttribute)
	}

	// Get or add TypeAttribute
	attr = attrSource.GetAttribute(TypeAttributeType)
	if attr != nil {
		f.typeAttr = attr.(TypeAttribute)
	} else {
		f.typeAttr = NewTypeAttribute()
		attrSource.AddAttributeImpl(f.typeAttr)
	}
}

// SetSplitOnCaseChange sets whether to split on case changes.
func (f *WordDelimiterGraphFilter) SetSplitOnCaseChange(split bool) {
	f.splitOnCaseChange = split
	f.iterator = NewWordDelimiterIterator(
		f.splitOnCaseChange,
		f.splitOnNumerics,
		f.stemEnglishPossessive,
	)
}

// SetSplitOnNumerics sets whether to split on letter-number transitions.
func (f *WordDelimiterGraphFilter) SetSplitOnNumerics(split bool) {
	f.splitOnNumerics = split
	f.iterator = NewWordDelimiterIterator(
		f.splitOnCaseChange,
		f.splitOnNumerics,
		f.stemEnglishPossessive,
	)
}

// SetStemEnglishPossessive sets whether to remove trailing "'s".
func (f *WordDelimiterGraphFilter) SetStemEnglishPossessive(stem bool) {
	f.stemEnglishPossessive = stem
	f.iterator = NewWordDelimiterIterator(
		f.splitOnCaseChange,
		f.splitOnNumerics,
		f.stemEnglishPossessive,
	)
}

// SetCatenateWords sets whether to concatenate word parts.
func (f *WordDelimiterGraphFilter) SetCatenateWords(catenate bool) {
	f.catenateWords = catenate
}

// SetCatenateNumbers sets whether to concatenate number parts.
func (f *WordDelimiterGraphFilter) SetCatenateNumbers(catenate bool) {
	f.catenateNumbers = catenate
}

// SetCatenateAll sets whether to concatenate all parts.
func (f *WordDelimiterGraphFilter) SetCatenateAll(catenate bool) {
	f.catenateAll = catenate
}

// SetPreserveOriginal sets whether to emit the original token.
func (f *WordDelimiterGraphFilter) SetPreserveOriginal(preserve bool) {
	f.preserveOriginal = preserve
}

// SetGenerateWordParts sets whether to emit word subwords.
func (f *WordDelimiterGraphFilter) SetGenerateWordParts(generate bool) {
	f.generateWordParts = generate
}

// SetGenerateNumberParts sets whether to emit number subwords.
func (f *WordDelimiterGraphFilter) SetGenerateNumberParts(generate bool) {
	f.generateNumberParts = generate
}

// IncrementToken advances to the next token in the graph.
func (f *WordDelimiterGraphFilter) IncrementToken() (bool, error) {
	for {
		// If we have buffered subwords to emit
		if f.currentSubWord < f.subWordCount {
			return f.emitSubWord(), nil
		}

		// If we need to emit the original token
		if f.emitOriginal {
			return f.emitOriginalToken(), nil
		}

		// If we need to emit a catenation
		if f.emitCatenation {
			return f.emitCatenationToken(), nil
		}

		// Get next token from input
		if f.inputExhausted {
			return false, nil
		}

		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !hasToken {
			f.inputExhausted = true
			return false, nil
		}

		// Process the input token
		f.newInputToken = true
		f.processInputToken()
	}
}

// processInputToken processes the current input token and prepares subwords.
func (f *WordDelimiterGraphFilter) processInputToken() {
	// Get the current token text
	if f.termAttr == nil {
		return
	}

	tokenText := f.termAttr.String()
	if tokenText == "" {
		return
	}

	// Convert to runes for proper Unicode handling
	f.text = []rune(tokenText)
	f.textLength = len(f.text)

	// Capture upstream offsets BEFORE attributes get cleared during
	// emission, so we can rebase per-subword offsets against the original
	// source text. When no upstream OffsetAttribute is wired, both
	// snapshots are 0 (matches Lucene's tokenStartOffset = 0 default).
	if f.offsetAttr != nil {
		f.inputStartOffset = f.offsetAttr.StartOffset()
		f.inputEndOffset = f.offsetAttr.EndOffset()
	} else {
		f.inputStartOffset = 0
		f.inputEndOffset = 0
	}

	// Build UTF-16 prefix table for Lucene-parity offset computation.
	// Lucene measures offsets in char (UTF-16 code units); Go uses rune
	// indices. For BMP-only text the two coincide; supplementary
	// (non-BMP) runes contribute 2 UTF-16 units each.
	if cap(f.utf16Prefix) < f.textLength+1 {
		f.utf16Prefix = make([]int, f.textLength+1)
	} else {
		f.utf16Prefix = f.utf16Prefix[:f.textLength+1]
	}
	cum := 0
	f.utf16Prefix[0] = 0
	for i, r := range f.text {
		if r > 0xFFFF {
			cum += 2
		} else {
			cum++
		}
		f.utf16Prefix[i+1] = cum
	}

	// Reset subword buffer
	f.subWords = f.subWords[:0]
	f.subWordCount = 0
	f.currentSubWord = 0

	// Set up the iterator
	f.iterator.SetText(f.text, f.textLength)

	// Collect all subwords
	position := 0
	for {
		end := f.iterator.Next()
		if end == DONE {
			break
		}

		start := f.iterator.Current()
		wordType := f.iterator.Type()

		// Determine if we should include this subword
		include := false
		if IsAlpha(wordType) && f.generateWordParts {
			include = true
		} else if IsDigit(wordType) && f.generateNumberParts {
			include = true
		}

		if include {
			f.subWords = append(f.subWords, subWord{
				start:    start,
				end:      end,
				wordType: wordType,
				position: position,
			})
			position++
		}
	}

	f.subWordCount = len(f.subWords)

	// Prepare catenation if needed
	if f.catenateAll && f.subWordCount > 1 {
		f.prepareCatenation(f.subWords, true)
	} else {
		if f.catenateWords {
			f.prepareWordCatenation()
		}
		if f.catenateNumbers {
			f.prepareNumberCatenation()
		}
	}

	// Check if we should emit the original token
	f.emitOriginal = f.preserveOriginal && f.subWordCount > 0 && !f.iterator.IsSingleWord()
}

// prepareCatenation prepares a catenation of the given subwords.
func (f *WordDelimiterGraphFilter) prepareCatenation(words []subWord, all bool) {
	if len(words) < 2 {
		return
	}

	f.catenationBuffer.Reset()
	for i, sw := range words {
		if i > 0 {
			f.catenationBuffer.WriteString(" ")
		}
		f.catenationBuffer.WriteString(string(f.text[sw.start:sw.end]))
	}

	f.emitCatenation = true
	f.catenationPosition = 0
	f.catenationLength = len(words)
}

// prepareWordCatenation prepares catenation of word parts.
func (f *WordDelimiterGraphFilter) prepareWordCatenation() {
	var words []subWord
	for _, sw := range f.subWords {
		if IsAlpha(sw.wordType) {
			words = append(words, sw)
		}
	}
	if len(words) > 1 {
		f.prepareCatenation(words, false)
	}
}

// prepareNumberCatenation prepares catenation of number parts.
func (f *WordDelimiterGraphFilter) prepareNumberCatenation() {
	var numbers []subWord
	for _, sw := range f.subWords {
		if IsDigit(sw.wordType) {
			numbers = append(numbers, sw)
		}
	}
	if len(numbers) > 1 {
		f.prepareCatenation(numbers, false)
	}
}

// utf16AtRune returns the UTF-16 code-unit offset that precedes the rune
// at index r in the current input token. Clamps to [0, len(utf16Prefix)-1]
// so callers can pass either a start or end rune index without bounds
// checks. Falls back to the rune index itself when no prefix table has
// been built yet (defensive: keeps tests that bypass processInputToken
// stable).
func (f *WordDelimiterGraphFilter) utf16AtRune(r int) int {
	if len(f.utf16Prefix) == 0 {
		return r
	}
	if r < 0 {
		return 0
	}
	if r >= len(f.utf16Prefix) {
		return f.utf16Prefix[len(f.utf16Prefix)-1]
	}
	return f.utf16Prefix[r]
}

// emitSubWord emits the next subword from the buffer.
func (f *WordDelimiterGraphFilter) emitSubWord() bool {
	if f.currentSubWord >= f.subWordCount {
		return false
	}

	sw := f.subWords[f.currentSubWord]
	f.currentSubWord++

	// Clear and set attributes
	f.ClearAttributes()

	// Set term
	if f.termAttr != nil {
		f.termAttr.SetValue(string(f.text[sw.start:sw.end]))
	}

	// Set offsets — port of Lucene's
	// org.apache.lucene.analysis.miscellaneous.WordDelimiterGraphFilter
	// offset bookkeeping. Each subword exposes (inputStart + sw.start,
	// inputStart + sw.end) where positions are converted from rune
	// indices to UTF-16 code-unit indices via utf16Prefix so they remain
	// compatible with Lucene's char-based source offsets.
	if f.offsetAttr != nil {
		startOff := f.inputStartOffset + f.utf16AtRune(sw.start)
		endOff := f.inputStartOffset + f.utf16AtRune(sw.end)
		if endOff > f.inputEndOffset {
			endOff = f.inputEndOffset
		}
		if startOff < f.inputStartOffset {
			startOff = f.inputStartOffset
		}
		f.offsetAttr.SetOffset(startOff, endOff)
	}

	// Set position increment
	if f.posIncrAttr != nil {
		if f.newInputToken {
			f.posIncrAttr.SetPositionIncrement(1)
			f.newInputToken = false
		} else {
			f.posIncrAttr.SetPositionIncrement(0)
		}
	}

	// Set position length
	if f.posLenAttr != nil {
		f.posLenAttr.SetPositionLength(1)
	}

	// Set type
	if f.typeAttr != nil {
		if IsAlpha(sw.wordType) {
			f.typeAttr.SetType("word")
		} else if IsDigit(sw.wordType) {
			f.typeAttr.SetType("number")
		} else {
			f.typeAttr.SetType("word")
		}
	}

	return true
}

// emitOriginalToken emits the original token.
func (f *WordDelimiterGraphFilter) emitOriginalToken() bool {
	f.emitOriginal = false

	// Clear attributes
	f.ClearAttributes()

	// Set term (already set from input)
	// We need to restore the original text
	if f.termAttr != nil {
		f.termAttr.SetValue(string(f.text))
	}

	// Restore offsets to the upstream token's span; ClearAttributes wiped
	// them above.
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(f.inputStartOffset, f.inputEndOffset)
	}

	// Set position increment
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(0)
	}

	// Set position length to span all subwords
	if f.posLenAttr != nil {
		f.posLenAttr.SetPositionLength(f.subWordCount)
	}

	// Set type
	if f.typeAttr != nil {
		f.typeAttr.SetType("word")
	}

	return true
}

// emitCatenationToken emits the catenated token.
func (f *WordDelimiterGraphFilter) emitCatenationToken() bool {
	f.emitCatenation = false

	// Clear attributes
	f.ClearAttributes()

	// Set term
	if f.termAttr != nil {
		f.termAttr.SetValue(f.catenationBuffer.String())
	}

	// Restore offsets to the upstream token's span — catenated tokens
	// cover the full input span regardless of internal subword count.
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(f.inputStartOffset, f.inputEndOffset)
	}

	// Set position increment
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(0)
	}

	// Set position length
	if f.posLenAttr != nil {
		f.posLenAttr.SetPositionLength(f.catenationLength)
	}

	// Set type
	if f.typeAttr != nil {
		f.typeAttr.SetType("word")
	}

	return true
}

// End performs end-of-stream operations.
func (f *WordDelimiterGraphFilter) End() error {
	return f.BaseTokenFilter.End()
}

// Reset resets the filter state for reuse.
func (f *WordDelimiterGraphFilter) Reset() error {
	f.subWords = f.subWords[:0]
	f.subWordCount = 0
	f.currentSubWord = 0
	f.emitOriginal = false
	f.emitCatenation = false
	f.firstToken = true
	f.inputExhausted = false
	f.newInputToken = true
	f.catenationBuffer.Reset()
	return f.BaseTokenFilter.End()
}

// Ensure WordDelimiterGraphFilter implements TokenFilter
var _ TokenFilter = (*WordDelimiterGraphFilter)(nil)

// WordDelimiterGraphFilterFactory creates WordDelimiterGraphFilter instances.
type WordDelimiterGraphFilterFactory struct {
	splitOnCaseChange     bool
	splitOnNumerics       bool
	stemEnglishPossessive bool
	catenateWords         bool
	catenateNumbers       bool
	catenateAll           bool
	preserveOriginal      bool
	generateWordParts     bool
	generateNumberParts   bool
}

// NewWordDelimiterGraphFilterFactory creates a new WordDelimiterGraphFilterFactory with default settings.
func NewWordDelimiterGraphFilterFactory() *WordDelimiterGraphFilterFactory {
	return &WordDelimiterGraphFilterFactory{
		splitOnCaseChange:     DEFAULT_SPLIT_ON_CASE_CHANGE,
		splitOnNumerics:       DEFAULT_SPLIT_ON_NUMERICS,
		stemEnglishPossessive: DEFAULT_STEM_ENGLISH_POSSESSIVE,
		catenateWords:         DEFAULT_CATENATE_WORDS,
		catenateNumbers:       DEFAULT_CATENATE_NUMBERS,
		catenateAll:           DEFAULT_CATENATE_ALL,
		preserveOriginal:      DEFAULT_PRESERVE_ORIGINAL,
		generateWordParts:     DEFAULT_GENERATE_WORD_PARTS,
		generateNumberParts:   DEFAULT_GENERATE_NUMBER_PARTS,
	}
}

// SetSplitOnCaseChange sets whether to split on case changes.
func (f *WordDelimiterGraphFilterFactory) SetSplitOnCaseChange(split bool) {
	f.splitOnCaseChange = split
}

// SetSplitOnNumerics sets whether to split on letter-number transitions.
func (f *WordDelimiterGraphFilterFactory) SetSplitOnNumerics(split bool) {
	f.splitOnNumerics = split
}

// SetStemEnglishPossessive sets whether to remove trailing "'s".
func (f *WordDelimiterGraphFilterFactory) SetStemEnglishPossessive(stem bool) {
	f.stemEnglishPossessive = stem
}

// SetCatenateWords sets whether to concatenate word parts.
func (f *WordDelimiterGraphFilterFactory) SetCatenateWords(catenate bool) {
	f.catenateWords = catenate
}

// SetCatenateNumbers sets whether to concatenate number parts.
func (f *WordDelimiterGraphFilterFactory) SetCatenateNumbers(catenate bool) {
	f.catenateNumbers = catenate
}

// SetCatenateAll sets whether to concatenate all parts.
func (f *WordDelimiterGraphFilterFactory) SetCatenateAll(catenate bool) {
	f.catenateAll = catenate
}

// SetPreserveOriginal sets whether to emit the original token.
func (f *WordDelimiterGraphFilterFactory) SetPreserveOriginal(preserve bool) {
	f.preserveOriginal = preserve
}

// SetGenerateWordParts sets whether to emit word subwords.
func (f *WordDelimiterGraphFilterFactory) SetGenerateWordParts(generate bool) {
	f.generateWordParts = generate
}

// SetGenerateNumberParts sets whether to emit number subwords.
func (f *WordDelimiterGraphFilterFactory) SetGenerateNumberParts(generate bool) {
	f.generateNumberParts = generate
}

// Create creates a WordDelimiterGraphFilter wrapping the given input.
func (f *WordDelimiterGraphFilterFactory) Create(input TokenStream) TokenFilter {
	return NewWordDelimiterGraphFilterWithFlags(
		input,
		f.splitOnCaseChange,
		f.splitOnNumerics,
		f.stemEnglishPossessive,
		f.catenateWords,
		f.catenateNumbers,
		f.catenateAll,
		f.preserveOriginal,
		f.generateWordParts,
		f.generateNumberParts,
	)
}

// Ensure WordDelimiterGraphFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*WordDelimiterGraphFilterFactory)(nil)

// toLower converts a string to lowercase using Unicode case folding.
func toLower(s string) string {
	return strings.Map(func(r rune) rune {
		return unicode.ToLower(r)
	}, s)
}
