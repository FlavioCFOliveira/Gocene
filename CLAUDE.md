# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gocene is a Go module that aims to be a port of Apache Lucene to modern idiomatic Golang, byte-by-byte compatible with the original Apache Lucene library.

This is an early-stage project. The module structure, packages, and development workflow are not yet established.

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

## Task Management

The `roadmap-manager` skill is responsible for generating and managing all project tasks. It must work in conjunction with the `go-gitflow` skill to ensure gitflow best practices are followed:

### Gitflow Integration

The `go-gitflow` skill enforces gitflow workflow. For each task, the appropriate branch type must be created:

- **feature/** - New features and enhancements
- **hotfix/** - Urgent bug fixes
- **release/** - Release preparation branches

The workflow for each task:
1. Use `/skill go-gitflow` to create the appropriate branch based on task nature
2. Develop the task on the created branch
3. Upon completion, execute the branch closure procedure (merge to main)
4. All operations must be confirmed by the user before execution

### Task Scope & Specialists

When initiating a ROADMAP task, identify the most appropriate specialists (skills or agents) to understand the task scope:

- Use available skills like `gocene-lucene-specialist` for Lucene compatibility analysis
- Use agents like `Explore` for codebase research
- Consult documentation and existing implementations for context

However, always remember: **the focus of any task is to contribute to the development of Gocene**. Avoid excessive research or analysis - the goal is implementation, not just understanding. Gather only the information necessary to complete the task.

### Task Focus

- Autonomously gather information for the ROADMAP
- Always ask the user which tasks to develop next
- Maintain focus on the specific requirements of each task
- Never implement features or changes that exceed the task's stated requirements

To start task planning, invoke: `/skill roadmap-manager`

## Project Status

- No source code yet committed
- No tests, benchmarks, or CI configuration
- Build and test commands will be defined as the project matures