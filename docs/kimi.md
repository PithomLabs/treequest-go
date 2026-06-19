
# Go Port of TreeQuest + EBP v2.1/Workbench2 Use Case — Implementation Overview

## 1. TreeQuest Core Library (Go) — Design Principles

### 1.1 Core Abstractions

The Python TreeQuest library has these key abstractions that translate cleanly to Go:

```
State              -> interface{} (or generic T)
Action/Generator   -> func(parentState T) (T, float64, error)
Algorithm          -> interface with InitTree(), Step(), AskBatch(), Tell()
SearchTree         -> immutable tree structure (functional style)
Trial              -> struct{Action string, ParentState T, TrialID string}
```

### 1.2 Why Go?

- **Concurrency**: Go's goroutines + channels are perfect for the batched ask-tell pattern
- **Type Safety**: Generics (Go 1.18+) let us make State type-safe without sacrificing flexibility
- **Performance**: Compiled, low memory overhead for large search trees
- **Tooling**: Excellent for building CLI tools (your `ebp` commands)
- **Deployment**: Single binary, easy to containerize

### 1.3 Library Structure

```
treequest/
├── algo/
│   ├── base.go              # Algorithm interface
│   ├── abmcts_a.go          # AB-MCTS with Node Aggregation
│   ├── abmcts_m.go          # AB-MCTS with Mixed Models (Bayesian)
│   ├── standard_mcts.go     # Standard MCTS
│   └── tree_of_thoughts.go  # Tree of Thoughts BFS
├── tree/
│   ├── node.go              # Tree node structure
│   ├── tree.go              # Search tree operations
│   └── backup.go            # Score backup strategies
├── bandit/
│   ├── thompson.go          # Thompson Sampling
│   └── ucb.go               # UCB1 / PUCT
├── batch/
│   ├── trial.go             # Trial struct
│   └── executor.go         # Concurrent trial execution
├── render/
│   ├── html.go              # HTML visualization
│   └── dot.go               # Graphviz DOT format
└── types.go                 # Shared types and interfaces
```

---

## 2. Key Algorithm: AB-MCTS-A (Node Aggregation)

This is the simpler and more practical variant for your EBP use case.

### 2.1 Algorithm Overview

AB-MCTS-A uses Thompson Sampling to decide at each node:
- **GEN**: Generate a new child (explore width)
- **CONT**: Continue refining an existing child (explore depth)

Each node maintains posterior distributions over:
- `GEN` node quality (Beta or Gaussian distribution)
- Each `CONT` child quality

Thompson Sampling draws a sample from each posterior and picks the action with highest sampled value.

### 2.2 Go Implementation Sketch

```go
package algo

import (
    "math"
    "math/rand"
)

// Node represents a node in the search tree
type Node[T any] struct {
    ID           string
    ParentID     string
    State        T
    Score        float64
    Visits       int

    // GEN node statistics (for generating new children)
    GenAlpha     float64  // Beta distribution alpha
    GenBeta      float64  // Beta distribution beta
    GenVisits    int

    // CONT children
    Children     []*Node[T]

    // Action that created this node
    Action       string
    Depth        int
}

// ABMCTSA implements AB-MCTS with Node Aggregation
type ABMCTSA[T any] struct {
    // Prior parameters for Beta distribution
    PriorAlpha   float64
    PriorBeta    float64

    // Threshold for when to stop expanding a node
    MaxDepth     int
}

func NewABMCTSA[T any]() *ABMCTSA[T] {
    return &ABMCTSA[T]{
        PriorAlpha: 1.0,
        PriorBeta:  1.0,
        MaxDepth:   10,
    }
}

// SelectExpansionTarget traverses the tree using Thompson Sampling
func (a *ABMCTSA[T]) SelectExpansionTarget(tree *SearchTree[T]) *Node[T] {
    node := tree.Root

    for !node.IsLeaf() {
        // Sample from GEN posterior
        genSample := a.sampleBeta(node.GenAlpha, node.GenBeta)

        // Sample from best CONT child posterior
        var bestContSample float64 = -1
        var bestChild *Node[T]

        for _, child := range node.Children {
            // Each child has its own Beta posterior
            childSample := a.sampleBeta(
                a.PriorAlpha + float64(child.Visits) * child.Score,
                a.PriorBeta + float64(child.Visits) * (1 - child.Score),
            )
            if childSample > bestContSample {
                bestContSample = childSample
                bestChild = child
            }
        }

        if genSample > bestContSample {
            // Choose GEN: expand this node with a new child
            return node
        }

        // Choose CONT: descend to best child
        node = bestChild
    }

    return node
}

func (a *ABMCTSA[T]) sampleBeta(alpha, beta float64) float64 {
    // Use math/rand or math/big for Beta sampling
    // For simplicity, use Gamma distribution method
    x := randGamma(alpha, 1)
    y := randGamma(beta, 1)
    return x / (x + y)
}
```

