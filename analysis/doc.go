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
package analysis
