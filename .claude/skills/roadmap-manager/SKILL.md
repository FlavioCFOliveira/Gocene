---
name: roadmap-manager
description: Autonomous orchestrator that commands audits from multiple skills, manages ROADMAP.md with sequential IDs, records completion dates, and organizes tasks into development phases based on dependencies. Use this skill when the user wants to manage project tasks, coordinate skill audits, track progress, organize tasks by dependencies and phases, replan the roadmap, or update the roadmap.
commands:
  - name: /roadmap-sync
    description: Initiates the autonomous multi-agent audit cycle, progress validation, and ROADMAP.md timestamp synchronization.
  - name: /roadmap-replan
    description: Reconstructs development phases by analyzing task dependencies, ensuring optimal ordering where tasks without dependencies come first and no tasks in the same phase depend on each other.
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

## 2. DEVELOPMENT PHASES MANAGEMENT (PHASES)

Tasks are organized into **development phases** based on dependency analysis. This ensures optimal execution order and clear progress tracking.

### Phase Organization Principles

1. **Dependency Analysis**: Each task is analyzed for dependencies on other tasks
2. **No Intra-Phase Dependencies**: Tasks within the same phase must NOT depend on each other
3. **Topological Ordering**: Phases are ordered so all dependencies of a phase are completed before the phase begins
4. **Phase Re-evaluation**: After each phase completes, dependencies are re-analyzed and the roadmap is updated

### Phase Structure

Each phase contains:
- **Phase Number**: Sequential identifier (Phase 1, Phase 2, etc.)
- **Phase Name**: Descriptive name for the phase
- **Tasks**: List of task IDs that can be executed in parallel
- **Dependencies**: Tasks from previous phases that must complete before this phase
- **Status**: PENDING, IN_PROGRESS, or COMPLETED

### Dependency Analysis Rules

When determining task dependencies:

1. **Interface Dependencies**: Task A implements interface used by Task B (A must come before B)
2. **Composition Dependencies**: Task B uses types from Task A (A must come before B)
3. **Data Dependencies**: Task B needs data structures from Task A
4. **Functional Dependencies**: Task B's functionality builds on Task A

### Using Specialists for Dependency Analysis

When analyzing dependencies for the `/roadmap-replan` command:

1. **Invoke go-elite-developer**: For analyzing Go code dependencies and implementation order
2. **Invoke gocene-lucene-specialist**: For understanding Lucene architecture dependencies
3. **Invoke go-performance-advisor**: For identifying performance-critical paths that should come early

Ask each specialist: *"Analyze the task list in ROADMAP.md and identify dependencies between tasks. For each task, list which other tasks it depends on and why, considering the technical implementation order."*

### Phase Assignment Algorithm

```
1. Collect all PENDING tasks from ROADMAP.md
2. Query specialists for dependency analysis
3. Build dependency graph (task -> list of tasks it depends on)
4. Initialize Phase 1
5. While unassigned tasks exist:
   a. Find tasks with NO remaining dependencies
   b. Assign these tasks to current phase
   c. Mark these tasks as "assigned" in dependency graph
   d. Remove completed dependencies from remaining tasks
   e. If no tasks assigned but unassigned tasks remain:
      - Report circular dependency error
   f. Increment phase number
6. Generate PHASES table in ROADMAP.md
```

### Phase Transition Protocol

When a phase completes:

1. **Validate Completion**: Verify all tasks in the phase are marked COMPLETED
2. **Re-analyze Dependencies**: Check if completion of these tasks unlocks new dependencies
3. **Update PHASES Table**: Mark phase as COMPLETED
4. **Review Next Phase**: Confirm next phase tasks have all dependencies satisfied
5. **Update ROADMAP**: Ensure PENDING TASKS table reflects current status

## 3. GITFLOW INTEGRATION (CRITICAL)

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

## 4. /roadmap-replan COMMAND

When `/roadmap-replan` is invoked:

### Step-by-Step Replanning Process:

**Step 1: Gather Current Tasks**
- Read ROADMAP.md
- Extract all PENDING tasks
- Note existing phases (if any) for reference

**Step 2: Invoke Dependency Analysts**
Simultaneously query:
- `go-elite-developer`: *"Analyze these pending tasks and identify implementation dependencies. For each task, list which other tasks it directly depends on based on code structure, interface requirements, and Go implementation patterns."*
- `gocene-lucene-specialist`: *"Analyze these pending tasks from a Lucene architecture perspective. Identify architectural dependencies where one Lucene component requires another to be implemented first."*

