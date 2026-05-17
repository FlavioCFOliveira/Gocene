// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package codecs

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// Lucene103BlockTreeTermsReader.java (Apache Lucene 10.4.0).
//
// This file declares the wire-format constants shared by the Writer
// (Sprint 17 task #99), the FieldReader (Sprint 17 task #95), and the Stats
// helper (Sprint 17 task #102). The actual Lucene103BlockTreeTermsReader
// implementation lives further down — it loads field metadata from the .tmd
// file at construction time, exposes per-field FieldReaders, and verifies
// the index/terms checksums against the lengths recorded in the meta tail.

// Lucene103BlockTreeTermsExtension is the file-extension constant used for
// the per-segment .tim file (term dictionary). Matches
// {@code Lucene103BlockTreeTermsReader.TERMS_EXTENSION}.
const Lucene103BlockTreeTermsExtension = "tim"

// Lucene103BlockTreeTermsIndexExtension is the file-extension constant for
// the per-segment .tip file (terms index). Matches
// {@code Lucene103BlockTreeTermsReader.TERMS_INDEX_EXTENSION}.
const Lucene103BlockTreeTermsIndexExtension = "tip"

// Lucene103BlockTreeTermsMetaExtension is the file-extension constant for
// the per-segment .tmd file (terms metadata / footer tail). Matches
// {@code Lucene103BlockTreeTermsReader.TERMS_META_EXTENSION}.
const Lucene103BlockTreeTermsMetaExtension = "tmd"

// Codec-name constants — these are passed verbatim to
// CodecUtil.writeIndexHeader / CodecUtil.checkIndexHeader and are part of
// the on-disk format.
const (
	Lucene103BlockTreeTermsCodecName      = "BlockTreeTermsDict"
	Lucene103BlockTreeTermsIndexCodecName = "BlockTreeTermsIndex"
	Lucene103BlockTreeTermsMetaCodecName  = "BlockTreeTermsMeta"
)

// Wire-format version range. The initial format is 0; bumping these
// requires a matching change in both Reader and Writer.
const (
	Lucene103BlockTreeVersionStart   int32 = 0
	Lucene103BlockTreeVersionCurrent int32 = 0
)

// Lucene103BlockTreeTermsReader is the strict Go port of
// org.apache.lucene.codecs.lucene103.blocktree.Lucene103BlockTreeTermsReader.
//
// Construction opens .tim, .tip, .tmd in turn, validates codec headers,
// then walks the meta file to materialise one [Lucene103FieldReader] per
// field. The trailing two longs of the meta file record the lengths of
// the index and terms files so the constructor can validate their CRC
// footers without re-scanning every byte.
//
// The reader is safe for concurrent term lookup once construction has
// returned; per-term iteration (Iterator / Intersect on FieldReader)
// requires the deferred SegmentTermsEnum port (backlog task #2692).
type Lucene103BlockTreeTermsReader struct {
	// Implementation lives further down once the FieldReader / TermsEnum
	// dependencies land in tasks #95 and #2692. The shell type is
	// declared here so the constants and the SPI shim can compile.
}
