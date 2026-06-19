# Porting TreeQuest to Go: Architecture Overview

This is a solid design direction. Let me walk through the architecture, key design decisions, and how your EBP use case maps onto it.

---

## 1. High-Level Separation

```
github.com/you/treequest-go/        ← general-purpose library, no LLM dependency
github.com/you/ebp-evaluator/       ← your EBP use case, imports treequest-go
```

This is the right call. The library owns the search algorithm and tree data structure. Your EBP codebase owns the domain logic (prompts, scoring criteria, worker roles).

---

## 2. Core Library Design (`treequest-go`)

### Package Layout

```
treequest-go/
├── go.mod
├── treequest.go            # public API facade
├── node.go                 # Node type
├── searchtree.go           # SearchTree (immutable-style, returns new copies)
├── trial.go                # Trial + Result types for ask/tell
├── algorithm.go            # Algorithm interface
├── thompson.go             # Thompson Sampling (Beta distribution sampling)
├── algorithms/
│   ├── abmcts_a.go         # AB-MCTS-A (node aggregation, simpler)
│   ├── abmcts_m.go         # AB-MCTS-M (mixed models, heavier)
│   ├── standard_mcts.go    # Vanilla MCTS baseline
│   └── best_first.go       # Best-first search baseline
├── render/
│   ├── html.go             # HTML tree visualization
│   └── dot.go              # Graphviz DOT export
└── checkpoint/
    └── checkpoint.go       # JSON serialization for save/resume
```

### Core Types

```go
package treequest

import "context"

// GenerateFn is the fundamental unit of work. It receives a parent state
// (nil for root expansion) and returns a new state with a score in [0,1].
// The library never knows what State is — that's your business.
type GenerateFn[S any] func(ctx context.Context, parent *S) (S, float64, error)

// Actions is a labeled set of generation strategies.
// Keys are human-readable names ("constructive", "adversarial", "refine", etc.)
type Actions[S any] map[string]GenerateFn[S]

// Trial represents a pending generation request (from ask_batch).
type Trial struct {
    ID         string
    Action     string
    ParentID   string  // empty if expanding from root
}

// Result wraps what tell() receives back.
type Result[S any] struct {
    State S
    Score float64
}

// TopResult is what top_k returns.
type ResultEntry[S any] struct {
    State S
    Score float64
    NodeID string
}

// Algorithm is the core interface. Every search strategy implements this.
// The tree is a value — methods return new trees (immutable pattern).
type Algorithm[S any] interface {
    InitTree() *SearchTree[S]

    // Step is the simple one-call-per-iteration API.
    Step(ctx context.Context, tree *SearchTree[S], actions Actions[S]) (*SearchTree[S], error)

    // Ask/Tell is the batched API for parallelism.
    AskBatch(tree *SearchTree[S], batchSize int, actionKeys []string) (*SearchTree[S], []Trial, error)
    Tell(tree *SearchTree[S], trialID string, result Result[S]) (*SearchTree[S], error)

    // TopK extracts the best results.
    TopK(tree *SearchTree[S], k int) []ResultEntry[S]
}
```

**Why generics**: Go 1.18+ generics let you use any state type (`string`, `struct{...}`, your EBP-specific type) without `interface{}` casts everywhere. This is what makes the library truly general-purpose.

**Why immutable-style return**: The Python version returns new tree objects. In Go, the same pattern avoids shared-mutation bugs when you later add parallelism. Internally the implementation can use structural sharing (copy-on-write) for efficiency — only the nodes on the path from root to the modified node need copying.

### Node and SearchTree

```go
package treequest

type Node[S any] struct {
    ID       string
    State    *S
    Score    float64
    ParentID string
    Children []string   // child node IDs
    Action   string     // which action produced this node
    Depth    int
}

type SearchTree[S any] struct {
    Nodes    map[string]*Node[S]
    RootID   string
    // Metadata for the algorithm (Thompson Sampling priors, visit counts, etc.)
    // Stored as algorithm-specific internal state.
    meta     map[string]interface{}
}
```

### Thompson Sampling Core

```go
package treequest

import (
    "math"
    "math/rand"
)

// BetaSample draws from Beta(alpha, beta) using the Jöhnk algorithm
// or Gamma-based method. For production, wrap gonum's Beta distribution.
func BetaSample(alpha, beta float64, rng *rand.Rand) float64 {
    // Gamma sampling approach
    g1 := gammaSample(alpha, rng)
    g2 := gammaSample(beta, rng)
    return g1 / (g1 + g2)
}
```

AB-MCTS-A maintains a Beta distribution per node (success/failure counts). AB-MCTS-M uses Bayesian mixed effects models — in Go you'd either call out to a C library (e.g., Stan math), use `gonum/stat`, or implement a simplified variational approximation. Start with AB-MCTS-A; it's sufficient for most use cases and much easier to implement.

