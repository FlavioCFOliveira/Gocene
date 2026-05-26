# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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

## General Principles

1. **Decision authority**: You are NOT AUTHORIZED to make decisions on your own. Whenever the instructions are insufficient, unclear, non-specific, non-concrete, or contain contradictions or ambiguities, you MUST ALWAYS ASK the user how to proceed. When asking questions, provide multiple options (a, b, c, ...) and indicate which one you recommend. When several clarifications are required, present each question to the user sequentially (one at a time).

2. **Documentation language**: All project documentation must be written in English, in the most perfect form possible, free of orthographic, grammatical, or syntactic errors. Use clear, simple, and unambiguous technical language intended for human readers.

3. **Documentation accuracy**: Documentation must be precise and faithful to the code.

4. **Workflow steps**: The workflow must always follow these steps, in order: **Specify -> Implement -> Test -> Document**.

## Development Guidelines

When implementing Lucene features in Go:

- Follow Go best practices and idioms while maintaining compatibility with Lucene's behavior
- Port algorithms and data structures from Lucene's Java implementation
- Consider how to translate Java's object-oriented patterns to Go's interface-based approach
- Test against Lucene's expected behavior for byte-level compatibility

## Lucene Reference Repository

The authoritative reference for the port is the upstream Apache Lucene source tree at release tag `releases/lucene/10.4.0` (commit `9983b7c`).

- **Expected local path**: `/tmp/lucene` (shallow clone of `https://github.com/apache/lucene.git` at tag `releases/lucene/10.4.0`).
- **If `/tmp/lucene` is absent or empty**, clone it before starting any inventory, planning, or porting task:

  ```bash
  git clone --depth=1 --branch releases/lucene/10.4.0 \
      https://github.com/apache/lucene.git /tmp/lucene
  ```

- Module sources live under `/tmp/lucene/lucene/<module>/src/java/...` (production code), `/tmp/lucene/lucene/<module>/src/java21/...` (JDK-21 specific code, where present), and `/tmp/lucene/lucene/<module>/src/test/...` (tests). Some modules also expose `src/test-files/...` (test resources).
- The reference tree must be treated as read-only context; never modify it.

## Initial Setup

Once development begins, initialize the Go module:

```bash
go mod init github.com/FlavioCFOliveira/Gocene
```

## Task Planning & Execution

For planning and coordinating task execution, you must use the `rmp` tool (a CLI available on the system for roadmap management). This tool is the **single source of truth** for project planning and task execution; no other mechanism may be used for this purpose.

### Planning

Carefully observe the scope of work proposed by the user and first determine whether it should be split across multiple development phases in order to properly accommodate the tasks involved. Each phase must contain a solid deliverable.

All tasks must have a very clear and objective definition of:

- **Objectives**
- **Functional requirements**
- **Technical requirements**
- **Acceptance criteria** — the conditions that confirm the task can be considered complete

Whenever a task is completed, it must be closed with a short summary of what was done.

Phases are represented as **Sprints** in the `rmp` tool, which serve to group tasks.

If the work being planned requires multiple phases (or sprints), planning must be performed in two distinct stages:

1. First, define which phases (or sprints) are necessary and the scope (objective) of each sprint.
2. Then, walk through each sprint individually to determine its tasks.

Always use `rmp` as the single source of truth throughout this process.

### Task Execution

Task execution is the natural continuation (the next step) of planning. You must always use the `rmp` tool to:

1. Check whether there is any open task not yet completed to follow up on.
2. Identify the next task.
3. Understand the objective of the task to be started, based on its description, functional requirements, and technical requirements.
4. Validate that the acceptance criteria are met before closing the task.
5. Close the task with a short summary of what was done.
6. After closing the task, and before moving on to the next one, perform a git commit following best practices, explaining what was done.

Sprint execution must always be **sequential**. Task execution should preferably be sequential as well; tasks may be executed in parallel only when there is clear justification to do so.

### Gitflow Integration

For each task, the appropriate branch type must be created following gitflow conventions:

- **feature/** — New features and enhancements
- **hotfix/** — Urgent bug fixes
- **release/** — Release preparation branches

The branching workflow for each task:

1. Create the appropriate branch based on the nature of the task.
2. Develop the task on that branch.
3. Upon completion, execute the branch closure procedure (merge to main).
4. All operations must be confirmed by the user before execution.

### Specialists & Research

When initiating a task, identify the most appropriate specialists (skills or agents) to understand the task scope:

- Use skills such as `gocene-lucene-specialist` for Lucene compatibility analysis.
- Use agents such as `Explore` for codebase research.
- Consult documentation and existing implementations for context.

However, always remember: **the focus of any task is to contribute to the development of Gocene**. Avoid excessive research or analysis — the goal is implementation, not just understanding. Gather only the information necessary to complete the task.

### Task Focus

- Maintain focus on the specific requirements of each task.
- Never implement features or changes that exceed the task's stated requirements.

## Project Status

- **Port complete (v1.0 candidate):** the full Apache Lucene 10.4.0 surface
  has been ported across 25 top-level packages (see `README.md` for the
  package inventory and `CHANGELOG.md` for the v0.1.0-alpha / Unreleased
  entries).
- **Binary-compatibility test suite in place:** the Java fixture harness
  under `tools/lucene-fixtures/` drives Lucene 10.4.0 directly via JDK 21
  and Maven, produces deterministic fixtures pinned in
  `tools/lucene-fixtures/manifests/baseline.tsv` (60+ scenarios across
  every audited package, plus six combined end-to-end scenarios), and is
  paired with a Go-side test layer under `internal/compat/` (per-package
  round-trips behind the `compat` build tag, plus integration scenarios
  gated by `GOCENE_COMPAT_HARNESS=1`).
- **CI gates every PR:** GitHub Actions runs a fast `build-and-test` job
  plus a `compat` matrix (three operating systems × two Go versions) that
  exercises the fixture harness and the Go compat suite.
- **Sprint 114 (Binary Compatibility Test Suite) closed 2026-05-26:** the
  105-row coverage audit, 21 per-package compat tasks, six combined
  scenarios, the mutation-diagnostic CLI, and the CI/contributing-guide
  hardening listed in `CHANGELOG.md` are all merged. Remaining deferrals
  are documented in `docs/compat-coverage.md`.
