# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Roadmap

**Name:** gocene

## Project Overview

Gocene is a Go module that is a port of Apache Lucene to modern idiomatic Golang. Its defining goal is byte-by-byte and behaviour-by-behaviour compatibility with the original Apache Lucene library — specifically the Apache Lucene 10.5.0 reference release. Every index file, codec envelope, directory artefact, and on-disk format produced by Gocene must be readable by Apache Lucene 10.5.0 without modification, and Gocene must be able to read, without loss or reinterpretation, every binary artefact produced by Apache Lucene 10.5.0.

Because Gocene is a port rather than a reimplementation, Lucene is the sole reference of truth. Implementation choices that deviate from observed Lucene behaviour are bugs in Gocene, not in Lucene. Correctness is measured against the Apache Lucene 10.5.0 source tree and the binaries it produces.

This is an early-stage project. The module structure, packages, and development workflow are still being established, but the compatibility mandate is non-negotiable and governs all development decisions.

## Binary Compatibility Mandate (TOP-PRIORITY, NON-NEGOTIABLE)

This requirement supersedes every other guideline in this document. If any other rule, convention, or stylistic preference conflicts with it, this requirement wins.

1. **Produce (write) and Consume (read).** Gocene **MUST** produce binary artefacts that Apache Lucene 10.5.0 can read without modification, **AND** Gocene **MUST** read, without loss or reinterpretation, every binary artefact produced by Apache Lucene 10.5.0. Compatibility is bidirectional and exact; "approximately compatible" is not compatible.

2. **Scope — everything Lucene serializes.** The mandate applies to *every* byte sequence Apache Lucene 10.5.0 emits or accepts, including but not limited to:
   - On-disk index formats: codecs, segment files, postings, doc values, stored fields, term vectors, norms, points/BKD trees, vectors/HNSW, FST dictionaries, compound files, segment infos, `.si`/`.cfs`/`.cfe`, deletes/updates files.
   - Directory/store-level artefacts: file naming, lock files, checksum framing (`CodecUtil`), header/footer envelopes.
   - Token-stream persistence: payloads, attribute serialisation where Lucene persists it.
   - Query- and analysis-side persisted artefacts: synonym/stop-word binary forms, Snowball/Hunspell compiled assets, classification models, suggester FSTs/blob formats.
   - Replication/wire formats: replicator protocol payloads, any IPC frames Lucene exposes.
   - Facets sidecar files, grouping/join persisted state, highlight offset stores, spatial/geo encodings.
   - Any future Lucene-serialised artefact discovered during porting.

3. **Byte-for-byte equality.** Default expectation is **byte-identical output** for the same logical input under the same configured codec/version. Where Lucene legitimately allows non-determinism (e.g., compression dictionaries, ordering driven by hash seeds), the divergence MUST be documented in the affected package, justified against the Lucene 10.5.0 source, and covered by a round-trip test (Gocene-write → Lucene-read → Gocene-read produces the original logical input).

4. **Mandatory compatibility tests — isolated AND in combination.** Every feature, no matter how small, MUST ship with compatibility tests proving the mandate. Compatibility is not assumed, inferred, or guaranteed by code review: it must be demonstrated by tests that exercise Gocene against the Apache Lucene 10.5.0 reference. There are two required test classes:
   - **Isolated**: round-trip and golden-corpus tests at the unit level for the feature alone, using fixtures produced by Lucene 10.5.0. At a minimum this must cover Gocene-write → Lucene-read and Lucene-write → Gocene-read for every serialized artefact the feature emits.
   - **Combined**: integration tests exercising the feature alongside the other features it composes with in real Lucene usage (e.g., codec + doc values + facets + queries used together).
   No feature is "done" until both test classes exist and pass against a Lucene 10.5.0 corpus. A gap in compatibility coverage must be visible as a failing test; it must never be hidden behind `t.Skip()` or a placeholder.

