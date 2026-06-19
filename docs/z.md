# Porting TreeQuest / AB-MCTS to Go — Implementation Overview

> Note: I cannot live-fetch `github.com/SakanaAI/treequest` or `sakana.ai/ab-mcts/` in this session, so the following is reconstructed from public knowledge of the AB-MCTS paper and the TreeQuest API shape. Treat the API sketches as design proposals to reconcile against the actual repo when you clone it. The conceptual mapping is correct; exact field names may need renaming to match Sakana's terminology.

---

## 1. What AB-MCTS / TreeQuest actually does (recap so we agree)

AB-MCTS is MCTS adapted to LLM-driven search where:

- **Branching is generated, not enumerated.** Children are produced on demand by an LLM (the generator).
- **Two branching modes coexist:**
  1. *Sibling generation* — produce a fresh alternative at the same level (the parent spawns a new child).
  2. *Refinement* — descend into an existing child and improve it (the child spawns its own child).
- **Selection compares two kinds of arms** with a UCB-style criterion:
  - "Generate a new sibling of X" — a virtual arm on X's parent.
  - "Refine existing child Y of X" — an arm on X for each existing child Y.
- **Evaluation** is done by a (possibly different) LLM producing a scalar reward (optionally multi-component).
- **Backpropagation** updates `N`, `Q`, best-reward, variance along the path to root.
- **Budgets** terminate the loop (iterations, time, API calls, tokens).

TreeQuest (Python) wraps this with:
- A `Graph`/`Node` data model with JSON checkpointing.
- A `Context` abstraction for LLM clients (model-agnostic).
- User-supplied *problem* and *solution* types plus conversion adapters.
- A `solve` driver that runs until budget exhausted and returns the best path.

Everything below mirrors that structure in idiomatic Go.

---

## 2. Goals re-checked

| # | Goal | How the design satisfies it |
|---|------|------------------------------|
| 1 | General-purpose, not EBP-specific, not LLM-tied | Library exposes `Generator`/`Evaluator` interfaces; no LLM client imports; problem/solution are `any`-typed generics. |
| 2 | EBP use case lives in a separate repo | `treequest-go` ships zero EBP/Workbench/Lean types. EBP repo imports it and supplies concrete adapters. |
| 3 | Output = research paper + turn-budget flag | Reasonable, with refinements (see §10). The flag should be both an *input* (declared budget) and an *output* (actual consumed) for auditability — which aligns with EBP v2.1's "accounting must be seamless" doctrine. |

---

## 3. Repository layout

### 3.1 `treequest-go` (the library)

```
treequest-go/
  pkg/
    graph/          Node, Graph, edges, JSON snapshot
    mcts/           AB-MCTS driver: select / expand / eval / backprop
    branching/      Adaptive branching policy (sibling vs refine)
    budget/         Budget interfaces + impls (turns, tokens, wallclock, composite)
    reward/         Reward normalization & aggregation
    provider/       Interfaces only: Generator, Evaluator, LLMClient (no concrete impls)
    config/         Config structs + YAML/JSON loaders
    checkpoint/     Filesystem / SQLite backends
    observability/  Hooks: OnSelect, OnExpand, OnEval, OnBackprop, OnBudgetEvent
  examples/
    toy/            Deterministic non-LLM example (validates algorithm)
  cmd/
    treequest/      CLI: run a search from a config file
  go.mod
```

### 3.2 `ebp-treequest-runner` (the use case, separate repo)

```
ebp-treequest-runner/
  pkg/
    paper/      PDF/Markdown/arXiv ingestion, claim extraction
    claim/      EBP Idea + Workbench metadata (function class, maturity, claim type)
    workers/    Worker A (constructive), Worker B (adversarial), Evaluator E
    adapters/   treequest.Generator / Evaluator implementations using the workers
    budget/     EBP-specific budget wiring (per-role turn caps)
    output/     Annotated paper + JSON sidecar
  cmd/
    ebp-eval/   CLI: ebp-eval --paper X.pdf --a-turns 50 --b-turns 50 --e-turns 100
```

---

## 4. Core types (sketch)

