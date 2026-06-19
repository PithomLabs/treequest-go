## Review verdict

`plan2.md` is now much stronger. I would mark it:

```text
accept_with_minor_repairs
```

The major architectural repairs from the previous review were addressed: neutral core types were added, “model” terminology was mostly removed, `context.Context` was added to the `Algorithm` interface, trial statuses were improved, `Reward` and `ResultHash` were added, snapshots now use flat `NodeRecord` plus `StateCodec[S]`, `ExpandIdx` was added, source-faithfulness caveats were added, and promotion language was corrected. 

## What is now accepted

The following are solid enough to proceed as the implementation baseline:

```text
treequest-go = generic core only
ebp-paper-evaluator = separate downstream app
AB-MCTS-A first
AB-MCTS-M deferred
OpenRouter only in app layer
Ask/Tell primary
Step only convenience
TrialStore first-class
Tell idempotent and order-independent
Scores normalized to [0,1]
StateCodec[S] for snapshots
Flat node records for persistence
Python trace parity before source-faithfulness claims
```

The OpenRouter isolation boundary is also correctly stated: `github.com/revrost/go-openrouter` can be used in the app adapter, while `treequest-go` remains network-free and provider-free. 

## Minor repairs still needed

### 1. Remove the last “model” wording in AB-MCTS-A section

This line still says:

```text
for each available action/model
```

Change to:

```text
for each available action
```

Small, but worth cleaning to preserve core neutrality.

### 2. Clarify `Running` vs `Pending` in `TrialStore`

The type now has:

```go
type TrialStore[S any] struct {
    Running  map[tree.TrialID]Trial[S] `json:"running"`
    Finished map[tree.TrialID]Trial[S] `json:"finished"`
}
```

But the lifecycle says `AskBatch` registers trials as `TrialPending`. Rename `Running` to `Pending`:

```go
type TrialStore[S any] struct {
    Pending  map[tree.TrialID]Trial[S] `json:"pending"`
    Finished map[tree.TrialID]Trial[S] `json:"finished"`
}
```

This avoids mismatch between type names and status names.

### 3. Add `Ask`, not only `AskBatch`

The prose says `AskBatch / Ask`, but the interface only shows `AskBatch`. Add the single-trial method explicitly:

```go
Ask(
    ctx context.Context,
    state AS,
    actions []tree.ActionLabel,
) (AS, Trial[S], error)
```

It can be implemented as `AskBatch(..., 1)` internally, but it should exist in the public interface or wrapper.

### 4. State that `ResultHash` may be computed by the app

Right now `StateScore` requires:

```go
ResultHash string
```

Good, but clarify who computes it. Recommended wording:

```text
If ResultHash is empty, the core derives it using the configured StateCodec[S] plus normalized score. If the application supplies ResultHash, the core verifies duplicate Tell calls against that hash.
```

This prevents every caller from having to implement hashing manually.

### 5. Strengthen EBP “complete state” invariant

The EBP flow still says Worker A updates `ConstructiveAnalysis` and Worker B updates `AdversarialReview`. That is fine, but add the invariant explicitly:

```text
Both actions must return a complete ClaimEvalState. A patch-only critique or partial worker output must be applied to the parent state before evaluator E scores the candidate and before TreeQuest receives Tell.
```

This was one of the strongest Codex recommendations and should be visible in the plan.

### 6. Add `budget_usage.json` to artifact bundle

The provenance file can include budget usage, but since budget accounting is central, I would add a separate file:

```text
budget_usage.json
```

So the bundle becomes:

```text
out/audit/
├── original_paper.pdf
├── assessment_report.md
├── claim_ledger.json
├── provenance.json
├── budget_usage.json
└── tree_snapshot.json
```

### 7. Acceptance criteria should not require “identical decisions” too early

This line is a little too strong:

```text
Parity test verifies identical decisions to Sakana AI's implementation.
```

Better:

```text
Parity fixtures verify identical decisions on controlled deterministic traces where sampler outputs, actions, and scores are fixed; broader source-faithfulness remains pending review.
```

This avoids overclaiming parity before all stochastic and implementation differences are understood.

## Revised status

```json
{
  "status": "accept_with_minor_repairs",
  "ready_for_implementation_after_repairs": true,
  "highest_priority_repairs": [
    "rename TrialStore.Running to Pending",
    "add explicit Ask method",
    "remove remaining action/model wording",
    "clarify ResultHash derivation",
    "state complete ClaimEvalState invariant",
    "soften parity acceptance wording"
  ],
  "core_architecture": "accepted",
  "ebp_app_boundary": "accepted",
  "openrouter_boundary": "accepted"
}
```

## EBP/PTW self-audit

**Claims:** `plan2.md` is mostly implementation-ready after minor wording/API repairs.

**needMap:** Satisfied. `treequest-go` and `ebp-paper-evaluator` boundaries are clear.

**needInvariant:** Mostly satisfied. Core neutrality, score bounds, app-layer OpenRouter, and AB-MCTS-A-first are preserved.

**needToyCheck:** Present. Add explicit duplicate `Tell`, out-of-order `Tell`, invalid score, snapshot/restore, and `S=string` fake-action tests.

**needNullModel:** Adequate for v0.1; v0.2 baselines remain planned.

**needObstruction:** Remaining obstructions are mostly implementation details: source parity, hash semantics, exact AB-MCTS-A update behavior, and OpenRouter adapter stability.

**needFaithfulnessReview:** Still required before claiming Sakana TreeQuest parity.

**Promotion status:** `implementation_plan_accept_with_minor_repairs`; not yet code-accepted or source-faithful until tests and parity fixtures pass.