---

## 3. AB-MCTS-A Implementation Strategy

The algorithm at each step:

```
1. Walk from root downward.
2. At each node, use Thompson Sampling on two Beta distributions:
   - "New" (go wider): generate an entirely new sibling
   - "Refine" (go deeper): descend into the best child
3. When you reach a leaf or decide to go wide:
   a. Pick which action to use (if multiple actions, Thompson Sampling again)
   b. Call the GenerateFn
   c. Insert the new node into the tree
   d. Backpropagate the score up the tree
```

The key data each node tracks:

```go
type abmctsNodeMeta struct {
    // For "wider vs deeper" decision
    widerAlpha  float64  // Beta prior for "go wider" successes
    widerBeta   float64
    deeperAlpha float64  // Beta prior for "go deeper" successes
    deeperBeta  float64

    // For action selection (if multiple actions exist)
    actionPriors map[string]*betaPrior
}
```

For Multi-LLM AB-MCTS, add a third dimension: action/LLM selection via Thompson Sampling across `Actions`.

---

## 4. Concurrency Model (Go Advantage)

Go gives you a natural advantage over Python here. The ask/tell batched interface maps directly to goroutines:

```go
// Parallel step using ask/tell
func ParallelStep[S any](ctx context.Context, algo Algorithm[S],
    tree *SearchTree[S], actions Actions[S], workers int) (*SearchTree[S], error) {

    keys := actionKeys(actions)
    var err error

    tree, trials, err := algo.AskBatch(tree, workers, keys)
    if err != nil { return nil, err }

    type indexedResult struct {
        trialID string
        result  Result[S]
        err     error
    }

    ch := make(chan indexedResult, len(trials))
    for _, t := range trials {
        go func(tr Trial) {
            gen := actions[tr.Action]
            var parent *S
            if tr.ParentID != "" {
                parent = tree.Nodes[tr.ParentID].State
            }
            state, score, genErr := gen(ctx, parent)
            ch <- indexedResult{tr.ID, Result[S]{state, score}, genErr}
        }(t)
    }

    for range trials {
        r := <-ch
        if r.err != nil { return nil, r.err }
        tree, err = algo.Tell(tree, r.trialID, r.result)
        if err != nil { return nil, err }
    }
    return tree, nil
}
```

---

## 5. Checkpointing

```go
// SaveTree serializes the search tree to JSON (or gob).
// The State type must be JSON-marshalable — that's the user's responsibility.
func SaveTree[S any](tree *SearchTree[S], path string) error { ... }

// LoadTree restores from disk. Caller must provide a type-matching generic.
func LoadTree[S any](path string) (*SearchTree[S], error) { ... }
```

This is critical for your EBP use case — LLM API calls are expensive and can fail mid-search.

---

## 6. EBP Use Case Design (`ebp-evaluator`)

### Conceptual Mapping

| AB-MCTS Concept | EBP Role |
|---|---|
| `GenerateFn` | Worker A (constructive), Worker B (adversarial) |
| Score | Evaluator E's assessment of A or B's output |
| Action label | `"constructive"` or `"adversarial"` |
| Search budget | Turn limit (`--max-turns`) |
| Node state | Current evaluation text + metadata |
| Tree | History of all A/B exchanges and E's scores |

### Workflow

```
1. Root node: EBP guidelines + research paper as initial state (score=0)
2. AB-MCTS decides: widen (new approach) or deepen (refine existing argument)
3. AB-MCTS picks action: constructive worker A or adversarial worker B
4. Selected worker LLM generates output (argument, critique, question)
5. Evaluator E scores the output against EBP 2.1 / Workbench2 criteria
6. Score flows back into the tree; Thompson Sampling updates priors
7. Repeat until --max-turns budget exhausted
8. Top-K extracts the best complete evaluation
```

### Codebase Structure

```
ebp-evaluator/
├── go.mod
├── main.go                  # CLI entry point
├── config.go                # Config struct + flags
├── roles/
│   ├── worker_constructive.go   # Worker A prompt + generation
│   ├── worker_adversarial.go    # Worker B prompt + generation
│   └── evaluator.go             # Evaluator E prompt + scoring
├── llm/
│   ├── client.go            # LLMClient interface (provider-agnostic)
│   ├── openai.go            # OpenAI-compatible impl
│   └── anthropic.go         # Anthropic impl (optional)
├── ebp/
│   ├── state.go             # EBP-specific state type
│   ├── scoring.go           # Scoring logic (EBP 2.1 criteria)
│   └── report.go            # Final report generation
└── cmd/
    └── run.go               # Cobra/Viper CLI setup
```

