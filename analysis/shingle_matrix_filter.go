// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// ShingleMatrixFilter is an advanced TokenFilter that generates shingles
// with multiple permutations in a matrix pattern.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.shingle.ShingleMatrixFilter.
//
// Unlike the standard ShingleFilter which generates shingles in a linear fashion,
// ShingleMatrixFilter creates a matrix of token combinations, allowing for:
//   - Multiple shingle sizes in a single pass
//   - Token permutations across different positions
//   - Matrix-style shingle generation with configurable dimensions
//
// The filter generates shingles by creating combinations of adjacent tokens
// in a matrix pattern. For example, with input tokens ["a", "b", "c"] and
// maxShingleSize=3, the matrix would include:
//   - Row 1 (size 1): "a", "b", "c"
//   - Row 2 (size 2): "a b", "b c"
//   - Row 3 (size 3): "a b c"
//
// The filter supports:
//   - Configurable minimum and maximum shingle sizes
//   - Token separator between shingled tokens
//   - Outputting unigrams (original tokens) alongside shingles
//   - Position increment and offset handling
//   - Matrix-style token permutation
//
// Example usage:
//
//	tokenizer := NewWhitespaceTokenizer()
//	tokenizer.SetReader(strings.NewReader("hello world test"))
//	filter := NewShingleMatrixFilter(tokenizer)
//	filter.SetMaxShingleSize(3)
//	// Produces: "hello", "hello world", "hello world test",
//	//           "world", "world test",
//	//           "test"
type ShingleMatrixFilter struct {
	*BaseTokenFilter

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// posIncrAttr holds the PositionIncrementAttribute from the shared attribute source
	posIncrAttr PositionIncrementAttribute

	// posLenAttr holds the PositionLengthAttribute from the shared attribute source
	posLenAttr *PositionLengthAttribute

	// offsetAttr holds the OffsetAttribute from the shared attribute source
	offsetAttr OffsetAttribute

	// minShingleSize is the minimum number of tokens in a shingle (default: 2)
	minShingleSize int

	// maxShingleSize is the maximum number of tokens in a shingle (default: 2)
	maxShingleSize int

	// tokenSeparator is the string inserted between tokens in a shingle (default: " ")
	tokenSeparator string

	// outputUnigrams determines if original tokens should be output (default: true)
	outputUnigrams bool

	// tokenMatrix stores tokens in a matrix structure for shingle generation
	tokenMatrix [][]*matrixTokenData

	// currentRow is the current row being processed in the matrix (shingle size - 1)
	currentRow int

	// currentCol is the current column being processed in the matrix
	currentCol int

	// inputExhausted is true when all input tokens have been consumed
	inputExhausted bool

	// isFirstToken is true for the first token in the stream
	isFirstToken bool

	// tokensConsumed is the count of tokens consumed from input
	tokensConsumed int

	// matrixBuilt is true when the token matrix has been fully built
	matrixBuilt bool
}

// matrixTokenData holds the data for a single token in the matrix.
type matrixTokenData struct {
	term              []byte
	startOffset       int
	endOffset         int
	positionIncrement int
	position          int
}

// NewShingleMatrixFilter creates a new ShingleMatrixFilter with default settings.
// Default: minShingleSize=2, maxShingleSize=2, tokenSeparator=" ", outputUnigrams=true
func NewShingleMatrixFilter(input TokenStream) *ShingleMatrixFilter {
	filter := &ShingleMatrixFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		minShingleSize:  2,
		maxShingleSize:  2,
		tokenSeparator:  " ",
		outputUnigrams:  true,
		isFirstToken:    true,
		tokenMatrix:     make([][]*matrixTokenData, 0),
		currentRow:      0,
		currentCol:      0,
		tokensConsumed:  0,
		matrixBuilt:     false,
		inputExhausted:  false,
	}

	// Get attributes from the shared AttributeSource
	filter.initAttributes()

	return filter
}

// NewShingleMatrixFilterWithSizes creates a new ShingleMatrixFilter with custom min/max sizes.
func NewShingleMatrixFilterWithSizes(input TokenStream, minShingleSize, maxShingleSize int) *ShingleMatrixFilter {
	filter := NewShingleMatrixFilter(input)
	// Set max first, then min, to avoid validation issues
	filter.SetMaxShingleSize(maxShingleSize)
	filter.SetMinShingleSize(minShingleSize)
	return filter
}

