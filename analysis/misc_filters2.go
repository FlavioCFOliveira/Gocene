// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// --- HyphenatedWordsFilter ---

// HyphenatedWordsFilter joins tokens that were broken by a hyphen at
// end of line. A token ending in '-' triggers concatenation with the
// next token; the result inherits the start offset of the first and
// the end offset of the second.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.HyphenatedWordsFilter from
// Apache Lucene 10.4.0.
type HyphenatedWordsFilter struct {
	*BaseTokenFilter

	hyphenated      strings.Builder
	savedStart      int
	savedHasState   bool
	lastEndOffset   int
	exhausted       bool
	termAttr        CharTermAttribute
	offsetAttr      OffsetAttribute
}

// NewHyphenatedWordsFilter wraps input.
func NewHyphenatedWordsFilter(input TokenStream) *HyphenatedWordsFilter {
	f := &HyphenatedWordsFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); a != nil {
			f.offsetAttr = a.(OffsetAttribute)
		}
	}
	return f
}

// IncrementToken concatenates hyphen-broken tokens.
func (f *HyphenatedWordsFilter) IncrementToken() (bool, error) {
	for !f.exhausted {
		ok, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !ok {
			break
		}
		if f.termAttr == nil {
			return true, nil
		}
		text := f.termAttr.String()
		if f.offsetAttr != nil {
			f.lastEndOffset = f.offsetAttr.EndOffset()
		}
		if len(text) > 0 && text[len(text)-1] == '-' {
			if !f.savedHasState {
				f.savedHasState = true
				if f.offsetAttr != nil {
					f.savedStart = f.offsetAttr.StartOffset()
				}
			}
			f.hyphenated.WriteString(text[:len(text)-1])
		} else if !f.savedHasState {
			return true, nil
		} else {
			f.hyphenated.WriteString(text)
			f.unhyphenate()
			return true, nil
		}
	}
	f.exhausted = true
	if f.savedHasState {
		f.hyphenated.WriteByte('-')
		f.unhyphenate()
		return true, nil
	}
	return false, nil
}

func (f *HyphenatedWordsFilter) unhyphenate() {
	joined := f.hyphenated.String()
	f.termAttr.SetEmpty()
	f.termAttr.AppendString(joined)
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(f.savedStart, f.lastEndOffset)
	}
	f.hyphenated.Reset()
	f.savedHasState = false
	f.savedStart = 0
}