### State Type (EBP-specific)

```go
package ebp

type EvalState struct {
    PaperTitle       string            `json:"paper_title"`
    PaperAbstract    string            `json:"paper_abstract"`
    Claims           []Claim           `json:"claims"`
    CurrentTurn      int               `json:"current_turn"`
    WorkerRole       string            `json:"worker_role"`      // "constructive" or "adversarial"
    WorkerOutput     string            `json:"worker_output"`    // latest output from A or B
    EvaluatorNotes   string            `json:"evaluator_notes"`  // E's reasoning
    EBPChecklist     map[string]EBPItem `json:"ebp_checklist"`   // checklist state
}

type Claim struct {
    ID          string `json:"id"`
    Text        string `json:"text"`
    Status      string `json:"status"` // "untested", "supported", "challenged", "resolved"
}

type EBPItem struct {
    Criterion string `json:"criterion"`
    Status    string `json:"status"` // "pass", "fail", "partial", "unclear"
    Evidence  string `json:"evidence"`
}
```

### Wiring It Together

```go
package main

import (
    tq "github.com/you/treequest-go"
    "github.com/you/ebp-evaluator/roles"
    "github.com/you/ebp-evaluator/ebp"
)

func run(cfg Config) error {
    // 1. Build the generate functions
    actions := tq.Actions[ebp.EvalState]{
        "constructive": roles.NewConstructiveWorker(cfg.LLMClient, cfg.Paper),
        "adversarial":  roles.NewAdversarialWorker(cfg.LLMClient, cfg.Paper),
    }

    // 2. The evaluator is embedded in the generate functions.
    //    Each worker calls E internally to self-score, OR
    //    you chain: worker generates → E scores → return (state, score).
    //    The second approach (chain) is cleaner for AB-MCTS.

    algo := tq.NewABMCTSA[ebp.EvalState]()
    tree := algo.InitTree()

    ctx := context.Background()
    var err error
    for i := 0; i < cfg.MaxTurns; i++ {
        tree, err = tq.ParallelStep(ctx, algo, tree, actions, 2)
        if err != nil { return err }

        // Optional: checkpoint every N turns
        if (i+1)%10 == 0 {
            tq.SaveTree(tree, fmt.Sprintf("checkpoint-%d.json", i+1))
        }
    }

    // 3. Extract best evaluation
    results := algo.TopK(tree, 1)
    best := results[0]
    report := ebp.GenerateReport(cfg.Paper, best.State)
    return report.WriteToFile(cfg.OutputPath)
}
```

---

## 7. The Turn Budget Question

Your instinct is right, but I'd suggest a slight refinement:

```
--max-turns 50              # total generation budget (maps to AB-MCTS iterations)
--max-turns-per-worker 0    # 0 = unlimited, just let AB-MCTS decide allocation
--batch-size 2              # how many parallel generations per step
```

**Why `--max-turns` is good**: It directly corresponds to the AB-MCTS generation budget parameter. Users think in terms of "how many LLM calls can I afford." Each turn = one LLM call to a worker + one call to Evaluator E for scoring. So `--max-turns 50` really means ~50 worker calls + ~50 evaluator calls = ~100 total API calls.

**Why not hard-limit per worker**: Part of AB-MCTS's value is that Thompson Sampling automatically allocates more budget to the worker that's being more productive for a given problem. Hard per-worker limits fight the algorithm. If you want a safety valve, a soft cap via `--max-turns-per-worker 30` that warns but doesn't crash is fine.

**Cost estimation**: Consider adding a `--dry-run` flag that estimates total API cost based on average token counts before actually running. This pairs well with the turn budget for practical budgeting.

---

## 8. Implementation Roadmap

| Phase | Scope | Effort |
|---|---|---|
| **1. Skeleton** | Core types (`Algorithm` interface, `SearchTree`, `Node`, `Trial`), `ABMCTSA` with single action | ~2 weeks |
| **2. Multi-action** | Multi-LLM AB-MCTS (Thompson Sampling for action selection), ask/tell batched API | ~1 week |
| **3. Parallelism** | Goroutine-based parallel step, proper context cancellation, error recovery | ~3 days |
| **4. Checkpointing** | JSON save/load, gob encoding option | ~3 days |
| **5. Visualization** | HTML tree render (D3.js or plain HTML/CSS), DOT export | ~1 week |
| **6. AB-MCTS-M** | Mixed models via gonum or C binding (optional, start with AB-MCTS-A) | ~2 weeks |
| **7. EBP codebase** | Wire up the evaluator, prompts, scoring, CLI | ~2 weeks |

Start with Phase 1 + 2 and validate with a simple toy problem (e.g., the random-score example from TreeQuest's quickstart) before building the EBP codebase on top.