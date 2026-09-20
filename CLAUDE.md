# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Absolute Rules — Read Before Acting (BINDING)

**These five rules govern every interaction, every task, and every action in this repository. They are stated here so they cannot be missed, and stated in full in their own sections below. Where anything else in this file, in a skill, or in a subagent's instructions appears to say otherwise, these rules win.**

| # | Absolute rule | In full |
|---|---|---|
| **A1** | **FAITHFUL PORT.** Gocene is Apache Lucene 10.5.0 expressed in Go — faithful in function, in technique, and in output. Results must be **100% equal to and interoperable with** Lucene's. Any divergence is a defect in Gocene. | *Prime Directive*, *Binary Compatibility Mandate*, *Source Fidelity Mandate* |
| **A2** | **RESTRAINT — NO PROACTIVITY.** Do exactly what the user asked, and nothing else. Any extra need, however small or obvious, is **reported** and executed **only after explicit authorisation**. | *Model Conduct — Restraint and Non-Proactivity*, § 1, point 1 |
| **A3** | **DELEGATE TO A SPECIALIST.** **ALL** work is executed by the subagent specialised in the requirements and objectives it is meant to achieve — **ALWAYS** the most suitable one. The main agent plans, delegates, validates, and reports. | § 9.1 |
| **A4** | **ONE SUBAGENT, ALWAYS.** **ONLY ONE** subagent runs in parallel with the main conversation — **NEVER** more. Use every subagent the objective needs, **in series, never in parallel**. Running more than one in parallel is an exception that requires **prior** authorisation, and that authorisation **expires immediately** and is always revoked at the end of the task. | § 9.2 |
| **A5** | **SYNERGY — CENTRAL TO ALL WORK.** The search for synergies governs how every piece of work is planned and executed, in three kinds: **of effort**, **technical**, **functional**. Work that is technically or functionally close **MUST** be joined into a **single** development effort, and like work inside a task is done in one pass. Effort is optimised to the maximum so that **much more work is delivered**, every objective being reached with the **fewest tasks and the fewest iterations possible**. | *Work Synergy — Reach Each Objective in the Fewest Efforts* |

### Self-check before every action

Before every action — every tool call, every edit, every command, every delegation — confirm all six. If any answer is "no", **STOP and ask the user**:

1. **Requested?** Is this exactly what the user asked for, and nothing beyond it? *(A2)*
2. **Authorised?** If it goes beyond the request, do I hold the user's explicit authorisation for this specific item? *(A2)*
3. **Delegated?** Is the work being carried out by the subagent specialised in it? *(A3)*
4. **Alone?** Is exactly one subagent running — this one? *(A4)*
5. **Faithful?** Does the result reproduce Apache Lucene 10.5.0 exactly — in behaviour, in construction, and in output? *(A1)*
6. **Synergised?** Have I sought all three synergies — of effort, technical, functional — and joined everything technically or functionally close to this work, so the objective is reached in the fewest iterations? *(A5)*

### Order of precedence

When two rules appear to conflict, resolve them in this order; the higher entry wins:

1. **Conduct rules A2, A3, A4.** They govern *whether you may act at all*, *who carries out the work*, and *how many agents do it*. Nothing overrides them: no mandate, no deadline, no efficiency argument, and no other section of this file authorises unrequested action, undelegated work, or parallel subagents.
2. **A5 — Work Synergy.** How much work is joined into a single effort, and in how few iterations the objective is reached. It wins over every other section of this file — notably the subdivision guidance of § 4.1 — and over the two mandates below; it never wins over the conduct rules A2–A4. Its supremacy is confined to the grouping question: A5 governs how work is grouped, never what the work must produce, and where grouping would cost byte-level or source fidelity, point 7 of *Work Synergy* stops it.
3. **Binary Compatibility Mandate** — the byte-level contract; it outranks every other rule about *how the work must be done*, save A5 above.
4. **Source Fidelity Mandate** — organisational and behavioural fidelity.
5. Every other section of this file, in the order in which it appears.

A1 (the *Prime Directive*) is the premise of 3 and 4 and is never traded away.

### No deviation

- **No exceptions by interpretation.** These are not defaults, not guidelines, and not subject to in-the-moment judgement about what would be faster, more helpful, or more complete.
- **Every deviation is a defect.** Treat it exactly like a failing test: state it to the user as soon as it is noticed, and correct it.
- **Nothing else may weaken them** — no section of this file, no skill, no subagent instruction, no habit from another project, no urgency. If any instruction appears to permit a deviation, it is being misread or it is wrong: **STOP and ask the user.**
- **Silence is not authorisation.** Neither is the absence of an objection, an authorisation previously granted for a similar case, nor the user's haste.

## Prime Directive — This Repository Is a FAITHFUL PORT of Apache Lucene (ABSOLUTE, NON-NEGOTIABLE)

**Read this before anything else. It states the identity of the project and it is the premise on which every other rule in this document rests.**

Gocene (this repository, the Go module `github.com/FlavioCFOliveira/Gocene`) is a **FAITHFUL PORT** of the **Apache Lucene** library — the search library written in **Java** by the Apache Software Foundation — translated into the **Go** language, targeting the **Apache Lucene 10.5.0** reference release. That is the whole of what this project is.

State it plainly, because everything else follows from it:

> **Gocene is Apache Lucene 10.5.0, expressed in Go.**
> The Java source is the specification. Go is merely the target language of the translation.
> Anything Gocene does that Apache Lucene 10.5.0 does not do — or does differently — is a **defect in Gocene**.

### What this project is NOT

- It is **not** a new search engine, and not an original design.
- It is **not** a library "inspired by", "based on", "in the spirit of", or "API-compatible with" Lucene.
- It is **not** a reimplementation, a reinterpretation, a redesign, or a modernisation of Lucene.
- It is **not** an opportunity to improve, simplify, generalise, optimise, clean up, or correct Lucene.
- It is **not** "Lucene-like search, the Go way". Idiomatic Go is a matter of spelling, never of substance.

### The three dimensions of fidelity — all three are mandatory

The port must be faithful in **all three dimensions simultaneously**. Failing any one of them is a defect, however well the other two hold:

1. **Functional fidelity — the same behaviour.** Gocene must *do* what Lucene 10.5.0 does: the same semantics for every operation, the same defaults, constants and limits, the same edge cases, the same validation, the same errors and boundary handling, the same observable side effects, the same concurrency and lifecycle contracts.

2. **Technical fidelity — the same construction.** Gocene must be *built the way* Lucene 10.5.0 is built: the same algorithms, the same data structures, the same control flow, the same iteration and traversal order, the same decomposition into components, the same package and class layout, and names that remain recognisable against the Java original after Go transliteration (see the *Source Fidelity Mandate*).

3. **Fidelity of results — 100% identical output.** Gocene must *produce* what Lucene 10.5.0 produces: byte-for-byte identical artefacts and result-for-result identical answers (see *The 100% rule* below, and the *Binary Compatibility Mandate*).

