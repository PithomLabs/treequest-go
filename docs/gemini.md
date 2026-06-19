Used tool: list_permissions
Used tool: ask_permission
Listed directory ab-mcts
Viewed ebp_v2.1.md:1-800
Viewed workbench2.md:1-800
Listed directory treequest-main
Viewed README.md:1-321
Listed directory treequest
Listed directory algos
Viewed types.py:1-22
Viewed base.py:1-74
Viewed tree.py:1-140
Viewed trial.py:1-239
Listed directory ab_mcts_a
Viewed algo.py:1-391
Viewed prob_state.py:1-485
Listed directory ab-mcts-arc2-main
Listed directory src
Listed directory ab_mcts_arc2
Listed directory treequest
Viewed treequest_go_port.md:1-800
Viewed treequest_go_port.md:800-871
Viewed claude.md:1-48
Viewed deepseek.md:1-245

Implementing a Go port of the **TreeQuest** library is a highly viable and elegant project. Using Go's native support for generics, you can create a general-purpose, high-performance, and thread-safe search tree library that is completely decoupled from any specific LLM API or domain framework like EBP/Workbench2.

Below is an implementation overview, package architecture, and feedback on your design goals.

---

### Part 1: Go Package Architecture Overview

To achieve your goals, you should decouple the search algorithm from the application domain by splitting them into two separate codebases or modules:

```text
treequest-go/ (Generic Library)
├── pkg/
│   ├── tree/
│   │   ├── node.go          # Generic Node[S] struct
│   │   ├── tree.go          # SearchTree[S] holding nodes and mutexes
│   │   └── checkpoint.go    # JSON state save/load helpers
│   └── algo/
│       ├── abmcts_a.go      # AB-MCTS-A Search Engine implementation
│       ├── bandit.go        # BetaBandit / Thompson Sampling primitives (using gonum/distuv)
│       └── types.go         # Generic GenerateFn[S] interface definitions
└── go.mod                   # Zero LLM dependencies (depends on gonum/v1/gonum)

ebp-paper-eval/ (Domain Application)
├── pkg/
│   ├── ebp/
│   │   ├── state.go         # ClaimEvalState (EBP & Workbench2 specific struct)
│   │   ├── prompts.go       # System prompts for A, B, and E
│   │   ├── worker_a.go      # Constructive worker (A) GenerateFn implementation
│   │   ├── worker_b.go      # Adversarial worker (B) GenerateFn implementation
│   │   └── evaluator.go     # Evaluator (E) JSON parser & scoring logic
│   └── llm/
│       ├── provider.go      # LLM API abstraction (Claude, GPT, DeepSeek)
│       └── anthropic.go     # Anthropic REST client implementation
└── cmd/
    └── evaluate/
        └── main.go          # CLI entry point to parse paper and run search
```

---

### Part 2: Core Data Structures in Go

#### 1. Generic Node Representation (`pkg/tree/node.go`)
By parameterizing the state `S`, the node does not need to know what data it is storing:

```go
package tree

import "sync"

type BetaParams struct {
    Alpha float64
    Beta  float64
}

func NewBetaParams() BetaParams {
    return BetaParams{Alpha: 1.0, Beta: 1.0} // Flat uniform prior
}

type Node[S any] struct {
    ID           string
    Depth        int
    Parent       *Node[S]
    Children     []*Node[S]
    State        S
    Score        float64 // Normalized ∈ [0, 1]
    WiderBandit  BetaParams
    DeeperBandit BetaParams
    ModelBandits map[string]BetaParams // Supports multi-agent/model selection
    GeneratedBy  string
    mu           sync.Mutex
}
```

#### 2. The Algorithmic Step Loop (`pkg/algo/abmcts_a.go`)
The MCTS algorithm relies on a clean, generic generator signature:

```go
package algo

import (
    "context"
    "fmt"
    tree "treequest-go/pkg/tree"
)

type GenerateFn[S any] func(ctx context.Context, parentState *S) (S, float64, error)
type GenerateFns[S any] map[string]GenerateFn[S]

type ABMCTSA[S any] struct {
    sampler *BetaSampler
}

func (a *ABMCTSA[S]) Step(ctx context.Context, t *tree.SearchTree[S], fns GenerateFns[S]) (*tree.Node[S], error) {
    // 1. SELECT: Walk the tree deciding whether to go wider vs. deeper using Thompson Sampling
    targetNode, path := a.selectNode(t.Root)

    // 2. CHOOSE MODEL: Decide which worker agent generates the next branch
    chosenModel := a.sampler.ChooseModel(targetNode.ModelBandits)
    fn := fns[chosenModel]

    // 3. EXPAND: Call closure to query worker LLM + evaluate score
    var parentState *S
    if targetNode.Parent != nil {
        parentState = &targetNode.State
    }
    newState, score, err := fn(ctx, parentState)
    if err != nil {
        return nil, err
    }

    // 4. ADD NODE & BACKPROPAGATE: Insert node and update bandits up the path
    child := tree.NewChildNode(targetNode, newState, score, chosenModel)
    t.AddNode(child)
    a.backpropagate(path, score, chosenModel)

    return child, nil
}
```

---

### Part 3: Addressing Your Specific Goals

#### Goal 1: Make treequest library general-purpose and not tied to any LLM
**Verdict:** **Excellent and fully achievable.**
* **Generics:** By using Go generics (`[S any]`), `treequest-go` is fully decoupled from LLM models, system prompts, and structured formats. 
* **Zero-dependency Core:** The generic library only depends on mathematical libraries (like `gonum/distuv` for sampling beta distributions) and local graph logic. It remains clean, simple, and testable using mock random-number generators.

#### Goal 2: EBP use case should be a separate codebase
**Verdict:** **Correct architecture.**
* Enforcing this codebase separation prevents EBP-specific domain concepts (like "debt items", "claims", or specific JSON system prompts) from polluting the search engine.
* It allows `treequest-go` to be reused in the future for other applications (such as code-generation search, math solvers, or games) without refactoring.

#### Goal 3: Turn budget control flag for worker LLMs
**Verdict:** **Reasonable and highly recommended design, with key refinements:**

To design this cleanly, you should conceptualize budget control across a **three-layer stack**:
1. **Search-level budget (Outermost):** Controlled by the overall `AB-MCTS` loop in `main.go` (e.g., maximum tree iterations).
2. **Worker-turn budget (Middle):** Capping how many conversational turns (critique/refinement) Worker A and Worker B can trade internally within a single `GenerateFn` invocation.
3. **Token budget (Innermost):** Capping token outputs per call.

##### EBP-Specific Refinement (Honest Incompleteness)
Under **EBP v2.1 / Workbench2** guidelines:
> *“The Workbench rewards honest incompleteness. It penalizes hidden incompleteness.”*

Therefore, if a worker hits the turn limit without fully resolving a claim, **it should not silently truncate and return**. Instead:
* The state structure `ClaimEvalState` should expose an explicit flag (e.g., `BudgetExhausted: true`).
* Evaluator E must inspect this flag and penalize the score or track it as unpaid debt.
* This budget flag should be surfaced in the downstream Workbench report so users can audit how much compute was spent on each claims validation.

---

### Next Steps for Implementation

1. **Implement `treequest-go` first:** Write the generic `Node[S]`, `SearchTree[S]`, and `BetaSampler` (using `gonum`). Write a simple mock harness where `S = string` and generations are simulated with `rand.Float64()` to verify that the tree branches and Thompson Sampling converges properly.
2. **Implement `ebp-paper-eval`:** Once the engine is tested, write the EBP domain model mapping, wire the LLM API clients (e.g., Anthropic Claude), and pass the worker calls into the generic `GenerateFn` closure signature.