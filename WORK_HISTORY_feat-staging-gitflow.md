# Work History: Queued Acceptance Tests Implementation

**Branch:** `feat/staging-gitflow`
**Started:** 2026-01-26
**Status:** Implementation Complete

## Context

**Original Goal:** Implement a staging branch gitflow with auto-merge and revert on failure.

**Final Decision:** After discussion, we chose a simpler approach - run acceptance tests on PRs with concurrency queue. This avoids the complexity of staging branch, auto-revert, and re-approval loops.

**Key Constraint:** Ory workspace only allows 2 production projects. Parallel acceptance test runs would fail due to slot exhaustion.

---

## Current State

### Repository Info
- **Repo:** `ory/terraform-provider-orynetwork`
- **Default branch:** `main`
- **Visibility:** Private (will become public)
- **Branch protection:** None currently configured

### Current Workflows
1. **unit-test.yml** - Runs on PRs to main, push to main
2. **acceptance-test.yml** - Runs on push to main only (not on PRs - this is the expensive test)
3. **release.yml** - Runs on tags (v*)

### Problem
- Acceptance tests only run AFTER merge to main
- No gate before merge - broken code can land on main
- Tests are expensive, don't want to run on every PR update

---

## Research Findings

### 1. GitHub Default PR Target Branch

**Question:** Can GitHub automatically target PRs to `staging` instead of `main`?

**Answer:** Yes, via repository settings:
- Settings → General → Default branch → Set to `staging`
- This makes `staging` the default target for new PRs

**Caveat:** This changes the "default" branch for the entire repo:
- README displays from default branch
- Clone/fork operations target default branch
- May confuse users expecting `main`

**Alternative:** Keep `main` as default, use branch naming convention and documentation to guide contributors to target `staging`.

### 2. Auto-merge Staging to Main

**Options:**
1. **GitHub Actions workflow** - On successful tests, create and merge PR from staging→main
2. **GitHub merge queue** - Built-in feature but designed for PRs, not branches
3. **Direct push** - After tests pass, push staging to main

**Recommendation:** Use GitHub Actions to create a PR from staging→main and auto-merge it. This provides:
- Audit trail
- Can still have branch protection on main
- Visible in PR history

### 3. Auto-revert on Test Failure

**Options:**
1. **Git revert commit** - Create a revert commit on staging
2. **Force push** - Reset staging to pre-merge state (dangerous for others)
3. **Revert PR** - Create a PR that reverts the changes

**Challenges:**
- Identifying which commit(s) to revert
- Handling merge commits vs squash commits
- Reopening the original PR is NOT directly supported by GitHub API

