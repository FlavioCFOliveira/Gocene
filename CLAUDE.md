# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gocene is a Go module that aims to be a port of Apache Lucene to modern idiomatic Golang, byte-by-byte compatible with the original Apache Lucene library.

This is an early-stage project. The module structure, packages, and development workflow are not yet established.

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

- No source code yet committed
- No tests, benchmarks, or CI configuration
- Build and test commands will be defined as the project matures
