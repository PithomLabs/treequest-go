# Porting Treequest to Go: Implementation Overview

**Target:** `treequest-go` — a general-purpose Go port of SakanaAI's `treequest` (AB-MCTS) library  
**Use case:** Multi-LLM adversarial evaluation of physics paper claims under EBP v2.1 + Workbench2

---

## 1. What You Are Porting

The Python library exposes two algorithms:

| Name | Mechanism | Dependency |
|------|-----------|------------|
| `ABMCTS-A` | Thompson Sampling over Beta distributions | scipy only |
| `ABMCTS-M` | Hierarchical Bayesian via PyMC | PyMC (heavy) |

The public API in Python is minimal:

```python
# generate_fns: dict[model_name -> Callable[[State | None], (State, float)]]
generate_fns = {"worker_a": fn_a, "worker_b": fn_b}

algo = tq.ABMCTSA()
tree = algo.init_tree()
for _ in range(50):
    tree = algo.step(tree, generate_fns)  # immutable-style update
```

The `generate_fn` receives the **parent state** (`None` at root) and must return `(new_state, score)` where score ∈ [0, 1]. The algorithm handles all search decisions: whether to go wider (new branch) or deeper (refine existing), and which model to use. You supply only the state type, the scoring, and the generation logic.

**Start with ABMCTS-A.** ABMCTS-M requires MCMC which has no idiomatic Go equivalent; defer it or bind a C library later.

---

## 2. AB-MCTS Algorithm Mechanics (The Math You Need)

### 2.1 The Two Decisions at Every Node

At each node `v` in the search tree the algorithm faces two choices:

1. **Direction**: generate a new child from `v` (go wider) vs. descend into an existing child of `v` (go deeper).
2. **Model**: which LLM/worker produced the generation (for Multi-LLM).

Both decisions are resolved by **Thompson Sampling** over Beta distributions.

### 2.2 Thompson Sampling over Beta Distributions

Each node `v` maintains two independent Beta distributions:

```
wider_dist  = Beta(α_w, β_w)   # distribution for "go wider from v"
deeper_dist = Beta(α_d, β_d)   # distribution for "go deeper into v's children"
```

Initial values: `α = 1.0, β = 1.0` (flat prior, i.e., uniform over [0,1]).

**Decision rule:**
```
s_w = sample(Beta(α_w, β_w))
s_d = sample(Beta(α_d, β_d))
if s_w > s_d: go wider (generate new child)
else: go deeper (pick best child, recurse)
```

**Update rule after generation returns score s ∈ [0, 1]:**
```
if action was "wider":
    α_w += s          # treat s as fractional success
    β_w += (1 - s)
if action was "deeper":
    α_d += s
    β_d += (1 - s)
```

This is fractional Thompson Sampling (continuous reward, not binary).

### 2.3 Multi-LLM Extension

When multiple models are registered, each node additionally maintains a Beta distribution per model:

```
model_dist[model_name] = Beta(α_m, β_m)
```

Model selection:
```
for each model m:
    s_m = sample(Beta(α_m[m], β_m[m]))
chosen_model = argmax(s_m over all m)
```

After generation by chosen model with score s:
```
α_m[chosen_model] += s
β_m[chosen_model] += (1 - s)
```

### 2.4 The Full Step Cycle

```
Step(tree, generate_fns):
1. SELECT: Walk from root, at each node sample wider vs deeper.
   - If "wider": current node is the expansion target → go to 2.
   - If "deeper": sample child via childselection (best mean of deeper_dist), recurse.
   - If leaf (no children): always "wider".
2. SELECT MODEL: Sample from model Beta distributions at expansion target → chosen_model.
3. EXPAND: Call generate_fns[chosen_model](node.state) → (new_state, score).
4. ADD NODE: Attach new child node to tree.
5. BACKPROPAGATE: Update Beta distributions up the selected path.
6. Return updated tree.
```

---

## 3. Go Package Architecture