**Recommendation:**
- Use squash merge for staging PRs (simpler to revert)
- On failure: Create a revert PR automatically
- Create a new issue or comment linking to the failed run (can't reopen closed PR via API)

---

## Professional DevOps Assessment

### Is This a Good Workflow?

**Pros:**
1. **Cost control** - Expensive acceptance tests only run after human review
2. **Quality gate** - Tests must pass before reaching main
3. **Clean main branch** - Main is always in a passing state
4. **Familiar pattern** - Similar to GitFlow which many devs know

**Cons:**
1. **Complexity** - More moving parts than simple trunk-based development
2. **Merge conflicts** - Staging can diverge from main if multiple PRs queue
3. **Contributor confusion** - External contributors used to targeting main/master
4. **Delayed feedback** - Contributors don't know if tests pass until after merge to staging

### Alternative Approaches

#### Option A: Simple Required Checks (Recommended for OSS)
- Keep single `main` branch
- Run acceptance tests on every PR (with caching/optimization)
- Require tests to pass before merge
- Simpler for contributors

**Why this might be better:**
- Terraform acceptance tests can be parallelized
- Tests run on contributor's PR, they see results before merge
- No complex staging→main synchronization
- Standard GitHub flow that contributors understand

#### Option B: Merge Queue (GitHub Native)
- Use GitHub's merge queue feature
- PRs queue and tests run before actual merge
- Native GitHub feature, less custom automation

#### Option C: Your Proposed Staging Flow
- Good for controlling costs
- Works well if acceptance tests are truly expensive and can't be optimized
- Requires documentation and contributor education

### Recommendation

For an **open source terraform provider**, I'd suggest:

1. **If tests can be optimized** → Option A (simple required checks)
2. **If tests MUST be expensive** → Option C with clear CONTRIBUTING.md docs

Given that you want the staging flow, let's implement it properly.

---

## Implementation Plan

### Phase 1: Branch Setup
1. Create `staging` branch from `main`
2. Configure branch protection rules:
   - `main`: Require PR from staging, require status checks
   - `staging`: Require PR approval, no direct push

### Phase 2: Workflow Updates
1. **unit-test.yml** - Run on PRs to staging AND main
2. **acceptance-test.yml** - Run on push to staging only
3. **staging-to-main.yml** (new) - Auto-promote staging→main on success
4. **revert-on-failure.yml** (new) - Revert staging and notify on failure

### Phase 3: Documentation
1. Update CONTRIBUTING.md with new flow
2. Add PR template targeting staging
3. Add branch naming guidelines

---

## Timeline

### 2026-01-26
- Created branch `feat/staging-gitflow`
- Analyzed current workflows
- Researched GitHub capabilities
- Documented professional assessment
- Starting implementation...

---

## Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `.github/workflows/unit-test.yml` | Modify | Target staging branch |
| `.github/workflows/acceptance-test.yml` | Modify | Run on staging push |
| `.github/workflows/staging-to-main.yml` | Create | Auto-promote on success |
| `.github/workflows/revert-on-failure.yml` | Create | Handle test failures |
| `CONTRIBUTING.md` | Create/Modify | Document workflow |
| `.github/PULL_REQUEST_TEMPLATE.md` | Modify | Update for staging flow |

---

## Critical Constraint: Project Slot Limit

**Discovery:** The Ory workspace only allows **2 production projects**. The acceptance tests:
1. Create a project at start (`run-acceptance-tests.sh` line 102)
2. Run all tests using that project
3. Clean up the project on exit (trap cleanup)

**Impact:** If multiple acceptance test runs execute in parallel, they will compete for project slots and fail.

**Solution Options:**

### Option 1: GitHub Concurrency Controls (Simple)

Use GitHub Actions' built-in `concurrency` feature:

```yaml
concurrency:
  group: acceptance-tests
  cancel-in-progress: false  # Queue instead of cancel
```

This ensures only ONE acceptance test runs at a time. Others queue.

**Pros:**
- Simple, native GitHub feature
- No staging branch needed
- Tests queue automatically

**Cons:**
- Queued PRs wait for earlier ones to complete
- Long wait times if many PRs

### Option 2: Staging Branch + Concurrency (Your Proposed Flow)

Combine staging branch with concurrency:
1. PRs target `staging`
2. On merge to staging, run acceptance tests with concurrency lock
3. Only one test runs at a time
4. On success, auto-merge to main
5. On failure, revert

**Pros:**
- Tests only run after review (less queue congestion)
- Clear separation of tested vs untested code

**Cons:**
- More complex
- Contributors need education

### Option 3: GitHub Merge Queue (Best of Both Worlds)

GitHub's native merge queue with concurrency:
1. PRs target `main` (familiar flow)
2. Require merge queue for main branch
3. Queue processes PRs one at a time
4. Tests run in queue before merge
5. Failed tests = PR rejected, stays open

**Pros:**
- Native GitHub feature, well-supported
- Contributors use standard flow (PR to main)
- Automatic queuing
- Failed PRs stay open (no revert needed)

**Cons:**
- Requires GitHub Team/Enterprise for private repos
- Queue can get long

---

## Revised Recommendation

Given the **2-project slot limit**, I now recommend:

**Option 3 (Merge Queue)** if you have GitHub Team/Enterprise
**Option 2 (Staging + Concurrency)** if you want the manual staging gate

Both need the `concurrency` key to prevent parallel runs.

---

## Final Implementation: Queued PR Tests

After discussing the trade-offs, we chose **Option 1: Concurrency Queue on PRs**.

### Why This Approach?

1. **Standard GitHub flow** - PRs target `main`, no staging branch confusion
2. **Tests before merge** - Contributors see results before approval
3. **No revert loops** - Failed tests = fix and push, no re-approval needed
4. **Resource protection** - Only one acceptance test runs at a time

### Flow Diagram

```
PR opened to main
       ↓
┌──────────────────────────────┐
│ Unit tests (parallel, fast)  │
└──────────────────────────────┘
       ↓
┌──────────────────────────────┐
│ Acceptance tests             │
│ ┌──────────────────────────┐ │
│ │ QUEUE (one at a time)    │ │
│ │ - PR #1 running...       │ │
│ │ - PR #2 waiting...       │ │
│ │ - PR #3 waiting...       │ │
│ └──────────────────────────┘ │
└──────────────────────────────┘
       ↓
   pass → mergeable
   fail → contributor fixes, re-queues
       ↓
Merge to main
       ↓
Tag for release (v1.x.x)
```

### Changes Made

**`.github/workflows/acceptance-test.yml`:**
1. Added `pull_request` trigger for `main` branch
2. Added `concurrency` block to queue tests:
   ```yaml
   concurrency:
     group: acceptance-tests
     cancel-in-progress: false
   ```
3. Updated test enable flags to include `pull_request` event

### Branch Protection (Recommended)

Configure in GitHub Settings → Branches → Add rule for `main`:
- Require status checks: `Acceptance Tests`, `Unit Tests`
- Require branches to be up to date before merging

---

## Timeline

### 2026-01-26
- Created branch `feat/staging-gitflow`
- Researched staging branch approach
- Discovered issues:
  - PRs auto-target default branch (would need to change default to staging)
  - Can't reopen merged PRs via API
  - Revert + re-approval loop is painful for contributors
- Pivoted to simpler queued PR approach
- Implemented concurrency queue in acceptance-test.yml

---

## Files Modified

| File | Change |
|------|--------|
| `.github/workflows/acceptance-test.yml` | Added PR trigger + concurrency queue |

---

## Next Steps

1. Push branch and create PR
2. Configure branch protection rules on `main`
3. Test the queue behavior with multiple PRs
