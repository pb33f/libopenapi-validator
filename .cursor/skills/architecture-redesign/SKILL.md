---
name: architecture-redesign
description: |
  Orchestrates the libopenapi-validator architecture redesign (10 phases). Use when
  the user wants to execute a phase of the redesign plan. Reads phase instructions from
  docs/plans/architecture_redesign.md and drives the refactor-planner, refactor-coder,
  refactor-reviewer, and refactor-tester agents through a repeatable loop.
---

# Architecture Redesign Orchestrator

This skill drives the 10-phase architecture redesign of libopenapi-validator. Each phase
follows the same repeatable loop. The phase-specific instructions live in the design doc
(progressive disclosure) — this skill defines the process, not the content.

## Context

- **Design doc**: [docs/plans/architecture_redesign.md](docs/plans/architecture_redesign.md) — read the relevant phase section before starting
- **Architecture review**: [docs/architecture-review.md](docs/architecture-review.md) — background on why the redesign is needed
- **Working directory**: /Users/zach.hamm/src/libopenapi-validator
- **Go module**: github.com/pb33f/libopenapi-validator

## Phase Index

Each phase has a section heading in the design doc. Read that section to get the goal,
files changed, work items, and commit message.

| Phase | Branch Name | Design Doc Section |
|-------|-------------|--------------------|
| 0 | `refactor/phase-0-baseline` | "Phase 0: Baseline and Safety Net" |
| 1 | `refactor/phase-1-path-matcher` | "Phase 1: pathMatcher Interface + Radix/Regex Matchers + Matcher Chain" |
| 2 | `refactor/phase-2-request-context` | "Phase 2: Define requestContext + buildRequestContext" |
| 3 | `refactor/phase-3-sync-path` | "Phase 3: Thread requestContext Through the Sync Path" |
| 4 | `refactor/phase-4-async-path` | "Phase 4: Thread requestContext Through the Async Path + Simplify Channels" |
| 5 | `refactor/phase-5-regex-params` | "Phase 5: Regex Matcher Extracts Path Params" |
| 6 | `refactor/phase-6-lazy-errors` | "Phase 6: Lazy Error Schema Resolution (WithLazyErrors)" |
| 7 | `refactor/phase-7-unify-body` | "Phase 7: Unify Request/Response Body Validation" |
| 8 | `refactor/phase-8-options-plumbing` | "Phase 8: Options Plumbing + Minor Optimizations" |
| 9 | `refactor/phase-9-final` | "Phase 9: Final Benchmarks + Documentation" |

## The Phase Loop

When the user asks to execute a phase, follow these 8 steps IN ORDER.

### Step 1: Read Instructions

Read the phase section from `docs/plans/architecture_redesign.md`. Extract:
- **Goal**: What this phase achieves
- **Files changed**: Which files to create/modify
- **Work items**: The numbered list of things to do
- **Commit message**: The italic commit message at the end of the phase

Also read the "Identified Issues" section for any issues tagged with this phase number —
those contain the rationale and specific fix descriptions.

### Step 2: Create Branch

```bash
git checkout -b <branch-name-from-phase-index>
git branch --show-current  # verify
```

If the branch already exists (resuming), just check it out:
```bash
git checkout <branch-name>
```

### Step 3: Plan — invoke `refactor-planner`

Use the `refactor-planner` agent (subagent_type: "generalPurpose"). Pass it:
- The phase instructions from step 1 (goal, files, work items)
- The "Identified Issues" entries for this phase (if any)
- The list of files likely involved

The planner returns a detailed implementation plan with file-by-file changes, ordering,
and risk areas.

**Review the plan yourself before proceeding.** If the plan misses files or has obvious
gaps, resume the planner with corrections.

### Step 4: Implement — invoke `refactor-coder`

Use the `refactor-coder` agent (subagent_type: "generalPurpose"). Pass it:
- The implementation plan from step 3
- If this is a retry: the reviewer/tester feedback that caused the retry