### 2.3 Ask-Tell Interface (Batched)

```go
package batch

import "context"

// Trial represents a pending expansion
type Trial[T any] struct {
    TrialID      string
    Action       string      // e.g., "worker-A", "worker-B", "evaluator-E"
    ParentNodeID string
    ParentState  T
    Depth        int
}

// Result is returned after executing a trial
type Result[T any] struct {
    TrialID  string
    NewState T
    Score    float64  // Must be in [0, 1]
    Error    error
}

// Executor runs trials concurrently
type Executor[T any] struct {
    workers int
}

func (e *Executor[T]) Run(ctx context.Context, trials []Trial[T], 
    generators map[string]func(T) (T, float64, error)) []Result[T] {

    results := make([]Result[T], len(trials))

    var wg sync.WaitGroup
    sem := make(chan struct{}, e.workers)

    for i, trial := range trials {
        wg.Add(1)
        sem <- struct{}{}

        go func(idx int, t Trial[T]) {
            defer wg.Done()
            defer func() { <-sem }()

            gen, ok := generators[t.Action]
            if !ok {
                results[idx] = Result[T]{TrialID: t.TrialID, Error: fmt.Errorf("unknown action: %s", t.Action)}
                return
            }

            newState, score, err := gen(t.ParentState)
            results[idx] = Result[T]{
                TrialID:  t.TrialID,
                NewState: newState,
                Score:    score,
                Error:    err,
            }
        }(i, trial)
    }

    wg.Wait()
    return results
}
```

---

## 3. EBP v2.1 + Workbench2 Use Case (Separate Codebase)

### 3.1 Architecture

```
ebp-evaluator/          # Separate repo from treequest
├── cmd/
│   └── ebp/
│       └── main.go       # CLI: ebp capture, ebp status, ebp retire, etc.
├── internal/
│   ├── paper/            # Paper ingestion & parsing
│   │   ├── loader.go     # PDF/text loading
│   │   └── extractor.go  # Claim extraction
│   ├── workbench/        # Workbench2 analysis engine
│   │   ├── flash.go      # Flash Workbench
│   │   ├── core.go       # Core Workbench (7 checks)
│   │   └── extended.go   # Extended Workbench
│   ├── ebp/              # EBP v2.1 protocol
│   │   ├── idea.go       # Idea struct, debt tracking
│   │   ├── debt.go       # DebtItem, retire/add logic
│   │   └── ledger.go     # Living ledger
│   ├── llm/              # LLM client abstractions
│   │   ├── client.go     # Interface: Generate(ctx, prompt) -> string
│   │   ├── openai.go     # OpenAI adapter
│   │   ├── anthropic.go  # Claude adapter
│   │   └── local.go      # Local model adapter
│   ├── search/           # AB-MCTS integration
│   │   ├── state.go      # Search state (current analysis context)
│   │   ├── actions.go    # Action definitions for A, B, E
│   │   └── scorer.go     # Score evaluation for tree nodes
│   └── report/           # Output generation
│       └── formatter.go  # Markdown/JSON report generation
├── prompts/              # Prompt templates
│   ├── worker_a_constructive.md
│   ├── worker_b_adversarial.md
│   └── evaluator_e.md
└── go.mod
```

