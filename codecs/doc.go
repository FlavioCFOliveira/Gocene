// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecs provides the SPI-based codec registry and all format
// interfaces used to encode and decode Lucene index files.
//
// A Codec bundles nine format implementations:
//
//   - FieldInfosFormat   — .fnm (field metadata)
//   - CompoundFormat     — .cfs/.cfe (compound segment files)
//   - PostingsFormat     — .tim/.tip/.doc/.pos/.pay (inverted index)
//   - DocValuesFormat    — .dvd/.dvm (columnar numeric/binary/sorted data)
//   - NormsFormat        — .nvd/.nvm (per-field similarity norms)
//   - StoredFieldsFormat — .fdt/.fdx (stored field values)
//   - TermVectorsFormat  — .tvd/.tvx (per-document term frequency vectors)
//   - PointsFormat       — .kdd/.kim/.kdi (BKD-tree spatial points)
//   - KnnVectorsFormat   — .vec/.vex/.vem (HNSW approximate nearest-neighbour)
//
// The current production codec is [Lucene104Codec], which writes the format
// produced by Apache Lucene 10.4.0. Sub-packages such as codecs/lucene104,
// codecs/hnsw, and codecs/compressing supply the concrete format
// implementations.
//
// Codecs are registered by name via [RegisterCodec]; look up a registered
// codec with [ForName]. The default name "Lucene104" maps to
// [Lucene104Codec].
//
// Port note: Java's SPI class-loading is replaced by explicit
// programmatic registration. Call [RegisterCodec] at init time or before
// opening any index.
package codecs