```
treequest-go/
├── go.mod
├── go.sum
│
├── pkg/
│   ├── tree/
│   │   ├── node.go          # Node[S] generic struct
│   │   ├── tree.go          # SearchTree[S] + thread safety
│   │   └── checkpoint.go    # JSON save/resume
│   │
│   ├── algo/
│   │   ├── abmcts_a.go      # ABMCTS-A: the main algorithm
│   │   ├── bandit.go        # BetaBandit: Thompson Sampling primitives
│   │   └── types.go         # GenerateFn, Algorithm interface
│   │
│   ├── llm/
│   │   ├── provider.go      # LLMProvider interface
│   │   ├── anthropic.go     # Anthropic API (claude-sonnet-4-6 etc.)
│   │   └── ratelimit.go     # Token bucket / retry logic
│   │
│   └── ebp/
│       ├── state.go         # ClaimEvalState (EBP-specific State type)
│       ├── prompts.go       # System prompts for workers A, B and evaluator E
│       ├── worker_a.go      # Constructive worker generate function
│       ├── worker_b.go      # Adversarial worker generate function
│       └── evaluator.go     # Evaluator E → score extraction
│
├── cmd/
│   └── evaluate/
│       └── main.go          # CLI: paper path + config → run search
│
└── examples/
    └── bohmian_bfr/         # Example against a Bohmian Field Theory paper
        └── run.go
```

Dependencies (go.mod):
```
gonum.org/v1/gonum           # Beta distribution via distuv.Beta
github.com/google/uuid       # node IDs
encoding/json                # checkpointing (stdlib)
sync                         # stdlib, for tree mutex
```

---

## 4. Core Data Structures

### 4.1 `Node[S any]` — `pkg/tree/node.go`

```go
package tree

import "sync"

// BetaParams holds the Thompson Sampling state for one action slot.
type BetaParams struct {
    Alpha float64 // successes (fractional)
    Beta  float64 // failures  (fractional)
}

func NewBetaParams() BetaParams {
    return BetaParams{Alpha: 1.0, Beta: 1.0}
}

// Sample draws from Beta(α, β) using the Gamma-ratio method.
// Requires a source of randomness; inject via a sampler (see bandit.go).

// Node is a generic search-tree node. S is the user-defined state type.
type Node[S any] struct {
    ID       string
    Depth    int
    Parent   *Node[S]
    Children []*Node[S]

    State S
    Score float64 // ∈ [0, 1]; 0 for root/uninitialised

    // Direction bandits (per-node)
    WiderBandit  BetaParams
    DeeperBandit BetaParams

    // Per-model bandits (for Multi-LLM)
    ModelBandits map[string]BetaParams

    // Which model generated this node (empty for root)
    GeneratedBy string

    mu sync.Mutex
}

func NewRootNode[S any](rootState S, models []string) *Node[S] {
    mb := make(map[string]BetaParams, len(models))
    for _, m := range models {
        mb[m] = NewBetaParams()
    }
    return &Node[S]{
        ID:           newID(),
        State:        rootState,
        WiderBandit:  NewBetaParams(),
        DeeperBandit: NewBetaParams(),
        ModelBandits: mb,
    }
}

func NewChildNode[S any](parent *Node[S], state S, score float64, generatedBy string) *Node[S] {
    // Inherit model bandit keys from parent
    mb := make(map[string]BetaParams, len(parent.ModelBandits))
    for m := range parent.ModelBandits {
        mb[m] = NewBetaParams()
    }
    return &Node[S]{
        ID:           newID(),
        Depth:        parent.Depth + 1,
        Parent:       parent,
        State:        state,
        Score:        score,
        WiderBandit:  NewBetaParams(),
        DeeperBandit: NewBetaParams(),
        ModelBandits: mb,
        GeneratedBy:  generatedBy,
    }
}
```

### 4.2 `SearchTree[S any]` — `pkg/tree/tree.go`

```go
package tree

import "sync"

type SearchTree[S any] struct {
    Root   *Node[S]
    AllNodes []*Node[S]  // flat index for iteration, checkpointing
    mu     sync.RWMutex
}

func NewSearchTree[S any](rootState S, models []string) *SearchTree[S] {
    root := NewRootNode[S](rootState, models)
    return &SearchTree[S]{
        Root:     root,
        AllNodes: []*Node[S]{root},
    }
}

func (t *SearchTree[S]) AddNode(child *Node[S]) {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.AllNodes = append(t.AllNodes, child)
}

// BestNode returns the node with the highest score (for final answer extraction).
func (t *SearchTree[S]) BestNode() *Node[S] {
    t.mu.RLock()
    defer t.mu.RUnlock()
    var best *Node[S]
    for _, n := range t.AllNodes {
        if best == nil || n.Score > best.Score {
            best = n
        }
    }
    return best
}
```

---

## 5. Thompson Sampling in Go — `pkg/algo/bandit.go`

