// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"errors"
	"io"
)

// MaxTokenizerInputSize is the maximum number of bytes a tokenizer or
// character filter will buffer from a single input stream. Tokenizers that
// must hold their whole input in memory (UAX#29 segmentation, regex-based
// splitting, HTML stripping, morphological decoding) read at most this many
// bytes; input beyond it is rejected with [ErrInputTooLarge] instead of being
// truncated or exhausting memory.
//
// The default of 100 MiB is comfortably larger than any realistic single
// document or query while still bounding worst-case per-call memory. It is a
// top-level input guard and is unrelated to Lucene's per-token length caps
// (e.g. StandardTokenizer.MAX_TOKEN_LENGTH).
const MaxTokenizerInputSize = 100 << 20 // 100 MiB

// ErrInputTooLarge is returned when a tokenizer or character filter is fed an
// input stream exceeding [MaxTokenizerInputSize].
var ErrInputTooLarge = errors.New("analysis: input exceeds MaxTokenizerInputSize")

// readAllLimited reads r fully, but never more than [MaxTokenizerInputSize]
// bytes. It returns [ErrInputTooLarge] if the stream would exceed that limit,
// so callers surface a clear error rather than silently truncating. A nil
// reader yields an empty slice and no error, matching io.ReadAll on an empty
// source.
func readAllLimited(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	// Read one byte past the cap so an input sitting exactly on the limit is
	// accepted while anything larger is detected without buffering it all.
	data, err := io.ReadAll(io.LimitReader(r, MaxTokenizerInputSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxTokenizerInputSize {
		return nil, ErrInputTooLarge
	}
	return data, nil
}
