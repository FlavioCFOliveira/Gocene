// Package sharedterms implements the Shared Terms variant of the Uniform Split postings format.
package sharedterms

import (
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
)

const (
	// NAME is the name of the Shared Terms Uniform Split postings format.
	NAME = "SharedTermsUniformSplit"

	// TERMS_DICTIONARY_EXTENSION is the extension of the file containing the terms dictionary.
	TERMS_DICTIONARY_EXTENSION = "stustd"

	// TERMS_BLOCKS_EXTENSION is the extension of the file containing the terms blocks.
	TERMS_BLOCKS_EXTENSION = "stustb"

	// VERSION_CURRENT is the current version of the format.
	VERSION_CURRENT = 1
)

// STUniformSplitPostingsFormat is the codec wrapper for Shared Terms Uniform Split.
type STUniformSplitPostingsFormat struct {
	uniformsplit.UniformSplitPostingsFormat
}

// NewSTUniformSplitPostingsFormat builds the format with default settings.
func NewSTUniformSplitPostingsFormat() *STUniformSplitPostingsFormat {
	return &STUniformSplitPostingsFormat{
		UniformSplitPostingsFormat: *uniformsplit.NewUniformSplitPostingsFormat(32),
	}
}

// NewSTUniformSplitPostingsFormatCustom builds the format with custom settings.
func NewSTUniformSplitPostingsFormatCustom(targetBlockSize int) *STUniformSplitPostingsFormat {
	return &STUniformSplitPostingsFormat{
		UniformSplitPostingsFormat: *uniformsplit.NewUniformSplitPostingsFormat(targetBlockSize),
	}
}