This is the non-trivial core. Go's stdlib has no Beta distribution; use `gonum/distuv`.

```go
package algo

import (
    "math/rand"

    "gonum.org/v1/gonum/stat/distuv"
    tree "treequest-go/pkg/tree"
)

// BetaSampler wraps gonum's Beta distribution for Thompson Sampling.
type BetaSampler struct {
    rng *rand.Rand
}

func NewBetaSampler(seed int64) *BetaSampler {
    return &BetaSampler{rng: rand.New(rand.NewSource(seed))}
}

// Sample draws one value from Beta(α, β).
func (s *BetaSampler) Sample(p tree.BetaParams) float64 {
    d := distuv.Beta{
        Alpha: p.Alpha,
        Beta:  p.Beta,
        Src:   s.rng,
    }
    return d.Rand()
}

// ChooseAction returns true if "wider" wins the Thompson Sampling duel.
func (s *BetaSampler) ChooseWider(wider, deeper tree.BetaParams) bool {
    return s.Sample(wider) > s.Sample(deeper)
}

// ChooseModel samples from each model's bandit and returns the winner's name.
func (s *BetaSampler) ChooseModel(bandits map[string]tree.BetaParams) string {
    var bestModel string
    bestScore := -1.0
    for name, bp := range bandits {
        v := s.Sample(bp)
        if v > bestScore {
            bestScore = v
            bestModel = name
        }
    }
    return bestModel
}

// UpdateBandit applies fractional Thompson Sampling update.
// score ∈ [0, 1]; alpha += score, beta += (1-score).
func UpdateBandit(bp tree.BetaParams, score float64) tree.BetaParams {
    return tree.BetaParams{
        Alpha: bp.Alpha + score,
        Beta:  bp.Beta + (1.0 - score),
    }
}
```

**Why Gamma-ratio is not needed explicitly:** `gonum/distuv.Beta` already implements the correct Rand() method using an internal Gamma sampler. You don't have to hand-code it.

---

## 6. ABMCTS-A Algorithm — `pkg/algo/abmcts_a.go`

```go
package algo

import (
    tree "treequest-go/pkg/tree"
    "context"
    "fmt"
)

// GenerateFn is the function signature each worker must implement.
// parentState == nil means expansion from root.
// Returns (newState, score ∈ [0,1], error).
type GenerateFn[S any] func(ctx context.Context, parentState *S) (S, float64, error)

// GenerateFns is a named collection of worker functions (one per model/role).
type GenerateFns[S any] map[string]GenerateFn[S]

// ABMCTSA implements the AB-MCTS-A algorithm.
type ABMCTSA[S any] struct {
    sampler *BetaSampler
}

func NewABMCTSA[S any](seed int64) *ABMCTSA[S] {
    return &ABMCTSA[S]{sampler: NewBetaSampler(seed)}
}

// Step runs one iteration of AB-MCTS-A.
// The tree is mutated in place; returns the newly created child node.
func (a *ABMCTSA[S]) Step(
    ctx context.Context,
    t *tree.SearchTree[S],
    fns GenerateFns[S],
) (*tree.Node[S], error) {

    // 1. SELECT: walk tree deciding wider/deeper at each node
    target, actionPath := a.select(t.Root)

    // 2. SELECT MODEL from target node's bandits
    chosenModel := a.sampler.ChooseModel(target.ModelBandits)
    fn, ok := fns[chosenModel]
    if !ok {
        return nil, fmt.Errorf("no generate function for model %q", chosenModel)
    }

    // 3. EXPAND: call generate function
    var parentStatePtr *S
    if target.Parent != nil || target.Depth > 0 {
        // Non-root: pass the target's state as parent context
        parentStatePtr = &target.State
    }
    // For root node: parentStatePtr remains nil → generate initial state
    newState, score, err := fn(ctx, parentStatePtr)
    if err != nil {
        return nil, fmt.Errorf("generate failed for model %q: %w", chosenModel, err)
    }

    // 4. ADD NODE
    child := tree.NewChildNode(target, newState, score, chosenModel)
    target.mu.Lock()
    target.Children = append(target.Children, child)
    target.mu.Unlock()
    t.AddNode(child)

    // 5. BACKPROPAGATE: update bandits along actionPath
    a.backpropagate(actionPath, score)

    return child, nil
}

// actionRecord tracks which action was taken at each node during selection.
type actionRecord[S any] struct {
    node      *tree.Node[S]
    wentWider bool
    model     string // only meaningful when wentWider=true (for model bandit update)
}

func (a *ABMCTSA[S]) select(root *tree.Node[S]) (*tree.Node[S], []actionRecord[S]) {
    var path []actionRecord[S]
    current := root

    for {
        current.mu.Lock()
        hasChildren := len(current.Children) > 0
        current.mu.Unlock()

        if !hasChildren {
            // Leaf: must go wider
            path = append(path, actionRecord[S]{node: current, wentWider: true})
            return current, path
        }

        // Sample direction
        goWider := a.sampler.ChooseWider(current.WiderBandit, current.DeeperBandit)

        if goWider {
            path = append(path, actionRecord[S]{node: current, wentWider: true})
            return current, path
        }

        // Go deeper: pick child with highest deeper_bandit mean
        path = append(path, actionRecord[S]{node: current, wentWider: false})
        current = a.bestChild(current)
    }
}

func (a *ABMCTSA[S]) bestChild[S any](parent *tree.Node[S]) *tree.Node[S] {
    parent.mu.Lock()
    defer parent.mu.Unlock()
    var best *tree.Node[S]
    bestMean := -1.0
    for _, c := range parent.Children {
        mean := c.DeeperBandit.Alpha / (c.DeeperBandit.Alpha + c.DeeperBandit.Beta)
        if mean > bestMean {
            bestMean = mean
            best = c
        }
    }
    return best
}

func (a *ABMCTSA[S]) backpropagate(path []actionRecord[S], score float64) {
    for _, rec := range path {
        n := rec.node
        n.mu.Lock()
        if rec.wentWider {
            n.WiderBandit = UpdateBandit(n.WiderBandit, score)
            // Also update chosen model's bandit
            if rec.model != "" {
                n.ModelBandits[rec.model] = UpdateBandit(n.ModelBandits[rec.model], score)
            }
        } else {
            n.DeeperBandit = UpdateBandit(n.DeeperBandit, score)
        }
        n.mu.Unlock()
    }
}
```

