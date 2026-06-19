The adversarial review is **high-signal** and should be accepted as a blocking review. I would immediately downgrade the prior status from:

```text id="ys2vkz"
code_accepted_for_current_mock_scope
```

to:

```text id="s6p2hz"
reject_for_now_pending_blocker_repairs
```

The review found four true blockers, not style issues: unsafe concurrent mutation in `TrialStore` / `AllRewards`, score laundering through evaluator-side clamping, nondeterministic action selection caused by Go map iteration, and missing deterministic debt-consistency validation. 

## Decision

```json id="wugkcu"
{
  "review_status": "reject_for_now",
  "previous_code_acceptance": "revoked_for_now",
  "reason": "silent correctness risks and EBP boundary violations",
  "next_action": "repair critical blockers before any further feature work"
}
```

## The repair ticket should be

```text id="y81t18"
TREEQUEST-REPAIR-0001: Concurrency, Score Validation, Determinism, and Debt-Consistency Repair
```

### Required scope

```text id="y1te4x"
1. Add mutex protection to ABMCTSAAlgoState / TrialStore / AllRewards.
2. Remove score clamping from RunEvaluator.
3. Return error on out-of-range, NaN, or Inf evaluator scores.
4. Stabilize selectAction by iterating over the supplied actions slice, not map order.
5. Add deterministic debt-consistency validator.
6. Replace PROMOTED_RESEARCH_ARTIFACT with EBP-safe audit language.
7. Persist declared budget alongside consumed budget.
8. Add tests listed by the adversarial review.
9. Re-run go test ./... and go test -race ./...
10. Re-run adversarial review after repairs.
```

## Minimum acceptance tests after repair

```text id="z54qmr"
treequest-go:
- TestTrialStore_ConcurrentTell
- TestTell_UnknownTrialID
- TestTell_OutOfOrder
- TestTell_DuplicateDifferentHash
- TestSnapshot_RoundTripWithPendingTrials
- TestSelectAction_DeterministicTiebreak
- TestTopK
- go test -race ./...

ebp-paper-evaluator:
- TestRunEvaluator_RejectsOutOfRangeScore
- TestRunEvaluator_RejectsNaNScore
- TestRunDeterministicValidators_DebtConsistency
- TestRunDeterministicValidators_AllSixDebtItems
- TestBudgetTracker_SeparateTokenAccounting
- TestSaveArtifactBundle_NoAPIKey
- TestMockMode_NoRealProviderCall
- TestGenerateA_And_GenerateB_Attribution
```

## Status language to use now

Do **not** say “code accepted” anymore. Use:

```text id="rqcdoq"
Implementation completed but rejected-for-now by adversarial review.
Core architecture remains accepted.
Current code requires blocker repairs before acceptance.
No source-faithful TreeQuest parity claim.
No EBP promotion claim.
```

## EBP/PTW self-audit

**Claims:** The adversarial review identifies genuine blockers and should override the earlier mock-scope acceptance.

**needMap:** Still structurally good: generic core and EBP app remain separated.

**needInvariant:** Failing: score-validation invariant and deterministic-action invariant are currently compromised.

**needToyCheck:** Insufficient: race, duplicate `Tell`, out-of-order `Tell`, and invalid-score tests must be added.

**needNullModel:** Still pending; not relevant until blockers are repaired.

**needObstruction:** Main blockers are concurrency unsafety, score laundering, nondeterminism, and LLM-only debt promotion logic.

**needFaithfulnessReview:** Still pending; source parity cannot be claimed.

**Promotion status:** `reject_for_now_pending_TREEQUEST_REPAIR_0001`.