// Reset returns the filter to its initial state.
func (f *HyphenatedWordsFilter) Reset() error {
	if r, ok := f.input.(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.hyphenated.Reset()
	f.savedHasState = false
	f.exhausted = false
	f.lastEndOffset = 0
	return nil
}

// Ensure HyphenatedWordsFilter implements TokenFilter.
var _ TokenFilter = (*HyphenatedWordsFilter)(nil)

// HyphenatedWordsFilterFactory creates instances.
type HyphenatedWordsFilterFactory struct{}

// NewHyphenatedWordsFilterFactory returns a fresh factory.
func NewHyphenatedWordsFilterFactory() *HyphenatedWordsFilterFactory {
	return &HyphenatedWordsFilterFactory{}
}

// Create wraps input.
func (f *HyphenatedWordsFilterFactory) Create(input TokenStream) TokenFilter {
	return NewHyphenatedWordsFilter(input)
}

// --- CapitalizationFilter ---

// CapitalizationDefaults captures the Lucene defaults.
const (
	CapitalizationDefaultMaxWordCount   = int(^uint(0) >> 1) // MAX_INT
	CapitalizationDefaultMaxTokenLength = int(^uint(0) >> 1)
)

// CapitalizationFilter applies case rules to incoming tokens: first
// character upper-case, remaining characters lower-case, with
// per-word and per-token guard conditions.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.CapitalizationFilter from
// Apache Lucene 10.4.0.
//
// Deviation from Lucene: the Java filter exposes a richer ruleset
// (keep words, ok prefixes, forceFirstLetter, onlyFirstWord). Gocene
// ports the most common configuration (always capitalize each word
// boundary, lower-case the remainder, observe min word length); the
// remaining options can be layered in subsequent sprints if a
// concrete consumer needs them.
type CapitalizationFilter struct {
	*BaseTokenFilter

	onlyFirstWord    bool
	forceFirstLetter bool
	minWordLength    int
	maxWordCount     int
	maxTokenLength   int
	termAttr         CharTermAttribute
}

// NewCapitalizationFilter wraps input with the default configuration.
func NewCapitalizationFilter(input TokenStream) *CapitalizationFilter {
	return NewCapitalizationFilterWithConfig(input, false, true, 0,
		CapitalizationDefaultMaxWordCount, CapitalizationDefaultMaxTokenLength)
}

// NewCapitalizationFilterWithConfig wraps input with the given
// configuration.
func NewCapitalizationFilterWithConfig(input TokenStream, onlyFirstWord, forceFirstLetter bool,
	minWordLength, maxWordCount, maxTokenLength int) *CapitalizationFilter {
	if minWordLength < 0 {
		panic("CapitalizationFilter: minWordLength must be non-negative")
	}
	if maxWordCount < 1 {
		panic("CapitalizationFilter: maxWordCount must be >= 1")
	}
	if maxTokenLength < 1 {
		panic("CapitalizationFilter: maxTokenLength must be >= 1")
	}
	f := &CapitalizationFilter{
		BaseTokenFilter:  NewBaseTokenFilter(input),
		onlyFirstWord:    onlyFirstWord,
		forceFirstLetter: forceFirstLetter,
		minWordLength:    minWordLength,
		maxWordCount:     maxWordCount,
		maxTokenLength:   maxTokenLength,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken capitalises the current token's first letter and
// lower-cases the rest.
func (f *CapitalizationFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	s := f.termAttr.String()
	if len(s) > f.maxTokenLength {
		return true, nil
	}
	words := splitWords(s)
	if len(words) > f.maxWordCount {
		return true, nil
	}
	for i, w := range words {
		if len(w) < f.minWordLength {
			continue
		}
		if f.onlyFirstWord && i > 0 {
			words[i] = strings.ToLower(w)
			continue
		}
		words[i] = capitalizeWord(w)
	}
	out := joinWords(s, words)
	if out != s {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(out)
	}
	return true, nil
}

func capitalizeWord(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

// splitWords splits on spaces and dots while preserving the
// delimiters in alignment with joinWords; the simple convention is
// each whitespace-or-dot run forms a word boundary.
func splitWords(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	start := 0
	for i, r := range s {
		if r == ' ' || r == '.' {
			if i > start {
				words = append(words, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		words = append(words, s[start:])
	}
	return words
}

// joinWords reconstructs the token text preserving the original
// delimiters by reading positional information from original.
func joinWords(original string, words []string) string {
	var b strings.Builder
	idx := 0
	start := 0
	for i, r := range original {
		if r == ' ' || r == '.' {
			if i > start {
				if idx < len(words) {
					b.WriteString(words[idx])
					idx++
				}
			}
			b.WriteRune(r)
			start = i + 1
		}
	}
	if start < len(original) && idx < len(words) {
		b.WriteString(words[idx])
	}
	return b.String()
}

// Ensure CapitalizationFilter implements TokenFilter.
var _ TokenFilter = (*CapitalizationFilter)(nil)

// CapitalizationFilterFactory creates instances.
type CapitalizationFilterFactory struct{}

// NewCapitalizationFilterFactory returns a fresh factory using the
// Lucene defaults.
func NewCapitalizationFilterFactory() *CapitalizationFilterFactory {
	return &CapitalizationFilterFactory{}
}

// Create wraps input.
func (f *CapitalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewCapitalizationFilter(input)
}

// --- FingerprintFilter ---

// FingerprintDefaults captures the upstream defaults.
const (
	FingerprintDefaultMaxOutputTokenSize = 1024
	FingerprintDefaultSeparator          = ' '
)

// FingerprintFilter consumes the entire input stream and emits a
// single token containing the sorted, de-duplicated set of input
// terms joined by separator. If the joined size exceeds
// maxOutputTokenSize the filter emits nothing.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.FingerprintFilter from
// Apache Lucene 10.4.0.
type FingerprintFilter struct {
	*BaseTokenFilter

	maxOutputTokenSize int
	separator          byte
	emitted            bool
	termAttr           CharTermAttribute
	posIncrAttr        PositionIncrementAttribute
	typeAttr           *TypeAttribute
	offsetAttr         OffsetAttribute
}

// NewFingerprintFilter wraps input with the Lucene defaults.
func NewFingerprintFilter(input TokenStream) *FingerprintFilter {
	return NewFingerprintFilterWithConfig(input, FingerprintDefaultMaxOutputTokenSize, FingerprintDefaultSeparator)
}

// NewFingerprintFilterWithConfig wraps input with the given config.
func NewFingerprintFilterWithConfig(input TokenStream, maxOutputTokenSize int, separator byte) *FingerprintFilter {
	f := &FingerprintFilter{
		BaseTokenFilter:    NewBaseTokenFilter(input),
		maxOutputTokenSize: maxOutputTokenSize,
		separator:          separator,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); a != nil {
			f.posIncrAttr = a.(PositionIncrementAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&TypeAttribute{})); a != nil {
			f.typeAttr = a.(*TypeAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&offsetAttribute{})); a != nil {
			f.offsetAttr = a.(OffsetAttribute)
		}
	}
	return f
}

// IncrementToken consumes the whole input stream on first call,
// builds the fingerprint, and emits a single token. Subsequent calls
// return false.
func (f *FingerprintFilter) IncrementToken() (bool, error) {
	if f.emitted {
		return false, nil
	}
	f.emitted = true
	seen := make(map[string]struct{})
	var lastEnd int
	for {
		ok, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !ok {
			break
		}
		if f.termAttr == nil {
			continue
		}
		s := f.termAttr.String()
		if _, dup := seen[s]; !dup {
			seen[s] = struct{}{}
		}
		if f.offsetAttr != nil {
			lastEnd = f.offsetAttr.EndOffset()
		}
	}
	terms := make([]string, 0, len(seen))
	for t := range seen {
		terms = append(terms, t)
	}
	sort.Strings(terms)
	joined := strings.Join(terms, string(f.separator))
	if len(joined) == 0 || len(joined) > f.maxOutputTokenSize {
		return false, nil
	}
	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(joined)
	}
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(1)
	}
	if f.typeAttr != nil {
		f.typeAttr.Type = "fingerprint"
	}
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(0, lastEnd)
	}
	return true, nil
}

// Reset clears the emitted flag.
func (f *FingerprintFilter) Reset() error {
	if r, ok := f.input.(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.emitted = false
	return nil
}

// Ensure FingerprintFilter implements TokenFilter.
var _ TokenFilter = (*FingerprintFilter)(nil)

// FingerprintFilterFactory creates instances.
type FingerprintFilterFactory struct {
	maxOutputTokenSize int
	separator          byte
}

// NewFingerprintFilterFactory returns a factory with default settings.
func NewFingerprintFilterFactory() *FingerprintFilterFactory {
	return &FingerprintFilterFactory{
		maxOutputTokenSize: FingerprintDefaultMaxOutputTokenSize,
		separator:          FingerprintDefaultSeparator,
	}
}

// NewFingerprintFilterFactoryWithConfig returns a configured factory.
func NewFingerprintFilterFactoryWithConfig(maxOutputTokenSize int, separator byte) *FingerprintFilterFactory {
	return &FingerprintFilterFactory{maxOutputTokenSize: maxOutputTokenSize, separator: separator}
}

// Create wraps input.
func (f *FingerprintFilterFactory) Create(input TokenStream) TokenFilter {
	return NewFingerprintFilterWithConfig(input, f.maxOutputTokenSize, f.separator)
}

// --- DropIfFlaggedFilter ---

// DropIfFlaggedFilter drops tokens whose FlagsAttribute, ANDed with
// dropFlags, equals dropFlags (i.e. every flag in dropFlags is set).
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.DropIfFlaggedFilter.
type DropIfFlaggedFilter struct {
	*BaseTokenFilter

	dropFlags int
	flagsAttr *FlagsAttribute
}

// NewDropIfFlaggedFilter wraps input with the drop mask.
func NewDropIfFlaggedFilter(input TokenStream, dropFlags int) *DropIfFlaggedFilter {
	f := &DropIfFlaggedFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		dropFlags:       dropFlags,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&FlagsAttribute{})); a != nil {
			f.flagsAttr = a.(*FlagsAttribute)
		}
	}
	return f
}

// IncrementToken drops matching tokens.
func (f *DropIfFlaggedFilter) IncrementToken() (bool, error) {
	for {
		ok, err := f.input.IncrementToken()
		if err != nil || !ok {
			return ok, err
		}
		if f.flagsAttr == nil {
			return true, nil
		}
		if (f.flagsAttr.GetFlags() & f.dropFlags) == f.dropFlags {
			continue
		}
		return true, nil
	}
}

// Ensure DropIfFlaggedFilter implements TokenFilter.
var _ TokenFilter = (*DropIfFlaggedFilter)(nil)

// DropIfFlaggedFilterFactory creates instances.
type DropIfFlaggedFilterFactory struct {
	dropFlags int
}

// NewDropIfFlaggedFilterFactory returns a factory using the given
// dropFlags mask.
func NewDropIfFlaggedFilterFactory(dropFlags int) *DropIfFlaggedFilterFactory {
	return &DropIfFlaggedFilterFactory{dropFlags: dropFlags}
}

// Create wraps input.
func (f *DropIfFlaggedFilterFactory) Create(input TokenStream) TokenFilter {
	return NewDropIfFlaggedFilter(input, f.dropFlags)
}

// --- DelimitedTermFrequencyTokenFilter ---

// DefaultDelimitedTermFrequencyDelimiter is the default '|' delimiter
// separating the term text from its encoded frequency.
const DefaultDelimitedTermFrequencyDelimiter byte = '|'

// DelimitedTermFrequencyTokenFilter parses a delimiter-separated
// term/term-frequency pair. Bytes before the delimiter form the
// token; bytes after are parsed as a decimal integer and assigned to
// the TermFrequencyAttribute.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.DelimitedTermFrequencyTokenFilter.
type DelimitedTermFrequencyTokenFilter struct {
	*BaseTokenFilter

	delimiter byte
	termAttr  CharTermAttribute
	tfAttr    *TermFrequencyAttribute
}

// NewDelimitedTermFrequencyTokenFilter wraps input with the default
// delimiter.
func NewDelimitedTermFrequencyTokenFilter(input TokenStream) *DelimitedTermFrequencyTokenFilter {
	return NewDelimitedTermFrequencyTokenFilterWithDelimiter(input, DefaultDelimitedTermFrequencyDelimiter)
}

// NewDelimitedTermFrequencyTokenFilterWithDelimiter wraps input with
// the given delimiter byte.
func NewDelimitedTermFrequencyTokenFilterWithDelimiter(input TokenStream, delimiter byte) *DelimitedTermFrequencyTokenFilter {
	f := &DelimitedTermFrequencyTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		delimiter:       delimiter,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&TermFrequencyAttribute{})); a != nil {
			f.tfAttr = a.(*TermFrequencyAttribute)
		}
	}
	return f
}

