# CLAUDE.md

Gocene is a Go module that ports the open-source Java library Apache Lucene to Go. The module aims to be a faithful port of the original library — both functionally and technically.

## Overriding goal

The overriding goal of this Go module is to make it possible to **READ AND WRITE** Lucene indexes through the `Gocene` library. The converse must hold equally: Apache Lucene must be able to read and write indexes that were created and maintained by Gocene. This means 100% binary compatibility in both directions, for both reading and writing.

The reference behaviours are **always and mandatorily** those of **Apache Lucene**. Lucene defines what is correct; Gocene conforms. Where Gocene and Lucene differ in behaviour, Gocene is wrong by definition and must be corrected — never the other way round, and never by adjusting the expectation to match what Gocene currently does.

The port targets **Apache Lucene Core** version **`10.5.0`** exclusively. Any reference, behaviour, API, or file format must be checked against that exact version — never against another Lucene release.

## Test framework

Beyond the Lucene Core library itself, Lucene's **test framework must also be ported integrally to Go**, together with the core test suite it drives. This is the guarantee mechanism: the module's code is exercised by the same tests as the original and must produce **exactly the same results**.

A ported component is not done because it compiles — it is done when the corresponding Lucene tests, ported faithfully, pass against it with identical outcomes. Do not weaken, skip, or rewrite a test so that Gocene passes it; a failing ported test is evidence of a defect in Gocene.

## Source of truth

Apache Lucene is open source. The official Git repository is cloned locally, at the exact release tag, to:

```
/tmp/lucene-10.5.0        # github.com/apache/lucene @ tag releases/lucene/10.5.0
```

Within that checkout:

| What | Path (under `/tmp/lucene-10.5.0`) |
| --- | --- |
| Core sources | `lucene/core/src/java` |
| Core tests | `lucene/core/src/test` |
| Core JDK-21 specific sources | `lucene/core/src/java21` |
| Test framework | `lucene/test-framework/src/java` |
| Test framework resources | `lucene/test-framework/src/resources` |

The reference tree is **read-only context — never modify it**.

This checkout is the **absolute source of truth** for the port. Never port from memory, documentation, or a blog — read the actual Java source at the `10.5.0` tag. If the folder is missing (a machine temp folder does not survive a reboot), re-create it with a shallow clone at that tag before continuing.

Gocene must stay faithful to that source: the package and file organisation, the type and member decomposition, the algorithms, and every default, constant, and reference value must mirror the original. When Gocene diverges, the divergence must be a deliberate, documented Go idiom adaptation — never an accident.

### Deciding what to port, and how

Every decision about **what** to port and **how** to port it is governed by fidelity to the original source. Consult, in this order:

1. **Your own knowledge** of the Lucene library — to orient the problem and form the hypothesis.
2. **The library's reference documentation** (javadoc, `CHANGES.txt`, `MIGRATE.md`) — to confirm intent and contracts.
3. **The cloned Lucene source code** at `/tmp/lucene-10.5.0` — the most absolute source of truth.

The source code always wins. Where knowledge or documentation disagrees with the checked-out code, the code is right and the decision follows the code.

## Token economy

This project follows a **strict token-economy policy**.

Any operation that can be performed locally on the machine must be performed locally — it must not be handed to the model to do "by reasoning" or by reading large amounts of content into context. Prefer a deterministic command that produces the answer over a model pass that infers it.

In practice:

- Use CLI tooling to search, count, compare, list, format, build, and test, instead of reading files to work it out.
- Before starting a task, evaluate the CLI tooling available on the machine and use whatever supports the work best; reach for a tool that already answers the question rather than reconstructing the answer in context.
- Read only the parts of a file that are actually needed, never whole files by default.
- Do not re-read, re-derive, or re-verify what has already been established.

## Task management

Use the `roadmap-manager` skill to manage all project tasks via the `rmp` CLI tool. Run `rmp --ai-help` at any time for the machine-readable command contract whenever there is any doubt about how to operate the CLI. All task tracking, progress, and sprint work must go through this CLI.

## Knowledge Graph

Use the `knowledge-authority` skill to manage the project's Knowledge Graph (KG).

The KG must hold all information essential to the purpose of porting Lucene to Gocene: the full structure of packages, classes, and related entities of both the original Apache Lucene codebase and the ported Gocene codebase, including the mapping between them — what has already been ported and what is still missing.

The KG is the central coordination piece for the Lucene → Gocene migration. It must be kept up to date continuously as work proceeds, so that at any moment it can answer the state of the migration: what is already done and what is still missing.

## Git

Use the `gitflow` skill for all Git management operations.

**ALL** git commands must be run alone and in isolation — never chained or combined with other commands (no `&&`, `;`, or pipes joining a git command to anything else). One git command per shell invocation.