**Step 3: Build Dependency Graph**
Consolidate findings from all specialists:
```
Task: GC-XXX
Dependencies: [GC-YYY, GC-ZZZ]  // Tasks that must complete BEFORE this one
Rationale: "Technical reason from specialist analysis"
```

**Step 4: Detect Cycles**
- Validate no circular dependencies exist
- If found, report to user for resolution

**Step 5: Assign Phases**
Apply the phase assignment algorithm:
```
Phase 1: Tasks with NO dependencies
Phase 2: Tasks depending only on Phase 1
Phase 3: Tasks depending on Phase 1 and/or 2
... continue until all tasks assigned
```

**Step 6: Validate Phase Constraints**
- Verify NO tasks within the same phase depend on each other
- Verify ALL dependencies of a phase are in earlier phases

**Step 7: Update ROADMAP.md**
Replace or update the DEVELOPMENT PHASES section with new phase assignments

**Step 8: Report Changes**
Provide summary to user:
- Number of phases created
- Tasks per phase
- Key dependency chains identified
- Any warnings or conflicts

### Replanning Output Format

```
## DEVELOPMENT PHASES (Auto-Generated)

### Phase 1: [Name]
**Status:** PENDING | **Tasks:** N
**Focus:** [Brief description of phase scope]

| Task ID | Task Name | Specialists | SEVERITY | PRIORITY |
|:--------|:----------|:------------|:---------|:---------|
| GC-XXX  | Name      | specialist  | HIGH     | HIGH     |

**Dependencies:** None (foundation tasks)

---

### Phase 2: [Name]
**Status:** PENDING | **Tasks:** N
**Focus:** [Brief description]

| Task ID | Task Name | Specialists | SEVERITY | PRIORITY |
|:--------|:----------|:------------|:---------|:---------|
| GC-YYY  | Name      | specialist  | MEDIUM   | HIGH     |

**Dependencies:** Phase 1 (GC-XXX, ...)
```

## 5. MANDATORY AUDIT PROTOCOL

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

## 9. SEVERITY vs PRIORITY

It is CRITICAL to distinguish between these two fields:

### SEVERITY
**Definition**: Impact on security vulnerabilities, bugs, or stability issues.
- **HIGH**: Critical security vulnerability (e.g., SQL injection, RCE), data corruption risk, or system crash potential
- **MEDIUM**: Bug affecting functionality but with workaround, or moderate stability concern
- **LOW**: Minor bug, cosmetic issue, or no immediate stability/security impact

### PRIORITY
**Definition**: Utility and impact of the task on the Gocene project as a whole.
- **HIGH**: Critical for project success, high user impact, foundational component that enables other features
- **MEDIUM**: Important feature, moderate impact on project goals
- **LOW**: Nice-to-have feature, low impact on overall project success

### Examples:
- **HIGH Severity + LOW Priority**: Security fix for a rarely-used experimental feature
- **LOW Severity + HIGH Priority**: Performance optimization that benefits all users
- **HIGH Severity + HIGH Priority**: Critical bug in core search functionality
- **LOW Severity + LOW Priority**: Documentation typo in internal API

## 10. ROADMAP.md MANAGEMENT

The `ROADMAP.md` file is unique and located at the project root. It contains the history of completed tasks and backlog.
This file is the consolidated result of audits and must follow this rigorous hierarchy:

### File Structure:

#### 1. DEVELOPMENT PHASES REFERENCE TABLE

Central reference table showing the relationship between phases, tasks, and their attributes. This table is automatically generated/managed by the `/roadmap-replan` command.

```
## DEVELOPMENT PHASES

| Phase | Status | Tasks | Focus | Dependencies |
|:------|:-------|:------|:------|:-------------|
| 1 | COMPLETED | GC-001 to GC-013 | Foundation Layer | None |
| 2 | COMPLETED | GC-014 to GC-032 | Document Model | Phase 1 |
| 3 | IN_PROGRESS | GC-033 to GC-045 | Analysis Pipeline | Phase 2 |
| 4 | PENDING | GC-046 to GC-052 | Index Operations | Phase 3 |
```

#### 2. PHASE DETAILS

Detailed breakdown of each phase with its tasks:

```
### Phase 1: Foundation Layer
**Status:** COMPLETED | **Tasks:** 13 | **Completed:** 2026-03-11

| Task ID | Task Name | Specialists | SEVERITY | PRIORITY |
|:--------|:----------|:------------|:---------|:---------|
| GC-001 | Store Layer - Directory | go-elite-developer | HIGH | HIGH |
| ...    | ...       | ...         | ...      | ...      |

**Dependencies:** None (foundation tasks)

---

### Phase 2: Document Model
**Status:** COMPLETED | **Tasks:** 19 | **Completed:** 2026-03-12

| Task ID | Task Name | Specialists | SEVERITY | PRIORITY |
|:--------|:----------|:------------|:---------|:---------|
| GC-014 | Document class | go-elite-developer | HIGH | HIGH |
| ...    | ...       | ...         | ...      | ...      |

**Dependencies:** Phase 1 (GC-001 through GC-013)
```

#### 3. PENDING TASKS

Table with tasks still to complete, ordered by severity (HIGH > MEDIUM > LOW) and priority (HIGH > MEDIUM > LOW).

```
| ID | SEVERITY | PRIORITY | TASK | SPECIALISTS | ACTIONABLE TECHNICAL DESCRIPTION |
| :--- | :--- | :--- | :--- | :--- | :--- |
| [SKILL]-001 | HIGH | HIGH | Task Name | go-elite-developer | Detailed technical instruction for execution. |
| [SKILL]-002 | MEDIUM | MEDIUM | Task Name | go-gitflow | Detailed technical instruction for execution. |
| [SKILL]-003 | LOW | LOW | Task Name | go-performance-advisor | Detailed technical instruction for execution. |
```

**SEVERITY column**: Indicates the impact on security vulnerabilities, bugs, or stability issues.
- HIGH: Critical security vulnerability, data loss risk, or system instability
- MEDIUM: Bug affecting functionality but with workaround, or moderate stability concern
- LOW: Minor bug, cosmetic issue, or no immediate stability impact

**PRIORITY column**: Indicates the utility/impact of the task on the Gocene project as a whole.
- HIGH: Critical for project success, high user impact, foundational component
- MEDIUM: Important feature, moderate impact on project goals
- LOW: Nice-to-have, low impact on overall project success

**SPECIALISTS column**: Required. List the skill or agent names that should resolve this task (e.g., `go-elite-developer`, `go-gitflow`, `red-team-hacker`, `go-performance-advisor`, `gocene-lucene-specialist`). Separate multiple specialists with commas.

#### 2. COMPLETED TASKS

Table with completed tasks, ordered by completion date (most recent first).

```
| ID | SEVERITY | PRIORITY | TASK | SPECIALISTS | COMPLETED | ACTIONABLE TECHNICAL DESCRIPTION |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| [SKILL]-001 | HIGH | HIGH | Task Name | go-elite-developer | 2026-12-31 | [Technical reference to solution or commit] |
| [SKILL]-002 | MEDIUM | MEDIUM | Task Name | go-gitflow | YYYY-MM-DD | [Technical reference to solution or commit] |
| [SKILL]-003 | LOW | LOW | Task Name | go-performance-advisor | YYYY-MM-DD | [Technical reference to solution or commit] |
```

## 11. EXPLICIT VALIDATION STEPS

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

## 12. TASK EXECUTION WORKFLOW

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

## 13. AGENTIC EXECUTION RULES

- **Unique Sequential Numbering**: Each task receives an immutable ID. Once assigned, the ID follows the task until completion.
- **Completion Timestamp**: Required to record the date in ISO 8601 format (YYYY-MM-DD) when the task is moved to "COMPLETED".
- **Fact Validation**: Before marking as complete, you MUST read the filesystem to confirm the implementation reflects the task.
- **Technical Specification**: Tasks must be described as execution orders (e.g., "ID-015: Implement input sanitization in authentication middleware").
- **Total Proactivity**: If an audit detects a risk, immediately insert it into the roadmap at the correct severity without human intervention.
- **Clear Communication**: Provide regular updates on audit progress and roadmap status, maintaining a professional and technical tone.
- **Technical References**: For each completed task, record the technical reference (e.g., link to commit, solution description implemented) in the roadmap to ensure traceability.
- **Task Closure**: After implementation, validate that the task was resolved by reading code and commits, then update the roadmap with completion date and technical reference.

## 14. QUALITY AND STYLE STANDARDS

- **Tone**: Professional, direct, and purely technical.
- **Emojis**: Strictly prohibited in all audit files and roadmap.
- **Formatting**: Clean Markdown, organized tables, and no decorative elements.