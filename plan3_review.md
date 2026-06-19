## Review verdict

`plan3.md` is now acceptable as the **final implementation plan baseline**.

I would mark it:

```text id="vof1r1"
accepted_for_implementation
```

The previous minor repairs were addressed: `TrialStore.Running` became `Pending`, `Ask` was added, the remaining action/model wording was cleaned, `ResultHash` derivation was clarified, the complete `ClaimEvalState` invariant was added, `budget_usage.json` was added, and parity wording was softened to controlled deterministic traces only. 

## What is accepted

The implementation plan now correctly locks:

```text id="64alwm"
treequest-go = generic, provider-free search core
ebp-paper-evaluator = separate LLM/EBP/OpenRouter app
AB-MCTS-A = v0.1 target
AB-MCTS-M = deferred
Ask / AskBatch / Tell = primary API
Step = convenience only
TrialStore = first-class
Tell = idempotent and order-independent
Score = normalized [0,1]
StateCodec[S] = required for generic snapshots
Flat NodeRecord snapshots = accepted
OpenRouter = app-layer adapter only
Artifact bundle = accepted output model
```

The task list at the end is also aligned with the plan and can be used as the first implementation checklist. 

## One remaining wording repair before coding

There is one small inconsistency in Section 22:

```text id="wbgner"
Promotion Status: implementation_plan_accept_with_minor_repairs
```

Since `plan3.md` now incorporated those repairs, change it to:

```text id="kw4r9w"
Promotion Status: implementation_plan_accepted_for_implementation;
not yet code-accepted or source-faithful until tests and parity fixtures pass.
```

That is the only textual repair I would require.

## Implementation gate

Proceed in this exact order:

```text id="d73m6g"
1. treequest-go core types
2. TrialStore + idempotent Tell
3. Beta sampler with injected randomness
4. SearchTree + flat snapshots + StateCodec[S]
5. AB-MCTS-A Ask / AskBatch / Tell
6. deterministic S=string toy tests
7. controlled Python parity fixtures
8. ebp-paper-evaluator mock LLM client
9. OpenRouter adapter
10. EBP validators, budget provenance, artifact bundle
```

Do **not** start the OpenRouter or EBP app until `treequest-go` passes deterministic toy tests.

## EBP/PTW self-audit

**Claims:** `plan3.md` is accepted as an implementation baseline, not as proof of source-faithful parity.

**needMap:** Satisfied. The generic search core and EBP/OpenRouter app boundaries are clear.

**needInvariant:** Satisfied. Core neutrality, score bounds, app-level budgets, and AB-MCTS-A-first are preserved.

**needToyCheck:** Pending implementation. Required first: `S=string`, fake actions, duplicate `Tell`, out-of-order `Tell`, invalid score rejection, snapshot/restore.

**needNullModel:** Planned. v0.2 baselines will provide Standard MCTS / Best-First / BFS comparisons.

**needObstruction:** Remaining risks are implementation bugs, exact upstream update-rule parity, beta sampler drift, and OpenRouter adapter instability.

**needFaithfulnessReview:** Still required before claiming Sakana TreeQuest compatibility.

**Promotion status:** `implementation_plan_accepted_for_implementation`; code remains unaccepted until tests and parity fixtures pass.