5. **Reference of truth.** The Apache Lucene 10.5.0 source tree (see *Lucene Reference Repository* below) and binaries produced by it are the **sole** reference. Implementation choices that contradict observed Lucene behaviour are bugs in Gocene, not in Lucene.

6. **Workflow consequence.** The standard workflow **Specify → Implement → Test → Document** is interpreted under this mandate:
   - *Specify* must record the exact Lucene 10.5.0 binary contract being targeted (file format, version constant, codec name, struct layout).
   - *Implement* must follow the Lucene 10.5.0 algorithms and data structures closely enough to preserve the binary contract; Go idioms are welcome, but they must not change the serialized form or observable behaviour.
   - *Test* must include compatibility tests against Lucene-produced fixtures before the task can be closed. Every deliverable must prove, with passing tests, that Gocene behaves as a faithful port of Lucene 10.5.0 for the functionality in question.
   - *Document* must state the Lucene 10.5.0 source references and the compatibility test coverage for the feature.

## Source Fidelity Mandate — Organisation and Behaviour (NON-NEGOTIABLE)

Gocene is a **port**, not a reimplementation. Beyond the byte-level contract established by the *Binary Compatibility Mandate* above, **all Gocene code owes fidelity to the Apache Lucene 10.5.0 code in two further dimensions — organisation and functionality.** This mandate is subordinate only to the *Binary Compatibility Mandate*; it prevails over every stylistic preference, personal judgement, or perceived improvement.

1. **Organisational fidelity.** The structure of Gocene must mirror the structure of Lucene 10.5.0: the package/namespace layout, the distribution of responsibilities across units, the decomposition into components, and the correspondence between a Lucene class and its Gocene counterpart. A Lucene package maps to the equivalent Gocene package; a Lucene class maps to the equivalent Gocene type in the equivalent file; a Lucene class hierarchy maps to the equivalent Go interface/embedding arrangement. Names must remain recognisable against the Lucene original after the necessary Go transliteration (exported identifiers in `CamelCase`, file names in `snake_case`). Do not merge, split, relocate, or rename Lucene units on your own initiative.

2. **Behavioural fidelity.** The functionality of Gocene must reproduce the functionality of Lucene 10.5.0: the same algorithms, the same data structures, the same control flow, the same defaults, constants and limits, the same iteration and traversal order, the same edge cases, the same error and boundary handling, and the same observable side effects. Go idioms are welcome **only** where they leave the observable behaviour and the serialized form unchanged.

3. **Lucene resolves every doubt.** In any case of **doubt, inconsistency, or incoherence** — within Gocene, between Gocene and Lucene, or between two candidate designs — **the formula followed by Lucene always prevails.** This is the default resolution and it requires no consultation: read the Lucene 10.5.0 source (§ 14), reproduce what it actually does, and record the source reference. A design that looks cleaner, simpler, faster, or "more Go" than Lucene's but diverges from it is a **bug in Gocene**, not an improvement. Lucene's apparent quirks, redundancies, and historical decisions are part of the contract and must be ported as they are.

4. **Consulting the user is the exception, not the rule.** Under this mandate, the obligation of § 1.1 to ask the user does **not** apply to doubts that Lucene itself settles — those you resolve by following Lucene and proceeding. Consult the user **only** in these specific cases:
   - Lucene 10.5.0 is itself genuinely ambiguous or contradictory on the point, and reading the source plus measurement (§ 7) cannot settle it;
   - Lucene relies on a JVM-only facility with no faithful Go equivalent (for example class-loading SPI, finalisation, the JVM threading and locking model, or `MemorySegment` mapping), so a design decision is unavoidable;
   - two equally faithful renderings exist and they differ in observable behaviour or in serialized form;
   - fidelity would require changing the project's scope, public API surface, architecture, or previously agreed requirements;
   - reproducing Lucene faithfully would introduce a safety problem, in the sense of § 10 (**correct → safe → fast**).

   In every other situation, follow Lucene and continue: do not stop to ask, and do not invent an alternative.

