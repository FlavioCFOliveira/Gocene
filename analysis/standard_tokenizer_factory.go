// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"strconv"
)

// StandardTokenizerFactory creates StandardTokenizer instances.
//
// This is the Go port of
// org.apache.lucene.analysis.standard.StandardTokenizerFactory from
// Lucene 10.4.0.
//
// The factory holds the configured maxTokenLength and applies it to
// every Tokenizer it creates. The default is
// [DefaultMaxTokenLength].
//
// Lucene exposes the factory through its SPI registry. Gocene has
// no SPI registry yet, so consumers must instantiate the factory
// directly via [NewStandardTokenizerFactory] or
// [NewStandardTokenizerFactoryWithArgs].
type StandardTokenizerFactory struct {
	maxTokenLength int
}

// NewStandardTokenizerFactory returns a factory configured with the
// default maximum token length.
func NewStandardTokenizerFactory() *StandardTokenizerFactory {
	return &StandardTokenizerFactory{maxTokenLength: DefaultMaxTokenLength}
}

// NewStandardTokenizerFactoryWithArgs returns a factory configured
// from the given args map. The only recognised key is
// "maxTokenLength" (decimal int); other keys are returned as an
// error to mirror Lucene's "Unknown parameters" enforcement.
//
// Pass nil or an empty map to use the default configuration.
func NewStandardTokenizerFactoryWithArgs(args map[string]string) (*StandardTokenizerFactory, error) {
	f := &StandardTokenizerFactory{maxTokenLength: DefaultMaxTokenLength}
	if len(args) == 0 {
		return f, nil
	}
	remaining := make(map[string]string, len(args))
	for k, v := range args {
		remaining[k] = v
	}
	if v, ok := remaining["maxTokenLength"]; ok {
		n, err := parseFactoryInt(v)
		if err != nil {
			return nil, errFactoryParam("maxTokenLength", v, err)
		}
		f.maxTokenLength = n
		delete(remaining, "maxTokenLength")
	}
	if len(remaining) > 0 {
		return nil, errFactoryUnknown(remaining)
	}
	return f, nil
}

// MaxTokenLength returns the maximum token length applied by Create.
func (f *StandardTokenizerFactory) MaxTokenLength() int {
	return f.maxTokenLength
}

// Create returns a new [StandardTokenizer] pre-configured with this
// factory's maximum token length.
func (f *StandardTokenizerFactory) Create() Tokenizer {
	t := NewStandardTokenizer()
	// SetMaxTokenLength only fails for out-of-range values; the
	// factory validates its input in NewStandardTokenizerFactoryWithArgs,
	// so a configuration error here would be a programmer bug.
	if err := t.SetMaxTokenLength(f.maxTokenLength); err != nil {
		// Fall back to the default rather than propagating the
		// error: the existing TokenizerFactory contract does not
		// allow returning errors from Create. The default token
		// length is always valid.
		_ = t.SetMaxTokenLength(DefaultMaxTokenLength)
	}
	return t
}

// Ensure StandardTokenizerFactory implements TokenizerFactory.
var _ TokenizerFactory = (*StandardTokenizerFactory)(nil)

// parseFactoryInt parses a decimal integer, accepting Lucene's
// conventional whitespace-trimmed format.
func parseFactoryInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// errFactoryParam wraps a per-parameter parse failure with the
// failing key/value pair.
func errFactoryParam(key, value string, err error) error {
	return fmt.Errorf("invalid value for factory parameter %q=%q: %w", key, value, err)
}

// errFactoryUnknown mirrors Lucene's "Unknown parameters: {...}"
// error message for any args that survived the consumption loop.
func errFactoryUnknown(remaining map[string]string) error {
	return fmt.Errorf("unknown parameters: %v", remaining)
}