### The 100% rule

Given the same input, the same configuration, and the same version, **Gocene must produce results that are 100% equal to, and 100% interoperable with, those of Apache Lucene 10.5.0.** Concretely, and without limitation:

- **The same bytes.** Every file, header, footer, checksum, envelope, segment, and serialised structure written by Gocene is byte-identical to what Lucene 10.5.0 writes for the same logical input.
- **The same answers.** The same query over the same index returns the same documents, in the same order, with the same scores, the same totals, the same explanations, and the same pagination behaviour.
- **The same analysis.** The same text through the same chain yields the same tokens, with the same offsets, positions, position increments, types, payloads, and attributes.
- **The same failures.** Invalid input fails in the same situations, at the same points, with the same meaning; limits, overflows, and rejections occur exactly where Lucene's occur.
- **Full interoperability in both directions.** Anything Gocene writes, Lucene 10.5.0 reads without modification; anything Lucene 10.5.0 writes, Gocene reads without loss or reinterpretation.

**"Almost identical", "equivalent", "functionally similar", "compatible in practice", "close enough" are FAILURES, not successes.** There is no partial credit: either the result is the same, or the port is wrong.

### Operational consequences

1. **Apache Lucene 10.5.0 is the sole arbiter.** Not intuition, not Go convention, not what looks better, not what another port did, not what a search engine "should" do. The reference tree (§ 14) settles every question of fact.
2. **In case of doubt, read the Java source and reproduce it exactly** — then record the source reference. This requires no consultation (see the *Source Fidelity Mandate*, point 4, for the few exceptions).
3. **Go idioms are admissible only where they change nothing observable** — not the behaviour, not the structure, not the serialised form. Where an idiom would change any of those, Lucene's form prevails.
4. **Do not add and do not omit.** Do not invent API, behaviour, options, or safeguards that Lucene 10.5.0 does not have; do not drop behaviour, branches, or constants that it does have.
5. **Lucene's quirks are part of the contract.** Apparent redundancies, historical decisions, odd constants, awkward names, and code that looks dead must be ported as they are. "Fixing" them here is a defect.
6. **Fidelity is proven, never asserted.** Every claim of faithfulness rests on measured evidence (§ 7) and on compatibility tests against Lucene 10.5.0 fixtures (*Binary Compatibility Mandate*, point 4). An untested claim of compatibility counts as no compatibility at all.

### Precedence

This Prime Directive is the premise; the two mandates that follow are how it is enforced. Among the rules that govern **how the work must be done**, the order is the one already established:

1. **A5 — Work Synergy** — how much work is joined into a single effort, and in how few iterations the objective is reached; subordinate only to the conduct rules A2–A4, and superior to the two mandates below on that grouping question alone — never on what the work must produce (*Work Synergy*, point 7).
2. **Binary Compatibility Mandate** — the byte-level contract; highest operative priority among the mandates, superseding every other guideline about how the work is done, save A5 above.
3. **Source Fidelity Mandate** — organisational and behavioural fidelity; subordinate to the mandate above and to A5.
4. Everything else in this document.

These mandates say *how* the work must be done; they never say *what* work to undertake. They therefore never authorise action the user has not requested, and they are always exercised through the conduct rules A2–A4 — see *Order of precedence* in *Absolute Rules* at the top of this file, which is the canonical ordering.

No rule, convention, preference, or judgement anywhere in this repository may be read in a way that weakens the Prime Directive.

## Roadmap

**Name:** gocene

## Project Overview

Gocene is a Go module that is a **faithful port** of Apache Lucene — the Java search library of the Apache Software Foundation — into Go, as established by the *Prime Directive* above. It is a translation, not a reimplementation: its defining goal is byte-by-byte, behaviour-by-behaviour, and result-by-result equality with the original Apache Lucene library — specifically the Apache Lucene 10.5.0 reference release. Go idioms are used only where they leave the behaviour, the structure, and the serialised form unchanged. Every index file, codec envelope, directory artefact, and on-disk format produced by Gocene must be readable by Apache Lucene 10.5.0 without modification, and Gocene must be able to read, without loss or reinterpretation, every binary artefact produced by Apache Lucene 10.5.0.

Because Gocene is a port rather than a reimplementation, Lucene is the **sole** reference of truth, in functionality and in technique alike. Implementation choices that deviate from observed Lucene behaviour are bugs in Gocene, not in Lucene, and results that merely resemble Lucene's are wrong results. Correctness is measured — never assumed — against the Apache Lucene 10.5.0 source tree and the binaries it produces.

## Binary Compatibility Mandate (TOP-PRIORITY, NON-NEGOTIABLE)

This requirement supersedes every other guideline in this document about **how the work must be done**, save A5 (*Work Synergy*), which outranks it. If any other rule, convention, or stylistic preference conflicts with it, this requirement wins. It does **not** authorise work the user has not requested: it constrains the work that is undertaken, and is subordinate to the conduct rules A2–A4 (*Absolute Rules*, top of this file) as to whether, by whom, and by how many agents that work is undertaken.

1. **Produce (write) and Consume (read).** Gocene **MUST** produce binary artefacts that Apache Lucene 10.5.0 can read without modification, **AND** Gocene **MUST** read, without loss or reinterpretation, every binary artefact produced by Apache Lucene 10.5.0. Compatibility is bidirectional and exact; "approximately compatible" is not compatible.

2. **Scope — everything Lucene serialises.** The mandate applies to *every* byte sequence Apache Lucene 10.5.0 emits or accepts, including but not limited to:
   - On-disk index formats: codecs, segment files, postings, doc values, stored fields, term vectors, norms, points/BKD trees, vectors/HNSW, FST dictionaries, compound files, segment infos, `.si`/`.cfs`/`.cfe`, deletes/updates files.
   - Directory/store-level artefacts: file naming, lock files, checksum framing (`CodecUtil`), header/footer envelopes.
   - Token-stream persistence: payloads, attribute serialisation where Lucene persists it.
   - Query- and analysis-side persisted artefacts: synonym/stop-word binary forms, Snowball/Hunspell compiled assets, classification models, suggester FSTs/blob formats.
   - Replication/wire formats: replicator protocol payloads, any IPC frames Lucene exposes.
   - Facets sidecar files, grouping/join persisted state, highlight offset stores, spatial/geo encodings.
   - Any future Lucene-serialised artefact discovered during porting.

3. **Byte-for-byte equality.** Default expectation is **byte-identical output** for the same logical input under the same configured codec/version. Where Lucene legitimately allows non-determinism (e.g., compression dictionaries, ordering driven by hash seeds), the divergence MUST be documented in the affected package, justified against the Lucene 10.5.0 source, and covered by a round-trip test (Gocene-write → Lucene-read → Gocene-read produces the original logical input).

