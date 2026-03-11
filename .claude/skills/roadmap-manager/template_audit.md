---
skill_name: "[SKILL_NAME]"
audit_date: "YYYY-MM-DD"
specialty: "[EX: SECURITY / PERFORMANCE / BACKEND / GOCENE]"
summary:
  high_priority: 0
  medium_priority: 0
  low_priority: 0
status: "COMPLETED"
---

# TECHNICAL AUDIT REPORT: [SKILL_NAME]

## 1. SPECIALTY SUMMARY

Objective technical analysis of the current state of the project in this specialty. Identification of immediate risks
and structural improvement opportunities.

## 2. TASK LIST BY SEVERITY

Tasks below must be described as direct execution orders for agentic flows.

| ID          | SEVERITY | TASK         | SPECIALISTS | ACTIONABLE TECHNICAL DESCRIPTION           |
|:------------|:---------|:-------------|:------------|:-------------------------------------------|
| [SKILL]-001 | HIGH     | Task Name    | go-elite-developer | Detailed technical instruction for execution. |
| [SKILL]-002 | MEDIUM   | Task Name    | go-gitflow, red-team-hacker | Detailed technical instruction for execution. |
| [SKILL]-003 | LOW      | Task Name    |              | Detailed technical instruction for execution. |

**SPECIALISTS**: List the skill/agent names that should resolve this task. Available: go-elite-developer, go-gitflow, red-team-hacker, go-performance-advisor, gocene-lucene-specialist, frontend-design.

## 3. TECHNICAL EVIDENCE AND DETAILED DIAGNOSIS

This section grounds the tasks listed above with evidence extracted from the code.

### ID: [SKILL]-001 (HIGH)

- **Location**: `src/api/auth.ts:45`
- **Problem**: Missing JWT token validation in route middleware.
- **Impact**: Possible authentication bypass in production environments.
- **Solution Suggestion**: Implement `jsonwebtoken` library and verify signature in request header.

### ID: [SKILL]-002 (MEDIUM)

- **Location**: `src/components/DataList.tsx:112`
- **Problem**: Render loop does not use memoization, causing re-renders on every global state change.
- **Impact**: UI performance degradation in lists with more than 100 items.
- **Solution Suggestion**: Apply `useMemo` in filtering calculations and `React.memo` in row component.

### ID: [SKILL]-003 (LOW)

- **Location**: `README.md`
- **Problem**: Installation instructions are outdated relative to the current Node.js version used in the project.
- **Impact**: Difficulty in onboarding new developers.
- **Solution Suggestion**: Update "Prerequisites" section to Node.js v20+.

## 4. SEVERITY CRITERIA APPLIED

- **HIGH**: System blocks, critical security vulnerabilities, or core functional failures.
- **MEDIUM**: Performance optimization needed, technical debt refactoring, or planned new features.
- **LOW**: Cosmetic improvements, technical documentation, or developer experience adjustments.