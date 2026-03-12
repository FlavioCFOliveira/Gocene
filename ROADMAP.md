# Gocene Project Roadmap

**Project:** Gocene - Apache Lucene Port to Go
**Module:** `github.com/FlavioCFOliveira/Gocene`
**Last Updated:** 2026-03-19

---

## Overview

This roadmap outlines the complete development plan for porting Apache Lucene 10.x to idiomatic Go. The project follows a phased approach with critical foundation components first, followed by core index/search functionality, and finally advanced features.

---

## PENDING TASKS

| ID | SEVERITY | PRIORITY | TASK | SPECIALISTS | ACTIONABLE TECHNICAL DESCRIPTION |
| :--- | :--- | :--- | :--- | :--- | :--- |

---

## Implementation Phases

### Phase 2: Document Model + Core Index Structures
**Tasks:** GC-014 through GC-032
**Focus:** Document/Field classes, Segment/Field metadata, Term abstractions

### Phase 3: Analysis Pipeline
**Tasks:** GC-033 through GC-045
**Focus:** TokenStream framework, StandardTokenizer, filters, analyzers

### Phase 4: IndexWriter/Reader
**Tasks:** GC-046 through GC-052
**Focus:** Document indexing, segment management, index reading

### Phase 5: Search Framework
**Tasks:** GC-053 through GC-067
**Focus:** Query types, IndexSearcher, scoring with BM25

### Phase 6: Codec System
**Tasks:** GC-068 through GC-073
**Focus:** Index format encoding/decoding, persistence

### Phase 7: Merge System
**Tasks:** GC-074 through GC-077
**Focus:** Merge policies, background merging

### Phase 8: Simple Query Types
**Status:** COMPLETED | **Tasks:** 3 | **Completed:** 2026-03-16
**Focus:** Basic query implementations building on existing infrastructure
**Dependencies:** Phase 5 (Search Framework)

| Task ID | Task Name | Specialists | SEVERITY | PRIORITY |
|:--------|:----------|:------------|:---------|:---------|
| GC-087 | MatchAllDocsQuery | go-elite-developer | LOW | LOW |
| GC-103 | FieldExistsQuery | go-elite-developer | LOW | LOW |
| GC-083 | PrefixQuery | go-elite-developer | LOW | LOW |

**Dependencies:** Phase 5 (GC-053 through GC-067)

### Phase 9: Additional Analysis Components
**Status:** COMPLETED | **Tasks:** 2 | **Completed:** 2026-03-16
**Focus:** Additional analyzers (WhitespaceAnalyzer, SimpleAnalyzer)
**Dependencies:** Phase 3 (Analysis Pipeline)

| Task ID | Task Name | Specialists | SEVERITY | PRIORITY |
|:--------|:----------|:------------|:---------|:---------|
| GC-099 | WhitespaceAnalyzer | go-elite-developer | LOW | LOW |
| GC-100 | SimpleAnalyzer | go-elite-developer | LOW | LOW |

### Phase 10: Complex Query Types
**Status:** COMPLETED | **Tasks:** 4 | **Completed:** 2026-03-17
**Focus:** Advanced query implementations (Phrase, Range, Wildcard, Fuzzy)
**Dependencies:** Phase 8 (Simple Query Types)

| Task ID | Task Name | Specialists | SEVERITY | PRIORITY |
|:--------|:----------|:------------|:---------|:---------|
| GC-082 | PhraseQuery | go-elite-developer | LOW | LOW |
| GC-084 | TermRangeQuery | go-elite-developer | LOW | LOW |
| GC-085 | WildcardQuery | go-elite-developer | LOW | LOW |
| GC-086 | FuzzyQuery | go-elite-developer | LOW | LOW |

**Dependencies:** Phase 8 (GC-087, GC-103, GC-083)

### Phase 11: Query Wrapper Types
**Tasks:** GC-093, GC-094, GC-095
**Focus:** Query decorators and combiners
**Dependencies:** Phase 8 (Simple Query Types)

### Phase 12: Alternative Similarity
**Tasks:** GC-096
**Focus:** TF/IDF scoring implementation
**Dependencies:** Phase 5 (Search Framework)

### Phase 13: QueryParser
**Tasks:** GC-078, GC-079
**Focus:** Query syntax parsing from text
**Dependencies:** Phases 8, 10, 11 (Query implementations must be complete)
**Status:** BLOCKED until query types are implemented

### Phase 14: Advanced Features (Blocked)
**Tasks:** GC-080, GC-081, GC-104
**Focus:** Numeric fields with Point indexing, DocValues, MoreLikeThis
**Dependencies:** Missing infrastructure (Point indexing, DocValues format, Term vectors)
**Status:** BLOCKED - requires additional infrastructure development

---

## DEVELOPMENT PHASES (Auto-Generated)