5. **Fidelity must be traceable.** Every ported unit must identify the Lucene 10.5.0 artefact it corresponds to (§ 5.1 records this correspondence in the Knowledge Graph as `PORTED_TO`), and any documented divergence must be justified against the Lucene source and covered by tests, exactly as required for binary divergences.

## 1. Base Rules

1. **You are NOT AUTHORIZED to make decisions on your own.** Whenever the instructions are insufficient, unclear, non-specific, or non-concrete, or whenever they contain contradictions or ambiguities, you MUST ALWAYS ASK the user how to proceed.
   - When asking, always provide multiple options (a, b, c, ...) and indicate which one you recommend.
   - When several clarifications are required, present each question to the user sequentially (one at a time), not all at once.
   - **Boundary between acting and asking:** obvious, low-risk corrections (for example, a pre-existing bug with an unequivocal solution) may proceed immediately; any decision that changes scope, expected behaviour, architecture, or requirements requires prior user approval.
   - **Exception — doubts that Lucene settles:** where the doubt, inconsistency, or incoherence concerns how Gocene should be organised or how it should behave, do not ask: apply the *Source Fidelity Mandate* above and follow Lucene 10.5.0. Only the specific cases listed in point 4 of that mandate require prior consultation.

2. **Documentation in English.** All project documentation (including this `CLAUDE.md`) must be written in the most correct English possible, free of orthographic, grammatical, or syntactic errors. Use clear, simple, and unambiguous technical language intended for human readers.

3. **Documentation faithful to the code.** Documentation must be precise and always reflect the real state of the code.

4. **Workflow.** Work always follows this order: **Specify → Implement → Test → Document.**

## 2. Self-Contained Development Policy

All development cycles must be self-contained. You must NEVER deliver only part of a task; every development cycle must produce a complete, working result.

When new needs are discovered during the course of a task — needs that were not anticipated beforehand — they must be resolved within the same development cycle, as immediately as possible. This means creating new tasks and executing them right away, rather than deferring them.

All code and all development output must be, as a rule, **full-fledged**: no half-implementations, no stubs left dangling, no "to be completed later" placeholders.

Tests must never use `t.Skip()`; a gap in coverage must fail, not be silenced.

Whenever you encounter pre-existing bugs during a task, fix them immediately and then continue with the original task.

### 2.1 No Error Suppression (NON-NEGOTIABLE)

**Errors must never be suppressed, hidden, silenced, or worked around. They must be fixed.**

If the code does not compile, or a test fails, the correct and only acceptable response is to fix the underlying cause. An error is information about a real defect; making the error disappear without removing its cause converts a visible problem into an invisible one and is strictly forbidden.

Prohibited — this list is illustrative, not exhaustive:

- `//go:build ignore` (or any build tag, build constraint, or file rename) used to exclude a file from compilation so the package "builds";
- deleting, emptying, or stubbing out a declaration purely to resolve a redeclaration or an undefined symbol, instead of reconciling the duplicates against the Lucene reference;
- `t.Skip()`, commented-out tests, or assertions weakened so a test passes;
- blank identifier assignments (`_ = err`), empty `catch`-style branches, or discarded errors that hide a real failure;
- linter or vet directives (`//nolint`, `//lint:ignore`, `//go:generate`-style tricks) applied to silence a genuine diagnostic;
- any comment, tag, or flag whose purpose is to stop a tool from reporting a defect.

A broken build is an accurate report that the code is broken. Leave it reporting that until the defect is genuinely repaired. If a fix cannot be completed within the current cycle, the work stays on its own branch with the failure visible — it is never merged behind a suppression.

Where suppressions already exist in the tree, they are technical debt to be removed: the file must be re-enabled and the real errors resolved.

## 3. Production Orientation

Every action you take — whether development, fixes, evaluations, analysis, audits, or any other work — must be treated with production-grade standards.