**Note on the model bandit update:** the chosen model name needs to be threaded from step 2 down into the backpropagation path. The `actionRecord` struct stores this for the expansion node only; simpler to record it separately and pass it in.

---

## 7. EBP v2.1 + Workbench2 State — `pkg/ebp/state.go`

This is your domain-specific `S` type — what gets threaded through the tree.

```go
package ebp

// ClaimEvalState is the State type S for the EBP physics-paper evaluation pipeline.
// Each node in the search tree represents one round of A/B analysis + E scoring
// for a specific claim extracted from the paper.
type ClaimEvalState struct {

    // ── Paper context ──────────────────────────────────────────────────────────
    PaperTitle   string `json:"paper_title"`
    PaperAbstract string `json:"paper_abstract"`  // injected at root

    // ── Claim identity ─────────────────────────────────────────────────────────
    ClaimID   string `json:"claim_id"`    // e.g. "claim-03"
    ClaimText string `json:"claim_text"`  // verbatim from paper

    // ── Workbench2 metadata (filled progressively) ────────────────────────────
    ClaimType    string `json:"claim_type"`     // from Workbench2 Part VIII
    MaturityStage string `json:"maturity_stage"` // Seed | Model | Framework | ...
    FunctionClass string `json:"function_class"` // from Workbench2 Part VII

    // ── EBP v2.1 debt tracking ────────────────────────────────────────────────
    RemainingDebt  []string `json:"remaining_debt"`  // DebtItem names still owed
    RetiredDebt    []string `json:"retired_debt"`     // DebtItem names paid
    FinalTruthFlag bool     `json:"final_truth_flag"` // true = contains final-truth language

    // ── Worker outputs ────────────────────────────────────────────────────────
    ConstructiveAnalysis string `json:"constructive_analysis"` // Worker A output
    AdversarialReview    string `json:"adversarial_review"`    // Worker B output

    // ── Evaluator E assessment ────────────────────────────────────────────────
    EvaluatorReasoning  string   `json:"evaluator_reasoning"`
    EBPScore            float64  `json:"ebp_score"`       // 0.0–1.0 (promotion readiness)
    WorkbenchFlags      []string `json:"workbench_flags"` // e.g. "bridge_missing", "undeclared_assumption"
    PromotionReady      bool     `json:"promotion_ready"` // true only if debt=[] && !FinalTruthFlag

    // ── Tree context ──────────────────────────────────────────────────────────
    GenerationDepth int    `json:"generation_depth"`
    RefinementFocus string `json:"refinement_focus"` // what this node attempted to improve
}
```