### 3.2 Search State Design

```go
package search

// AnalysisState is the node state in the MCTS tree
type AnalysisState struct {
    // The paper being analyzed
    PaperID      string
    PaperText    string

    // Current claim under analysis
    ClaimIndex   int
    ClaimText    string

    // Workbench analysis so far
    FlashReport  *workbench.FlashReport
    CoreReport   *workbench.CoreReport

    // EBP tracking
    Idea         *ebp.Idea
    DebtRetired  []ebp.DebtItem

    // Turn tracking for budget control
    TurnCount    int
    MaxTurns     int  // <-- YOUR BUDGET CONTROL FLAG

    // Current evaluator assessment
    Score        float64  // [0,1] from evaluator E

    // History of turns
    TurnHistory  []TurnRecord
}

type TurnRecord struct {
    TurnNumber   int
    Actor        string  // "A", "B", or "E"
    Prompt       string
    Response     string
    ScoreDelta   float64
}
```

### 3.3 Action Definitions (A, B, E)

```go
package search

// ActionType identifies which worker/evaluator is acting
type ActionType string

const (
    ActionWorkerA      ActionType = "worker-A"      // Constructive
    ActionWorkerB      ActionType = "worker-B"      // Adversarial
    ActionEvaluatorE   ActionType = "evaluator-E"   // Assessment
)

// ActionConfig holds prompt templates and LLM config for each actor
type ActionConfig struct {
    Action      ActionType
    LLMClient   llm.Client
    PromptTpl   *template.Template
    Temperature float64
}

// GenerateFn creates the generator function for treequest
func (ac *ActionConfig) GenerateFn() func(AnalysisState) (AnalysisState, float64, error) {
    return func(parent AnalysisState) (AnalysisState, float64, error) {
        // Check budget
        if parent.TurnCount >= parent.MaxTurns {
            return parent, parent.Score, fmt.Errorf("budget exhausted")
        }

        // Build prompt from template
        prompt, err := ac.buildPrompt(parent)
        if err != nil {
            return parent, 0, err
        }

        // Call LLM
        response, err := ac.LLMClient.Generate(context.Background(), prompt)
        if err != nil {
            return parent, 0, err
        }

        // Parse response and update state
        newState := parent.Clone()
        newState.TurnCount++
        newState.TurnHistory = append(newState.TurnHistory, TurnRecord{
            TurnNumber: newState.TurnCount,
            Actor:      string(ac.Action),
            Prompt:     prompt,
            Response:   response,
        })

        // If evaluator E, extract score
        if ac.Action == ActionEvaluatorE {
            score, err := extractScore(response)
            if err != nil {
                return newState, 0, err
            }
            newState.Score = score
            return newState, score, nil
        }

        // For workers A/B, score comes from subsequent evaluator E turn
        // Return intermediate score (will be updated when E evaluates)
        return newState, parent.Score, nil
    }
}
```

### 3.4 Evaluator E Scoring Logic

```go
package search

// EvaluatorScorer extracts a [0,1] score from evaluator E's response
func extractScore(response string) (float64, error) {
    // Strategy 1: Look for explicit score marker
    // e.g., "SCORE: 0.85" or "Rating: 8.5/10"

    // Strategy 2: Parse structured output (JSON)
    // e.g., {"score": 0.85, "reasoning": "..."}

    // Strategy 3: Use LLM to extract score if not explicit

    // For EBP specifically, score based on:
    // - Debt retirement progress (0.0 = none, 1.0 = all)
    // - Claim clarity (0-1)
    // - Inverse constraint strength (0-1)
    // - Risk flags triggered (penalty)

    var score float64
    // ... parsing logic ...
    return math.Max(0, math.Min(1, score)), nil
}

// CompositeScore for EBP evaluation
type CompositeScore struct {
    DebtCoverage     float64  // % of debt items addressed
    ClaimClarity     float64  // How well-defined is the claim
    BridgeStrength   float64  // Quality of bridge principles
    InverseConstraint float64  // Falsifiability score
    RiskPenalty      float64  // Penalty for triggered risk flags
}

func (cs CompositeScore) Final() float64 {
    raw := (cs.DebtCoverage*0.3 + 
            cs.ClaimClarity*0.2 + 
            cs.BridgeStrength*0.2 + 
            cs.InverseConstraint*0.3) - cs.RiskPenalty
    return math.Max(0, math.Min(1, raw))
}
```