```go
// pkg/graph/node.go
package graph

type NodeID string

type Node struct {
    ID        NodeID
    ParentID  *NodeID
    Depth     int
    Problem   any            // user-defined
    Solution  any            // user-defined; may be nil until generated
    Reward    *Reward        // nil until evaluated
    Stats     Stats
    Children  []NodeID
    Meta      map[string]any // free-form provenance, EBP debt, etc.
    CreatedAt time.Time
}

type Stats struct {
    N          int       // visit count
    SumReward  float64
    Q          float64   // mean reward
    BestReward float64
    Variance   float64
    RefineN    int       // how many refinements spawned
    SiblingN   int       // how many siblings spawned (set on parent)
}

type Reward struct {
    Score         float64
    Components    map[string]float64  // e.g., {clarity:.7, rigor:.6, ...}
    Justification string
    EvaluatorID   string
}
```

```go
// pkg/graph/graph.go
type Graph struct {
    mu    sync.RWMutex
    nodes map[NodeID]*Node
    root  NodeID
    next  uint64 // monotonic ID source
}

func (g *Graph) Root() *Node
func (g *Graph) Get(id NodeID) (*Node, bool)
func (g *Graph) AddChild(parentID NodeID, problem, solution any) *Node
func (g *Graph) UpdateStats(id NodeID, r Reward)
func (g *Graph) BestPathFromRoot() []*Node
func (g *Graph) Snapshot() ([]byte, error)   // JSON
func LoadSnapshot(b []byte) (*Graph, error)
```

---

## 5. Provider interfaces (the model-agnostic seam)

This is the heart of goal #1. The library imports **no** LLM SDK. Users supply:

```go
// pkg/provider/provider.go
package provider

type Generator interface {
    // Produce a new child for `parent` (or a refinement of `node`).
    // mode tells the generator whether the user wants a sibling or a refinement.
    Generate(ctx context.Context, req GenRequest) (GenResponse, error)
}

type GenRequest struct {
    Mode      GenMode          // GenSibling | GenRefine
    Parent    *graph.Node      // nil at root
    Node      *graph.Node      // node being refined (GenRefine only)
    Siblings  []*graph.Node    // existing siblings (for diversity hints)
    Depth     int
    Budget    budget.Budget    // generator must check before calling its LLM
    Hint      map[string]any   // free-form
}

type GenResponse struct {
    Problem  any
    Solution any
    Meta     map[string]any
}

type Evaluator interface {
    Evaluate(ctx context.Context, req EvalRequest) (EvalResponse, error)
}

type EvalRequest struct {
    Node   *graph.Node
    Path   []*graph.Node   // ancestry for context
    Budget budget.Budget
}

type EvalResponse struct {
    Reward        graph.Reward
    Meta          map[string]any
}
```

Why this shape:
- `Problem` and `Solution` are `any` — the library never inspects them. EBP can put its `Idea`/`Evaluation` types here; a math-search repo can put theorem statements; a code-search repo can put function specs.
- `Budget` is passed in so generators/evaluators can short-circuit cleanly without the driver pre-checking every call (defense in depth).
- `Hint` lets the EBP layer pass Workbench context (function class, maturity stage, claim type) without the library knowing what those mean.

---

## 6. AB-MCTS algorithm port

### 6.1 Selection (UCB over two arm types)

```go
// pkg/mcts/select.go
func (s *Solver) Select(root NodeID) (Selection, error) {
    cur := root
    for {
        node := s.g.Get(cur)
        if len(node.Children) == 0 && node.Stats.SiblingN == 0 {
            return Selection{Kind: SelExpandRoot, Node: cur}, nil
        }
        // Compute UCB for each existing child (refinement arms)
        bestScore := math.Inf(-1)
        bestChild := NodeID("")
        for _, cid := range node.Children {
            c := s.g.Get(cid)
            score := s.ucb(node.Stats.N, c.Stats.N, c.Stats.Q, s.cfg.C)
            if score > bestScore {
                bestScore, bestChild = score, cid
            }
        }
        // UCB for the "spawn new sibling" virtual arm
        // Uses mean Q over existing children as the arm's estimated value,
        // with virtual count = SiblingN (number of siblings already spawned).
        siblingScore := s.ucbSibling(node, s.cfg.C)

        if siblingScore > bestScore && !s.budget.Exhausted() {
            return Selection{Kind: SelGenSibling, Node: cur}, nil
        }
        // descend
        cur = bestChild
        if s.g.Get(cur).Reward == nil {
            return Selection{Kind: SelEvaluate, Node: cur}, nil
        }
        // If leaf with reward but no children, we can still refine or evaluate-deeper
        if len(s.g.Get(cur).Children) == 0 {
            return Selection{Kind: SelRefine, Node: cur}, nil
        }
    }
}
```

