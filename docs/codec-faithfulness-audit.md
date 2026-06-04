# Codec Faithfulness Audit (rmp #135)

**Date:** 2026-06-04
**Trigger:** `codecs/scalar_quantized_vectors.go` was discovered (during rmp #21)
to be a *fabrication* — it framed files with an invented magic number
(`0x51564543`) instead of `CodecUtil.WriteIndexHeader`, used a naive
`(v+1)*127.5` quantization unrelated to Lucene's `OptimizedScalarQuantizer`, and
shipped name-only "tests". The concern this audit answers: **are any other
`codecs/` formats similarly fabricated, silently breaking the byte-compatibility
mandate?**

## Scope & method

Read-only audit of every on-disk format writer / consumer / producer / reader
under `codecs/` and its sub-packages (`lucene90`, `lucene103`, `lucene104`,
`backward_codecs`/placeholders, `simpletext`, `blockterms`, `blocktreeords`,
`bloom`, `uniformsplit`, `hnsw`). Three empirical discriminators were applied,
each backed by a repository-wide scan:

1. **Framing check** — does the writer frame files with `CodecUtil`
   (`WriteIndexHeader` / `WriteHeader` / `WriteFooter`, i.e. the real
   `CODEC_MAGIC 0x3FD76C17` + codec name + version + segment id + suffix), or
   does it write a per-format hex magic directly? (Note: `codecs/codec_util.go`
   *legitimately* defines `CODEC_MAGIC` — that is the framework, not a
   fabrication.)
2. **Invented-magic scan** — `grep` for `Write(Int32|UInt32|Int|Int64)(... 0x……)`
   excluding `CODEC_MAGIC` and bit masks. A non-empty result is a candidate
   fabrication.
3. **Silent-non-faithful scan** — any writer that emits bytes
   (`WriteInt32`/`WriteVInt`/`WriteBytes`/…) yet **never** calls `CodecUtil`
   framing **and** never returns an honest "not implemented" error. Such a
   writer would produce non-Lucene bytes silently.

## Headline result

| Discriminator | Result |
|---|---|
| Invented magic numbers (anti-pattern #2) | **0 found** (after the #21 fix) |
| Byte-writers lacking CodecUtil framing *and* an honest error (anti-pattern #3) | **0 found** |
| Format writers confirmed using `CodecUtil` framing | postings (Lucene104 + Lucene103 block-tree), doc-values (Lucene90), norms (Lucene90), flat vectors (Lucene99), HNSW vectors (Lucene99), scalar-quantized (Lucene104, post-#21) |

**The `scalar_quantized_vectors.go` fabrication was an isolated case.** No other
format writes an invented magic, and no format silently emits non-Lucene bytes:
every incomplete format either uses faithful `CodecUtil` framing or fails
**honestly** with a tracked error.

## Remaining gaps — all HONEST, tracked stubs (not fabrications)

These formats are incomplete but **refuse to write** (return a specific error)
rather than emitting wrong bytes, so they do not violate the byte-compat
mandate. They are already tracked:

| Format | File | Behaviour | Tracking |
|---|---|---|---|
| SimpleText points | `simpletext/simple_text_points_writer.go` | `WriteField` returns `"deferred — requires SimpleTextBKDWriter"` | rmp #22 |
| Backward postings/DV/KnnVectors | `backward_format_placeholders.go` | `ErrReadOnlyFormat` / "FieldsProducer not yet implemented (backward format not ported)" | rmp #25 / #26 / #27 |
| HNSW flat `DocsWithFieldSet` | `hnsw/flat_field_vectors_writer.go` | forward-deps placeholder (doc comment); the concrete `lucene99_flat_vectors_writer.go` is faithful and round-trips | rmp #29 |

## Verdicts

- **FABRICATED:** `scalar_quantized_vectors.go` — **resolved** in rmp #21
  (replaced by the byte-faithful `lucene104_scalar_quantized_vectors_writer.go`;
  invented magic removed; round-trip test added).
- **SUSPECT (honest stubs, tracked):** SimpleText points (#22), backward formats
  (#25/#26/#27), HNSW flat forward-deps (#29). These are read-only / deferred by
  design and fail loudly; no mandate violation.
- **FAITHFUL:** all other audited writers frame with `CodecUtil` and match the
  Lucene layout at the spot-checked level.

## Secondary recommendation (coverage, not fabrication)

The fabrication slipped in partly because its "tests" only checked names. A
follow-up worth filing: ensure every faithful format has an explicit
**byte-level round-trip or golden-fixture** test (write → read → assert values,
plus `CheckIntegrity`). Several formats already have these (norms, points,
doc-values, compound, scalar-quantized after #21, KNN vectors via the merge
tests); a sweep to close the remaining coverage gaps would prevent a future
fabrication from passing CI unnoticed. This is a test-coverage hardening item,
distinct from the byte-faithfulness verdicts above.

## Conclusion

The byte-compatibility mandate is **not** systemically compromised: the
scalar-quantized fabrication was a single isolated lapse, now fixed. No other
`codecs/` format silently emits non-Lucene bytes. The remaining incomplete
formats fail honestly and are already tracked (#22, #25–#27, #29).