---

## 4. Budget Control: Turn Limit Flag

### 4.1 Is This Good Design?

**Yes, with caveats.** Here's the assessment:

**Pros:**
- Simple, predictable cost control
- Maps directly to API call budgeting
- Easy to reason about: "analyze this paper with max 20 turns"
- Prevents runaway costs from deep MCTS exploration
- Aligns with EBP v2.1's "accounting must never become the work"

**Cons / Considerations:**
- Fixed turns may cut off promising branches prematurely
- Better: use a **budget** (max API calls) rather than turns, since different actions may have different costs
- Even better: make it **adaptive** — stop when score improvement plateaus

**Recommended Design:**

```go
type BudgetConfig struct {
    // Hard limit on total LLM calls across all actors
    MaxTotalCalls int

    // Per-actor limits (optional)
    MaxWorkerACalls int
    MaxWorkerBCalls int
    MaxEvaluatorCalls int

    // Early stopping: stop if no improvement after N evaluator calls
    EarlyStopPatience int

    // Minimum score threshold to accept a result
    MinScoreThreshold float64
}
```

### 4.2 Integration with TreeQuest

```go
// In your main loop:
budget := BudgetConfig{
    MaxTotalCalls:     30,  // e.g., 10 A + 10 B + 10 E
    EarlyStopPatience: 5,
    MinScoreThreshold: 0.7,
}

algo := treequest.NewABMCTSA[AnalysisState]()
tree := algo.InitTree()

generators := map[string]func(AnalysisState) (AnalysisState, float64, error){
    "worker-A":    actionA.GenerateFn(),
    "worker-B":    actionB.GenerateFn(),
    "evaluator-E": actionE.GenerateFn(),
}

for step := 0; step < budget.MaxTotalCalls; step++ {
    // Ask for next batch of trials
    tree, trials := algo.AskBatch(tree, 1, []string{"worker-A", "worker-B", "evaluator-E"})

    // Execute (concurrently if batch_size > 1)
    results := executor.Run(ctx, trials, generators)

    // Tell results back
    for _, res := range results {
        tree = algo.Tell(tree, res.TrialID, res.NewState, res.Score)
    }

    // Check early stopping
    if shouldEarlyStop(tree, budget) {
        break
    }
}

bestState, bestScore := treequest.TopK(tree, algo, 1)[0]
```

---

## 5. Prompt Engineering for EBP + Workbench

### 5.1 Worker A (Constructive) Prompt Template

```markdown
# Worker A: Constructive Analysis

You are analyzing a physics research paper through the Elephant Bridge Protocol v2.1 and Workbench2 framework.

## Current State
Paper: {{.PaperTitle}}
Claim under analysis: {{.ClaimText}}
Current debt status: {{.DebtStatus}}
Previous analysis: {{.PreviousAnalysis}}

## Your Task
Act as a constructive theorist. Your goal is to:
1. Clarify the claim and its type (math, ontology, dynamics, observable, etc.)
2. Propose how to retire one debt item
3. Identify one bridge principle needed
4. Suggest the next smallest useful move

## Rules
- Do not claim final truth
- Use EBP-allowed language: "candidate", "toy-supported", "formal obligation"
- One useful move at a time
- If debt is already clear, propose a map, invariant, or toy check

## Output Format
```
ANALYSIS: <your constructive analysis>
DEBT_RETIRED: <which debt item is addressed, or "none">
BRIDGE_PROPOSED: <bridge principle, or "none">
NEXT_MOVE: <one sentence describing the next smallest useful move>
```
```