Throughout the entire work cycle (analysis → planning → development → testing), the objective must always be that the result produced is **production-grade**. You must apply not only the maximum of your knowledge but also the maximum of your effort to ensure that every piece of work is delivered as code ready to be used in production.

There is no acceptable "draft" or "experimental" mode for delivered work: every commit, every closed task, every merged branch must meet production standards.

## 4. Task Planning and Execution

For operations related to Tasks or Sprints, use the `roadmap-manager` skill.

Use the `rmp` tool (the roadmap-management CLI available on the system) to plan and coordinate task execution. Treat `rmp` as the **single source of truth** for planning and executing the tasks of this project. No other management mechanism may be used for this purpose.

Use the **Knowledge Graph** to understand the project, its components, and the relationships between them, so that you can more easily identify the scope and impact of each task.

### 4.1 Planning

Carefully analyse the scope of work proposed by the user and determine whether it should be split across multiple development phases. Each phase must correspond to a solid deliverable.

Every task must have a clear and objective definition of:

- objectives;
- functional requirements;
- technical requirements;
- acceptance criteria — the conditions that confirm the task is complete.

Phases are represented as **Sprints** in the `rmp` tool and serve to group tasks.

When the work requires multiple phases, planning must be performed in two distinct stages:
1. define which phases (sprints) are necessary and the scope/objective of each;
2. only afterwards, sprint by sprint, define the tasks within each sprint.

In both stages, use `rmp` as the single source of truth.

Use the **Knowledge Graph** to identify the highest-gain or highest-impact tasks, foundational tasks, and tasks that unblock other tasks or features, so that the execution order can be optimised. By default, always work from the highest-gain tasks towards the least essential. Foundational tasks and tasks that unblock other work are always prioritised.

When a task is too large to be executed in one go by an AI agent such as Claude Code, subdivide it into smaller parts while respecting the principles already defined (in particular, the self-contained task principle).

### 4.2 Execution

Execution is the natural next step after planning. Always use `rmp` and follow this sequence:

1. Check whether any open task remains unfinished so it can be continued.
2. Identify the next task.
3. Understand the objective of the task to be started, based on its description, functional requirements, and technical requirements.
4. Determine the most appropriate subagent and delegate execution to them.
5. Always validate the acceptance criteria before closing the task.
6. Close the task with a short summary of what was done.
7. After closing the task and before moving on to the next, perform a `git commit` following best practices, explaining what was done.
8. Update the Knowledge Graph.

Execution notes:

- You may develop **only one task at a time**, in strict sequential order. Active development work must never be parallelised across multiple tasks.
- Whenever possible, adapt the model and its effort level to the requirements of each individual task operation.
- Task and sprint execution is **sequential**.
- Evaluations and audits may run in parallel, but such parallel execution must **ALWAYS be authorised by the user**.

### 4.3 Gitflow Integration

For each task, create the appropriate branch following gitflow conventions:

