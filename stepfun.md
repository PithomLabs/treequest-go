Here's an initial overview based on the TreeQuest source and your use case.

## What TreeQuest actually does (from the source)
- **Core abstraction**: A generic tree of `Node[StateT]` plus an `AlgoStateT` owned by algorithms (`ABMCTSAState`, `ABMCTSMState`, etc.). The library says algorithms are **stateless**; the evolving tree/search state lives in `AlgoStateT` and is returned from `init_tree`, `step`, `ask_batch`, and `tell`.
- **Trial/TrialStore**: enables the **ask-tell** checkpointing pattern used for concurrency and resume.
- **Scoring contract**: every node score must be in **[0, 1]**.
- **Actions**: string keys mapped to `generate_fn(parent_state) -> (new_state, score)`.
- **Algorithms in source**:
  - `StandardMCTS` / `TreeOfThoughtsBFS` / `BestFirstSearch` / `MultiArmedBanditUCB` — queue-based.
  - `ABMCTSA` — node aggregation + Thompson Sampling, pure Python, no heavy deps.
  - `ABMCTSM` — Bayesian mixed models via **PyMC/numpyro/joblib** (Python-only scientific stack).

## What the port needs structurally
Go generics map cleanly to the Python `TypeVar` nodes (`StateT`, `AlgoStateT`). The cleanest shape is:

```text
treequest/
  node.go          // Node[StateT] : state, score, parent, children, expand_idx
  tree.go          // Tree[StateT]  : root, size, AddNode, GetNodes, StateScorePairs
  algo.go          // Algorithm[StateT, AlgoStateT] interface
  trial.go         // Trial, TrialStore, TrialStoreWithNodeQueue (for queue algos)
  abmctsa.go       // ABMCTSA state + algo (Thompson Sampling per node)
  abmctsm.go       // optional if you need it
  ranker.go        // TopK(state, algo, k)
  types.go         // GenerateFn, StateScore, RankingFn, NodeId/TrialId
```

The `Algorithm` interface translates almost 1:1:

```go
type Algorithm[StateT any, AlgoStateT any] interface {
    InitTree() AlgoStateT
    Step(state AlgoStateT, generateFns map[string]GenerateFn[StateT]) AlgoStateT
    Ask(state AlgoStateT, actions []string) (AlgoStateT, Trial[StateT])
    AskBatch(state AlgoStateT, batchSize int, actions []string) (AlgoStateT, []Trial[StateT])
    Tell(state AlgoStateT, trialID string, result StateScore[StateT]) AlgoStateT
    StateScorePairs(state AlgoStateT) []StateScore[StateT]
}
```

`GenerateFn` in Go:
```go
type GenerateFn[StateT any] func(parentState *StateT) (StateT, float64)
```

## EBP use case mapping (separate codebase)
Your evaluator `E` and workers `A`/`B` map to **actions** passed into the algorithm:
- `generateFn["constructive"]` calls worker A.
- `generateFn["adversarial"]` calls worker B.
- Each node's `state` is the Workbench report fragment (or an evolving PartialReport).
- `score` is produced by `E` after seeing the node output.
- With `ask_batch` + `tell`, `E` can score each trial independently and in any order.

Resume/checkpointing is handled by serializing `AlgoStateT` (tree + trial store + algo-specific bookkeeping), which fits Go well via JSON, gob, or protobuf.

## On the "budget flag" design
Embedding a **budget-per-worker** in the paper output is reasonable, but it conflates **metadata of the evaluation run** with **content of the paper artifact**. Typical design: the run config carries the budget, and the paper output carries an audit trail ("assessed under budget X with workers A,B,E"). Don't bake budget into the paper schema itself unless you need reproducibility metadata inside the artifact — then keep it in a clearly separated `meta` block.

---

One decision that matters a lot for the scope of the port: **ABMCTS-M depends on PyMC/numpyro/joblib, which are Python-only.** Do you want the initial Go port to include ABMCTS-M (requiring either a Python FFI/exec boundary, or reimplementing the Bayesian mixed-model priors in Go/gonum), or is ABMCTS-A (pure Thompson Sampling, easy to port) sufficient for your EBP use case?