### 5.2 Worker B (Adversarial) Prompt Template

```markdown
# Worker B: Adversarial Review

You are analyzing a physics research paper through the Elephant Bridge Protocol v2.1 and Workbench2 framework.

## Current State
Paper: {{.PaperTitle}}
Claim under analysis: {{.ClaimText}}
Current debt status: {{.DebtStatus}}
Worker A's last move: {{.WorkerALastMove}}

## Your Task
Act as an adversarial reviewer. Your goal is to:
1. Challenge one assumption
2. Identify one risk flag (R1-R10) that may apply
3. Propose one obstruction or null model
4. Check if any claim type crossing happened without a bridge

## Rules
- Do not reject the idea; only create new debt
- Use EBP language: "this creates debt", "obstruction filed", "faithfulness concern"
- Be specific: one sentence per challenge
- If the claim is too vague, ask for clarification (creates needFaithfulnessReview debt)

## Output Format
```
CHALLENGE: <specific challenge>
RISK_FLAG: <R1-R10 or "none">
NEW_DEBT: <debt item created, or "none">
OBSTRUCTION: <obstruction statement, or "none">
```
```

### 5.3 Evaluator E Prompt Template

```markdown
# Evaluator E: Assessment

You are evaluating the current state of analysis for a physics research paper.

## Current State
Paper: {{.PaperTitle}}
Claim: {{.ClaimText}}
Turn history:
{{.TurnHistory}}

Debt status:
{{.DebtStatus}}

## Your Task
Assess the quality of the analysis so far. Consider:
1. How many debt items have been meaningfully addressed? (0-6)
2. Is the claim clearly typed? (math/ontology/dynamics/etc.)
3. Are bridge principles present for cross-type moves?
4. Are risk flags appropriately flagged?
5. Is the analysis making progress or going in circles?

## Scoring
Provide a score from 0.0 to 1.0 where:
- 0.0 = No progress, confused, or overclaimed
- 0.5 = Some debt retired, but gaps remain
- 1.0 = All current debt retired, clear claim, strong bridges, no final-truth language

## Output Format
```json
{
  "score": 0.75,
  "debt_coverage": 0.6,
  "claim_clarity": 0.8,
  "bridge_strength": 0.7,
  "inverse_constraint": 0.6,
  "risk_flags_triggered": ["R2", "R7"],
  "reasoning": "<one paragraph explaining the score>",
  "recommendation": "<what should happen next: A or B turn>"
}
```
```

---

## 6. Output: Research Paper with Turn Flag

### 6.1 Report Structure

```go
type EBPReport struct {
    Metadata struct {
        PaperID       string
        PaperTitle    string
        AnalysisDate  time.Time
        MaxTurnsUsed  int       // <-- YOUR FLAG
        TotalTurns    int
        BestScore     float64
        Algorithm     string    // "AB-MCTS-A" or "AB-MCTS-M"
    }

    // The best analysis path found
    BestPath struct {
        Score         float64
        Turns         []TurnRecord
        FinalState    AnalysisState
    }

    // All explored paths (for transparency)
    AllPaths      []PathSummary

    // EBP-specific output
    Idea          ebp.Idea
    DebtStatus    []ebp.DebtItem
    Promoted      bool

    // Workbench output
    FlashReport   *workbench.FlashReport
    CoreReport    *workbench.CoreReport

    // Risk flags
    RiskFlags     []workbench.RiskFlag

    // Verdict
    Verdict       workbench.Verdict
}
```

### 6.2 Markdown Report Template

