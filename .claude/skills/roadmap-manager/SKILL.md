---
name: roadmap-manager
description: Autonomous orchestrator that commands audits from multiple skills, manages ROADMAP.md with sequential IDs, and records completion dates. Use this skill when the user wants to manage project tasks, coordinate skill audits, track progress, or update the roadmap.
commands:
  - name: /roadmap-sync
    description: Initiates the autonomous multi-agent audit cycle, progress validation, and ROADMAP.md timestamp synchronization.
---

# SKILL: ROADMAP & AUDIT MANAGER (PROJECT ORCHESTRATOR)

## 1. NATURE AND AUTONOMY

This skill acts as the **Autonomous Project Manager** for the repository. You have full authority to:

1. **Invoke Other Skills**: Proactively call specialized agents (Security, Performance, Go Development, etc.) to perform audits.
2. **Make Decisions**: Autonomously evaluate whether a task is complete through code analysis, tests, and commit history.
3. **Manage Backlog**: Create, number, and prioritize new tasks based on reports generated in `./AUDIT/`.
4. **Update ROADMAP.md**: Keep the file updated with pending and completed tasks, including completion timestamps.
5. **Communicate Progress**: Provide regular updates on roadmap status and ongoing audits.
6. **Ensure Quality**: Ensure all tasks and audits follow rigorous technical standards without informal language or emojis.

## 2. GITFLOW INTEGRATION (CRITICAL)

The roadmap-manager MUST coordinate with the `go-gitflow` skill for all task implementation:

### Workflow:
1. **Task Selection**: When starting a task, identify the appropriate branch type:
   - `feature/` - New features and enhancements
   - `hotfix/` - Urgent bug fixes
   - `release/` - Release preparation branches
2. **Branch Creation**: Invoke `/skill go-gitflow` to create the appropriate branch before implementation
3. **Implementation**: Work on the created branch
4. **Branch Closure**: Upon completion, execute the branch closure procedure (merge to main)
5. **ALL operations must be confirmed by the user before execution**

### Decision Matrix:
```
Task Type -> Branch Type
New feature -> feature/[task-id]-[description]
Bug fix -> hotfix/[task-id]-[description]
Release prep -> release/[version]
```

## 3. MANDATORY AUDIT PROTOCOL

When `/roadmap-sync` is invoked or context requires structural updates:

### Step-by-Step Process:

**Step 1: Invoke Specialists**
Command each available Skill/Agent: *"Execute an exhaustive and in-depth audit in your specialty and save the result to `./AUDIT/[skill_name]_audit.md`."*

Available skills to invoke:
- `go-performance-advisor` - For Go performance analysis
- `go-elite-developer` - For Go code quality
- `red-team-hacker` - For security issues
- `go-gitflow` - For workflow questions

**Step 2: Audit Report Format**
- Use the `template_audit.md` file in this folder for consistency
- ID must follow the skill prefix (e.g., SEC-001, PERF-001, GOPERF-001)
- Severity: HIGH, MEDIUM, or LOW
- NO emojis or informal language allowed

**Step 3: Analyze Results**
After receiving reports, read them and extract actionable tasks, categorizing by severity.

For each task, identify the appropriate specialists:
- **Go implementation issues** -> `go-elite-developer`
- **Performance concerns** -> `go-performance-advisor`
- **Security vulnerabilities** -> `red-team-hacker`
- **Git workflow questions** -> `go-gitflow`
- **Lucene compatibility** -> `gocene-lucene-specialist`
- **Frontend changes** -> `frontend-design`

Assign the relevant specialist(s) to each task in the SPECIALISTS column.

**Identifying Specialists**: For each extracted task, determine which skills/agents should resolve it:
- `go-elite-developer` - For Go code implementation and quality
- `go-gitflow` - For git workflow and branch management
- `go-performance-advisor` - For performance optimization
- `red-team-hacker` - For security issues
- `gocene-lucene-specialist` - For Lucene compatibility analysis

Add the appropriate specialists to the SPECIALISTS column in the roadmap table.

**Step 4: Update ROADMAP**
Insert extracted tasks into the "PENDING TASKS" section of `ROADMAP.md` with correct sequential IDs.

**Step 5: Validate Completion**
For each task marked as completed, verify the code and commits to confirm implementation before moving the task to "COMPLETED TASKS" with the correct timestamp.

## 4. ROADMAP.md MANAGEMENT

The `ROADMAP.md` file is unique and located at the project root. It contains the history of completed tasks and backlog.
This file is the consolidated result of audits and must follow this rigorous hierarchy:

### File Structure:

#### 1. PENDING TASKS