- **feature/** — new features and enhancements;
- **hotfix/** — urgent bug fixes;
- **release/** — release preparation branches.

The branching workflow for each task:

1. Create the appropriate branch based on the nature of the task.
2. Develop the task on that branch.
3. Upon completion, execute the branch closure procedure (merge to main).
4. All operations must be confirmed by the user before execution.

## 5. Knowledge Graph

Manage the Knowledge Graph with the assistance of the `knowledge-authority` skill.

**Every change to the Knowledge Graph or to its model is made EXCLUSIVELY through the `knowledge-authority` skill (NON-NEGOTIABLE).** This applies without exception to the graph's data (creating, updating, or deleting nodes, edges, and properties), to its schema (labels, predicates, properties, constraints, indexes), and to the model document `knowledge-model.md`. No other skill, agent, script, or direct `rmp graph` invocation may write to the graph or edit `knowledge-model.md`. Other skills and agents — `roadmap-manager` included — may **read** the graph for planning and reporting, but every write and every act of maintenance routes through `knowledge-authority`. A change made by any other route is a defect and must be reverted and redone through the skill.

Use the Graph features of `rmp` (Groadmap) to create, maintain (update), and query a knowledge graph for the project. This graph **MUST CONTAIN EVERYTHING** that is useful to know about the project. Examples:

- which features exist and where they are specified and implemented;
- which tests exist and what they test;
- which components exist, how they relate, and what dependencies exist between them;
- in which `git commit` each feature was specified, implemented, and tested;
- the `rmp` tasks and their connection to components.

The graph **MUST ALWAYS BE UPDATED on every `git commit`**, recording the changes to graph objects. Each node and edge update must identify the corresponding commit and date.

**This graph is the absolute truth about the project.** Keep it as up-to-date as possible so that, before reading files, you can query the graph and obtain what you need.

Create whichever node and edge types make the most sense for the project. Use the graph together with tasks and sprints to coordinate work.

### 5.1 Graph Fidelity Requirement (MANDATORY)

The Knowledge Graph **MUST be faithful to the real state of the project**. Fidelity is not aspirational and is never assumed: it must be demonstrable by measurement (§ 7) against the Apache Lucene 10.5.0 reference tree (§ 14) and the Gocene working tree. Four requirements are mandatory and each one must hold independently:

1. **Faithful representation of the source library (Apache Lucene).** The graph must represent the structure of Apache Lucene 10.5.0 — its components and their organisation and hierarchy (modules, packages/namespaces, classes, and their members: methods, fields, constants) — exactly as it exists in the reference tree at `/tmp/lucene`. Every `LuceneModule`, `LucenePackage`, and `LuceneClass` node must correspond to an artefact that is actually present in that tree, and the containment chain (module → package → class → member) must reproduce the real hierarchy. **The representation must not stop at class level:** methods and constructors, fields, constants, enum values, nested and inner classes, interfaces, records, and annotations must be represented as nodes of their own whenever the port depends on them. Nothing may be invented, inferred, or recorded from memory; conversely, no artefact of the reference tree that is in scope for the port may be missing from the graph.

2. **Faithful representation of the target module (Gocene).** The graph must represent the structure of the Gocene module — its components and their organisation and hierarchy (packages, files, structs, interfaces, types, methods, functions, constants, variables, tests) — exactly as it exists in the repository working tree. Every `Package`, `File`, `Symbol`, `Component`, and `Feature` node must correspond to an artefact that is actually present in the repository, and the containment chain (package → file → symbol → member) must reproduce the real hierarchy. **The representation must not stop at type level:** `Symbol` must cover not only structs, interfaces, type aliases, and tests, but also functions, methods, constants, and package-level variables, at the granularity at which the port is actually carried out. Renames, moves, additions, and deletions in the code must be reflected in the graph in the same development cycle that performs them.

3. **Faithful representation of the port relation and its status.** The graph must record the link between each Apache Lucene component and its Gocene counterpart, together with the **port state of that pair (Lucene → Gocene)**. The `PORTED_TO` predicate is the sole authority on port status: for every Lucene artefact it must state whether it is ported and, if so, to which Gocene artefact. **The relation must hold at every granularity at which porting actually happens** — module to package, package to package, class to type, and equally method to method, field to field, and constant to constant — so that port status is answerable per element and never merely per class. The absence of the relation means **"not ported"**, never "unknown" — unported artefacts must therefore be visible as such by query alone. Derived attributes (for example `LuceneClass.is_ported`) must be computed from the relation and never written independently, so the graph can never assert a port status that contradicts its own edges.

4. **The graph must carry every structure the port needs to succeed.** Granularity is dictated by the needs of the porting/translation work, not by convenience: the graph must represent whatever is required to plan, execute, and verify the translation of Lucene 10.5.0 into Go, on both sides of the port and in the relation between them. **A representation that stops at the class or type level is insufficient** — functions, methods, constructors, fields, constants, enum values, nested types, signatures, and any other element on which the port depends must be present. If an element that the port depends on cannot be expressed by the current labels, properties, or predicates, **the model must be extended** and `knowledge-model.md` updated accordingly (§ 5): the schema is never a valid reason to omit structure, and structure is never simplified away because the schema does not yet accommodate it.

Consequences of this requirement:

- **Fidelity is measured, not claimed.** Any statement about port coverage, gaps, or scope must come from graph queries reconciled against both trees, with the evidence cited (§ 7).
- **A divergence between the graph and either tree is a defect**, and it must be corrected immediately, within the current development cycle (§ 2). It must never be tolerated, annotated as acceptable, or deferred.
- **Every `git commit` must leave the graph faithful**, including the commit that records the change (§ 5). Fidelity is a precondition for closing a task, not a follow-up task.
- **`knowledge-model.md` must conform to the live graph** and is regenerated from measurements, never hand-written from memory. Use the `knowledge-authority` skill to sync, refresh, and audit fidelity.

## 6. Never Guess

All interactions on the project must be based **exclusively** on verified knowledge. You must never try to guess the intended answer.

When available information is insufficient, seek answers from official or authoritative sources: specifications, RFCs, papers, books, or recognised authors in the relevant field.

Use the **Knowledge Graph** as the primary source of information — both to look up what is already known and to record the relationships you discover as you go.

## 7. Measure to Decide

Whenever it is necessary to evaluate **performance**, **completeness** (whether something is fully done), or **correctness** (whether something behaves as required), you must ALWAYS gather evidence from the project itself to determine the answer. Decisions of this kind must be **empirical**.

Concretely, this means:

- Run the relevant tests, benchmarks (`go test -bench=. -benchmem`), or profilers (`pprof`) and read their output before claiming a property holds.
- Inspect actual generated artefacts (bytes on disk, fixture outputs) rather than reasoning only about expected behaviour.
- Cite the captured evidence (test names, benchmark numbers, byte diffs) when reporting conclusions.

Assumptions, intuition, or prior recall are not acceptable substitutes for measured evidence in these three dimensions.

## 8. Regression Prevention

Whenever a bug is identified, create the necessary regression tests to ensure that the same bug does not recur as a consequence of future development.

## 9. Team of Subagents

You have at your disposal a team composed of all available subagents (global, user-defined, or project-defined).

Use them collaboratively and in a complementary way so that each task is completed with maximum confidence, effectiveness, and accuracy.

Each subagent should contribute proactively with their specialisation.

When initiating a task, identify the most appropriate specialists (skills or agents) to understand the task scope. However, always remember: **the focus of any task is to contribute to the development of Gocene.** Avoid excessive research or analysis — the goal is implementation, not just understanding. Gather only the information necessary to complete the task.

## 10. Decision Framework

To decide what is expected as a project result — whether during evaluations and audits or during code implementation — follow this priority order: **correct → safe → fast.**

1. **Is it correct?** Does the result match the objective, the project specification, and the applicable authoritative sources (RFCs, standards, etc.)?
2. **Is it safe?** Does the decision or task introduce any characteristic or behaviour that compromises the safe use of the deliverable?
3. **Is it fast?** Is it the fastest achievable without compromising correctness or safety? What can be done to maximise the performance of the deliverable?

If conflicts arise between these criteria, or if difficulty arises in following them, ask the user immediately how to proceed, presenting the possible options.

## 11. Segregation of Responsibilities

Each package, component, and function must follow a strict pattern of segregation of responsibilities in order to maximise code reuse.

## 12. Memory

Use the Knowledge Graph as the memory for the project, the agents, and the skills. Leverage the relational capabilities of the graph database to optimise how you read and write your memories. Use this method to save the token cost of reading files.

**ALWAYS** update the Knowledge Graph whenever project files are changed, so that you maintain the ability to understand the project through the graph.

## 13. Development Guidelines

When implementing Lucene features in Go:

- Follow Go best practices and idioms while maintaining compatibility with Lucene's behavior.
- Port algorithms and data structures from Lucene's Java implementation.
- Consider how to translate Java's object-oriented patterns to Go's interface-based approach.
- Test against Lucene's expected behavior for byte-level compatibility.
- Apply the *Source Fidelity Mandate* (organisation and behaviour): Go idioms are admissible only where they leave the structure, the observable behaviour, and the serialized form faithful to Lucene 10.5.0; where they do not, Lucene prevails.

## 14. Lucene Reference Repository

The authoritative reference for the port is the upstream Apache Lucene source tree at release tag `releases/lucene/10.5.0` (commit `9983b7c`).

- **Expected local path**: `/tmp/lucene` (shallow clone of `https://github.com/apache/lucene.git` at tag `releases/lucene/10.5.0`).
- **If `/tmp/lucene` is absent or empty**, clone it before starting any inventory, planning, or porting task:

  ```bash
  git clone --depth=1 --branch releases/lucene/10.5.0 \
      https://github.com/apache/lucene.git /tmp/lucene
  ```

- Module sources live under `/tmp/lucene/lucene/<module>/src/java/...` (production code), `/tmp/lucene/lucene/<module>/src/java21/...` (JDK-21 specific code, where present), and `/tmp/lucene/lucene/<module>/src/test/...` (tests). Some modules also expose `src/test-files/...` (test resources).
- The reference tree must be treated as read-only context; never modify it.

## 15. Initial Setup

Once development begins, initialize the Go module:

```bash
go mod init github.com/FlavioCFOliveira/Gocene
```

## 16. Project Status

- **Port in progress (pre-v1.0):** 33 top-level packages ported from Apache Lucene 10.5.0 (see `README.md` for the package inventory). The project is in active development across 8 sprints: S1–S5 (closed), S6 (Stubbed subsystems — closed 2026-06-11), S7 (Test-suite health — closed 2026-06-11), S8 (Documentation accuracy — in progress).
- **Known deferred items:** 660 `t.Fatal` blockers across 33 packages (see `docs/skipped-tests-audit.md`). Major gaps include: NRT reader integration, RandomIndexWriter test infrastructure, spatial/geo query factories, HNSW seeded strategies, facets/taxonomy write path, and codec format completeness (Lucene99, PerField, DocValuesSkipper).
- **Binary-compatibility test suite in place:** the Java fixture harness under `tools/lucene-fixtures/` drives Lucene 10.5.0 directly via JDK 21 and Maven, produces deterministic fixtures pinned in `tools/lucene-fixtures/manifests/baseline.tsv` (60+ scenarios across every audited package, plus six combined end-to-end scenarios). A Go-side test layer under `internal/compat/` provides per-package round-trips behind the `compat` build tag plus integration scenarios gated by `GOCENE_COMPAT_HARNESS=1`. Note: compat coverage is currently read-path focused (Lucene→Gocene); write-path (Gocene→Lucene) legs are in progress (see `docs/compat-coverage.md`).
- **CI gates every PR:** GitHub Actions runs a fast `build-and-test` job, a skip-guard lint gate, a race-detector job (x86_64), fuzz smoke tests, and a `compat` matrix (three operating systems × two Go versions) that exercises the fixture harness and the Go compat suite.
- **Sprint 7 (Test-suite health) closed 2026-06-11:** refreshed `docs/skipped-tests-audit.md` (660 blockers across 33 packages), enforced blocker token convention in `scripts/check-skips.sh`, added CI/local reconciliation document, and added `Makefile` with `race-test` target.
- **Sprint 6 (Stubbed subsystems) closed 2026-06-11:** resolved 21 PARTIAL/MISSING tasks across 10 packages — expressions compiler with full JS operators, MemoryIndex search, QueryDecomposer, CollectingMatcher, MonitorQuerySerializer, BBoxValueSource, S2PrefixTree geometry, and more.