4. **Mandatory compatibility tests — isolated AND in combination.** Every feature, no matter how small, MUST ship with compatibility tests proving the mandate. Compatibility is not assumed, inferred, or guaranteed by code review: it must be demonstrated by tests that exercise Gocene against the Apache Lucene 10.5.0 reference. There are two required test classes:
   - **Isolated**: round-trip and golden-corpus tests at the unit level for the feature alone, using fixtures produced by Lucene 10.5.0. At a minimum this must cover Gocene-write → Lucene-read and Lucene-write → Gocene-read for every serialised artefact the feature emits.
   - **Combined**: integration tests exercising the feature alongside the other features it composes with in real Lucene usage (e.g., codec + doc values + facets + queries used together).
   No feature is "done" until both test classes exist and pass against a Lucene 10.5.0 corpus. A gap in compatibility coverage must be visible as a failing test; it must never be hidden behind `t.Skip()` or a placeholder.

5. **Reference of truth.** The Apache Lucene 10.5.0 source tree (see *Lucene Reference Repository* below) and binaries produced by it are the **sole** reference. Implementation choices that contradict observed Lucene behaviour are bugs in Gocene, not in Lucene.

6. **Workflow consequence.** The standard workflow **Specify → Implement → Test → Document** is interpreted under this mandate:
   - *Specify* must record the exact Lucene 10.5.0 binary contract being targeted (file format, version constant, codec name, struct layout).
   - *Implement* must follow the Lucene 10.5.0 algorithms and data structures closely enough to preserve the binary contract; Go idioms are welcome, but they must not change the serialised form or observable behaviour.
   - *Test* must include compatibility tests against Lucene-produced fixtures before the task can be closed. Every deliverable must prove, with passing tests, that Gocene behaves as a faithful port of Lucene 10.5.0 for the functionality in question.
   - *Document* must state the Lucene 10.5.0 source references and the compatibility test coverage for the feature.

### Index Interoperability — The Round Trip Must Close (NON-NEGOTIABLE)

Point 1 of this mandate (*Produce (write) and Consume (read)*) states the contract in general terms; this states it for the artefact that matters most, so that it cannot be read weakly. **An index is not a format Gocene supports — it is a format Gocene and Apache Lucene 10.5.0 share.** Both directions are mandatory, and they are symmetric.

1. **Read direction — flawless, first time, every time.** Any index written by Apache Lucene 10.5.0 **MUST** open, be traversed, be searched, and be closed by Gocene **without a single error, warning, fallback, degradation, partial read, reinterpretation, or loss of information**. This holds for the index exactly as it is found on disk: every segment, every field kind, and every format enumerated in point 2 of this mandate (*Scope — everything Lucene serialises*); the compound and non-compound layouts; deletions and doc-values updates; and the whole commit and generation history (`segments_N`) as Lucene exposes it. Gocene reads a Lucene index because Gocene *is* Lucene, not because it recognises a foreign format.

2. **Write direction — indistinguishable, not merely readable.** Any index written by Gocene **MUST** be accepted by Apache Lucene 10.5.0 **as if Lucene itself had produced it**. Readable is not the standard; **undetectable as foreign** is the standard: the same file names and extensions, the same headers and footers, the same checksums, the same segment metadata, the same codec identifiers, the same version constants. Nothing in the bytes may reveal which of the two libraries wrote them.

3. **The round trip must close in both orders.** Each of these sequences must preserve the logical content exactly, with no drift at any hop:
   - Lucene-write → Gocene-read → Gocene-write → Lucene-read;
   - Gocene-write → Lucene-read → Lucene-write → Gocene-read.

4. **Any failure of the above is a defect in Gocene.** An error, an exception, a `CorruptIndexException`, an `IndexFormatTooOldException` or `IndexFormatTooNewException` on an index Apache Lucene 10.5.0 itself accepts, an unsupported-format path, a lenient or "best effort" read, a silently skipped field or segment, or **any need to special-case a Lucene-produced index** is a **defect in Gocene** — never a limitation to document, to work around, or to accept. It is fixed at its cause (§ 2.1) and never suppressed.

5. **Proven against real Lucene fixtures, never asserted.** This interoperability is established empirically (§ 7) under the test obligation of point 4 of this mandate (*Mandatory compatibility tests — isolated AND in combination*): with fixtures produced by Apache Lucene 10.5.0 itself, through the Java fixture harness under `tools/lucene-fixtures/` (fixture digests pinned in `manifests/baseline.tsv`) and the Go-side compatibility layer under `internal/compat/` (per-package round trips behind the `compat` build tag; integration scenarios gated by `GOCENE_COMPAT_HARNESS=1`). Both legs — Lucene-write → Gocene-read **and** Gocene-write → Lucene-read — must be covered for every index artefact a feature touches. **An untested direction is an unsupported direction.**

## Source Fidelity Mandate — Organisation and Behaviour (NON-NEGOTIABLE)

Gocene is a **port**, not a reimplementation. Beyond the byte-level contract established by the *Binary Compatibility Mandate* above, **all Gocene code owes fidelity to the Apache Lucene 10.5.0 code in two further dimensions — organisation and functionality.** This mandate is subordinate to the *Binary Compatibility Mandate* and to A5 (*Work Synergy*); it prevails over every stylistic preference, personal judgement, or perceived improvement.

1. **Organisational fidelity.** The structure of Gocene must mirror the structure of Lucene 10.5.0: the package/namespace layout, the distribution of responsibilities across units, the decomposition into components, and the correspondence between a Lucene class and its Gocene counterpart. A Lucene package maps to the equivalent Gocene package; a Lucene class maps to the equivalent Gocene type in the equivalent file; a Lucene class hierarchy maps to the equivalent Go interface/embedding arrangement. Names must remain recognisable against the Lucene original after the necessary Go transliteration (exported identifiers in `CamelCase`, file names in `snake_case`). Do not merge, split, relocate, or rename Lucene units on your own initiative.

2. **Behavioural fidelity.** The functionality of Gocene must reproduce the functionality of Lucene 10.5.0: the same algorithms, the same data structures, the same control flow, the same defaults, constants and limits, the same iteration and traversal order, the same edge cases, the same error and boundary handling, and the same observable side effects. Go idioms are welcome **only** where they leave the observable behaviour and the serialised form unchanged.

3. **Lucene resolves every doubt.** In any case of **doubt, inconsistency, or incoherence** — within Gocene, between Gocene and Lucene, or between two candidate designs — **the formula followed by Lucene always prevails.** This is the default resolution and it requires no consultation: read the Lucene 10.5.0 source (§ 14), reproduce what it actually does, and record the source reference. A design that looks cleaner, simpler, faster, or "more Go" than Lucene's but diverges from it is a **bug in Gocene**, not an improvement. Lucene's apparent quirks, redundancies, and historical decisions are part of the contract and must be ported as they are.