// initAttributes retrieves attributes from the shared AttributeSource.
// Adds PositionLengthAttribute if not present, as it's needed for shingles.
func (f *ShingleMatrixFilter) initAttributes() {
	attrSource := f.GetAttributeSource()
	if attrSource != nil {
		attr := attrSource.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if attr != nil {
			f.termAttr = attr.(CharTermAttribute)
		}

		attr = attrSource.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{}))
		if attr != nil {
			f.posIncrAttr = attr.(PositionIncrementAttribute)
		}

		// PositionLengthAttribute is needed for shingles - add it if not present
		posLenType := reflect.TypeOf(&PositionLengthAttribute{})
		attr = attrSource.GetAttributeByType(posLenType)
		if attr != nil {
			f.posLenAttr = attr.(*PositionLengthAttribute)
		} else {
			// Add PositionLengthAttribute to the attribute source
			f.posLenAttr = NewPositionLengthAttribute()
			attrSource.AddAttribute(f.posLenAttr)
		}

		attr = attrSource.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
		if attr != nil {
			f.offsetAttr = attr.(OffsetAttribute)
		}
	}
}

// SetMaxShingleSize sets the maximum number of tokens in a shingle.
// Must be >= minShingleSize and >= 2.
func (f *ShingleMatrixFilter) SetMaxShingleSize(size int) {
	if size < 2 {
		size = 2
	}
	if size < f.minShingleSize {
		size = f.minShingleSize
	}
	f.maxShingleSize = size
}

// GetMaxShingleSize returns the maximum shingle size.
func (f *ShingleMatrixFilter) GetMaxShingleSize() int {
	return f.maxShingleSize
}

// SetMinShingleSize sets the minimum number of tokens in a shingle.
// Must be >= 2 and <= maxShingleSize.
func (f *ShingleMatrixFilter) SetMinShingleSize(size int) {
	if size < 2 {
		size = 2
	}
	if size > f.maxShingleSize {
		size = f.maxShingleSize
	}
	f.minShingleSize = size
}

// GetMinShingleSize returns the minimum shingle size.
func (f *ShingleMatrixFilter) GetMinShingleSize() int {
	return f.minShingleSize
}

// SetTokenSeparator sets the string inserted between tokens in a shingle.
func (f *ShingleMatrixFilter) SetTokenSeparator(separator string) {
	f.tokenSeparator = separator
}

// GetTokenSeparator returns the token separator.
func (f *ShingleMatrixFilter) GetTokenSeparator() string {
	return f.tokenSeparator
}

// SetOutputUnigrams sets whether original tokens should be output.
// When true, both unigrams and shingles are output.
// When false, only shingles are output.
func (f *ShingleMatrixFilter) SetOutputUnigrams(output bool) {
	f.outputUnigrams = output
}

// IsOutputUnigrams returns whether unigrams are being output.
func (f *ShingleMatrixFilter) IsOutputUnigrams() bool {
	return f.outputUnigrams
}

// IncrementToken advances to the next token in the stream.
// This implements the core matrix shingle generation logic.
func (f *ShingleMatrixFilter) IncrementToken() (bool, error) {
	// Build the token matrix if not already done
	if !f.matrixBuilt {
		if err := f.buildMatrix(); err != nil {
			return false, err
		}
	}

	// Emit tokens from the matrix in order
	for {
		// Check if we've exhausted all rows
		if f.currentRow >= f.maxShingleSize {
			return false, nil
		}

		// Calculate the actual shingle size for this row (1-based)
		shingleSize := f.currentRow + 1

		// Check if we should skip this row based on min/max settings
		if shingleSize < f.minShingleSize && !f.outputUnigrams && shingleSize == 1 {
			// Skip unigrams if not outputting them
			f.currentRow++
			f.currentCol = 0
			continue
		}

		if shingleSize > f.maxShingleSize {
			// Move to next row
			f.currentRow++
			f.currentCol = 0
			continue
		}

		// Check if we should skip this row entirely (below minShingleSize)
		if shingleSize > 1 && shingleSize < f.minShingleSize {
			f.currentRow++
			f.currentCol = 0
			continue
		}

		// Get the row from the matrix
		if f.currentRow >= len(f.tokenMatrix) {
			// No more rows to process
			f.currentRow++
			f.currentCol = 0
			continue
		}

		row := f.tokenMatrix[f.currentRow]

		// Check if we've exhausted this row
		if f.currentCol >= len(row) {
			// Move to next row
			f.currentRow++
			f.currentCol = 0
			continue
		}

		// Emit the token at current position
		tokenData := row[f.currentCol]
		if tokenData != nil {
			f.emitToken(tokenData, shingleSize)
			f.currentCol++
			return true, nil
		}

		// Skip nil tokens
		f.currentCol++
	}
}