**Why this shape:** Each node represents a full analysis round. Going **wider** = fresh A+B analysis of the same claim (diversity in approach). Going **deeper** = A or B responds to the previous round's critique (iterative refinement). The evaluator E provides the score each time.

---

## 8. LLM Provider Interface — `pkg/llm/provider.go`

```go
package llm

import "context"

// Message is a minimal chat message.
type Message struct {
    Role    string // "system" | "user" | "assistant"
    Content string
}

// LLMProvider abstracts over Anthropic, OpenAI, DeepSeek, etc.
type LLMProvider interface {
    // Complete sends messages and returns the assistant response.
    Complete(ctx context.Context, messages []Message) (string, error)

    // Name returns the identifier used in the search tree (e.g. "claude-sonnet-4-6").
    Name() string
}
```

Implement `AnthropicProvider` wrapping the Anthropic REST API (`/v1/messages`). You can add OpenAI or others later for true Multi-LLM runs.

---

## 9. EBP Evaluation Pipeline — Workers A, B and Evaluator E

### 9.1 Worker A (Constructive) — `pkg/ebp/worker_a.go`

```go
// GenerateA produces a constructive EBP v2.1 analysis of the claim.
// If parentState is nil: initial analysis. Otherwise: respond to B's critique.
func GenerateA(provider llm.LLMProvider) algo.GenerateFn[ClaimEvalState] {
    return func(ctx context.Context, parent *ClaimEvalState) (ClaimEvalState, float64, error) {

        prompt := buildConstructivePrompt(parent)
        response, err := provider.Complete(ctx, prompt)
        if err != nil {
            return ClaimEvalState{}, 0, err
        }

        // Worker A returns a new state draft
        newState := cloneState(parent)
        newState.ConstructiveAnalysis = response
        newState.RefinementFocus = "constructive"

        // E evaluates both A and B (B from parent if available)
        score, updatedState, err := runEvaluator(ctx, evaluatorProvider, newState)
        if err != nil {
            return ClaimEvalState{}, 0, err
        }
        return updatedState, score, nil
    }
}
```

### 9.2 Worker B (Adversarial) — `pkg/ebp/worker_b.go`

Parallel structure to A, but uses the Workbench2 adversarial prompt:

```go
func GenerateB(provider llm.LLMProvider) algo.GenerateFn[ClaimEvalState] {
    return func(ctx context.Context, parent *ClaimEvalState) (ClaimEvalState, float64, error) {
        prompt := buildAdversarialPrompt(parent)  // Workbench2 seven-check schema
        response, err := provider.Complete(ctx, prompt)
        ...
        newState.AdversarialReview = response
        newState.RefinementFocus = "adversarial"
        score, updatedState, err := runEvaluator(ctx, evaluatorProvider, newState)
        ...
    }
}
```

### 9.3 Evaluator E — `pkg/ebp/evaluator.go`

E sees both A and B's output and scores per EBP v2.1 promotion criteria:

```go
// EBP promotion criteria encoded in the scoring prompt:
// - debt items retired (needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview)
// - no final-truth language
// - no Workbench2 bridge violations (claim-type crossings without bridge principles)
// - explanation level (derives=4 > explains=3 > accommodates=2 > postulates=1)
// Score = weighted sum, normalized to [0, 1]

func runEvaluator(ctx context.Context, e llm.LLMProvider, state ClaimEvalState) (float64, ClaimEvalState, error) {
    prompt := buildEvaluatorPrompt(state)   // includes A's analysis + B's critique
    response, err := e.Complete(ctx, prompt)
    if err != nil {
        return 0, state, err
    }

    score, updatedDebt, flags, reasoning, err := parseEvaluatorResponse(response)
    // parseEvaluatorResponse expects structured JSON from E

    state.EBPScore           = score
    state.WorkbenchFlags     = flags
    state.RemainingDebt      = updatedDebt
    state.EvaluatorReasoning = reasoning
    state.PromotionReady     = len(updatedDebt) == 0 && !state.FinalTruthFlag
    return score, state, nil
}
```

**Evaluator prompt structure (ask E to return JSON):**
```json
{
  "score": 0.73,
  "remaining_debt": ["needToyCheck", "needNullModel"],
  "workbench_flags": ["bridge_missing:ontology_to_mathematical"],
  "final_truth_language_detected": false,
  "reasoning": "..."
}
```