The coder makes the changes and runs `make all` to catch formatting/lint issues early.

### Step 5: Review — invoke `refactor-reviewer`

Use the `refactor-reviewer` agent (subagent_type: "generalPurpose", readonly: true). Pass it:
- The git diff: run `git diff` and include the output
- The original phase instructions (goal + work items)
- The implementation plan from step 3

The reviewer returns PASS or FAIL.

**If FAIL**: Resume the `refactor-coder` with the reviewer's specific issues. The coder
fixes them and you re-invoke the reviewer. **Maximum 3 review loops** — if still failing
after 3 rounds, present the issues to the user for judgment.

**If PASS**: Continue to step 6.

### Step 6: Test — invoke `refactor-tester`

Use the `refactor-tester` agent (subagent_type: "generalPurpose"). Pass it:
- The list of files changed (from the coder's report)
- A description of what changed

The tester runs `make all`, then `make test-short` (fast pass), then `make test` (race
detector, only if concurrency is involved), checks coverage, and adds tests if needed.

**If FAIL**: Resume the `refactor-coder` with the tester's specific failures. After
fixes, re-run the reviewer (step 5) then the tester (step 6). **Maximum 3 test loops** —
escalate to user after 3.

**If PASS**: Continue to step 7.

### Step 7: Benchmark (quick sanity check)

Run a quick benchmark sanity check — NOT the full statistical suite. This is just to
catch obvious regressions, not to produce publishable numbers.

```bash
make bench-fast   # count=1, ~1-2 minutes
```

**Do NOT run `make bench-compare` (count=5) or `make bench-baseline` during the iteration
loop.** Those take 5+ minutes each and are for establishing statistical baselines only.

Phase-specific guidance:
- **Phase 0**: Run `make bench-baseline` (one-time, establishes the starting point)
- **Phases 1-8**: Run `make bench-fast` and eyeball the numbers. If something looks
  obviously regressed (2x+ worse), investigate. Minor variance is expected with count=1.
- **Phase 9**: Run `make bench-compare` for the final statistical comparison.

### Step 8: Commit

```bash
git add -A
git commit -m "<commit message from phase instructions>"
```

Report to the user:
- Phase N complete
- Summary of what changed (files, key changes)
- Benchmark comparison (if applicable)
- Any issues encountered and how they were resolved

## Verification Protocol

These checks run as part of steps 5-7, but for reference:

```bash
make all          # gofumpt + import ordering + golangci-lint
make test-short   # fast tests (~30s)
make test         # full tests with race detector (~1-2 min, for concurrency changes)
make bench-fast   # quick benchmark sanity check (~1-2 min, count=1)
```

`make bench-compare` (count=5, ~5 min) is only for Phase 0 (baseline) and Phase 9 (final).

## Special Phase Notes

### Phase 0 (Baseline)

Phase 0 is different — it doesn't involve code refactoring. It creates the Makefile and
captures baseline benchmarks. The planner/reviewer/tester loop is lighter here:
- The planner designs the Makefile targets
- The coder creates the Makefile
- The tester verifies that `make test` and `make bench-fast` work
- Save baseline results

### Phase 9 (Final)

Phase 9 is also different — it's documentation and benchmarks, not code changes:
- Run final fast-suite and production benchmarks
- Generate benchstat comparison against phase 0 baseline
- Update docs/architecture-review.md with results
- Write PR summary

## Error Recovery

If a phase gets stuck (3 review or test loops exhausted):
1. Present all issues to the user
2. Ask for guidance: fix manually, skip the issue, or abort the phase
3. If the user provides a fix direction, resume the coder with that guidance

If a benchmark regression is detected:
1. Report the specific benchmark and the magnitude of regression
2. Check if the regression is in the expected area (e.g., a structural change that adds
   a small overhead now but enables larger savings in a later phase)
3. Ask the user whether to proceed or investigate