Two implementation notes:
- The original AB-MCTS uses an *infinite-arm bandit* for the sibling-spawning arm. A common closed form is `Q̄ + C·sqrt(2·ln(N) / k)` where `k` is the number of siblings so far and `Q̄` is their mean. This is what `ucbSibling` computes.
- The first-visit rule: any node with `Reward == nil` must be evaluated before being refined. This avoids spending refinement budget on un-evaluated guesses.

### 6.2 Expansion

```go
func (s *Solver) Expand(sel Selection) (*graph.Node, error) {
    switch sel.Kind {
    case SelExpandRoot, SelGenSibling:
        parent := s.g.Get(sel.Node)
        resp, err := s.gen.Generate(s.ctx, provider.GenRequest{
            Mode:   provider.GenSibling,
            Parent: parent,
            ...
        })
        if err != nil { return nil, err }
        return s.g.AddChild(sel.Node, resp.Problem, resp.Solution), nil
    case SelRefine:
        node := s.g.Get(sel.Node)
        resp, err := s.gen.Generate(s.ctx, provider.GenRequest{
            Mode: provider.GenRefine,
            Node: node,
            ...
        })
        return s.g.AddChild(sel.Node, resp.Problem, resp.Solution), nil
    }
}
```

### 6.3 Evaluation

```go
func (s *Solver) Evaluate(node *graph.Node) (graph.Reward, error) {
    r, err := s.eval.Evaluate(s.ctx, provider.EvalRequest{Node: node, ...})
    if err != nil { return graph.Reward{}, err }
    s.g.SetReward(node.ID, r.Reward)
    return r.Reward, nil
}
```

### 6.4 Backpropagation

```go
func (s *Solver) Backprop(id NodeID, r graph.Reward) {
    cur := id
    for cur != "" {
        n := s.g.Get(cur)
        n.Stats.N++
        n.Stats.SumReward += r.Score
        n.Stats.Q = n.Stats.SumReward / float64(n.Stats.N)
        if r.Score > n.Stats.BestReward { n.Stats.BestReward = r.Score }
        // Welford variance
        updateVariance(&n.Stats, r.Score)
        if n.ParentID == nil { break }
        cur = *n.ParentID
    }
}
```

### 6.5 Driver loop

```go
func (s *Solver) Run(ctx context.Context) (*Result, error) {
    for !s.budget.Exhausted() {
        sel, err := s.Select(s.g.Root().ID)
        if err != nil { return nil, err }

        var node *graph.Node
        switch sel.Kind {
        case SelEvaluate:
            n := s.g.Get(sel.Node)
            r, err := s.Evaluate(n)
            if err != nil { return nil, err }
            s.Backprop(sel.Node, r)
        default:
            node, err = s.Expand(sel)
            if err != nil { return nil, err }
            r, err := s.Evaluate(node)
            if err != nil { return nil, err }
            s.Backprop(node.ID, r)
        }

        s.cfg.Observability.OnIteration(s.g)
        if s.cfg.CheckpointEvery > 0 && s.iterCount%s.cfg.CheckpointEvery == 0 {
            s.checkpoint()
        }
        s.iterCount++
    }
    return s.buildResult(), nil
}
```

### 6.6 Adaptive branching policy

The "AB" of AB-MCTS is essentially the comparison in `Select` between `siblingScore` and `bestChild`'s UCB. But Sakana's paper introduces an *adaptive* temperature on this comparison: early in the search, sibling generation is favored (breadth); late, refinement is favored (depth). Port this as a `BranchingPolicy` interface so users can swap it:

```go
type BranchingPolicy interface {
    ShouldSpawnSibling(g *graph.Graph, nodeID graph.NodeID, iter, total int) bool
}
```

Default impl: `AdaptivePolicy{BreadthFrac: 0.5, Schedule: "linear"}` — favors siblings while `iter < total*BreadthFrac`, then linearly shifts toward refinement.

---

## 7. Budget subsystem

This addresses goal #3 directly.