// IncrementToken splits at the delimiter and parses the trailing
// integer as the term frequency.
func (f *DelimitedTermFrequencyTokenFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	buf := f.termAttr.Buffer()
	length := f.termAttr.Length()
	for i := 0; i < length; i++ {
		if buf[i] == f.delimiter {
			f.termAttr.SetLength(i)
			if f.tfAttr != nil {
				s := string(buf[i+1 : length])
				if v, err := strconv.Atoi(s); err == nil && v >= 0 {
					f.tfAttr.SetTermFrequency(v)
				}
			}
			return true, nil
		}
	}
	return true, nil
}

// Ensure DelimitedTermFrequencyTokenFilter implements TokenFilter.
var _ TokenFilter = (*DelimitedTermFrequencyTokenFilter)(nil)

// DelimitedTermFrequencyTokenFilterFactory creates instances.
type DelimitedTermFrequencyTokenFilterFactory struct {
	delimiter byte
}

// NewDelimitedTermFrequencyTokenFilterFactory returns a factory.
func NewDelimitedTermFrequencyTokenFilterFactory() *DelimitedTermFrequencyTokenFilterFactory {
	return &DelimitedTermFrequencyTokenFilterFactory{delimiter: DefaultDelimitedTermFrequencyDelimiter}
}

// NewDelimitedTermFrequencyTokenFilterFactoryWithDelimiter returns a
// configured factory.
func NewDelimitedTermFrequencyTokenFilterFactoryWithDelimiter(delimiter byte) *DelimitedTermFrequencyTokenFilterFactory {
	return &DelimitedTermFrequencyTokenFilterFactory{delimiter: delimiter}
}

