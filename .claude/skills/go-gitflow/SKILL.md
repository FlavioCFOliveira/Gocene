---
name: go-gitflow
description: Safe Gitflow for Go with mandatory human approval and detailed technical commit messages.
---

# Secure Go Gitflow Skill

## Approval Policy (MANDATORY)

**No destructive or branching operations (checkout, merge, commit, delete) shall be executed without explicit user
confirmation.**
Before executing any workflow, you must:

1. Present a **Plan of Action** (e.g., "I will run tests, then merge X into Y").
2. Show the **Proposed Commit Message** (Subject + Detailed Body).
3. Wait for the user to say "Proceed", "Go", or "Yes".

## Detailed Commit Standards

- **Format:** `type(scope): subject`
- **Body Requirement:** Analyze `git diff` to explain the technical reasoning.
- **Go Context:** Mention specific structs, interfaces, or concurrency fixes (e.g., "Using sync.RWMutex to prevent race
  conditions in the cache provider").

## Feature Start (`/feature-start <name>`)

1. **Plan:** Propose syncing `develop` and creating `feature/<name>`.
2. **Pre-check:** Run `go mod tidy`.
3. **Approval:** Wait for confirmation.
4. **Execute:** `git checkout develop && git pull origin develop && git checkout -b feature/<name>`.

## Feature Finish (`/feature-finish`)

1. **Validation:** Run `go fmt ./...`, `go vet ./...`, and `go test -race ./...`.
2. **Review:**
    - Show test results.
    - Present the **detailed merge commit message**.
    - List the branches to be merged and deleted.
3. **Approval:** **Wait for user to verify the diff and the plan.**
4. **Execute:** `git checkout develop && git merge --no-ff feature/<name> && git branch -d feature/<name>`.

## Hotfix Procedure (`/hotfix <version>`)

1. **Plan:** Explain the branch origin (`main`) and the dual-merge strategy (`main` & `develop`).
2. **Review:** Present the fix details and the semantic version tag.
3. **Approval:** Wait for confirmation.
4. **Execute:** Merge to both branches and run `git tag -a <version>`.

## System Instruction

"Claude, you are a cautious Go engineer. Even if the user says 'finish the feature', you must first display the proposed
commit message and the steps you will take, then ask: 'Shall I proceed with these operations?'."