4. **Consulting the user is the exception, not the rule.** Under this mandate, the obligation of § 1, point 1 to ask the user does **not** apply to doubts that Lucene itself settles — those you resolve by following Lucene and proceeding. Consult the user **only** in these specific cases:
   - Lucene 10.5.0 is itself genuinely ambiguous or contradictory on the point, and reading the source plus measurement (§ 7) cannot settle it;
   - Lucene relies on a JVM-only facility with no faithful Go equivalent (for example class-loading SPI, finalisation, the JVM threading and locking model, or `MemorySegment` mapping), so a design decision is unavoidable;
   - two equally faithful renderings exist and they differ in observable behaviour or in serialised form;
   - fidelity would require changing the project's scope, public API surface, architecture, or previously agreed requirements;
   - reproducing Lucene faithfully would introduce a safety problem, in the sense of § 10 (**correct → safe → fast**).

   In every other situation, follow Lucene and continue: do not stop to ask, and do not invent an alternative.

5. **Fidelity must be traceable.** Every ported unit must identify the Lucene 10.5.0 artefact it corresponds to (§ 5.1 records this correspondence in the Knowledge Graph as `PORTED_TO`), and any documented divergence must be justified against the Lucene source and covered by tests, exactly as required for binary divergences.

## Model Conduct — Restraint and Non-Proactivity (NON-NEGOTIABLE)

This rule governs the behaviour of the Claude model itself. It applies to every interaction, every task, and every mode of work.

1. **Maximum restraint is the required default.** Your action must be **HIGHLY** directed at the objective of each piece of work. Do exactly what the user asked — no more, no less. The user's request defines the entire scope of the work. Anything outside that request is out of scope by default.

2. **You are FORBIDDEN from being proactive, from acting on your own initiative, and from voluntarily starting tasks that were not EXPLICITLY requested.** Do not anticipate needs, do not add improvements, do not extend scope, do not "while I was here" anything. Specifically, and without limitation, you must NOT, unless it was requested:
   - refactor, reorganise, rename, or reformat code that the request did not target;
   - add features, options, helpers, abstractions, or configuration that were not requested;
   - create, delete, or rename files, directories, branches, tasks, or documents;
   - fix unrelated bugs, warnings, or lint findings discovered along the way;
   - add or rewrite tests, documentation, or comments beyond what the request covers;
   - run commands with side effects (commits, merges, pushes, installs, migrations, graph or roadmap writes) that the request did not call for;
   - "clean up" anything on your own judgement.

   The closing operations prescribed by § 4.2 (steps 7 and 8) — the commit and the Knowledge Graph update — belong to the authorised task-closure procedure and are therefore not unrequested proactivity. A2 continues to govern every side effect outside that procedure, and the user confirmation required by § 4.3, step 4 still applies to the git operations.

3. **Any extra need requires prior authorisation.** If, while carrying out the request, you identify a need outside the scope of the task being executed — one that is not expressed in the request or is not clear from it: a prerequisite, a side effect, an adjacent defect, a missing piece — you may **NOT** act on it, and you must **NEVER** start that task proactively. You must **STOP, REPORT it to the user, and ASK** how to proceed. You may only act after the user explicitly authorises it. Silence, absence of objection, or a previous authorisation for a similar case is **not** authorisation.

4. **When asking, follow § 1, point 1.** Present the situation briefly and objectively, offer multiple options (a, b, c, ...), state which one you recommend, and ask one question at a time.

5. **Report, do not act.** Observations, suspicions, improvement ideas, and detected defects are to be **reported** to the user and left there. Reporting is the deliverable; acting on them is not, until authorised.

6. **This rule does not license incomplete work.** Restraint applies to the *scope* of the work, never to its *quality* or *completeness*: what the user did ask for must still be delivered in full, finished and production-grade (§ 2, § 3). Do not use this rule as a reason to stop halfway through the requested work.

## Work Synergy — Reach Each Objective in the Fewest Efforts (NON-NEGOTIABLE)

**The search for synergies is a central element in the conduct of all work in this project.** It is not one policy among others: it governs how every piece of work is planned, grouped, and executed. Its purpose is to optimise effort to the maximum, so that **much more work is delivered**.

1. **Three kinds of synergy, sought in every piece of work.**
   - **Synergy of effort** — one pass of an activity serves many units of work, instead of one pass per unit.
   - **Technical synergy** — work that shares algorithms, data structures, serialised formats, packages, files, or tooling is carried out together, once.
   - **Functional synergy** — work that serves the same feature, the same behaviour, or the same Lucene component is carried out together, once.

2. **Joining is an obligation, not an aspiration.** Whenever tasks — tracked in `rmp` or not — are technically or functionally close, they **MUST** be joined into a single development effort. Always seek to maximise the synergy of one development effort across several tasks; a boundary between tracked tasks is never a reason to keep close work apart.

3. **Batch like work inside a task.** By strategy and by default, identify the synergies within a task. If code must be written and then tested, write all of the code in one pass and test all of it in one pass, instead of writing small fragments and testing each in isolation. The same applies to documentation: treat all of it in one pass or, where the scope is too large, identify blocks and treat each block whole. Apply this to every kind of work, without exception.

4. **The principle is constant.** The search for synergy and the optimisation of effort govern every way of working in this project. **Objectives must be reached with the fewest tasks and the fewest iterations possible.**

5. **Synergy never overrides restraint (A2).** Joining work the user did not request is a change of scope: report the synergy, propose the grouping, and act only after explicit authorisation. Inside the scope the user has already authorised, batch aggressively and without asking.

6. **Synergy is not parallelism (A4, § 4.2).** A joined effort is still one effort: one subagent at a time, one task at a time, in series. Grouping scope is never a licence to run subagents or tasks concurrently.

7. **Synergy never trades away completeness or fidelity.** Fewer iterations must never mean less delivered (§ 2, § 2.2) and never a weaker port (*Prime Directive*, the two mandates). If compacting the work would cost completeness, correctness, or fidelity, the work is not compacted.

## Mandatory Skills — Who Operates What (NON-NEGOTIABLE)

These three skills are the sole operators of their domains: an operation performed by any other route is a defect — a write made by another route must be reverted and redone through the skill, and a read obtained by another route must be discarded and obtained again through the skill.

| Skill | Domain | Rule |
|---|---|---|
| **`gitflow`** | Git write operations | **Every** git command that writes — branch creation and deletion, checkout of new branches, `add`, `commit`, `merge`, `rebase`, `tag`, `push`, and any other mutating operation — is executed **by a specialised subagent** through the `gitflow` skill, following the gitflow branching methodology for this repository (§ 4.3): the skill is the operator, the subagent is the executor. Read-only inspection (`status`, `log`, `diff`, `show`) does not require it. |
| **`roadmap-manager`** | Tasks, sprints, and comments | Coordination and management of tasks, sprints, and the typed comment log go through the `roadmap-manager` skill, as does **every** operation of the `rmp` CLI, **with the sole exception of `rmp graph *`** (§ 4). |
| **`knowledge-authority`** | The Knowledge Graph | Every operation on the project's Knowledge Graph — **reads as much as writes** — is handled **exclusively** by the `knowledge-authority` skill: every `rmp graph *` invocation, queries included, every change to the graph's data and schema, and `knowledge-model.md` (§ 5). Other skills and agents obtain graph information through this skill, never by invoking `rmp graph` themselves. |