```go
// pkg/budget/budget.go
type Budget interface {
    Exhausted() bool
    Consume(role string, cost Cost) error
    Remaining(role string) int
    Snapshot() Snapshot
}

type Cost struct {
    Turns  int
    Tokens int
}

type Snapshot struct {
    PerRole map[string]RoleUsage
    Total   RoleUsage
}
```

Concrete impls:

- `TurnBudget{A: 50, B: 50, E: 100}` — what the user asked for.
- `TokenBudget` — same but tokens.
- `WallClockBudget{Deadline: time.Now().Add(30*time.Minute)}`.
- `CompositeBudget{...}` — all must be non-exhausted.

The driver checks `budget.Exhausted()` per iteration; generators/evaluators check `budget.Consume(role, cost)` per LLM call. The EBP layer maps role strings `"A"`, `"B"`, `"E"` to its workers.

---

## 8. Concurrency & persistence

- **Graph mutation**: single `sync.RWMutex` on the graph is simplest and fine for I/O-bound LLM workloads. If you want parallel expansion, wrap the lock and have `Select` return a "reservation" that holds a write lock until backprop completes (or use a copy-on-write snapshot per iteration).
- **Parallel LLM calls**: a worker pool sized to per-provider concurrency limits. Each `Generate`/`Evaluate` runs in its own goroutine; the graph is updated serially via a results channel.
- **Checkpointing**: JSON snapshot to disk every N iterations + on graceful shutdown. Optional `Backend` interface (`FileBackend`, `SQLiteBackend`, `S3Backend`) — start with File.
- **Resume**: `LoadSnapshot` + `Run` re-enters the loop using existing stats. Budget must be *persisted alongside* the snapshot or you'll overspend on resume.

---

## 9. EBP use case wiring (separate repo)

This is where EBP v2.1 and Workbench2 enter. The library stays clean.

### 9.1 Problem and Solution types (in EBP repo)

```go
// ebp-treequest-runner/pkg/claim/types.go

// Problem = a claim to be evaluated under Workbench2.
type Claim struct {
    ID            string
    Text          string
    Source        string  // paper section
    FunctionClass string  // Workbench: fundamental candidate / reconstruction / ...
    Maturity      string  // Seed | Model | Framework | Candidate | Serious Contender
    ClaimType     string  // Math | Ontology | Dynamics | Observable | ...
    ParentClaimID string
}

// Solution = an evaluation of the claim produced by a worker.
type Evaluation struct {
    WorkerRole    string                 // "A" or "B"
    WorkerModel   string
    Workbench     WorkbenchReport        // filled partially per Workbench2 schema
    DebtChanges   []ebp.DebtItem         // EBP retire/add-debt events
    Notes         string
    TurnIndex     int
}
```

### 9.2 Generator implementation

```go
// ebp-treequest-runner/pkg/adapters/generator.go

type EBPGenerator struct {
    workerA LLMClient  // constructive
    workerB LLMClient  // adversarial
    budget  budget.Budget
}

func (g *EBPGenerator) Generate(ctx context.Context, req provider.GenRequest) (provider.GenResponse, error) {
    claim := req.Parent.Problem.(claim.Claim)
    switch req.Mode {
    case provider.GenSibling:
        // Alternate between A and B to produce diverse siblings.
        role := pickRole(req)
        if err := g.budget.Consume(role, budget.Cost{Turns: 1}); err != nil {
            return provider.GenResponse{}, err
        }
        eval := g.produceEvaluation(ctx, role, claim, req.Siblings)
        return provider.GenResponse{Problem: claim, Solution: eval}, nil
    case provider.GenRefine:
        // Ask the same worker to revise its prior evaluation
        // in response to a sibling's challenge.
        prior := req.Node.Solution.(claim.Evaluation)
        if err := g.budget.Consume(prior.WorkerRole, budget.Cost{Turns: 1}); err != nil {
            return provider.GenResponse{}, err
        }
        refined := g.refine(ctx, prior, req.Siblings)
        return provider.GenResponse{Problem: claim, Solution: refined}, nil
    }
}
```

### 9.3 Evaluator implementation

