// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package analysis is the Go port of the Apache Lucene analysis
// pipeline. It hosts the Lucene-faithful TokenStream/Tokenizer/Filter
// surface and the token attribute interfaces consumers register on a
// [util.AttributeSource].
//
// The attribute SPI primitives ([util.AttributeImpl],
// [util.AttributeReflector], [util.AttributeFactory],
// [util.AttributeSource]) all live in the util package; this package
// re-exposes them only through the Attribute interfaces it defines
// (e.g. [CharTermAttribute], [OffsetAttribute], [BoostAttribute], ...),
// each backed by a concrete impl that satisfies
// [util.AttributeImpl] and registers itself via
// [util.AttributeInterfaceProvider].
//
// Sprint 54 retired the legacy package-local AttributeImpl /
// AttributeSource / AttributeReflector aliases, the Attribute marker
// and the analysis-specific factory plumbing; every consumer now goes
// through util.* directly. The string-keyed
// [BaseTokenStream.GetAttribute] back-compat shim survives only as a
// thin wrapper that resolves through the
// [canonicalAttributeInterfaces] registry to the typed
// [util.AttributeSource.GetAttribute] API.
//
// # Input size guard
//
// Several tokenizers and character filters in this package (and in the
// language-specific sub-packages) buffer their entire input in memory in
// a single pass, because the underlying algorithm (UAX#29 segmentation,
// regex splitting, HTML stripping, Viterbi decoding) needs random access
// to the whole text. To prevent a single caller-controlled document or
// query from exhausting memory, every such read is bounded by
// [MaxTokenizerInputSize]. Input exceeding the limit yields
// [ErrInputTooLarge] (surfaced directly where the API returns an error,
// or through the tokenizer's established error path otherwise) rather
// than being silently truncated. This limit is a top-level input guard
// and is independent of Lucene's per-token caps such as
// StandardTokenizer.MAX_TOKEN_LENGTH.
package analysis