| Phase | Status | Tasks | Focus | Dependencies |
|:------|:-------|:------|:------|:-------------|
| 2 | COMPLETED | GC-014 to GC-032 | Document Model | - |
| 3 | COMPLETED | GC-033 to GC-045 | Analysis Pipeline | Phase 2 |
| 4 | COMPLETED | GC-046 to GC-052 | Index Operations | Phase 3 |
| 5 | COMPLETED | GC-053 to GC-067 | Search Framework | Phase 4 |
| 6 | COMPLETED | GC-068 to GC-073 | Codec System | Phase 4 |
| 7 | COMPLETED | GC-074 to GC-077, GC-088 to GC-089, GC-091 to GC-092 | Merge System + Utilities | Phase 6 |
| 8 | COMPLETED | GC-087, GC-103, GC-083 | Simple Query Types | Phase 5 |
| 9 | COMPLETED | GC-099 to GC-100 | Additional Analysis | Phase 3 |
| 10 | COMPLETED | GC-082, GC-084 to GC-086 | Complex Query Types | Phase 8 |
| 11 | COMPLETED | GC-093 to GC-095 | Query Wrapper Types | Phase 8 |
| 12 | COMPLETED | GC-096 | Alternative Similarity | Phase 5 |
| 13 | COMPLETED | GC-078 to GC-079 | QueryParser | Phases 8, 10, 11 |
| 14 | BLOCKED | GC-080 to GC-081, GC-104 | Advanced Features | Infrastructure |

---

## Component Dependencies

```
                    QueryParser
                         |
        Analysis ----+   +---- Search ---- Similarity
            |        |          |
            +--------+----------+
                         |
                       Index
                  (Writer/Reader)
                         |
        +----------------+-------------+
        |                |             |
     Codec            Store        Document
        |
    MergePolicy
```

**Dependency Order:** Store -> Document -> Index (Core) -> Analysis -> Search -> Codec/Merge

---

## Task Status Legend

- **HIGH Severity:** Critical foundation components - must be implemented first
- **MEDIUM Severity:** Core functionality - required for basic search capability
- **LOW Severity:** Extended features - can be deferred until core is complete

---

## Audit References

- Lucene Architecture Audit: `./AUDIT/lucene_architecture_audit.md`
- Last Audit Date: 2026-03-11
- Lucene Version Analyzed: Apache Lucene 10.x

---

## Replanning Summary (2026-03-15)

### Phase Breakdown of Remaining Tasks (GC-078 to GC-104)

The Phase 8 (Query Parser + Advanced Features) has been replanned into 7 distinct phases based on dependency analysis:

**Phase 8: Simple Query Types (3 tasks)**
- GC-087: MatchAllDocsQuery - matches all documents
- GC-103: FieldExistsQuery - find documents with specific field
- GC-083: PrefixQuery - prefix matching on terms
- *Dependencies: Phase 5 (Search Framework)*

**Phase 9: Additional Analysis (2 tasks)**
- GC-099: WhitespaceAnalyzer
- GC-100: SimpleAnalyzer
- *Dependencies: Phase 3 (Analysis Pipeline)*

**Phase 10: Complex Query Types (4 tasks)**
- GC-082: PhraseQuery - exact phrase matching with slop
- GC-084: RangeQuery - term and point range queries
- GC-085: WildcardQuery - pattern matching (? and *)
- GC-086: FuzzyQuery - approximate matching with edit distance
- *Dependencies: Phase 8 (Simple Query Types)*

**Phase 11: Query Wrapper Types (3 tasks)**
- GC-093: DisjunctionMaxQuery - disjunction with max scoring
- GC-094: BoostQuery - score multiplier wrapper
- GC-095: ConstantScoreQuery - constant score wrapper
- *Dependencies: Phase 8 (Simple Query Types)*

**Phase 12: Alternative Similarity (1 task)**
- GC-096: ClassicSimilarity - TF/IDF scoring
- *Dependencies: Phase 5 (Search Framework)*

**Phase 13: QueryParser (2 tasks) - BLOCKED**
- GC-078: QueryParser - classic Lucene query syntax parser
- GC-079: QueryParserTokenManager - token manager for parser
- *Dependencies: Phases 8, 10, 11 (all query types must exist)*
- *Status: BLOCKED until query implementations are complete*

**Phase 14: Advanced Features (3 tasks) - BLOCKED**
- GC-080: Numeric Fields - IntField, LongField, FloatField, DoubleField with Point types
- GC-081: DocValues Fields - columnar storage for sorting/faceting
- GC-104: MoreLikeThis - similar document finding
- *Dependencies: Missing infrastructure (Point indexing, DocValues format, Term vectors)*
- *Status: BLOCKED - requires significant infrastructure development*

### Critical Infrastructure Gaps Identified

1. **Point Indexing (BKD Tree)**: Required for proper numeric field range queries (GC-080)
2. **DocValues Format**: Required for DocValues field storage and retrieval (GC-081)
3. **Term Vectors**: Required for MoreLikeThis feature (GC-104)

### Recommended Implementation Order

1. Complete Phase 8 (Simple Query Types)
2. Complete Phase 9 (Additional Analysis) - can be done in parallel with Phase 8
3. Complete Phase 10 (Complex Query Types)
4. Complete Phase 11 (Query Wrapper Types)
5. Complete Phase 12 (ClassicSimilarity)
6. Implement Phase 13 (QueryParser) - after all query types are ready
7. Plan and implement infrastructure for Phase 14 (requires new tasks)

---

*End of Roadmap*
