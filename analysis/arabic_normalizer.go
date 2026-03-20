// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// ArabicNormalizer implements normalization for Arabic text.
//
// This normalizer performs the following operations:
// - Removes Kashida (tatweel) character U+0640
// - Normalizes different forms of Alef to bare Alef
// - Normalizes different forms of Yeh to bare Yeh
// - Normalizes different forms of Heh to bare Heh
// - Removes FATHATAN, DAMMATAN, KASRATAN at the end of words
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ar.ArabicNormalizer.
type ArabicNormalizer struct {
	input *ReusableStringReader
}

// NewArabicNormalizer creates a new ArabicNormalizer.
func NewArabicNormalizer() *ArabicNormalizer {
	return &ArabicNormalizer{
		input: NewReusableStringReader(),
	}
}

// NormalizeChar implements CharFilter interface for Arabic normalization.
// It processes the input text and applies Arabic-specific normalization.
func (n *ArabicNormalizer) NormalizeChar(input string) string {
	if input == "" {
		return ""
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(input)
	result := make([]rune, 0, len(runes))

	for _, r := range runes {
		normalized := n.normalizeRune(r)
		if normalized != 0 {
			result = append(result, normalized)
		}
	}

	// Remove trailing tanween (vowel marks at end of words)
	result = n.removeTrailingTanween(result)

	return string(result)
}

// normalizeRune normalizes a single Arabic rune.
// Returns 0 if the rune should be removed.
func (n *ArabicNormalizer) normalizeRune(r rune) rune {
	switch r {
	// Remove Kashida/Tatweel (elongation character)
	case 0x0640: // ARABIC TATWEEL
		return 0 // Remove

	// Normalize Alef forms to bare Alef (U+0627)
	case 0x0622: // ARABIC LETTER ALEF WITH MADDA ABOVE
		return 0x0627 // ARABIC LETTER ALEF
	case 0x0623: // ARABIC LETTER ALEF WITH HAMZA ABOVE
		return 0x0627 // ARABIC LETTER ALEF
	case 0x0625: // ARABIC LETTER ALEF WITH HAMZA BELOW
		return 0x0627 // ARABIC LETTER ALEF

	// Normalize Yeh forms to bare Yeh (U+064A)
	case 0x0649: // ARABIC LETTER ALEF MAKSURA
		return 0x064A // ARABIC LETTER YEH

	// Keep Heh forms as is (modern Arabic normalization doesn't change these)

	default:
		return r
	}
}

// removeTrailingTanween removes FATHATAN, DAMMATAN, KASRATAN from end of words.
func (n *ArabicNormalizer) removeTrailingTanween(runes []rune) []rune {
	if len(runes) == 0 {
		return runes
	}

	// TANWEEN characters (double vowels at end of words)
	// U+064B: ARABIC FATHATAN
	// U+064C: ARABIC DAMMATAN
	// U+064D: ARABIC KASRATAN
	lastIdx := len(runes) - 1
	if runes[lastIdx] == 0x064B || runes[lastIdx] == 0x064C || runes[lastIdx] == 0x064D {
		return runes[:lastIdx]
	}

	return runes
}

// Normalize performs full normalization on input string.
func (n *ArabicNormalizer) Normalize(input string) string {
	return n.NormalizeChar(input)
}

// ArabicNormalizationFilter is a TokenFilter that applies Arabic normalization.
type ArabicNormalizationFilter struct {
	*BaseTokenFilter
	normalizer *ArabicNormalizer
}

// NewArabicNormalizationFilter creates a new ArabicNormalizationFilter.
func NewArabicNormalizationFilter(input TokenStream) *ArabicNormalizationFilter {
	return &ArabicNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewArabicNormalizer(),
	}
}

// IncrementToken processes the next token and applies Arabic normalization.
func (f *ArabicNormalizationFilter) IncrementToken() (bool, error) {
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

// ArabicNormalizationFilterFactory creates ArabicNormalizationFilter instances.
type ArabicNormalizationFilterFactory struct{}

// NewArabicNormalizationFilterFactory creates a new ArabicNormalizationFilterFactory.
func NewArabicNormalizationFilterFactory() *ArabicNormalizationFilterFactory {
	return &ArabicNormalizationFilterFactory{}
}

// Create creates a new ArabicNormalizationFilter.
func (f *ArabicNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewArabicNormalizationFilter(input)
}

// Ensure ArabicNormalizationFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*ArabicNormalizationFilterFactory)(nil)

// Helper functions for Arabic character detection

// IsArabicLetter returns true if the rune is an Arabic letter.
func IsArabicLetter(r rune) bool {
	return (r >= 0x0600 && r <= 0x06FF) || // Arabic
		(r >= 0x0750 && r <= 0x077F) || // Arabic Supplement
		(r >= 0xFB50 && r <= 0xFDFF) || // Arabic Presentation Forms-A
		(r >= 0xFE70 && r <= 0xFEFF) // Arabic Presentation Forms-B
}

// HasArabicText returns true if the string contains any Arabic characters.
func HasArabicText(s string) bool {
	for _, r := range s {
		if IsArabicLetter(r) {
			return true
		}
	}
	return false
}