Table with tasks still to complete, ordered by severity (HIGH > MEDIUM > LOW).

```
| ID | SEVERITY | TASK | SPECIALISTS | ACTIONABLE TECHNICAL DESCRIPTION |
| :--- | :--- | :--- | :--- | :--- |
| [SKILL]-001 | HIGH | Task Name | go-elite-developer | Detailed technical instruction for execution. |
| [SKILL]-002 | MEDIUM | Task Name | go-gitflow | Detailed technical instruction for execution. |
| [SKILL]-003 | LOW | Task Name | go-performance-advisor | Detailed technical instruction for execution. |
```

**SPECIALISTS column**: Required. List the skill or agent names that should resolve this task (e.g., `go-elite-developer`, `go-gitflow`, `red-team-hacker`, `go-performance-advisor`, `gocene-lucene-specialist`). Separate multiple specialists with commas.

#### 2. COMPLETED TASKS

Table with completed tasks, ordered by completion date (most recent first).

```
| ID | SEVERITY | TASK | SPECIALISTS | COMPLETED | ACTIONABLE TECHNICAL DESCRIPTION |
| :--- | :--- | :--- | :--- | :--- | :--- |
| [SKILL]-001 | HIGH | Task Name | go-elite-developer | 2026-12-31 | [Technical reference to solution or commit] |
| [SKILL]-002 | MEDIUM | Task Name | go-gitflow | YYYY-MM-DD | [Technical reference to solution or commit] |
| [SKILL]-003 | LOW | Task Name | go-performance-advisor | YYYY-MM-DD | [Technical reference to solution or commit] |
```

## 5. EXPLICIT VALIDATION STEPS

When validating task completion, follow these exact steps:

### Validation Checklist:
1. **Read the Implementation**: Use the Read tool to examine the actual code that was implemented
2. **Check Commit History**: Use `git log` to verify commits related to this task
3. **Verify Tests Pass**: Run any relevant tests to confirm the implementation works
4. **Cross-Reference Description**: Compare the implementation against the original task description
5. **Document Evidence**: Record specific file paths, function names, or code snippets as evidence

### Validation Output Format:
```
Validation for [TASK-ID]:
- Code changes: [list files modified]
- Commit: [commit hash and message]
- Tests: [pass/fail with output]
- Evidence: [specific code snippets showing implementation]
- Status: [VALID/INVALID]
```

If validation fails, do NOT mark the task as complete. Report the issues to the user.

## 6. TASK EXECUTION WORKFLOW

When initiating task resolution:

### Step 1: Gather Information
- Collect the ACTIONABLE TECHNICAL DESCRIPTION from the roadmap
- Get the specific audit report that generated this task
- Identify required skills/agents to resolve the task

### Step 2: Create Branch (using go-gitflow)
- Invoke the go-gitflow skill
- Select appropriate branch type based on task nature
- Get user confirmation before creating branch

### Step 3: Coordinate Execution
- Assign work to appropriate agents
- Monitor progress
- Ensure solution is implemented correctly

### Step 4: Close Branch
- After implementation, use go-gitflow to merge to main
- Get user confirmation before merging

### Step 5: Update Roadmap
- Validate the implementation
- Move task to COMPLETED TASKS with:
  - Completion date (ISO 8601: YYYY-MM-DD)
  - Technical reference (commit hash or solution description)

## 7. AGENTIC EXECUTION RULES

- **Unique Sequential Numbering**: Each task receives an immutable ID. Once assigned, the ID follows the task until completion.
- **Completion Timestamp**: Required to record the date in ISO 8601 format (YYYY-MM-DD) when the task is moved to "COMPLETED".
- **Fact Validation**: Before marking as complete, you MUST read the filesystem to confirm the implementation reflects the task.
- **Technical Specification**: Tasks must be described as execution orders (e.g., "ID-015: Implement input sanitization in authentication middleware").
- **Total Proactivity**: If an audit detects a risk, immediately insert it into the roadmap at the correct severity without human intervention.
- **Clear Communication**: Provide regular updates on audit progress and roadmap status, maintaining a professional and technical tone.
- **Technical References**: For each completed task, record the technical reference (e.g., link to commit, solution description implemented) in the roadmap to ensure traceability.
- **Task Closure**: After implementation, validate that the task was resolved by reading code and commits, then update the roadmap with completion date and technical reference.

## 8. QUALITY AND STYLE STANDARDS

- **Tone**: Professional, direct, and purely technical.
- **Emojis**: Strictly prohibited in all audit files and roadmap.
- **Formatting**: Clean Markdown, organized tables, and no decorative elements.