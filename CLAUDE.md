# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Roadmap

**Name:** gocene

## Project Overview

Gocene is a Go module that aims to be a port of Apache Lucene to modern idiomatic Golang, byte-by-byte compatible with the original Apache Lucene library.

This is an early-stage project. The module structure, packages, and development workflow are not yet established.

## Binary Compatibility Mandate (TOP-PRIORITY, NON-NEGOTIABLE)

This requirement supersedes every other guideline in this document. If any other rule, convention, or stylistic preference conflicts with it, this requirement wins.

1. **Produce (write) and Consume (read).** Gocene **MUST** produce binary artefacts that Apache Lucene 10.4.0 can read without modification, **AND** Gocene **MUST** read, without loss or reinterpretation, every binary artefact produced by Apache Lucene 10.4.0. Compatibility is bidirectional and exact; "approximately compatible" is not compatible.

2. **Scope — everything Lucene serializes.** The mandate applies to *every* byte sequence Apache Lucene 10.4.0 emits or accepts, including but not limited to:
   - On-disk index formats: codecs, segment files, postings, doc values, stored fields, term vectors, norms, points/BKD trees, vectors/HNSW, FST dictionaries, compound files, segment infos, `.si`/`.cfs`/`.cfe`, deletes/updates files.
   - Directory/store-level artefacts: file naming, lock files, checksum framing (`CodecUtil`), header/footer envelopes.
   - Token-stream persistence: payloads, attribute serialisation where Lucene persists it.
   - Query- and analysis-side persisted artefacts: synonym/stop-word binary forms, Snowball/Hunspell compiled assets, classification models, suggester FSTs/blob formats.
   - Replication/wire formats: replicator protocol payloads, any IPC frames Lucene exposes.
   - Facets sidecar files, grouping/join persisted state, highlight offset stores, spatial/geo encodings.
   - Any future Lucene-serialised artefact discovered during porting.

3. **Byte-for-byte equality.** Default expectation is **byte-identical output** for the same logical input under the same configured codec/version. Where Lucene legitimately allows non-determinism (e.g., compression dictionaries, ordering driven by hash seeds), the divergence MUST be documented in the affected package, justified against the Lucene 10.4.0 source, and covered by a round-trip test (Gocene-write → Lucene-read → Gocene-read produces the original logical input).

4. **Mandatory tests — isolated AND in combination.** Every feature, no matter how small, MUST ship with compatibility tests proving the mandate. There are two required test classes:
   - **Isolated**: round-trip and golden-corpus tests at the unit level (this feature alone, with fixtures produced by Lucene 10.4.0).
   - **Combined**: integration tests exercising the feature alongside the other features it composes with in real Lucene usage (e.g., codec + doc values + facets + queries used together).
   No feature is "done" until both test classes exist and pass against a Lucene 10.4.0 corpus.

5. **Reference of truth.** The Apache Lucene 10.4.0 source tree (see *Lucene Reference Repository* below) and binaries produced by it are the **sole** reference. Implementation choices that contradict observed Lucene behaviour are bugs in Gocene, not in Lucene.

6. **Workflow consequence.** The standard workflow **Specify → Implement → Test → Document** is interpreted under this mandate:
   - *Specify* must record the exact Lucene 10.4.0 binary contract being targeted (file format, version constant, codec name, struct layout).
   - *Test* must include compatibility tests against Lucene-produced fixtures before the task can be closed.

## 1. Base Rules

1. **You are NOT AUTHORIZED to make decisions on your own.** Whenever the instructions are insufficient, unclear, non-specific, or non-concrete, or whenever they contain contradictions or ambiguities, you MUST ALWAYS ASK the user how to proceed.
   - When asking, always provide multiple options (a, b, c, ...) and indicate which one you recommend.
   - When several clarifications are required, present each question to the user sequentially (one at a time), not all at once.
   - **Boundary between acting and asking:** obvious, low-risk corrections (for example, a pre-existing bug with an unequivocal solution) may proceed immediately; any decision that changes scope, expected behaviour, architecture, or requirements requires prior user approval.

2. **Documentation in English.** All project documentation (including this `CLAUDE.md`) must be written in the most correct English possible, free of orthographic, grammatical, or syntactic errors. Use clear, simple, and unambiguous technical language intended for human readers.

3. **Documentation faithful to the code.** Documentation must be precise and always reflect the real state of the code.

4. **Workflow.** Work always follows this order: **Specify → Implement → Test → Document.**

## 2. Self-Contained Development Policy