// buildMatrix builds the token matrix from the input stream.
func (f *ShingleMatrixFilter) buildMatrix() error {
	// First, consume all input tokens
	var inputTokens []*matrixTokenData

	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return err
		}
		if !hasToken {
			break
		}

		// Extract token data from attributes
		token := f.extractTokenData(len(inputTokens))
		inputTokens = append(inputTokens, token)
	}

	f.inputExhausted = true
	f.tokensConsumed = len(inputTokens)

	// Build the matrix
	// Row 0: unigrams (if outputUnigrams is true or we need them for larger shingles)
	// Row 1: bigrams
	// Row 2: trigrams, etc.
	f.tokenMatrix = make([][]*matrixTokenData, f.maxShingleSize)

	// Always build row 0 (unigrams) as it's needed for larger shingles
	if len(inputTokens) > 0 {
		f.tokenMatrix[0] = make([]*matrixTokenData, len(inputTokens))
		for i, token := range inputTokens {
			f.tokenMatrix[0][i] = token
		}
	}

	// Build rows for larger shingles
	for size := 2; size <= f.maxShingleSize; size++ {
		rowIndex := size - 1
		numShingles := len(inputTokens) - size + 1
		if numShingles < 0 {
			numShingles = 0
		}

		f.tokenMatrix[rowIndex] = make([]*matrixTokenData, numShingles)

		for i := 0; i < numShingles; i++ {
			// Create a composite token from inputTokens[i] to inputTokens[i+size-1]
			f.tokenMatrix[rowIndex][i] = f.createShingleToken(inputTokens, i, size)
		}
	}

	f.matrixBuilt = true

	// Set starting row based on whether we output unigrams
	if f.outputUnigrams {
		f.currentRow = 0
	} else {
		f.currentRow = f.minShingleSize - 1
	}
	f.currentCol = 0

	return nil
}

// extractTokenData extracts token data from the current input token.
func (f *ShingleMatrixFilter) extractTokenData(position int) *matrixTokenData {
	token := &matrixTokenData{
		position: position,
	}

	if f.termAttr != nil {
		token.term = f.termAttr.Bytes()
	}

	if f.offsetAttr != nil {
		token.startOffset = f.offsetAttr.StartOffset()
		token.endOffset = f.offsetAttr.EndOffset()
	}

	if f.posIncrAttr != nil {
		token.positionIncrement = f.posIncrAttr.GetPositionIncrement()
	}

	return token
}

// createShingleToken creates a shingle token from the given input tokens.
func (f *ShingleMatrixFilter) createShingleToken(inputTokens []*matrixTokenData, startIdx, size int) *matrixTokenData {
	if startIdx+size > len(inputTokens) {
		return nil
	}

	// Calculate total term length
	var totalLen int
	for i := 0; i < size; i++ {
		token := inputTokens[startIdx+i]
		if token != nil {
			totalLen += len(token.term)
		}
	}
	// Add space for separators
	if size > 1 && f.tokenSeparator != "" {
		totalLen += len(f.tokenSeparator) * (size - 1)
	}

	// Build the term
	term := make([]byte, 0, totalLen)
	for i := 0; i < size; i++ {
		token := inputTokens[startIdx+i]
		if token != nil {
			if i > 0 && f.tokenSeparator != "" {
				term = append(term, []byte(f.tokenSeparator)...)
			}
			term = append(term, token.term...)
		}
	}

	// Get offsets from first and last token
	startOffset := inputTokens[startIdx].startOffset
	endOffset := inputTokens[startIdx+size-1].endOffset

	return &matrixTokenData{
		term:              term,
		startOffset:       startOffset,
		endOffset:         endOffset,
		positionIncrement: 0, // Shingles have position increment 0
		position:          startIdx,
	}
}