### 9.4 Wiring the Three Roles into Multi-LLM AB-MCTS

```go
// In cmd/evaluate/main.go:

fns := algo.GenerateFns[ebp.ClaimEvalState]{
    "constructive": ebp.GenerateA(anthropicProvider),
    "adversarial":  ebp.GenerateB(anthropicProvider), // or a different LLM
}

// Use the same provider for both or different ones:
// fns["adversarial"] = ebp.GenerateB(openAIProvider)

algo := algo.NewABMCTSA[ebp.ClaimEvalState](seed)
tree := tree.NewSearchTree[ebp.ClaimEvalState](rootState, slices.Collect(maps.Keys(fns)))

for i := range maxIterations {
    child, err := algo.Step(ctx, tree, fns)
    if err != nil { ... }
    log.Printf("iter %d | model=%s | score=%.3f | depth=%d | promotion_ready=%v",
        i, child.GeneratedBy, child.Score, child.Depth, child.State.PromotionReady)

    if child.State.PromotionReady {
        log.Println("EBP PROMOTION THRESHOLD MET — stopping early")
        break
    }
}

best := tree.BestNode()
fmt.Printf("Best analysis (score=%.3f):\n%+v\n", best.Score, best.State)
```

---

## 10. Checkpointing — `pkg/tree/checkpoint.go`

The Python library has this as a named feature. In Go, since the tree is JSON-serializable (as long as `S` is), it's straightforward:

```go
package tree

import (
    "encoding/json"
    "os"
)

// Save serializes the entire search tree to a JSON file.
// S must be json.Marshal-able (use struct tags on ClaimEvalState).
func (t *SearchTree[S]) Save(path string) error {
    t.mu.RLock()
    defer t.mu.RUnlock()
    data, err := json.MarshalIndent(t, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}

// LoadSearchTree deserializes a previously saved tree.
func LoadSearchTree[S any](path string) (*SearchTree[S], error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var t SearchTree[S]
    return &t, json.Unmarshal(data, &t)
}
```

Checkpoint after every N steps (or on signal) — critical given LLM call costs.

---

## 11. Key Differences from Python Version

| Python treequest | Go port |
|---|---|
| Immutable-style: `step()` returns new tree | Mutable: tree updated in place, mutex-guarded |
| Dynamic typing for State | `[S any]` generics; S must be JSON-serializable |
| `scipy.stats.beta` for Beta sampling | `gonum.org/v1/gonum/stat/distuv.Beta` |
| ABMCTS-M uses PyMC (MCMC) | Defer or skip ABMCTS-M initially; no Go equivalent |
| Python async for LLM calls | `context.Context` + goroutines for parallel steps |
| `pathlib.Path` checkpointing | `os.WriteFile` / `json.Marshal` |
| `dict` for `generate_fns` | `map[string]GenerateFn[S]` |

---

## 12. Non-Trivial Implementation Challenges

### 12.1 Beta Distribution Sampling Without gonum
If you want zero external deps, implement the Gamma-ratio method:

```go
import "math/rand"

// sampleGamma samples from Gamma(shape, 1) using Marsaglia-Tsang method.
func sampleGamma(rng *rand.Rand, shape float64) float64 { ... }

func sampleBeta(rng *rand.Rand, alpha, beta float64) float64 {
    x := sampleGamma(rng, alpha)
    y := sampleGamma(rng, beta)
    return x / (x + y)
}
```

This avoids gonum entirely but adds ~60 lines of numerical code. gonum is the cleaner path.

### 12.2 Concurrent Steps
If you want to run multiple `Step()` calls in parallel (to fully utilize LLM throughput):

- The tree mutation (child attachment + node list append) must be mutex-guarded ✓ (already shown)
- The Beta distribution updates during backprop must be per-node locked ✓
- The LLM calls themselves are safe to run concurrently since they are pure function calls

Parallel step loop:
```go
var wg sync.WaitGroup
for i := range concurrency {
    wg.Add(1)
    go func() {
        defer wg.Done()
        algo.Step(ctx, tree, fns)
    }()
}
wg.Wait()
```

### 12.3 Evaluator E as a Separate Concern
E is not a `GenerateFn` — it doesn't go in `GenerateFns`. E is called inside A and B's generate functions as a sub-step. This is clean because E's output determines the score that gets returned from GenerateFn, which is exactly what AB-MCTS uses for Thompson Sampling. E never appears in the search algorithm — it's encapsulated within the workers.

