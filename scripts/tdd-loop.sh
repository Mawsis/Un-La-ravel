#!/usr/bin/env bash
#
# tdd-loop.sh — run the `tdd` skill on a list of GitHub issues, ONE AT A TIME,
# each in a FRESH headless Claude process (empty context), on its own branch,
# opening a PR per issue, waiting for CI, and MERGING TO MAIN before the next
# issue starts — so each slice branches off a main that already contains its
# prerequisites.
#
# Context is cleared between issues by construction: every issue gets a brand-new
# `claude -p` invocation — no --continue/--resume.
#
# Merge gate (per issue): push → open PR → wait for the repo's GitHub Actions CI
# (.github/workflows/ci.yml: gofmt, vet, build, go test -race -cover) → merge on
# GREEN. On RED, the loop STOPS with the branch + PR intact for you to inspect —
# a broken slice must never poison main for the slices built on top of it.
#
# Special case: issue #21 changes the versioned contract and regenerates a
# golden. Per CLAUDE.md ("never regenerate goldens blind"), the loop opens its
# PR, waits for CI, and then STOPS for you to review the golden diff and merge
# manually. Set GOLDEN_REVIEW_ISSUES to change which issues get this treatment.
#
# Billing: `claude -p` draws from your Claude subscription (Pro/Max) usage, same
# as interactive Claude Code, AS LONG AS ANTHROPIC_API_KEY is unset. If it's set,
# -p bills as metered API — this script refuses to run in that case.
#
# Usage:
#   scripts/tdd-loop.sh 20 22 24 23 25 27 28 29     # PR 1 slices, dependency order
#   scripts/tdd-loop.sh 21 26 30                     # PR 2 slices
#   DRY_RUN=1 scripts/tdd-loop.sh 20 22              # print, don't execute
#
# Env knobs:
#   MODEL                 model for each run (default: opus)
#   PERM_MODE             acceptEdits (default) | plan | bypassPermissions
#   BASE_BRANCH           branch to cut each issue from and merge into (default: main)
#   MERGE_METHOD          merge (default) | squash | rebase
#   CI_TIMEOUT            seconds to wait for CI per slice (default: 1800)
#   GOLDEN_REVIEW_ISSUES  space-separated issues that pause for human merge (default: "21")
#   DRY_RUN=1             show what would run without invoking claude/git/gh

set -euo pipefail

# ---- guardrails -------------------------------------------------------------

if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
  echo "✋ ANTHROPIC_API_KEY is set — 'claude -p' would bill as metered API, not your subscription."
  echo "   Unset it (unset ANTHROPIC_API_KEY) for subscription billing, then re-run."
  exit 1
fi

if [ "$#" -eq 0 ]; then
  echo "usage: $0 <issue-number> [issue-number ...]"
  echo "   e.g. $0 20 22 24 23 25 27 28 29"
  exit 1
fi

command -v claude >/dev/null || { echo "claude CLI not found on PATH"; exit 1; }
command -v gh     >/dev/null || { echo "gh CLI not found on PATH"; exit 1; }
gh auth status   >/dev/null 2>&1 || { echo "gh not authenticated (run: gh auth login)"; exit 1; }

MODEL="${MODEL:-opus}"
PERM_MODE="${PERM_MODE:-acceptEdits}"
BASE_BRANCH="${BASE_BRANCH:-main}"
MERGE_METHOD="${MERGE_METHOD:-merge}"
CI_TIMEOUT="${CI_TIMEOUT:-1800}"
GOLDEN_REVIEW_ISSUES="${GOLDEN_REVIEW_ISSUES:-21}"
DRY_RUN="${DRY_RUN:-0}"

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

if [ -n "$(git status --porcelain)" ]; then
  echo "✋ Working tree is dirty. Commit, stash, or clean it before looping."
  exit 1
fi

