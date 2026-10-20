# Outcome-First Maintenance Policy Plan (Issue #38)

**Goal:** Commit the policy from [issue #38](https://github.com/ventodeco/annota-gemini-hackathon-3/issues/38) to root `AGENTS.md`. It covers outcome-first maintenance, verification in proportion to risk, and a 30-minute total test-debugging budget per task. The goal is that maintenance sessions deliver the requested result and report bounded, honest verification, rather than spending hours on test loops.

**User story:** As the project owner, I want agents to deliver the requested outcome and report bounded verification so that I can assess progress without long unproductive test-debugging loops.

**Architecture:** Documentation only. Add one subsection to root `AGENTS.md` under "Agent workflow and skills", plus this plan. No application, CI, auth, or ops changes.

**Status:** Implemented under the owner's plan-then-execute authorization. Independent review covers the PR only.

## Global Constraints

- Branch: `backlog/issue-38`, reset to `origin/dev` at `3560f04` (includes #54 CORS and #56 Dependabot docs). The PR targets `dev`.
- Prefix shell commands with `rtk`. Markdown only.
- The root checkout has an uncommitted, owner-approved draft of this policy in `AGENTS.md`. It was read without modifying it, and its text is the source for this change. The root checkout, the issue #43 plan, and `docs/research/` stay untouched.
- Leave the `docs/github-workflow.md` "Dependency Updates (Dependabot)" section from #50/#56 unchanged.

## Evidence (2026-10-20)

| Issue #38 scope item | State on `dev` at `3560f04` |
|---|---|
| Outcome-first work, smallest clear change, proportionate checks | Partly covered. The DRY/KISS bullet says "smallest clear change"; no rule says verification should be proportionate. |
| Documentation-only changes use diff/content review | Missing |
| 30-minute total budget shared across failures, retries, waits, sessions, and agents, and not reset by changing command, failure, or worker | Missing |
| Stop early for missing keys, services, or tools; at the limit record command, failure, hypothesis, attempts, and next step in the issue | Missing |
| Continue independent work; only the user can extend the budget | Missing |
| Honest reporting: no weakened tests or hidden failures; implementation, passed, blocked, and live-provider-pending are reported separately; release blockers stay blockers | Missing |
| Small live-AI samples after keys arrive, with usage and latency evidence | Missing |

Conflict check: `web/AGENTS.md` and `backend/AGENTS.md` list test commands and test style but require no full-suite run or retry loop, so the new policy doesn't conflict with them. `web/AGENTS.md` already defers to the root workflow ("verification before completion"), which the new policy bounds without replacing.

## Scope

1. Add a `### Outcome-first delivery and bounded verification` subsection after the "Respect local ownership" bullet in root `AGENTS.md`. Use the owner's draft text with two clarifications for edge cases the issue lists:
   - **Budget unit:** "per task" means per GitHub issue. The budget is shared across that issue's pull requests and CI runs too, so a follow-up PR doesn't reset it.
   - **Unrelated baseline failures:** record them in the issue as pre-existing, with evidence, rather than widening the task to fix them or reporting them as passes.
2. Comment on issue #38 with the PR, the verification evidence, and the merge result.

Out of scope:
- The existing "Plan before changing code … wait for confirmation" bullet. Session-level instructions currently override it; changing it isn't part of #38.
- `docs/github-workflow.md`, the frontend and backend `AGENTS.md`, CI, and application code.

## Edge Cases (how the policy resolves them)

- **Multiple failing suites:** one shared budget across all failures and commands.
- **Repeated sessions, delegated agents, multiple PRs:** the same budget continues; it doesn't reset per session, agent, PR, or run.
- **Runtime or credentials unavailable:** stop early, record the blocker, and continue independent work.
- **Unrelated baseline failures:** record them as pre-existing with evidence; don't fix them silently or report them as passing.
- **Release-blocking regression at the limit:** it stays a release blocker. Record it and ask for direction; never mark it as passed.

## Acceptance Criteria

- [ ] Root `AGENTS.md` states every scope bullet from issue #38 unambiguously.
- [ ] Nothing in it lets agents claim unverified work as passing, or conflicts with `web/AGENTS.md` or `backend/AGENTS.md`.
- [ ] The diff is Markdown only: `AGENTS.md` plus this plan.
- [ ] `git diff --check` is clean.

## Verification

Documentation-only change: content review against the issue scope, plus `rtk git diff --check`. No application test suite, as the issue's Definition of Done states. Required CI checks still run on the PR and must pass before merge. The PR gets an independent GPT-6.1 Sol review.
