# Gocene Architecture Overview

This document gives a high-level map of how Gocene is structured and how its
pieces fit together at runtime. It is aimed at newcomers who want to understand
the system before diving into a specific package. It is faithful to the code as
it exists today; where a mechanism is still evolving or partially deferred, that
is called out explicitly.

Gocene is an idiomatic Go port of [Apache Lucene 10.4.0](https://github.com/apache/lucene)
(release tag `releases/lucene/10.4.0`, commit `9983b7c`). Its overriding
constraint is **bidirectional, byte-for-byte binary compatibility** with that
Lucene release — see [Relationship to Apache Lucene 10.4.0](#relationship-to-apache-lucene-1040)
below and the full mandate in [`../CLAUDE.md`](../CLAUDE.md).

## Table of contents

- [The big picture](#the-big-picture)
- [Package layering and dependency graph](#package-layering-and-dependency-graph)
- [Key abstractions](#key-abstractions)
- [SPI and codec layering](#spi-and-codec-layering)
- [Codec registration](#codec-registration)
- [The read path](#the-read-path)
- [The write path](#the-write-path)
- [Relationship to Apache Lucene 10.4.0](#relationship-to-apache-lucene-1040)
- [Major design decisions](#major-design-decisions)

---

## The big picture

A search engine has two halves that meet at a serialized, on-disk format:

- The **write half** turns documents into segments and persists them to a
  directory. In Gocene this is driven by `index.IndexWriter`, which buffers
  documents in RAM, flushes them through *codec writers* into Lucene-format
  files, and records the result in a `segments_N` file.
- The **read half** opens a directory, discovers its segments, and reconstructs
  the per-segment *codec readers* needed to answer queries. In Gocene this is
  driven by `index.OpenDirectoryReader` plus the `search` package, which
  executes queries and scores hits.

Everything that is written to disk or read from disk passes through a **codec**.
The codec is the single contract that defines the byte layout of every index
component (postings, stored fields, doc values, term vectors, points, vectors,
field infos, segment infos, compound files). Because the codec is the choke
point for the binary format, it is also the choke point for the binary-
compatibility mandate.

---

## Package layering and dependency graph

Gocene is organized into ~38 top-level packages (see the inventory table in
[`../README.md`](../README.md)). The dependency graph is deliberately layered so
that the lowest layers can be imported from anywhere without creating import
cycles.

```
                    ┌─────────────────────────────────────────────┐
   leaf utilities   │  util/   store/                              │
                    └───────────────┬─────────────────────────────┘
                                    │
   structural types │  schema/   (SegmentInfo, FieldInfo, Term,    │
                    │             DocValuesType, Sort, … )          │
                                    │
   codec SPI        │  spi/      (Codec + the per-component         │
                    │             format interfaces and the        │
                    │             SegmentRead/WriteState structs)   │
                                    │
            ┌───────────────────────┴───────────────────────┐
            │                                                │
   index/   │  IndexWriter / DirectoryReader /        codecs/│  concrete
   (core    │  SegmentCoreReaders; re-exports the      (Lucene90…Lucene104,
   indexing │  spi/ + schema/ types as Go aliases)     BlockTree, BKD, HNSW,
   engine)  │                                          Compressing, PerField)
            └───────────────────────┬───────────────────────┘
                                    │
   higher level     │  search/  facets/  queries/  queryparser/  highlight/
   features         │  grouping/  join/  suggest/  spatial*/  monitor/  …
```

Key rules of the graph:

- `util/` and `store/` are foundational. `store/` defines the `Directory`
  abstraction, `DataInput`/`DataOutput`, IO contexts, and the `CodecUtil`-style
  header/footer/checksum framing. `util/` holds automata, FSTs, BKD, packed
  ints, compression, HNSW, quantization, and the primitive collections.
- `schema/` is a **leaf** package (it imports only the standard library,
  `store/`, and `util/`). It is the canonical declaration site for the
  structural types — `SegmentInfo`, `FieldInfo`/`FieldInfos`, `Term`,
  `DocValuesType`, `Sort` — that both `index/` and `codecs/` operate on.
- `spi/` depends only on `schema/`, `store/`, and `util/`. It declares the
  codec service-provider interfaces (see
  [SPI and codec layering](#spi-and-codec-layering)).
- `codecs/` imports `index/` to reach concrete types it needs (`*SegmentInfo`,
  `*FieldInfos`, `*Term`, …). The reverse edge — `index/` importing `codecs/` —
  is **forbidden**, because it would create a cycle. This single asymmetry
  shapes the codec registration mechanism described below.

---

## Key abstractions

| Abstraction | Package | Role |
|---|---|---|
| `store.Directory` | `store` | A flat namespace of named files with atomic rename and lock support. Backed by `MMapDirectory`, `NIOFSDirectory`, `SimpleFSDirectory`, or the in-memory `ByteBuffersDirectory`. |
| `spi.Codec` | `spi` | The format contract. Exposes nine per-component format accessors (postings, stored fields, field infos, segment info, segment infos, term vectors, compound, KNN vectors, doc values). |
| `schema.SegmentInfo` | `schema` | Immutable per-segment metadata: name, doc count, codec name, files, sort, compound-file flag. |
| `index.SegmentInfos` (alias of `spi.SegmentInfos`) | `index`/`spi` | The list of `SegmentCommitInfo`s that make up a commit point; persisted as `segments_N`. |
| `index.IndexWriter` | `index` | Buffers documents, flushes segments through codec writers, and commits. |
| `index.DirectoryReader` | `index` | A point-in-time, read-only view over all segments in a directory; a composite of per-segment `SegmentReader`s. |
| `index.SegmentCoreReaders` | `index` | The bundle of codec readers (postings, stored fields, term vectors, doc values, norms, points, KNN vectors) shared by all readers of one segment. |
| `search.Query` / `search.Weight` / `search.Scorer` | `search` | The query-execution triad: a `Query` is compiled into a `Weight`, which produces per-segment `Scorer`s. |

---

## SPI and codec layering

The codec interfaces live in a dedicated leaf package, `spi/`. This is the
result of the **SPI unification** work (tracked under rmp #4669, completed
across Sprints 117–118).

### Why `spi/` exists

Before unification, `index/` and `codecs/` each declared their own copy of every
codec-facing interface (`Codec`, `PostingsFormat`, `StoredFieldsFormat`,
`FieldInfosFormat`, …) plus the `SegmentReadState` / `SegmentWriteState` structs
that travel through them. The duplication was unavoidable given the one-way
`codecs/ → index/` import edge: `index/` could not import `codecs/` to share the
types. A dedicated bridge package then had to translate between the two
near-identical interface families on every call, which added overhead and
masked subtle signature drift.

`spi/` resolves this by lifting the shared interfaces and state structs into a
package that sits *below* both `index/` and `codecs/` in the graph. Both
packages then re-export the SPI types via Go **type aliases**:

```go
// index/codec_interface.go
type Codec = spi.Codec
type PostingsFormat = spi.PostingsFormat
type SegmentWriteState = spi.SegmentWriteState
// … and so on for every codec-facing type
```

Because `type X = spi.X` is an alias (not a new named type), `index.Codec` and
`codecs.Codec` are *the same type* at the type-system level. A codec
implementation in `codecs/` satisfies an `index.Codec`-typed field with no
adapter, and callers that historically reached for `index.PostingsFormat` keep
compiling unchanged.

### What `spi/` declares

- `Codec` and its nine per-component format interfaces: `PostingsFormat`
  (with `FieldsConsumer`/`FieldsProducer`), `StoredFieldsFormat`,
  `FieldInfosFormat`, `SegmentInfoFormat`, `SegmentInfosFormat`,
  `TermVectorsFormat`, `CompoundFormat`, `KnnVectorsFormat`, and
  `DocValuesFormat`.
- The doc-values family: `DocValuesProducer` / `DocValuesConsumer` and the six
  iterator-shaped value types (`NumericDocValues`, `BinaryDocValues`,
  `SortedDocValues`, `SortedSetDocValues`, `SortedNumericDocValues`,
  `DocValuesSkipper`).
- `SegmentInfos` / `SegmentCommitInfo` (so the `segments_N` read/write path does
  not need to import `index/`).
- `SegmentReadState` / `SegmentWriteState` — the per-segment context structs that
  carry the `Directory`, `SegmentInfo`, `FieldInfos`, and segment suffix into a
  format's reader/writer factory.
- Codec envelope helpers: `CodecMagic`, `FooterMagic`, `WriteIndexHeader`,
  `CheckIndexHeader`, `WriteFooter`, `CheckFooter`.

See [`../spi/doc.go`](../spi/doc.go) for the authoritative, task-by-task
breakdown of what was lifted and when.

### Concrete codecs

The `codecs/` package and its sub-packages implement these interfaces. The
production codec for Lucene 10.4.0 composes the current-generation formats:
the Lucene104 postings format, the Lucene90 doc-values and stored-fields
formats, the Lucene99 HNSW and Lucene104 scalar-quantized vector formats, the
BlockTree terms dictionary, BKD points, and the compressing stored-fields /
term-vectors machinery. `PerField*` wrapper formats dispatch per field to a
delegate format, mirroring Lucene's `perfield` codecs.

---

## Codec registration

Because `index/` cannot import `codecs/`, the default codec cannot be wired by a
direct reference. Gocene therefore uses **programmatic registration** instead of
a service-loader, split across three mechanisms:

1. **Format registries inside `codecs/`** — `codecs/spi_init.go` contains an
   `init()` that calls `RegisterPostingsFormat`, `RegisterDocValuesFormat`, and
   `RegisterKnnVectorsFormat` for every format the package ships (Lucene104
   postings, Lucene90 doc values, Lucene99/Lucene104 vectors, and the
   `PerField*` wrappers). These populate by-name registries that
   `PostingsFormatByName`, `DocValuesFormatByName`, and `KnnVectorsFormatByName`
   resolve. This mirrors the entries Lucene declares under
   `META-INF/services/…` and lets the per-field codecs reconstruct a format from
   the name stamped in a segment's field attributes.

2. **A default-codec registry inside `index/`** — `index/default_codec.go`
   holds a process-wide registry (`RegisterDefaultCodec` / `GetDefaultCodec`
   plus `RegisterNamedCodec` / `LookupCodecByName`). It is a small slot guarded
   by an `RWMutex`. It exists precisely because `index/` cannot name a concrete
   codec type; it can only hold an `index.Codec` (alias) value that something
   else installs.

3. **The default-codec installer in `codecs/register.go`** — production
   callers blank-import the `codecs/` package. Its `init()` builds the concrete
   `*codecs.Lucene104Codec` and calls `index.RegisterDefaultCodec` (plus
   `RegisterNamedCodec` for the codec name) so that `NewIndexWriterConfig` picks
   up the real codec by default and `OpenDirectoryReader` can resolve the codec
   named in a persisted segment. A companion init in
   `codecs/lucene90/compressing/register.go` installs the temporary
   stored-fields format used by the sorting stored-fields consumer.

   ```go
   import _ "github.com/FlavioCFOliveira/Gocene/codecs"
   ```

This installer replaced the former `internal/codecbridge` package. Once the SPI
unification collapsed every codec-facing interface into `spi/` (so `index.Codec`
is a type alias of `spi.Codec` and `*codecs.Lucene104Codec` satisfies
`index.Codec` directly), the bridge's adapter role disappeared and it was
deleted; `codecs/register.go` carries a compile-time assertion
(`var _ index.Codec = (*Lucene104Codec)(nil)`) that keeps the codec aligned with
the SPI surface. A config built without the `codecs/` blank import, and without
an explicit `IndexWriterConfig.SetCodec`, will surface `index.ErrNoCodec` from
any flush path.

The codec-resolution policy on the read side is: prefer the codec name stamped
on the segment (`SegmentInfo.Codec()`), looked up via `LookupCodecByName`; fall
back to the registered default; and, when no name is stamped (a freshly written
in-memory segment), take a codec-less path used by structural unit tests.

---

## The read path

Opening an index and reading from it flows as follows:

1. **`index.OpenDirectoryReader(directory)`** reads the latest commit by calling
   `ReadSegmentInfos(directory)`. A freshly created directory with no
   `segments_N` is treated as an empty index rather than an error.

2. For each `SegmentCommitInfo` in the commit, **`openSegmentReader`** resolves
   the codec for that segment:
   - It reads the codec name from `SegmentInfo.Codec()` and looks it up with
     `LookupCodecByName`, falling back to `GetDefaultCodec`.
   - It resolves the segment's `FieldInfos`, preferring an in-memory copy and
     otherwise reading the `.fnm` file via `codec.FieldInfosFormat().Read(...)`.

3. **`NewSegmentCoreReaders(directory, segmentInfo, fieldInfos, codec, ctx)`**
   constructs the per-segment codec readers, gated on what the segment actually
   contains:
   - a `FieldsProducer` (postings) via `codec.PostingsFormat().FieldsProducer`,
     when `fieldInfos.HasPostings()`;
   - a `TermVectorsReader` via `codec.TermVectorsFormat()`, when
     `fieldInfos.HasTermVectors()`;
   - a `StoredFieldsReader` via `codec.StoredFieldsFormat()`;
   - and, as the segment requires, doc-values, norms, points, and KNN-vector
     readers.

   Each format's factory receives a `SegmentReadState` carrying the directory,
   segment info, field infos, and segment suffix.

4. **Compound files (`.cfs`/`.cfe`).** When a segment was written as a compound
   file, the loose per-component files are packed into a single `.cfs` blob with
   a `.cfe` entry table. The codec's `CompoundFormat` exposes the compound blob
   as a virtual `Directory`, so the format readers above open their files
   through that compound directory transparently — the rest of the read path is
   identical whether or not the segment is compound.

5. The resulting `SegmentReader`s are assembled into a composite
   `DirectoryReader`. The `search` package then runs a `Query` by compiling it
   into a `Weight` and obtaining a per-segment `Scorer` from each leaf, using the
   codec readers above to walk postings and fetch values.

`SegmentCoreReaders` is reference-counted so that the codec readers — which are
the expensive, shareable part of a segment — can be shared across reopened
`DirectoryReader` generations and released exactly once.

---

## The write path

> **Implementation status (2026-06-11):** The codec-level write path (stored
> fields, postings, term vectors, field infos, norms, compound files) is
> implemented and passes byte-level compatibility tests against
> Lucene 10.4.0. The high-level `IndexWriter` write path is functional for
> basic flows (AddDocument, Commit, ForceMerge) but deferred items remain:
> NRT reader refresh, full delete/update pipeline, live-docs merging,
> and MockDirectoryWrapper integration for randomised testing. See
> `docs/skipped-tests-audit.md` and `CLAUDE.md` §Project Status for details.

Indexing documents and persisting them flows as follows:

1. **`index.IndexWriter.AddDocument(doc)`** forwards the document to the
   `DocumentsWriter`, which buffers it in a `DocumentsWriterPerThread` (DWPT).
   The DWPT accumulates stored fields, postings, and field infos in RAM. Updates
   and deletes are tracked alongside the buffered documents.

2. **Flush.** `flushPendingDocsLocked` snapshots the per-thread pool. When a
   codec is configured, the raw DWPTs are carried forward in a `pendingSegment`
   so the upcoming commit can materialise their contents to disk. (When no codec
   is wired — the structural-unit-test path — postings are merged into a single
   in-memory `FieldsProducer` instead.) `PrepareCommit` performs this flush as
   the first phase of a two-phase commit; file sync and the `segments_N` write
   happen in the second phase.

3. **Materialisation in `Commit`.** For each pending segment whose codec exposes
   real stored-fields, postings, and field-infos formats, the writer builds a
   `SegmentWriteState` and flushes each DWPT through the codec writers in order:
   `flushStoredFields`, `flushTermVectors`, `flushPostings`, `flushFieldInfos`.
   The codec name is then stamped onto the `SegmentInfo` so the reader can later
   resolve the same codec.

4. **Optional compound file.** When `IndexWriterConfig.UseCompoundFile()` is set
   and the codec supports it, the writer collects every per-component file it
   wrote for the segment (excluding `.si`, `.cfs`, `.cfe`), packs them via
   `codec.CompoundFormat().Write(...)`, deletes the now-redundant loose files,
   records the segment's files as the `.cfs`/`.cfe`/`.si` triple, and marks the
   segment as compound.

5. **Per-segment metadata.** The `.si` file is written via the
   `SegmentInfoFormat` (`writeSegmentInfo`) before the segment is registered, so
   external tools and `CheckIndex` can verify per-segment integrity.

6. **The commit point.** Finally, `WriteSegmentInfos` writes the new
   `segments_N` listing every `SegmentCommitInfo` in the commit, together with
   any user commit data, the parent field, and the index sort. This atomic
   `segments_N` write is what makes the freshly written segments visible to a
   subsequent `OpenDirectoryReader`.

All Gocene codec writers frame their files with the same index-header /
checksum-footer envelope Lucene uses (`WriteIndexHeader` / `WriteFooter`), which
is both a compatibility requirement and a precondition for the compound-file
packing step.

---

## Relationship to Apache Lucene 10.4.0

Gocene is not merely *inspired by* Lucene — it is a format-exact port. The
governing rule (full text in [`../CLAUDE.md`](../CLAUDE.md), *Binary Compatibility
Mandate*) is bidirectional and exact:

> Gocene **MUST** produce binary artefacts that Apache Lucene 10.4.0 can read
> without modification, **AND** Gocene **MUST** read, without loss or
> reinterpretation, every binary artefact produced by Apache Lucene 10.4.0.

Consequences that shape the codebase:

- **Lucene is the sole reference of truth.** The upstream source tree at
  `releases/lucene/10.4.0` (commit `9983b7c`) is the authority. Any Gocene
  behaviour that diverges from observed Lucene behaviour is a Gocene bug.
- **Byte-for-byte by default.** For the same logical input under the same codec
  and version, Gocene aims to emit byte-identical output. Where Lucene itself is
  legitimately non-deterministic (for example, compression dictionaries or
  hash-seed-driven ordering), the divergence is documented in the affected
  package and covered by a round-trip test.
- **A two-layer compatibility suite enforces this.** A Java fixture harness
  under [`../tools/lucene-fixtures/`](../tools/lucene-fixtures/) drives
  Lucene 10.4.0 directly on JDK 21 and emits byte-deterministic fixtures, pinned
  by SHA-256 in `tools/lucene-fixtures/manifests/baseline.tsv`. A Go-side suite
  under `internal/compat/` re-reads those fixtures and asserts byte parity
  (per-package round-trips behind the `compat` build tag, plus end-to-end
  combined scenarios gated by `GOCENE_COMPAT_HARNESS=1`). Both layers run as a
  required CI job on every pull request. See
  [`compat-coverage.md`](compat-coverage.md) for the coverage matrix and the
  documented deferrals.

When a Java idiom does not map cleanly onto Go, the port favours an idiomatic Go
shape (interfaces over inheritance, explicit error returns over exceptions,
composition over abstract base classes) **as long as the serialized bytes are
unchanged**. The byte format is the contract; the in-memory API is free to be
Go-native.

---

## Major design decisions

The following decisions are the ones a newcomer is most likely to need context
for. Each is recorded here because it explains structure that would otherwise
look surprising.

### 1. Programmatic codec registration (no service-loader)

Go has no equivalent of Java's `ServiceLoader` / `META-INF/services` runtime
discovery, and Gocene does not attempt to emulate one. Codecs and formats are
registered by explicit `init()`-time function calls
(see [Codec registration](#codec-registration)). The trade-off is that a binary
must blank-import the `codecs/` package (or call `SetCodec` explicitly) to
obtain the production codec; in exchange, the wiring is fully static, free of
reflection, and easy to follow.

### 2. SPI unification with type aliases

Rather than maintaining two parallel interface families and a translating
bridge, the codec-facing interfaces live once in `spi/` and are re-exported as
Go type aliases from both `index/` and `codecs/`
(see [SPI and codec layering](#spi-and-codec-layering)). This removed the
per-format adapter layer, eliminated signature drift between the two copies, and
shrank the build graph, while keeping every historical `index.X` / `codecs.X`
identifier source-compatible.

### 3. Leaf packages `schema/` and `spi/`

The one-way `codecs/ → index/` import edge would normally force shared types to
be duplicated. Lifting the structural types into `schema/` and the codec
interfaces into `spi/` — both leaf packages that depend only on `store/` and
`util/` — lets every layer share a single definition without cycles. `index/`
and `codecs/` re-export those definitions as aliases, so the leaf packages are
an implementation detail callers rarely need to name directly.

### 4. Doc values on the SPI iterator surface

Lucene 9+ exposes doc values as forward-only iterators rather than
random-access lookups. Gocene's doc-values value types in `spi/`
(`NumericDocValues`, `BinaryDocValues`, `SortedDocValues`,
`SortedSetDocValues`, `SortedNumericDocValues`) therefore expose the
iterator primitives `DocID()`, `NextDoc()`, `Advance(target)`, and
`AdvanceExact(target)` — matching Lucene's `DocIdSetIterator`-shaped contract.
The structural-collapse work (rmp #4708–#4710) removed the older random-access
`Get(docID)` / `GetOrd(docID)` projection from the production implementations
and made the `index/` value-type identifiers plain aliases of their `spi/`
counterparts, so there is now exactly one doc-values surface across the tree.

### 5. The store layer abstracts the filesystem

All I/O goes through `store.Directory`. Production code uses the
memory-mapped, NIO, or simple filesystem directories; tests frequently use the
in-memory `ByteBuffersDirectory`, which exercises the exact same codec read/write
paths without touching disk. This keeps the codecs unaware of where their bytes
live and makes round-trip compatibility tests cheap to run.

---

For the per-package inventory and the mapping back to Lucene modules, see
[`../README.md`](../README.md). For the binary-compatibility coverage matrix and
the developer workflow, see [`compat-coverage.md`](compat-coverage.md) and
[`../CONTRIBUTING.md`](../CONTRIBUTING.md).