```go
type EBPEvaluator struct {
    evaluatorE LLMClient
    budget     budget.Budget
}

func (e *EBPEvaluator) Evaluate(ctx context.Context, req provider.EvalRequest) (provider.EvalResponse, error) {
    if err := e.budget.Consume("E", budget.Cost{Turns: 1}); err != nil {
        return provider.EvalResponse{}, err
    }
    eval := req.Node.Solution.(claim.Evaluation)
    score, justification := e.evaluatorE.ScoreClaimEvaluation(ctx, req.Node.Problem.(claim.Claim), eval)
    return provider.EvalResponse{
        Reward: graph.Reward{
            Score:         score,
            Components:    workbenchComponents(eval.Workbench), // clarity, rigor, falsifiability...
            Justification: justification,
            EvaluatorID:   "E",
        },
    }, nil
}
```

### 9.4 Output: annotated paper + sidecar

```go
type PaperReport struct {
    Paper        *paper.Paper
    Annotations  []ClaimAnnotation
    BudgetUsed   budget.Snapshot  // ← the "flag" the user mentioned
    TreeStats    graph.TreeStats
    GeneratedAt  time.Time
    Config       ebp.Config       // declared budgets for reproducibility
}

type ClaimAnnotation struct {
    ClaimID       string
    BestEvaluation claim.Evaluation
    BestPath      []string        // node IDs
    BestReward    float64
    DebtStatus    ebp.DebtStatus  // under EBP v2.1
    Alternatives  []claim.Evaluation  // siblings considered
    NextMove      string          // EBP's "next smallest useful move"
}
```

The output is two files:
- `paper_annotated.md` — the original paper with inline annotations per claim.
- `paper_report.json` — the sidecar with budget consumed per role, tree stats, and debt status.

---

## 10. Assessment of goal #3 — is "paper + turn-budget flag" reasonable?

**Yes, reasonable, with these refinements:**

1. **Make the flag bidirectional.** The *declared* budget (input) and the *consumed* budget (output) should both appear in the sidecar. This matches EBP v2.1 §10 "Seamless Accounting": the accounting is invisible during the run but auditable afterward.

2. **Turns alone under-count cost.** Two turns can differ by 100× in tokens. Use a `CompositeBudget` of turns ∧ tokens ∧ wall-clock. Expose all three in the sidecar.

3. **Distinguish three budget scopes:**
   - *Per-role* (A, B, E) — the user's primary control.
   - *Per-claim* (sub-budget across the tree) — optional cap to avoid one claim eating the whole run.
   - *Per-paper* (total) — hard ceiling.
   
   Recommend per-role + per-paper as the default; per-claim as opt-in.

4. **Budget should map to EBP's debt semantics.** A claim that hits its turn budget without convergence should be marked `dormant` (EBP §8) — not killed. This is a clean bridge between the library's budget exhaustion and EBP's "debt does not kill" doctrine.

5. **Reproducibility.** The flag should include the model IDs and seed, so the artifact is reproducible. EBP §16 emphasizes faithfulness; reproducibility is its operational cousin.

6. **The "flag" should not just be a CLI flag — it should be a first-class field on the output artifact.** Treating it as metadata on the paper output makes the paper self-describing, which is critical if the annotated paper is shared without its sidecar.

So: yes, good design. Just promote "turn budget" from a CLI flag to a typed `BudgetConfig` struct that appears in both input and output, and let it be composite (turns + tokens + wall-clock).

---

## 11. Milestone plan

| Milestone | Scope | Verifiable end state |
|---|---|---|
| **M1** | `graph` + `budget` packages, JSON snapshot round-trip | Unit tests; `go test ./pkg/graph` passes |
| **M2** | Toy non-LLM `Generator`/`Evaluator` (e.g., number-guessing game); `mcts` driver | Toy search finds optimum within N iterations |
| **M3** | Adaptive branching policy; UCB sibling vs refine | Compare tree shape vs non-adaptive MCTS on toy problem |
| **M4** | Concurrency: parallel eval worker pool; checkpoint/resume | Resume from snapshot reproduces identical stats |
| **M5** | `provider` interfaces frozen; observability hooks | Public API stable; `examples/toy` documented |
| **M6** | CLI `cmd/treequest` runs from config file | `treequest run --config toy.yaml` works end-to-end |
| **M7** | `ebp-treequest-runner` v0: single LLM provider, single paper | Annotated paper + sidecar produced for one arXiv paper |
| **M8** | Multi-provider (A, B, E distinct models); budget per role | Budget exhaustion correctly halts each role independently |
| **M9** | EBP debt bridging: `dormant` claims on budget exhaustion | Sidecar marks unconverged claims `dormant` not `dead` |

