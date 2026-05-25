# Lucene 10.4.0 Golden Fixtures

Pre-generated binary index files written by Apache Lucene 10.4.0 (Java).
Used by Gocene cross-engine tests to verify byte-level compatibility.

## Contents

| File | Description |
|------|-------------|
| `segments_1` | SegmentInfos — index-level metadata (version, segment list) |
| `_0.si` | SegmentInfo for segment 0 — per-segment metadata |
| `_0.cfs` | Compound file — contains all per-segment codec files |
| `_0.cfe` | Compound file entries — directory listing for `_0.cfs` |

The `.cfs` compound file embeds (among others):
- `_0.fnm` — FieldInfos (Lucene90FieldInfosFormat)
- `_0.fdt` / `_0.fdx` / `_0.fdm` — Stored fields (Lucene90StoredFieldsFormat)
- `_0.doc` / `_0.pos` / `_0.pay` / `_0.tim` / `_0.tip` / `_0.tmd` — Postings (Lucene104PostingsFormat)
- `_0.dvd` / `_0.dvm` — DocValues (Lucene90DocValuesFormat)
- `_0.vec` / `_0.vex` / `_0.vem` / `_0.veq` — KNN vectors (Lucene99HnswVectorsFormat)
- `_0.liv` — Live docs

## Corpus

20 documents with fields:
- `id` (stored, keyword)
- `body` (full-text, stored, with term vectors + positions + offsets)
- `tag` (keyword, sorted doc-values)
- `num_dv` (numeric doc-values)
- `bin_dv` (binary doc-values)
- `sorted_dv` (sorted doc-values)
- `sorted_num_dv` (sorted-numeric doc-values, 2 values per doc)
- `sorted_set_dv` (sorted-set doc-values, 2 values per doc)
- `float_vec` (KNN float32 vector, dim=4)
- `byte_vec` (KNN byte vector, dim=4)
- `int_point` (2D IntPoint for BKD/points format)

## Regeneration

Requires Docker:

```bash
cd <repo-root>
./tools/fixture-gen/run.sh
```

This pulls `maven:3.9-eclipse-temurin-21` and generates fixtures using the
Java program at `tools/fixture-gen/src/main/java/FixtureGen.java`.
The generator uses Lucene 10.4.0 with the default `Lucene104Codec`.