```markdown
# EBP v2.1 / Workbench2 Analysis Report

**Paper:** {{.Metadata.PaperTitle}}  
**Analysis Date:** {{.Metadata.AnalysisDate}}  
**Algorithm:** {{.Metadata.Algorithm}}  
**Budget:** {{.Metadata.TotalTurns}} / {{.Metadata.MaxTurnsUsed}} turns  
**Best Score:** {{.Metadata.BestScore}}

---

## Executive Summary

{{.Verdict.Summary}}

**Promoted:** {{.Promoted}}  
**Status:** {{if .Promoted}}Serious candidate artifact{{else}}Alive with debt{{end}}

---

## Claim Under Analysis

{{.Idea.Claim}}

**Owner:** {{.Idea.Owner}}  
**Claim Type:** {{.FlashReport.ClaimType}}  
**Maturity Stage:** {{.FlashReport.MaturityStage}}  
**Function Class:** {{.FlashReport.FunctionClass}}

---

## Debt Status

| Debt Item | Status | Evidence |
|-----------|--------|----------|
{{range .DebtStatus}}
| {{.Name}} | {{if .Retired}}✅ Retired{{else}}⏳ Unpaid{{end}} | {{.Evidence}} |
{{end}}

---

## Analysis Path (Best)

{{range .BestPath.Turns}}
### Turn {{.TurnNumber}}: {{.Actor}}

{{.Response}}

---
{{end}}

## Risk Flags

{{range .RiskFlags}}
- **{{.Code}}: {{.Name}}** — {{.Description}}
{{end}}

---

## Verdict

**Earned:** {{.Verdict.Earned}}  
**Not Earned:** {{.Verdict.NotEarned}}  
**Strongest Next Test:** {{.Verdict.StrongestNextTest}}  
**Extended Analysis Warranted:** {{.Verdict.ShouldEscalate}}

---

*Generated via AB-MCTS with Worker A (Constructive), Worker B (Adversarial), and Evaluator E (Assessment).*
```

---

## 7. Implementation Roadmap

### Phase 1: TreeQuest Core (2-3 weeks)
1. Port base types and tree structure
2. Implement AB-MCTS-A (simpler variant)
3. Implement batched ask-tell interface
4. Add HTML/DOT visualization
5. Unit tests with toy problems

### Phase 2: EBP Integration (1-2 weeks)
1. Define AnalysisState and action configs
2. Implement prompt templates
3. Build LLM client abstraction
4. Wire up A-B-E loop
5. Score extraction and composite scoring

### Phase 3: CLI and Reports (1 week)
1. `ebp capture` — ingest paper
2. `ebp analyze` — run AB-MCTS
3. `ebp status` — show debt
4. `ebp report` — generate markdown
5. Budget control flags

### Phase 4: Polish (ongoing)
1. Checkpoint/resume for long runs
2. Multi-paper batch analysis
3. Lean debt-payment module integration
4. Extended Workbench auto-escalation

---

## 8. Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| State type | `interface{}` vs generics | Use generics `T any` for type safety; `AnalysisState` is concrete |
| Tree mutability | Immutable (functional) | Easier to checkpoint, resume, and debug; aligns with Python version |
| Concurrency | Goroutines + channels | Native Go, clean batched execution |
| LLM abstraction | Interface + adapters | Not tied to any provider; easy to swap |
| Scoring | [0,1] normalized | Required by TreeQuest; composite for EBP |
| Budget control | Max turns + early stop | Simple, predictable, cost-controlled |
| Report format | Markdown | Human-readable, version-controllable, diffable |
| Separation | treequest vs ebp-evaluator | Clean boundaries, reusable library |

---

## 9. Sample Go Module Setup

```go
// treequest/go.mod
module github.com/yourname/treequest

go 1.22

require (
    github.com/google/uuid v1.6.0
)
```

```go
// ebp-evaluator/go.mod
module github.com/yourname/ebp-evaluator

go 1.22

require (
    github.com/yourname/treequest v0.1.0
    github.com/sashabaranov/go-openai v1.24.0
    github.com/anthropics/anthropic-sdk-go v0.2.0
    github.com/spf13/cobra v1.8.0
)
```
