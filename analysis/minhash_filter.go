// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// MinHashFilter computes MinHash signatures for tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.minhash.MinHashFilter.
//
// MinHash is a technique for quickly estimating how similar two sets are.
// This filter generates hash buckets for input tokens using multiple hash functions.
type MinHashFilter struct {
	*BaseTokenFilter

	// hashCount is the number of hash functions to use
	hashCount int

	// bucketCount is the number of buckets per hash function
	bucketCount int

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// currentToken holds the current token being processed
	currentToken string

	// hashIndex tracks which hash function we're currently emitting for
	hashIndex int

	// bucketIndex tracks which bucket we're currently emitting for
	bucketIndex int
}

// NewMinHashFilter creates a new MinHashFilter wrapping the given input.
// hashCount specifies the number of hash functions to use.
// bucketCount specifies the number of buckets per hash function.
func NewMinHashFilter(input TokenStream, hashCount, bucketCount int) *MinHashFilter {
	filter := &MinHashFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		hashCount:       hashCount,
		bucketCount:     bucketCount,
		hashIndex:       0,
		bucketIndex:     0,
	}

	// Get the CharTermAttribute from the shared AttributeSource
	attrSrc := filter.GetAttributeSource()
	if attrSrc != nil {
		attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
	}

	return filter
}

// IncrementToken advances to the next token and generates MinHash signatures.
// Each input token generates hashCount * bucketCount output tokens.
func (f *MinHashFilter) IncrementToken() (bool, error) {
	// If we're in the middle of generating hashes for a token, continue
	if f.hashIndex < f.hashCount && f.currentToken != "" {
		return f.emitNextHash(), nil
	}

	// Get the next token from input
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	// Save the current token
	if f.termAttr != nil {
		f.currentToken = f.termAttr.String()
	}

	// Reset hash counters
	f.hashIndex = 0
	f.bucketIndex = 0

	// Start emitting hashes
	return f.emitNextHash(), nil
}

// emitNextHash emits the next hash value for the current token.
func (f *MinHashFilter) emitNextHash() bool {
	if f.hashIndex >= f.hashCount {
		return false
	}

	// Compute hash for current position
	hash := f.computeHash(f.currentToken, f.hashIndex, f.bucketIndex)

	// Set the token value to the hash
	if f.termAttr != nil {
		f.termAttr.SetValue(hashToString(hash))
	}

	// Advance to next position
	f.bucketIndex++
	if f.bucketIndex >= f.bucketCount {
		f.bucketIndex = 0
		f.hashIndex++
	}

	return true
}

// computeHash computes a hash value for the given token, hash function index, and bucket.
func (f *MinHashFilter) computeHash(token string, hashIdx, bucketIdx int) uint64 {
	// Simple hash function combining token, hash index, and bucket index
	// In a production implementation, this would use a more sophisticated hash
	var hash uint64 = 14695981039346656037 // FNV-1a offset basis

	// Mix in token bytes
	for i := 0; i < len(token); i++ {
		hash ^= uint64(token[i])
		hash *= 1099511628211 // FNV-1a prime
	}

	// Mix in hash index
	hash ^= uint64(hashIdx)
	hash *= 1099511628211

	// Mix in bucket index
	hash ^= uint64(bucketIdx)
	hash *= 1099511628211

	return hash
}

// hashToString converts a hash value to a string.
func hashToString(hash uint64) string {
	// Convert to base62 for shorter strings
	const chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if hash == 0 {
		return "0"
	}

	result := make([]byte, 0, 11) // max 11 chars for uint64 in base62
	for hash > 0 {
		result = append(result, chars[hash%62])
		hash /= 62
	}

	// Reverse the result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// GetHashCount returns the number of hash functions.
func (f *MinHashFilter) GetHashCount() int {
	return f.hashCount
}

// GetBucketCount returns the number of buckets per hash function.
func (f *MinHashFilter) GetBucketCount() int {
	return f.bucketCount
}

// Ensure MinHashFilter implements TokenFilter
var _ TokenFilter = (*MinHashFilter)(nil)

// MinHashFilterFactory creates MinHashFilter instances.
type MinHashFilterFactory struct {
	hashCount   int
	bucketCount int
}

// NewMinHashFilterFactory creates a new MinHashFilterFactory.
func NewMinHashFilterFactory(hashCount, bucketCount int) *MinHashFilterFactory {
	return &MinHashFilterFactory{
		hashCount:   hashCount,
		bucketCount: bucketCount,
	}
}

// Create creates a MinHashFilter wrapping the given input.
func (f *MinHashFilterFactory) Create(input TokenStream) TokenFilter {
	return NewMinHashFilter(input, f.hashCount, f.bucketCount)
}

// Ensure MinHashFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*MinHashFilterFactory)(nil)
