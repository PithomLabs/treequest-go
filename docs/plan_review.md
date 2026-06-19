## Review verdict

`plan.md` is **strong and close to implementable**, but I would mark it:

```text
accept_with_repairs
```

It correctly captures the biggest architectural decisions: two codebases, `treequest-go` as a generic library, OpenRouter only in the downstream app, AB-MCTS-A first, AB-MCTS-M deferred, Ask/Tell, TrialStore, score validation, `StateCodec[S]`, deterministic testing, EBP validators, budget provenance, and artifact-bundle output. 

The plan should not be treated as final yet because a few details still leak LLM terminology into the core, and some API sketches conflict with the stronger source-faithfulness / production-hardening rules we already adopted.

## What is already good

The plan correctly keeps `treequest-go` free of LLMs, OpenRouter, prompts, EBP, Workbench, and domain concepts. That is the most important boundary. It also correctly puts OpenRouter support in `ebp-paper-evaluator` only, via `github.com/revrost/go-openrouter`; that package exists as an unofficial Go client and is installed with `go get github.com/revrost/go-openrouter`. ([GitHub][1])

It also correctly chooses **AB-MCTS-A first** and defers AB-MCTS-M. That matches the prior consensus: pure-Go AB-MCTS-A is the useful first milestone, while M requires heavier Bayesian runtime decisions.

The best sections are:

```text
7. TrialStore and idempotent Tell
8. Score validation
9. Randomness/reproducibility
10. Snapshot/restore and StateCodec[S]
13. Python parity/replay test strategy
16. Deterministic EBP validators
17. Budget and provenance
18. Output artifact bundle
```

Those sections materially improve correctness and auditability.

## Required repairs before implementation

### 1. Remove “model” terminology from `treequest-go`

The core type sketch still uses:

```go
ModelBandits map[string]BetaParams
GeneratedBy  string
```

and the algorithm section says “model selection.” That violates the neutral-core rule.

Repair to:

```go
ActionBandits map[ActionLabel]BetaParams
GeneratedByAction ActionLabel
```

and rewrite “model selection” as:

```text
action selection
```

OpenRouter models belong only in `ebp-paper-evaluator`.

### 2. Add `context.Context` and errors to the low-level API

The plan says the public API uses Ask/Tell, but the code sketch omits `context.Context` in the `Algorithm` interface:

```go
AskBatch(state AS, batchSize int, actions []string) ...
Tell(state AS, trialID string, stateScore StateScore[S]) ...
```

Repair to:

```go
type Algorithm[S any, AS any] interface {
    InitTree(ctx context.Context) (AS, error)

    AskBatch(
        ctx context.Context,
        state AS,
        batchSize int,
        actions []ActionLabel,
    ) (AS, []Trial[S], error)

    Tell(
        ctx context.Context,
        state AS,
        trialID TrialID,
        result StateScore[S],
    ) (AS, error)

    StateScorePairs(state AS) []StateScore[S]
}
```

This keeps cancellation and error semantics Go-native.

### 3. Replace `TrialStatusRunning` with clearer lifecycle states

Current statuses:

```go
RUNNING
COMPLETE
INVALID
```

Better:

```go
const (
    TrialPending   TrialStatus = "pending"
    TrialCompleted TrialStatus = "completed"
    TrialFailed    TrialStatus = "failed"
    TrialCanceled  TrialStatus = "canceled"
)
```

`AskBatch` creates pending trials. `Tell` completes them. Failed and canceled trials are useful for retry/provenance.

### 4. Add result hash / canonical identity for idempotent `Tell`

The plan says duplicate `Tell` should compare hash or value, but the types do not include a result hash.

Add:

```go
type StateScore[S any] struct {
    State S
    Score Score
    Meta map[string]any
    ResultHash string
}
```

or keep hash internal by deriving it through the configured `StateCodec[S]`.

Without a canonical result hash, idempotency is ambiguous for complex generic state.

### 5. Do not JSON-serialize runtime `Node[S]` directly

The runtime `Node[S]` has JSON tags and direct `State S`:

```go
State S `json:"state"`
```

But the plan also says snapshots use `StateCodec[S]`. These conflict.

Repair by separating runtime node from snapshot node:

```go
type Node[S any] struct {
    ID NodeID
    ParentID *NodeID
    ChildrenIDs []NodeID
    State S
    Score Score
    Depth int
    ExpandIdx int
}

type NodeRecord struct {
    ID NodeID `json:"id"`
    ParentID *NodeID `json:"parent_id,omitempty"`
    ChildrenIDs []NodeID `json:"children_ids,omitempty"`
    EncodedState []byte `json:"encoded_state"`
    Score Score `json:"score"`
    Depth int `json:"depth"`
    ExpandIdx int `json:"expand_idx"`
}
```