### 12.4 Root State Construction
The root state should contain the full paper text (or its parsed claim list). One recommended pattern: run a Flash Workbench pass on the paper first (separately, not in the tree) to extract individual claim-level items. Then launch a separate AB-MCTS tree per claim, or run all claims through a single tree where the root state contains the paper and the first generation step picks which claim to analyze.

---

## 13. Recommended Build Order

1. **`pkg/tree`** — `Node[S]`, `SearchTree[S]`, checkpoint. No external deps. Unit-testable with `S = string`.
2. **`pkg/algo/bandit.go`** — `BetaSampler` using gonum. Test: confirm Beta(1,1) is uniform, Beta(10,1) skews right.
3. **`pkg/algo/abmcts_a.go`** — the step loop. Test with a dummy `GenerateFn` (no LLM, just `rand.Float64()`).
4. **`pkg/llm/provider.go` + `anthropic.go`** — wrap the Anthropic `/v1/messages` endpoint.
5. **`pkg/ebp/state.go` + `prompts.go`** — design the EBP/Workbench2 prompts for A, B, E. This is the domain-hardest part.
6. **`pkg/ebp/evaluator.go`** — define the JSON schema E must return, write the parser.
7. **`pkg/ebp/worker_a.go` + `worker_b.go`** — wire A and B into `GenerateFn[ClaimEvalState]`.
8. **`cmd/evaluate/main.go`** — CLI entrypoint with flags for paper path, iterations, seed, checkpoint path.

---

## 14. EBP Debt Item → Workbench2 Flag Mapping (for Evaluator E Prompt)

Include this table in E's system prompt so E knows what to look for:

| EBP Debt Item | Workbench2 trigger |
|---|---|
| `needMap` | Missing domain/codomain/translation in any structural claim |
| `needInvariant` | Survival claim without stated invariant |
| `needToyCheck` | Map+invariant present but no finite model test |
| `needNullModel` | Uniqueness/necessity claim without a null model or explicit disclaimer |
| `needObstruction` | Known blocker (Lorentz recovery, Born rule, anomaly) not addressed |
| `needFaithfulnessReview` | Lean/formal structure used but faithfulness of mapping not confirmed |
| `FinalTruthFlag` | Any promoted claim containing "proves", "demonstrates that nature is", "establishes the fundamental truth of" |
| Workbench `bridge_missing` | Claim type crossing (e.g. mathematical → physical ontology) without bridge principle |
| Workbench `accommodation` | Result labeled as derivation when it is actually a parameter fit |
| Workbench `undeclared_assumption` | Hidden assumption identified by B not acknowledged by claim |

Score formula suggestion for E:
```
base = (6 - len(remaining_debt)) / 6.0        # fraction of debt retired
bridge_penalty = 0.15 * len(workbench_flags)   # per flag
final_truth_penalty = 0.5 if FinalTruthFlag else 0
score = clamp(base - bridge_penalty - final_truth_penalty, 0, 1)
```

---

## 15. Minimum Viable First Run

The smallest thing that proves the Go port works end-to-end (before writing EBP prompts):

```go
type SimpleState = string

func dummyGenerate(_ context.Context, parent *SimpleState) (SimpleState, float64, error) {
    if parent == nil {
        return "root analysis", rand.Float64(), nil
    }
    return "refined: " + *parent, rand.Float64(), nil
}

fns := algo.GenerateFns[SimpleState]{
    "model_a": dummyGenerate,
    "model_b": dummyGenerate,
}
algo := algo.NewABMCTSA[SimpleState](42)
tree := tree.NewSearchTree[SimpleState]("initial claim", []string{"model_a", "model_b"})

for range 20 {
    child, _ := algo.Step(context.Background(), tree, fns)
    fmt.Printf("depth=%d model=%s score=%.3f\n", child.Depth, child.GeneratedBy, child.Score)
}
```

Get this working before touching the LLM layer. The tree growth and Thompson Sampling update logic is independently testable.

---

*Document scope: Implementation architecture and Go-specific design decisions. Algorithm math sourced from Sakana AI's paper (arXiv 2503.04412) and treequest library (Apache 2.0). EBP v2.1 doctrine per Pithom Labs specification. Workbench2 per compact edition v2.0.*