---

## 12. Key design decisions to lock down before coding

1. **Generics or `any`?** Recommend `any` for `Problem`/`Solution` with type-assertion in user code. Generics would force every helper to carry type parameters and complicate the public API. The Python library uses untyped `Any` for the same reason.

2. **Sync or async style?** Go has no async/await; use plain functions returning `(T, error)` plus a worker pool for parallelism. This is actually cleaner than Python's async for I/O-bound LLM calls.

3. **Determinism.** Add an optional `Seed` config that:
   - Seeds the sibling-vs-refine coin flip when scores tie.
   - Seeds any internal randomness in selection.
   - Does *not* (cannot) seed LLM outputs — record them in the sidecar for reproducibility instead.

4. **Reward normalization.** Different evaluators may produce scores on different scales (0–1, 0–10, 0–100). Add a `Normalizer` interface in `pkg/reward`:
   ```go
   type Normalizer interface { Normalize(graph.Reward) float64 }
   ```
   Default: identity. EBP repo can supply a Workbench-weighted normalizer that combines clarity/rigor/falsifiability components into one score per Workbench2 §XVI evidence discipline.

5. **Multi-objective rewards.** Keep the core algorithm single-objective (AB-MCTS is single-objective). Allow the EBP layer to scalarize via the `Normalizer`. This avoids over-engineering the library.

6. **Lean-shaped types from EBP v2.1.** The Lean structures (`Idea`, `DebtItem`, etc.) belong in the EBP repo as Go structs — *not* in `treequest-go`. The library must remain EBP-ignorant.

7. **Observability hooks vs structured logging.** Prefer typed hooks (`OnSelect`, `OnExpand`, ...) over `log.Printf`. The EBP layer can adapt hooks into its own provenance trail.

8. **Error semantics.** Distinguish *recoverable* (LLM 429, transient network) from *fatal* (budget exhausted, malformed config) errors. The driver should retry recoverable errors with exponential backoff; bubble fatal errors up immediately.

---

## 13. Pitfalls specific to this port

- **UCB cold-start.** Every arm must be pulled once before UCB is meaningful. The driver must force-evaluate the root's first child before selection kicks in. Otherwise `Q` is undefined and `bestScore` is `-inf`.
- **Sibling diversity.** Without a diversity hint, LLMs tend to produce near-duplicate siblings. Pass `req.Siblings` to the generator and let the EBP layer prompt "produce an evaluation distinct from these prior ones."
- **Refinement loops.** A worker can refine forever without improving. Add an optional `MaxRefineDepth` per node and a `MinRewardDelta` to prune refinements that don't move the needle.
- **Reward leakage.** If the same LLM serves as both worker and evaluator, scores inflate. EBP's design (separate E from A/B) avoids this; enforce it in the EBP repo's config validation, not in the library.
- **Checkpoint/budget drift.** On resume, you must reload the budget snapshot too, or you'll overspend. Persist `budget.Snapshot` next to the graph snapshot.
- **Tree explosion.** AB-MCTS can produce wide-and-deep trees. Add a `MaxNodes` and `MaxDepth` config as hard ceilings independent of budget.

---

## 14. TL;DR

- **Library (`treequest-go`)**: ports the Graph, AB-MCTS driver (select/expand/eval/backprop with adaptive sibling-vs-refine branching), budget subsystem, checkpointing, and provider *interfaces*. It is generic over `Problem`/`Solution` types and imports no LLM SDK. EBP, Workbench, and Lean types are entirely absent.
- **Use case repo (`ebp-treequest-runner`)**: defines `Claim` (Workbench2-typed) and `Evaluation` types, wires Worker A / Worker B / Evaluator E LLMs into `treequest-go`'s `Generator`/`Evaluator` interfaces, and emits an annotated paper + JSON sidecar.
- **Goal #3 (turn-budget flag)**: reasonable. Refine to a composite `BudgetConfig` (turns + tokens + wall-clock) that appears in both input and output, scoped per-role (A/B/E) and per-paper. On budget exhaustion, mark unconverged claims `dormant` per EBP v2.1 §8 — never `dead`.
- **First milestone**: get a *toy* non-LLM search running end-to-end in `treequest-go` before any EBP wiring. This validates the algorithm port in isolation, which is the highest-risk part of the project.