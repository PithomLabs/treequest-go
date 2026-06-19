# Implementation Plan: TREEQUEST-REPAIR-0001 (Adversarial Review Repairs)

This plan details the repairs required to address critical correctness, concurrency, and validation issues raised during the adversarial review.

## User Review Required

> [!IMPORTANT]
> The promotion status is downgraded to `reject_for_now_pending_TREEQUEST_REPAIR_0001`.
> No code acceptance or EBP promotion claims will be made until these repairs and their minimum acceptance tests are fully completed and validated.

---

## Proposed Changes

### Component 1: `treequest-go` (Generic Search Library)

#### [MODIFY] [node.go](file:///home/chaschel/Documents/go/treequest/pkg/tree/node.go)
- Move `Reward` and `StateScore[S]` structures from the `algo` package to the `tree` package to prevent circular package imports when adding `TopK` to the `SearchTree`.

#### [MODIFY] [tree.go](file:///home/chaschel/Documents/go/treequest/pkg/tree/tree.go)
- Add `TopK(k int) []StateScore[S]` to `SearchTree[S]` that returns the top $k$ highest-scoring non-root nodes.
- Update `BestNode()` to break ties deterministically using `ExpandIdx` to ensure stable selections.

#### [MODIFY] [types.go](file:///home/chaschel/Documents/go/treequest/pkg/algo/types.go)
- Remove `Reward` and `StateScore[S]` declarations (moved to `tree`).
- Add a `sync.RWMutex` to `TrialStore[S]` to protect `Pending` and `Finished` map mutations.
- Expose safe thread-safe getters, setters, and deletion helpers on `TrialStore[S]`.
- Add a thread-safe `Clone()` and custom JSON serialization support if needed.

#### [MODIFY] [bandit.go](file:///home/chaschel/Documents/go/treequest/pkg/algo/bandit.go)
- Update `NewBetaSampler` to use a SplitMix64-based mixer to derive the second PCG seed component, fully utilizing the seed space.
- Stabilize the `ChooseAction` utility's map iteration order by sorting action labels before selection.

#### [MODIFY] [abmcts_a.go](file:///home/chaschel/Documents/go/treequest/pkg/algo/abmcts_a.go)
- Add a `sync.RWMutex` to `ABMCTSAAlgoState[S]` to protect `AllRewards` reads and writes.
- Update `selectAction` signature to take `*ABMCTSAAlgoState[S]` and read `AllRewards` thread-safely.
- Implement `SaveAlgoSnapshot` and `RestoreAlgoSnapshot` to snapshot and restore the entire algorithm state (including the pending/finished trials in `TrialStore` and global rewards) using a custom `AlgoStateSnapshot` record structure.

---

### Component 2: `ebp-paper-evaluator` (Downstream Application)

#### [MODIFY] [validators.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/ebp/validators.go)
- Add a deterministic `ValidateDebtConsistency` function checking that `RemainingDebt` and `RetiredDebt` are disjoint and their union equals the canonical six EBP debt checklist items. Set `PromotionReady = false` and append a `debt_inconsistency` flag if this check fails.
- Update `RunDeterministicValidators` to scan constructive analyses, adversarial critiques, and evaluator reasoning text fields for final-truth overclaims.

#### [MODIFY] [evaluator.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/ebp/evaluator.go)
- Remove score clamping in `RunEvaluator` and return an error if the score is out of range $[0, 1]$, `NaN`, or `Inf`.
- Implement a robust balanced-brace search helper (`extractJSON`) in `parseJSONResponse` to prevent greedy regex failures.

#### [MODIFY] [budget.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/ebp/budget.go)
- Track prompt, completion, and total tokens separately for workers vs. evaluators. Add `WorkerPromptTokens`, `WorkerCompletionTokens`, `EvaluatorPromptTokens`, and `EvaluatorCompletionTokens` to `BudgetTracker`.

#### [MODIFY] [bundle.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/ebp/bundle.go)
- Replace `PROMOTED_RESEARCH_ARTIFACT` with candidate-oriented EBP-safe audit language (`EBP_VERIFIED_WITHOUT_DEBT`).
- Save declared budgets (`MaxTurnsDeclared`, `MaxIterationsDeclared`) in `provenance.json` and `budget_usage.json`.