All development cycles must be self-contained. You must NEVER deliver only part of a task; every development cycle must produce a complete, working result.

When new needs are discovered during the course of a task — needs that were not anticipated beforehand — they must be resolved within the same development cycle, as immediately as possible. This means creating new tasks and executing them right away, rather than deferring them.

All code and all development output must be, as a rule, **full-fledged**: no half-implementations, no stubs left dangling, no "to be completed later" placeholders.

Tests must never use `t.Skip()`; a gap in coverage must fail, not be silenced.

Whenever you encounter pre-existing bugs during a task, fix them immediately and then continue with the original task.

## 3. Production Orientation

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

Use the **Knowledge Graph** to identify the highest-gain or highest-impact tasks, foundational tasks, and tasks that unblock other tasks or features, so that the execution order can be optimised. By default, always work from the highest-gain tasks towards the least essential.

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

Use the Graph features of `rmp` (Groadmap) to create, maintain (update), and query a knowledge graph for the project. This graph **MUST CONTAIN EVERYTHING** that is useful to know about the project. Examples:

- which features exist and where they are specified and implemented;
- which tests exist and what they test;
- which components exist, how they relate, and what dependencies exist between them;
- in which `git commit` each feature was specified, implemented, and tested;
- the `rmp` tasks and their connection to components.

The graph **MUST ALWAYS BE UPDATED on every `git commit`**, recording the changes to graph objects. Each node and edge update must identify the corresponding commit and date.

**This graph is the absolute truth about the project.** Keep it as up-to-date as possible so that, before reading files, you can query the graph and obtain what you need.

Create whichever node and edge types make the most sense for the project. Use the graph together with tasks and sprints to coordinate work.

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

## 14. Lucene Reference Repository

The authoritative reference for the port is the upstream Apache Lucene source tree at release tag `releases/lucene/10.4.0` (commit `9983b7c`).

- **Expected local path**: `/tmp/lucene` (shallow clone of `https://github.com/apache/lucene.git` at tag `releases/lucene/10.4.0`).
- **If `/tmp/lucene` is absent or empty**, clone it before starting any inventory, planning, or porting task:

  ```bash
  git clone --depth=1 --branch releases/lucene/10.4.0 \
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

- **Port in progress (pre-v1.0):** 33 top-level packages ported from Apache Lucene 10.4.0 (see `README.md` for the package inventory). The project is in active development across 8 sprints: S1–S5 (closed), S6 (Stubbed subsystems — closed 2026-06-11), S7 (Test-suite health — closed 2026-06-11), S8 (Documentation accuracy — in progress).
- **Known deferred items:** 660 `t.Fatal` blockers across 33 packages (see `docs/skipped-tests-audit.md`). Major gaps include: NRT reader integration, RandomIndexWriter test infrastructure, spatial/geo query factories, HNSW seeded strategies, facets/taxonomy write path, and codec format completeness (Lucene99, PerField, DocValuesSkipper).
- **Binary-compatibility test suite in place:** the Java fixture harness under `tools/lucene-fixtures/` drives Lucene 10.4.0 directly via JDK 21 and Maven, produces deterministic fixtures pinned in `tools/lucene-fixtures/manifests/baseline.tsv` (60+ scenarios across every audited package, plus six combined end-to-end scenarios). A Go-side test layer under `internal/compat/` provides per-package round-trips behind the `compat` build tag plus integration scenarios gated by `GOCENE_COMPAT_HARNESS=1`. Note: compat coverage is currently read-path focused (Lucene→Gocene); write-path (Gocene→Lucene) legs are in progress (see `docs/compat-coverage.md`).
- **CI gates every PR:** GitHub Actions runs a fast `build-and-test` job, a skip-guard lint gate, a race-detector job (x86_64), fuzz smoke tests, and a `compat` matrix (three operating systems × two Go versions) that exercises the fixture harness and the Go compat suite.
- **Sprint 7 (Test-suite health) closed 2026-06-11:** refreshed `docs/skipped-tests-audit.md` (660 blockers across 33 packages), enforced blocker token convention in `scripts/check-skips.sh`, added CI/local reconciliation document, and added `Makefile` with `race-test` target.
- **Sprint 6 (Stubbed subsystems) closed 2026-06-11:** resolved 21 PARTIAL/MISSING tasks across 10 packages — expressions compiler with full JS operators, MemoryIndex search, QueryDecomposer, CollectingMatcher, MonitorQuerySerializer, BBoxValueSource, S2PrefixTree geometry, and more.