- **The choice of skill is never optional.** It is never replaced by ad-hoc commands, scripts, or direct CLI invocations, however convenient or quick they appear.
- **Skills and subagents are complementary.** The work is delegated to the subagent specialised in it (A3), and that subagent operates these domains through the skills named here.

## Language Used in Instructions and Work

You must seek to use (write) and to interpret language in a way that, at every moment, allows you to:

- **BE EXPLICIT**, so that what is intended is clear;
- **BE OBJECTIVE**, so that what is to be executed is always known;
- **BE CLOSED**, so that the scope of the work to be done is defined;
- **BE CONCISE**, so that few words are used to describe what is intended.

These four requirements govern all documentation as well, not only instructions.

## 1. Base Rules

1. **You are NOT AUTHORISED to make decisions on your own.** Whenever the instructions are insufficient, unclear, non-specific, or non-concrete, or whenever they contain contradictions or ambiguities, you MUST ALWAYS ASK the user how to proceed.
   - When asking, always provide multiple options (a, b, c, ...) and indicate which one you recommend.
   - When several clarifications are required, present each question to the user sequentially (one at a time), not all at once.
   - **Boundary between acting and asking:** the boundary is **the user's request**, not the size or the risk of the change. Whatever lies inside the request is executed; whatever lies outside it — including an obvious, low-risk correction, or a pre-existing bug with an unequivocal solution — is **reported and awaits explicit authorisation** (Absolute Rule A2, *Model Conduct — Restraint and Non-Proactivity*). A decision that changes scope, expected behaviour, architecture, or requirements always requires prior user approval.
   - **Exception — doubts that Lucene settles:** where the doubt, inconsistency, or incoherence concerns how Gocene should be organised or how it should behave, do not ask: apply the *Source Fidelity Mandate* above and follow Lucene 10.5.0. Only the specific cases listed in point 4 of that mandate require prior consultation.

2. **Documentation in English.** All project documentation (including this `CLAUDE.md`) must be written in the most correct English possible, professional in tone, and free of orthographic, grammatical, or syntactic errors. Use clear, simple, and unambiguous technical language intended for human readers. This covers **all** documentation — from the main `README.md` to the specification, including code documentation — and every one of those documents must exercise the four requirements of *Language Used in Instructions and Work*: explicit, objective, closed, and concise.

3. **Documentation faithful to the code.** Documentation must be precise and always reflect the real state of the code.

4. **Workflow.** Work always follows this order: **Specify → Implement → Test → Document.**

## 2. Self-Contained Development Policy

All development cycles must be self-contained. You are FORBIDDEN from executing tasks or work only partially: at every moment, you must ensure that every piece of work started is carried out to its full extent. You must NEVER deliver only part of a task; every development cycle must produce a complete, working result. **DO NOT LEAVE TASKS HALF-DONE OR PARTIALLY DONE.**

Self-containment applies to the scope the user has authorised. When new needs are discovered during the course of a task — needs that were not anticipated beforehand — they must be **reported to the user and authorised before being acted upon**, as required by the *Model Conduct — Restraint and Non-Proactivity* mandate above. Once authorised, they must be resolved within the same development cycle, as immediately as possible, rather than deferred. Without authorisation they are reported and left undone; they are never executed on your own initiative.

All code and all development output must be, as a rule, **full-fledged**: no half-implementations, no stubs left dangling, no "to be completed later" placeholders.

Tests must never use `t.Skip()`; a gap in coverage must fail, not be silenced.

Whenever you encounter pre-existing bugs during a task, **report them to the user and ask whether to fix them**. Only after explicit authorisation do you fix them and continue with the original task; without it, you record the finding and carry on with the task as requested.

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

Where suppressions already exist in the tree, they are technical debt to be removed: within the scope of the work at hand, the file must be re-enabled and the real errors resolved; outside it, the suppression is reported and awaits authorisation (A2, § 2).

### 2.2 Complete Components Only — A Partial Port Is Not a Port (NON-NEGOTIABLE)

**Porting work is always carried out over complete components. A partial port is a defect in Gocene; only a complete port is acceptable.**

The unit of porting work is the whole Apache Lucene 10.5.0 component — a class or interface together with its full member set: every constructor and method, every field, constant and enum value, every nested and inner type, and the behaviour of each of them. It is never the subset that happens to make a caller compile.

Prohibited — this list is illustrative, not exhaustive:

- porting "just enough" of a class to satisfy a caller: members left unported, branches or edge cases dropped, behaviour narrowed to the path currently in use;
- a stand-in body where Lucene has an algorithm — a `return nil`, a constant answer, an identity function where Lucene transforms, an empty collection where Lucene computes one;
- a type or interface declaring fewer members than the Java original, or a signature reshaped so that less has to be ported;
- an invented substitute with no counterpart in Lucene 10.5.0 — a type, a helper, a format — standing in for a component that is simply absent (*Prime Directive*, *Operational consequences*, point 4: **do not add and do not omit**).

**If a component cannot be ported completely within the current cycle, port none of it and report** (Absolute Rule A2). Leaving it absent is the correct outcome, and the resulting compile error is an accurate report that must be left standing (§ 2.1). A partial port removes that report without removing its cause: it compiles, it passes, and it turns the build green over code that does not exist in Lucene. **A partial port is therefore worse than no port at all** — a missing component announces itself, a fabricated one does not.

Dependencies are part of the assessment. If a faithful port of a component requires another component that is itself absent, that is **reported and awaits authorisation** — never worked around by inventing a narrow local substitute for the missing dependency.

Completeness is **measured, never asserted** (§ 7): established member by member against the Apache Lucene 10.5.0 source (§ 14), recorded at that granularity in the `PORTED_TO` relation (§ 5.1, point 3), and reported with the evidence cited. "Most of the class", "the parts we use", and "enough to compile" are not completeness.

## 3. Production Orientation

Every action you take — whether development, fixes, evaluations, analysis, audits, or any other work — must be treated with production-grade standards.

Throughout the entire work cycle (analysis → planning → development → testing), the objective must always be that the result produced is **production-grade**. You must apply not only the maximum of your knowledge but also the maximum of your effort to ensure that every piece of work is delivered as code ready to be used in production.

There is no acceptable "draft" or "experimental" mode for delivered work: every commit, every closed task, every merged branch must meet production standards.

## 4. Task Planning and Execution

For operations related to Tasks or Sprints, use the `roadmap-manager` skill. **Every** `rmp` operation runs through that skill, with the sole exception of `rmp graph *`, which runs **exclusively** through the `knowledge-authority` skill (*Mandatory Skills*, § 5).

Use the `rmp` tool (the roadmap-management CLI available on the system) to plan and coordinate task execution. Treat `rmp` as the **single source of truth** for planning and executing the tasks of this project. No other management mechanism may be used for this purpose.

Use the **Knowledge Graph**, queried through the `knowledge-authority` skill, to understand the project, its components, and the relationships between them, so that you can more easily identify the scope and impact of each task.

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