slugify() { echo "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+|-+$//g' | cut -c1-40; }

is_golden_review_issue() {
  local n="$1"
  for g in $GOLDEN_REVIEW_ISSUES; do [ "$g" = "$n" ] && return 0; done
  return 1
}

run() { echo "  \$ $*"; [ "$DRY_RUN" = "1" ] || "$@"; }

# stop_loop prints a clear reason and exits non-zero so an unattended run halts
# instead of cascading a bad state into every later slice.
stop_loop() {
  echo ""
  echo "🛑 STOPPING loop: $1"
  echo "   Branch and PR for this issue are left intact for you to inspect."
  echo "   Fix, merge manually if appropriate, then re-run the loop with the"
  echo "   REMAINING issues (the merged ones are already on $BASE_BRANCH)."
  exit 1
}

echo "════════════════════════════════════════════════════════════════"
echo " tdd-loop | model=$MODEL perm=$PERM_MODE base=$BASE_BRANCH merge=$MERGE_METHOD dry=$DRY_RUN"
echo " issues: $*"
echo " golden-review (pause for manual merge): $GOLDEN_REVIEW_ISSUES"
echo "════════════════════════════════════════════════════════════════"

for N in "$@"; do
  echo ""
  echo "────────────────────────────────────────────────────────────────"
  echo " Issue #$N"
  echo "────────────────────────────────────────────────────────────────"

  TITLE="$(gh issue view "$N" --json title --jq .title 2>/dev/null || true)"
  if [ -z "$TITLE" ]; then
    stop_loop "could not fetch issue #$N (deleted? wrong number?)"
  fi
  BRANCH="issue-$N-$(slugify "$TITLE")"
  echo "  title : $TITLE"
  echo "  branch: $BRANCH"

  # Fresh base for this issue — includes everything merged by prior iterations.
  run git checkout "$BASE_BRANCH"
  run git pull --ff-only
  run git checkout -b "$BRANCH"

  PROMPT="Use the tdd skill to implement GitHub issue #$N in this repository.

First run: gh issue view $N  — and read its full body and acceptance criteria.
Follow the repository's CLAUDE.md conventions exactly (determinism, non-nil empty
slices, versioned contract, parser isolation, golden-file discipline, vertical
slices). Implement ONLY the scope of issue #$N — nothing from other issues.

Work test-first: write failing tests for each acceptance criterion, then the
minimal implementation to pass, then refactor. When done, ensure the full suite
passes (go build ./... && go test ./... -race -cover) and that gofmt -l cmd/
internal/ is empty and go vet ./... is clean — these are exactly what CI checks.

Do NOT merge anything. Do NOT touch other issues. Stop when the acceptance
criteria are met and all checks are green."

  echo "  → launching fresh headless Claude (empty context) for #$N"
  if [ "$DRY_RUN" = "1" ]; then
    echo "  \$ claude -p <prompt> --model $MODEL --permission-mode $PERM_MODE"
  else
    # No --continue / --resume: guarantees context is cleared between issues.
    claude -p "$PROMPT" --model "$MODEL" --permission-mode "$PERM_MODE"
  fi

  # If nothing changed, the run produced no work — stop rather than open an
  # empty PR or silently skip a dependency.
  if [ "$DRY_RUN" != "1" ] && [ -z "$(git status --porcelain)" ] && git diff --quiet "$BASE_BRANCH"..HEAD; then
    stop_loop "issue #$N produced no changes — nothing to merge"
  fi

  run git add -A
  # commit may be a no-op if the run already committed; tolerate that.
  if [ "$DRY_RUN" = "1" ]; then
    echo "  \$ git commit -m 'feat: implement #$N — $TITLE'"
  else
    git commit -m "feat: implement #$N — $TITLE

Closes #$N" || echo "  (nothing new to commit — run already committed)"
  fi
  run git push -u origin "$BRANCH"
  run gh pr create --fill --base "$BASE_BRANCH" --head "$BRANCH"

  # ---- CI gate ------------------------------------------------------------
  echo "  ⏳ waiting for CI on PR for $BRANCH (timeout ${CI_TIMEOUT}s)…"
  if [ "$DRY_RUN" = "1" ]; then
    echo "  \$ gh pr checks $BRANCH --watch --fail-fast"
  else
    if ! timeout "$CI_TIMEOUT" gh pr checks "$BRANCH" --watch --fail-fast; then
      stop_loop "CI failed (or timed out) for #$N — not merging; main stays clean"
    fi
    echo "  ✓ CI green for #$N"
  fi

  # ---- merge gate ---------------------------------------------------------
  if is_golden_review_issue "$N"; then
    echo ""
    echo "  ✋ Issue #$N is a golden/contract change — PAUSING for human review."
    echo "     CI is green, but per CLAUDE.md the golden diff must be reviewed"
    echo "     before merge. Review the PR, merge it into $BASE_BRANCH manually,"
    echo "     then re-run the loop with the issues that come AFTER #$N."
    if [ "$DRY_RUN" != "1" ]; then run git checkout "$BASE_BRANCH"; fi
    echo "  (loop stops here by design for #$N)"
    exit 0
  fi

  echo "  → merging #$N into $BASE_BRANCH"
  run gh pr merge "$BRANCH" --"$MERGE_METHOD" --delete-branch
  run git checkout "$BASE_BRANCH"
  run git pull --ff-only
  echo "  ✓ #$N merged into $BASE_BRANCH — next slice will branch from it"
done

echo ""
echo "════════════════════════════════════════════════════════════════"
echo " done. all requested slices merged into $BASE_BRANCH."
echo " open PRs (if any paused for review):  gh pr list"
echo "════════════════════════════════════════════════════════════════"