Runtime structs can be convenient; snapshots must be flat and codec-driven.

### 6. Add `ExpandIdx`

Stepfun identified `expand_idx` as part of source-faithful TreeQuest behavior. The plan omits it.

Add:

```go
ExpandIdx int
```

to runtime and snapshot records.

### 7. Tighten AB-MCTS-A update language

The plan’s AB-MCTS-A section is good, but it should avoid implying the update rules are final until compared against the Python source.

Add a note:

```text
Exact wider/deeper/action bandit update rules must be confirmed against upstream TreeQuest traces before claiming source-faithful parity.
```

This is important because small differences in where wider/deeper updates are applied can change behavior.

### 8. Rename “convergence” in the EBP self-audit

This line is too strong:

```text
Thompson sampling mathematical convergence is addressed.
```

Replace with:

```text
AB-MCTS-A Thompson/Beta update behavior is implemented as a testable algorithmic assumption; mathematical convergence is not proved by this implementation plan.
```

Do not imply convergence has been proven.

### 9. Fix “Promotion Status”

The plan ends with:

```text
Promotion Status: Awaiting user approval.
```

Better:

```text
Promotion status: implementation_plan_pending_human_review; not implemented, not tested, not accepted as source-faithful until parity tests pass.
```

User approval is not EBP promotion.

### 10. Clarify OpenRouter SDK isolation and fallback

The plan correctly uses `github.com/revrost/go-openrouter`, but add an adapter boundary note:

```text
OpenRouter adapter is replaceable. If the unofficial client changes or the official OpenRouter Go SDK becomes preferable, only the app-layer adapter changes.
```

OpenRouter also has an official beta Go SDK under `github.com/OpenRouterTeam/go-sdk`, so keeping the `LLMClient` boundary is the right hedge. ([GitHub][2])

## Optional but recommended repairs

Add `Reward` even if the algorithm only uses scalar score:

```go
type Reward struct {
    Score Score `json:"score"`
    Components map[string]float64 `json:"components,omitempty"`
    Justification string `json:"justification,omitempty"`
    Meta map[string]any `json:"meta,omitempty"`
}
```

Then `StateScore[S]` can carry `Reward`, and the core can still use only `Reward.Score`.

Also add a package-level neutral type set:

```go
type ActionLabel string
type NodeID string
type TrialID string
type Score float64
```

Avoid raw `string` in public API where the distinction matters.

## Revised acceptance status

I would classify the plan like this:

```json
{
  "status": "accept_with_repairs",
  "ready_for_implementation": false,
  "highest_priority_repairs": [
    "remove model terminology from treequest-go",
    "add context/errors to Algorithm interface",
    "separate runtime Node from codec-based snapshot records",
    "add result hash/idempotency mechanism",
    "add ExpandIdx",
    "soften convergence and promotion language"
  ],
  "core_architecture": "accepted",
  "ebp_app_boundary": "accepted",
  "openrouter_boundary": "accepted"
}
```

## EBP/PTW self-audit

**Claims:** The plan is architecturally strong but needs repairs before implementation.

**needMap:** Mostly satisfied: `treequest-go` maps to generic search; `ebp-paper-evaluator` maps to EBP/Workbench/OpenRouter workflows.

**needInvariant:** Core invariants are mostly present: no LLM dependencies, normalized scores, AB-MCTS-A first, app-level budgets. Needs terminology repair.

**needToyCheck:** Present, but should explicitly include duplicate `Tell`, out-of-order `Tell`, invalid scores, snapshot/restore, and fake `S=string`.

**needNullModel:** Present but thin. Add Standard MCTS / Best-First / BFS as v0.2 null baselines against AB-MCTS-A.

**needObstruction:** Main obstructions are source-faithfulness uncertainty, idempotency hashing ambiguity, snapshot codec complexity, and OpenRouter adapter instability.

**needFaithfulnessReview:** Required before claiming parity with Sakana TreeQuest.

**Promotion status:** `implementation_plan_accept_with_repairs`; not yet implementation-ready until the required repairs above are applied.

[1]: https://github.com/revrost/go-openrouter?utm_source=chatgpt.com "reVrost/go-openrouter: Unofficial API SDK for ..."
[2]: https://github.com/OpenRouterTeam/go-sdk?utm_source=chatgpt.com "OpenRouterTeam/go-sdk"