#### [MODIFY] [main.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/cmd/evaluate/main.go)
- Log and pass configured declared budgets to `BudgetTracker` and `ProvenanceLog`.
- Ensure `RunDeterministicValidators` runs *after* `RunEvaluator` compiles scores and debt lists.

---

## Verification Plan

### Automated Tests

#### `treequest-go` new unit tests:
- `TestTrialStore_ConcurrentTell`: Spawns multiple goroutines calling `Tell` on distinct trials to verify concurrency safety (`go test -race`).
- `TestTell_UnknownTrialID`: Asserts that `Tell` rejects unknown trial IDs with `ErrUnknownTrial`.
- `TestTell_OutOfOrder`: Evaluates concurrent batch trial execution where out-of-order `Tell` calls occur.
- `TestTell_DuplicateDifferentHash`: Verifies that duplicate `Tell` calls with mismatching state hashes fail with `ErrResultMismatch`.
- `TestSnapshot_RoundTripWithPendingTrials`: Verifies serializing and restoring an active search state containing pending/finished trials.
- `TestSelectAction_DeterministicTiebreak`: Asserts deterministic tiebreaks in bandit selection.
- `TestTopK`: Verifies `SearchTree.TopK` returns top nodes correctly sorted.

#### `ebp-paper-evaluator` new unit tests:
- `TestRunEvaluator_RejectsOutOfRangeScore`: Asserts score bounds are validated.
- `TestRunEvaluator_RejectsNaNScore`: Verifies NaN score values fail gracefully.
- `TestRunDeterministicValidators_DebtConsistency`: Checks that overlap or missing checklist items are flagged.
- `TestRunDeterministicValidators_AllSixDebtItems`: Confirms promotion is rejected if debt lists do not cover the 6 canonical checklist items.
- `TestBudgetTracker_SeparateTokenAccounting`: Asserts correct token attribution logic.
- `TestSaveArtifactBundle_NoAPIKey`: Verifies the final audit bundle contains no API key fragments.
- `TestMockMode_NoRealProviderCall`: Verifies that no network client instantiations or requests occur when mock is active.
- `TestGenerateA_And_GenerateB_Attribution`: Checks budget tracking call allocation.

---

## Status language to use:
```text
Implementation completed but rejected-for-now by adversarial review.
Core architecture remains accepted.
Current code requires blocker repairs before acceptance.
No source-faithful TreeQuest parity claim.
No EBP promotion claim.
```


# Tasks: TREEQUEST-REPAIR-0001 (Adversarial Review Repairs)

- `[ ]` treequest-go: Move Reward and StateScore[S] to node.go (`pkg/tree/node.go` and `pkg/algo/types.go`)
- `[ ]` treequest-go: Stabilize SearchTree.BestNode and add TopK (`pkg/tree/tree.go`)
- `[ ]` treequest-go: Add mutex protection to TrialStore (`pkg/algo/types.go`)
- `[ ]` treequest-go: Stabilize NewBetaSampler seed mixer and ChooseAction map iteration (`pkg/algo/bandit.go`)
- `[ ]` treequest-go: Add mutex protection and snapshot/restore functions to ABMCTSAAlgoState (`pkg/algo/abmcts_a.go`)
- `[ ]` ebp-paper-evaluator: Implement deterministic ValidateDebtConsistency check (`pkg/ebp/validators.go`)
- `[ ]` ebp-paper-evaluator: Implement score validation (no clamping, reject out-of-range/NaN/Inf) and balanced brace extractor (`pkg/ebp/evaluator.go`)
- `[ ]` ebp-paper-evaluator: Track separate worker vs evaluator prompt/completion tokens (`pkg/ebp/budget.go`)
- `[ ]` ebp-paper-evaluator: Persist declared budgets and replace overclaiming promotion string (`pkg/ebp/bundle.go`, `cmd/evaluate/main.go`)
- `[ ]` treequest-go: Write and execute minimum acceptance unit tests (`pkg/algo/abmcts_a_test.go`)
- `[ ]` ebp-paper-evaluator: Write and execute minimum acceptance unit tests (`pkg/ebp/...`, `pkg/llm/...`)
- `[ ]` Verification: Run go test ./... and go test -race ./... inside both directories
- `[ ]` Verification: Run evaluator mock CLI and inspect bundle output files