// Create wraps input.
func (f *DelimitedTermFrequencyTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewDelimitedTermFrequencyTokenFilterWithDelimiter(input, f.delimiter)
}

// --- ASCIIFoldingFilterFactory (ASCIIFoldingFilter pre-existed) ---

// ASCIIFoldingFilterFactory creates ASCIIFoldingFilter instances
// configured with preserveOriginal off by default.
type ASCIIFoldingFilterFactory struct {
	preserveOriginal bool
}

// NewASCIIFoldingFilterFactory returns a factory that does not
// preserve the original token.
func NewASCIIFoldingFilterFactory() *ASCIIFoldingFilterFactory {
	return &ASCIIFoldingFilterFactory{preserveOriginal: false}
}

// NewASCIIFoldingFilterFactoryWithPreserve returns a factory with the
// preserveOriginal flag set as specified.
func NewASCIIFoldingFilterFactoryWithPreserve(preserveOriginal bool) *ASCIIFoldingFilterFactory {
	return &ASCIIFoldingFilterFactory{preserveOriginal: preserveOriginal}
}

// Create wraps input.
func (f *ASCIIFoldingFilterFactory) Create(input TokenStream) TokenFilter {
	if f.preserveOriginal {
		return NewASCIIFoldingFilterWithOptions(input, true)
	}
	return NewASCIIFoldingFilter(input)
}