Use the **Knowledge Graph**, queried through the `knowledge-authority` skill, to identify the highest-gain or highest-impact tasks, foundational tasks, and tasks that unblock other tasks or features, so that the execution order can be optimised. By default, always work from the highest-gain tasks towards the least essential. Foundational tasks and tasks that unblock other work are always prioritised.

When a task is too large to be executed in one go by an AI agent such as Claude Code, subdivide it into smaller parts while respecting the principles already defined (in particular, the self-contained task principle). Subdivision is the exception, applied only when a single effort genuinely cannot be completed in one go: by default, planning **must** join work that is technically or functionally close into a single effort, so that the objective is reached in the fewest tasks (A5, *Work Synergy*), and grouping never weakens the self-contained-task principle.

### 4.2 Execution

Execution is the natural next step after planning. Always use `rmp`, through the `roadmap-manager` skill (*Mandatory Skills*, § 4), and follow this sequence:

1. Check whether any open task remains unfinished so it can be continued.
2. Identify the next task.
3. Understand the objective of the task to be started, based on its description, functional requirements, and technical requirements.
4. Determine the most appropriate subagent and delegate execution to them.
5. Always validate the acceptance criteria before closing the task.
6. Close the task with a short summary of what was done.
7. After closing the task and before moving on to the next, have a specialised subagent perform a `git commit` through the `gitflow` skill, following best practices and explaining what was done (*Mandatory Skills*, § 4.3).
8. Have a specialised subagent update the Knowledge Graph through the `knowledge-authority` skill (*Mandatory Skills*, § 5).

Execution notes:

- You may develop **only one task at a time**, in strict sequential order. Active development work must never be parallelised across multiple tasks. Joining tasks that are technically or functionally close into a single development effort (A5, *Work Synergy*) is not parallelisation: the joined effort remains a single, sequential unit of work.
- Whenever possible, adapt the model and its effort level to the requirements of each individual task operation.
- Task and sprint execution is **sequential**.
- Evaluations and audits may run in parallel **only** under the exceptional, single-use authorisation described in § 9.2: parallel execution must **ALWAYS be authorised by the user beforehand**, and that authorisation expires immediately once the authorised run ends.
- Every task is executed by a specialised subagent (§ 9.1), and never more than one subagent at a time (§ 9.2).
- The closing operations of a task — the commit (step 7) and the Knowledge Graph update (step 8) — are delegated like any other work: a specialised subagent executes them, in series (A3, A4).

### 4.3 Gitflow Integration

All branch and commit operations described in this section are executed **by a specialised subagent** through the `gitflow` skill (*Mandatory Skills*, A3): the skill is the operator, the subagent is the executor. User confirmation (step 4 below) precedes execution; it does not transfer execution to the user.

For each task, create the appropriate branch following gitflow conventions:

- **feature/** — new features and enhancements;
- **hotfix/** — urgent bug fixes;
- **release/** — release preparation branches.

The branching workflow for each task:

1. Create the appropriate branch based on the nature of the task.
2. Develop the task on that branch.
3. Upon completion, execute the branch closure procedure for the branch type: `feature/` merges to `develop`; `release/` and `hotfix/` merge to `main` and back to `develop`.
4. All operations must be confirmed by the user before execution.

## 5. Knowledge Graph

The Knowledge Graph is the **central element** of this project: the instrument through which the structure of Apache Lucene 10.5.0, the structure of Gocene, and the relation between the two are known, recorded, queried, and kept current (§ 5.2). The `knowledge-authority` skill is its sole operator.

**Every access to the Knowledge Graph and to its model — reads as much as writes — is made EXCLUSIVELY through the `knowledge-authority` skill (NON-NEGOTIABLE).** This applies without exception to the graph's data (creating, updating, or deleting nodes, edges, and properties), to its schema (labels, predicates, properties, constraints, indexes), to **every `rmp graph *` invocation, queries included**, and to the model document `knowledge-model.md` (*Mandatory Skills*). No other skill, agent, or script may write to the graph, edit `knowledge-model.md`, or invoke `rmp graph` at all. Other skills and agents — `roadmap-manager` included — obtain graph information **through** `knowledge-authority`, never by invoking `rmp graph` themselves. A change made by any other route is a defect and must be reverted and redone through the skill; a read made by any other route is a defect on the same terms — its result must be discarded and obtained again through the skill.

Use the Graph features of `rmp` (Groadmap), always through the `knowledge-authority` skill, to create, maintain (update), and query a knowledge graph for the project. This graph **MUST CONTAIN EVERYTHING** that is useful to know about the project. Examples:

- which features exist and where they are specified and implemented;
- which tests exist and what they test;
- which components exist, how they relate, and what dependencies exist between them;
- in which `git commit` each feature was specified, implemented, and tested;
- the `rmp` tasks and their connection to components.

The graph **MUST ALWAYS BE UPDATED on every `git commit`**, recording the changes to graph objects. Each node and edge update must identify the corresponding commit and date.

**This graph is the absolute truth about the project.** Keep it as up-to-date as possible so that, before reading files, you can obtain what you need from the graph through the `knowledge-authority` skill.

Create whichever node and edge types make the most sense for the project. Use the graph together with tasks and sprints to coordinate work.

### 5.1 Graph Fidelity Requirement (MANDATORY)

The Knowledge Graph **MUST be faithful to the real state of the project**. Fidelity is not aspirational and is never assumed: it must be demonstrable by measurement (§ 7) against the Apache Lucene 10.5.0 reference tree (§ 14) and the Gocene working tree. Four requirements are mandatory and each one must hold independently:

1. **Faithful representation of the source library (Apache Lucene).** The graph must represent the structure of Apache Lucene 10.5.0 — its components and their organisation and hierarchy (modules, packages/namespaces, classes, and their members: methods, fields, constants) — exactly as it exists in the reference tree at `/tmp/lucene`. Every `LuceneModule`, `LucenePackage`, and `LuceneClass` node must correspond to an artefact that is actually present in that tree, and the containment chain (module → package → class → member) must reproduce the real hierarchy. **The representation must not stop at class level:** methods and constructors, fields, constants, enum values, nested and inner classes, interfaces, records, and annotations must be represented as nodes of their own whenever the port depends on them. Nothing may be invented, inferred, or recorded from memory; conversely, no artefact of the reference tree that is in scope for the port may be missing from the graph.

2. **Faithful representation of the target module (Gocene).** The graph must represent the structure of the Gocene module — its components and their organisation and hierarchy (packages, files, structs, interfaces, types, methods, functions, constants, variables, tests) — exactly as it exists in the repository working tree. Every `Package`, `File`, `Symbol`, `Component`, and `Feature` node must correspond to an artefact that is actually present in the repository, and the containment chain (package → file → symbol → member) must reproduce the real hierarchy. **The representation must not stop at type level:** `Symbol` must cover not only structs, interfaces, type aliases, and tests, but also functions, methods, constants, and package-level variables, at the granularity at which the port is actually carried out. Renames, moves, additions, and deletions in the code must be reflected in the graph in the same development cycle that performs them.

3. **Faithful representation of the port relation and its status.** The graph must record the link between each Apache Lucene component and its Gocene counterpart, together with the **port state of that pair (Lucene → Gocene)**. The `PORTED_TO` predicate is the sole authority on port status: for every Lucene artefact it must state whether it is ported and, if so, to which Gocene artefact. **The relation must hold at every granularity at which porting actually happens** — module to package, package to package, class to type, and equally method to method, field to field, and constant to constant — so that port status is answerable per element and never merely per class. The absence of the relation means **"not ported"**, never "unknown" — unported artefacts must therefore be visible as such by query alone. Derived attributes (for example `LuceneClass.is_ported`) must be computed from the relation and never written independently, so the graph can never assert a port status that contradicts its own edges.

4. **The graph must carry every structure the port needs to succeed.** Granularity is dictated by the needs of the porting/translation work, not by convenience: the graph must represent whatever is required to plan, execute, and verify the translation of Lucene 10.5.0 into Go, on both sides of the port and in the relation between them. **A representation that stops at the class or type level is insufficient** — functions, methods, constructors, fields, constants, enum values, nested types, signatures, and any other element on which the port depends must be present. If an element that the port depends on cannot be expressed by the current labels, properties, or predicates, **the model must be extended** and `knowledge-model.md` updated accordingly (§ 5): the schema is never a valid reason to omit structure, and structure is never simplified away because the schema does not yet accommodate it.

Consequences of this requirement:

- **Fidelity is measured, not claimed.** Any statement about port coverage, gaps, or scope must come from graph queries reconciled against both trees, with the evidence cited (§ 7).
- **A divergence between the graph and either tree is a defect**, and where it lies within the work at hand it must be corrected immediately, within the current development cycle (§ 2), never deferred; where it lies outside that work, it is reported and awaits authorisation (A2, § 5.2, point 5). It must never be tolerated or annotated as acceptable.
- **Every `git commit` must leave the graph faithful**, including the commit that records the change (§ 5). Fidelity is a precondition for closing a task, not a follow-up task.
- **`knowledge-model.md` must conform to the live graph** and is regenerated from measurements, never hand-written from memory. Use the `knowledge-authority` skill to sync, refresh, and audit fidelity.

### 5.2 The Graph Is the Central Instrument of the Port (MANDATORY)

**The Knowledge Graph — its nodes and its edges — is the central element of the porting work.** It is not documentation written about the work after the fact: it is the medium through which the port is known, planned, executed, verified, and remembered. Every porting decision starts from a graph query and ends with a graph update.

Over each of the three objects below you must be able to perform the full cycle — **read, analyse, recognise, persist, query, and maintain** — using the graph as the instrument. Every operation of that cycle — queries and reads as much as writes and maintenance — goes exclusively through the `knowledge-authority` skill (§ 5).

1. **Know the structure of the source library, Apache Lucene (Java), faithfully.**
   - **Read** the reference tree at `/tmp/lucene` (§ 14) — never memory, never assumption (§ 6).
   - **Analyse and recognise** the artefacts it actually contains: modules, packages, classes, interfaces, enums, records, annotations, nested and inner types, constructors, methods, fields, constants, enum values, signatures, and the relations between them (containment, inheritance, implementation, use).
   - **Persist** them as nodes and edges that reproduce that structure and hierarchy exactly (§ 5.1, point 1).
   - **Query** them through the `knowledge-authority` skill to establish the scope, the shape, and the dependencies of any piece of work before touching code.
   - **Maintain** them as the port advances into new areas of the library, so the source side of the graph is always the real Lucene 10.5.0 tree.

2. **Know the structure of the target module, Gocene (Go), faithfully.**
   - **Read** the repository working tree as it actually is.
   - **Analyse and recognise** its packages, files, types, structs, interfaces, functions, methods, constants, variables, and tests, together with their relations and dependencies.
   - **Persist** them as nodes and edges that reproduce that structure and hierarchy exactly (§ 5.1, point 2).
   - **Query** them through the skill before reading files: the graph answers what exists, where it lives, and what depends on it (§ 12).
   - **Maintain** them in the **same development cycle** that adds, renames, moves, or deletes code — never in a later one.

3. **Know and maintain the relation between origin (1) and destination (2), so that it serves the porting work.**
   - The `PORTED_TO` edges are the link and the sole authority on port status, at **every granularity at which porting actually happens** (§ 5.1, point 3): module→package, package→package, class→type, method→method, field→field, constant→constant.
   - The purpose of the relation is **utility to the port**, so the graph must answer, by query alone through the `knowledge-authority` skill and without reading a single file:
     - **what is already ported**, and to exactly which Go artefact;
     - **what remains to be ported**, element by element — the absence of `PORTED_TO` *is* the answer, and it means "not ported", never "unknown";
     - **port coverage**, per module, per package, per class, and per member;
     - **what a given Lucene artefact became in Go**, and conversely, what a given Go artefact came from;
     - **what blocks a piece of work** — the unported dependencies a task would need first;
     - **the impact and scope of a change**, on either side of the port.
   - **Maintain** the relation with the same cycle as the code: a port that is done but not recorded is, for every purpose in this project, a port that has not happened.

**Operational protocol — binding:**

1. **Query the graph first.** Before planning, before choosing what to port next, before reading source files, and before answering any factual question about either tree, query the graph through the `knowledge-authority` skill (§ 5, *Mandatory Skills*).
2. **Act with the graph as the map.** Scope, order of work, dependencies, and blockers are taken from the graph, not from intuition or recollection.
3. **Update the graph immediately after.** Every change to either tree is reflected in nodes and edges within the same development cycle and recorded on the corresponding `git commit` (§ 5).
4. **Coverage is measured, never estimated.** Any statement about what is ported, what is missing, or how much is done must be produced by a graph query reconciled against both trees, with the evidence cited (§ 7, § 5.1).
5. **A gap in the graph is a defect**, handled like any other defect: report it to the user, and correct it when the work at hand covers it or once authorised (Absolute Rule A2).

## 6. Never Guess

All interactions on the project must be based **exclusively** on verified knowledge. You must never try to guess the intended answer.

When available information is insufficient, seek answers from official or authoritative sources: specifications, RFCs, papers, books, or recognised authors in the relevant field.

Use the **Knowledge Graph**, through the `knowledge-authority` skill (*Mandatory Skills*, § 5), as the primary source of information — both to look up what is already known and to record the relationships you discover as you go.

## 7. Measure to Decide

Whenever it is necessary to evaluate **performance**, **completeness** (whether something is fully done), or **correctness** (whether something behaves as required), you must ALWAYS gather evidence from the project itself to determine the answer. Decisions of this kind must be **empirical**.

Concretely, this means:

- Run the relevant tests, benchmarks (`go test -bench=. -benchmem`), or profilers (`pprof`) and read their output before claiming a property holds.
- Inspect actual generated artefacts (bytes on disk, fixture outputs) rather than reasoning only about expected behaviour.
- Cite the captured evidence (test names, benchmark numbers, byte diffs) when reporting conclusions.

Assumptions, intuition, or prior recall are not acceptable substitutes for measured evidence in these three dimensions.

## 8. Regression Prevention

Whenever a bug is fixed within the scope of the work at hand, create the necessary regression tests to ensure that the same bug does not recur as a consequence of future development. A bug identified outside that scope is reported and awaits authorisation (A2, § 2); its regression test is written together with the authorised fix.

## 9. Team of Subagents

You have at your disposal a team composed of all available subagents (global, user-defined, or project-defined). You **MUST** use every subagent you need to achieve your objective, but they are used **in series, never in parallel** — one at a time and in strict sequence (§ 9.2): the strength of a task comes from choosing the right specialist for it, never from running several at once.

Each task is carried out by the single subagent whose specialisation matches its requirements and objectives (§ 9.1), so that the task is completed with maximum confidence, effectiveness, and accuracy. Where a task genuinely requires more than one specialisation, the specialists are used **sequentially** — one finishes and reports before the next is launched — and each works strictly within the scope it was given, contributing its specialisation to that scope and nothing beyond it.

When initiating a task, identify the single most appropriate subagent for the task's scope: the specialist chosen for a task is always a subagent (A3, § 9.1). Skills are not an alternative to that choice — they are the mandatory operators of their domains (git, tasks and sprints, the Knowledge Graph) and are used **by** the chosen subagent (*Mandatory Skills*). However, always remember: **the focus of any task is to contribute to the development of Gocene.** Avoid excessive research or analysis — the goal is implementation, not just understanding. Gather only the information necessary to complete the task.

### 9.1 Mandatory Delegation to a Specialised Subagent (NON-NEGOTIABLE)

**ALL work in this project must delegate its execution to a subagent specialised in the requirements and objectives that the work is meant to achieve, and you must ALWAYS choose the most suitable subagent.** This is not a preference and not an optimisation: it is the required mode of execution.

1. **No task is executed directly.** Before starting any task, identify the requirements and the objective of the task, choose the subagent whose specialisation matches them, and delegate the execution to that subagent. The main agent plans, chooses the specialist, delegates, validates the acceptance criteria, and reports — it does not do the work itself.
2. **The choice must be justified by the match.** The subagent chosen is **ALWAYS** the most suitable one: it is chosen because its specialisation covers what the task actually requires (language, subsystem, domain, type of work), never by convenience or habit. If no existing subagent matches the task, stop and ask the user which subagent to use or to create.
3. **Delegation does not transfer responsibility.** Every rule of this document applies in full to the delegated work, and the result must be validated against the task's acceptance criteria before the task is closed (§ 4.2).

### 9.2 One Subagent at a Time (ABSOLUTELY FORBIDDEN to Exceed)

**You must use ONLY ONE SINGLE subagent in parallel with the main Claude Code conversation. You are ABSOLUTELY FORBIDDEN from running more than one subagent in parallel — you must NEVER use more than one. One subagent, on every occasion, without exception.**

1. **Strictly one at a time.** Launch a subagent, wait for it to finish, read its result, and only then consider the next one. Never dispatch two or more subagents in the same message, never start a second while a first is still running, and never fan out work across several subagents "to save time".
2. **This applies to every kind of work** — development, research, exploration, review, evaluation, audit, documentation, measurement — and to every mechanism of delegation, including background execution and workflows.
3. **Parallelism requires explicit prior authorisation from the user**, and whenever the user authorises more than one subagent in parallel, that authorisation is an exception. Ask, state how many subagents and for exactly what, and wait for the answer.
4. **The authorisation expires immediately and is always revoked at the end of the task.** It is valid only for the single, specific occasion for which it was granted; it lapses the instant that parallel execution ends and, in every case, is revoked at the end of the task for which it was granted. It is never a standing permission, is never carried over to a similar case, and is never extended by analogy. The next occasion requires a new authorisation.

## 10. Decision Framework

To decide what is expected as a project result — whether during evaluations and audits or during code implementation — follow this priority order: **correct → safe → fast.**

1. **Is it correct?** Does the result match the objective, the project specification, and the applicable authoritative sources (RFCs, standards, etc.)?
2. **Is it safe?** Does the decision or task introduce any characteristic or behaviour that compromises the safe use of the deliverable?
3. **Is it fast?** Is it the fastest achievable without compromising correctness or safety? What can be done to maximise the performance of the deliverable?

If conflicts arise between these criteria, or if difficulty arises in following them, ask the user immediately how to proceed, presenting the possible options.

## 11. Segregation of Responsibilities

Each package, component, and function must follow a strict pattern of segregation of responsibilities in order to maximise code reuse. That segregation follows the decomposition of Apache Lucene 10.5.0 and never replaces it: where the two would diverge, the *Source Fidelity Mandate* prevails, and units are not merged, split, relocated, or renamed on Gocene's own initiative.

## 12. Memory

Use the Knowledge Graph as the memory for the project, the agents, and the skills. Leverage the relational capabilities of the graph database to optimise how you read and write your memories; every one of those reads and writes goes through the `knowledge-authority` skill (§ 5, *Mandatory Skills*). Use this method to save the token cost of reading files.

**ALWAYS** update the Knowledge Graph, through the `knowledge-authority` skill, whenever project files are changed, so that you maintain the ability to understand the project through the graph.

## 13. Development Guidelines

When implementing Lucene features in Go:

- Follow Go best practices and idioms while maintaining compatibility with Lucene's behaviour.
- Port algorithms and data structures from Lucene's Java implementation.
- Consider how to translate Java's object-oriented patterns to Go's interface-based approach.
- Test against Lucene's expected behaviour for byte-level compatibility.
- Apply the *Source Fidelity Mandate* (organisation and behaviour): Go idioms are admissible only where they leave the structure, the observable behaviour, and the serialised form faithful to Lucene 10.5.0; where they do not, Lucene prevails.

## 14. Lucene Reference Repository

The authoritative reference for the port is the upstream Apache Lucene source tree at release tag `releases/lucene/10.5.0` (commit `f6eaee8`).

- **Expected local path**: `/tmp/lucene` (shallow clone of `https://github.com/apache/lucene.git` at tag `releases/lucene/10.5.0`).
- **If `/tmp/lucene` is absent or empty**, clone it before starting any inventory, planning, or porting task:

  ```bash
  git clone --depth=1 --branch releases/lucene/10.5.0 \
      https://github.com/apache/lucene.git /tmp/lucene
  ```

- Module sources live under `/tmp/lucene/lucene/<module>/src/java/...` (production code), `/tmp/lucene/lucene/<module>/src/java21/...` (JDK-21 specific code, where present), and `/tmp/lucene/lucene/<module>/src/test/...` (tests). Some modules also expose `src/test-files/...` (test resources).
- The reference tree must be treated as read-only context; never modify it.
