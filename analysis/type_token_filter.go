// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TypeTokenFilter filters tokens based on their type attribute.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TypeTokenFilter.
//
// The filter can either accept only tokens with types in the configured set (whitelist)
// or reject tokens with types in the configured set (blacklist).
type TypeTokenFilter struct {
	*BaseTokenFilter

	// types is the set of token types to filter by
	types map[string]bool

	// useWhitelist if true, only tokens in types are kept; if false, tokens in types are removed
	useWhitelist bool

	// typeAttr holds the TypeAttribute from the shared attribute source
	typeAttr TypeAttribute
}

// NewTypeTokenFilter creates a new TypeTokenFilter wrapping the given input.
// If useWhitelist is true, only tokens with types in the types set are kept.
// If useWhitelist is false, tokens with types in the types set are removed.
func NewTypeTokenFilter(input TokenStream, types map[string]bool, useWhitelist bool) *TypeTokenFilter {
	return &TypeTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		types:           types,
		useWhitelist:    useWhitelist,
	}
}

// typeAttribute returns the TypeAttribute, resolving it lazily after the first
// IncrementToken call on the underlying stream has populated the shared source.
func (f *TypeTokenFilter) typeAttribute() TypeAttribute {
	if f.typeAttr != nil {
		return f.typeAttr
	}
	attrSrc := f.GetAttributeSource()
	if attrSrc == nil {
		return nil
	}
	attr := attrSrc.GetAttribute(TypeAttributeType)
	if attr == nil {
		return nil
	}
	if ta, ok := attr.(TypeAttribute); ok {
		f.typeAttr = ta
	}
	return f.typeAttr
}

// IncrementToken advances to the next token, filtering by type.
func (f *TypeTokenFilter) IncrementToken() (bool, error) {
	for {
		hasToken, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !hasToken {
			return false, nil
		}

		// Resolve TypeAttribute lazily (populated after the first token is produced).
		// When the upstream tokenizer does not register a TypeAttribute, fall back to
		// the Lucene default type ("word"), which is what all standard tokenizers emit.
		ta := f.typeAttribute()
		tokenType := DefaultTokenType // "word"
		if ta != nil {
			tokenType = ta.GetType()
		}

		inSet := f.types[tokenType]
		if f.useWhitelist {
			if inSet {
				return true, nil
			}
		} else {
			if !inSet {
				return true, nil
			}
		}
		// Token does not match the filter criterion; skip it.
	}
}

// IsUseWhitelist returns true if this filter uses whitelist mode.
func (f *TypeTokenFilter) IsUseWhitelist() bool {
	return f.useWhitelist
}

// GetTypes returns the set of token types.
func (f *TypeTokenFilter) GetTypes() map[string]bool {
	return f.types
}

// Ensure TypeTokenFilter implements TokenFilter
var _ TokenFilter = (*TypeTokenFilter)(nil)

// TypeTokenFilterFactory creates TypeTokenFilter instances.
type TypeTokenFilterFactory struct {
	types        map[string]bool
	useWhitelist bool
}

// NewTypeTokenFilterFactory creates a new TypeTokenFilterFactory.
func NewTypeTokenFilterFactory(types map[string]bool, useWhitelist bool) *TypeTokenFilterFactory {
	return &TypeTokenFilterFactory{
		types:        types,
		useWhitelist: useWhitelist,
	}
}

// Create creates a TypeTokenFilter wrapping the given input.
func (f *TypeTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTypeTokenFilter(input, f.types, f.useWhitelist)
}

// Ensure TypeTokenFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*TypeTokenFilterFactory)(nil)