// emitToken emits a single token from the matrix.
func (f *ShingleMatrixFilter) emitToken(token *matrixTokenData, shingleSize int) {
	// Clear attributes for the new token
	f.ClearAttributes()

	// Set term
	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.Append(token.term)
	}

	// Set offsets
	if f.offsetAttr != nil {
		f.offsetAttr.SetStartOffset(token.startOffset)
		f.offsetAttr.SetEndOffset(token.endOffset)
	}

	// Set position increment
	if f.posIncrAttr != nil {
		if f.isFirstToken {
			f.posIncrAttr.SetPositionIncrement(1)
			f.isFirstToken = false
		} else {
			f.posIncrAttr.SetPositionIncrement(0)
		}
	}

	// Set position length for shingles
	if f.posLenAttr != nil {
		if shingleSize > 1 {
			f.posLenAttr.SetPositionLength(shingleSize)
		} else {
			f.posLenAttr.SetPositionLength(1)
		}
	}
}

// End performs end-of-stream operations.
func (f *ShingleMatrixFilter) End() error {
	// Set final offset if available
	if f.offsetAttr != nil && f.input != nil {
		if hasAttrSrc, ok := f.input.(interface{ GetAttributeSource() *AttributeSource }); ok {
			src := hasAttrSrc.GetAttributeSource()
			if src != nil {
				attr := src.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
				if attr != nil {
					inputOffset := attr.(OffsetAttribute)
					f.offsetAttr.SetEndOffset(inputOffset.EndOffset())
				}
			}
		}
	}

	return f.BaseTokenFilter.End()
}

// Reset resets the filter state for reuse.
func (f *ShingleMatrixFilter) Reset() error {
	f.tokenMatrix = f.tokenMatrix[:0]
	f.currentRow = 0
	f.currentCol = 0
	f.tokensConsumed = 0
	f.matrixBuilt = false
	f.inputExhausted = false
	f.isFirstToken = true

	return f.BaseTokenFilter.End()
}

// Ensure ShingleMatrixFilter implements TokenFilter
var _ TokenFilter = (*ShingleMatrixFilter)(nil)

// ShingleMatrixFilterFactory creates ShingleMatrixFilter instances.
type ShingleMatrixFilterFactory struct {
	minShingleSize int
	maxShingleSize int
	tokenSeparator string
	outputUnigrams bool
}

// NewShingleMatrixFilterFactory creates a new ShingleMatrixFilterFactory with default settings.
func NewShingleMatrixFilterFactory() *ShingleMatrixFilterFactory {
	return &ShingleMatrixFilterFactory{
		minShingleSize: 2,
		maxShingleSize: 2,
		tokenSeparator: " ",
		outputUnigrams: true,
	}
}

// NewShingleMatrixFilterFactoryWithSizes creates a ShingleMatrixFilterFactory with custom sizes.
func NewShingleMatrixFilterFactoryWithSizes(minShingleSize, maxShingleSize int) *ShingleMatrixFilterFactory {
	factory := NewShingleMatrixFilterFactory()
	factory.minShingleSize = minShingleSize
	factory.maxShingleSize = maxShingleSize
	return factory
}

// SetMaxShingleSize sets the maximum shingle size.
func (f *ShingleMatrixFilterFactory) SetMaxShingleSize(size int) {
	f.maxShingleSize = size
}

// SetMinShingleSize sets the minimum shingle size.
func (f *ShingleMatrixFilterFactory) SetMinShingleSize(size int) {
	f.minShingleSize = size
}

// SetTokenSeparator sets the token separator.
func (f *ShingleMatrixFilterFactory) SetTokenSeparator(separator string) {
	f.tokenSeparator = separator
}

// SetOutputUnigrams sets whether to output unigrams.
func (f *ShingleMatrixFilterFactory) SetOutputUnigrams(output bool) {
	f.outputUnigrams = output
}

// Create creates a ShingleMatrixFilter wrapping the given input.
func (f *ShingleMatrixFilterFactory) Create(input TokenStream) TokenFilter {
	filter := NewShingleMatrixFilterWithSizes(input, f.minShingleSize, f.maxShingleSize)
	filter.SetTokenSeparator(f.tokenSeparator)
	filter.SetOutputUnigrams(f.outputUnigrams)
	return filter
}

// Ensure ShingleMatrixFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*ShingleMatrixFilterFactory)(nil)
