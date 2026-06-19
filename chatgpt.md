## prompt

sakana ai has a treequest library at https://github.com/SakanaAI/treequest 

read details at https://sakana.ai/ab-mcts/

I like to port treequest to golang, the aim is to make it general purpose and my use case is: given a physics research paper I like to evaluate its claims per EBP 2.1 and Workbench2 guidelines such that I have at least one LLM as evaluator E of 2 worker LLMs (constructive A and adversarial B) that the evaluator E LLM assess A and B input one at a time per AB-MCTS algorithm and treequest library

give me an overview on how to implement the treequest library to golang




Yes. The right Go port is **not** “rewrite Sakana’s Python line-for-line.” It should be a **Go-native TreeQuest-style search engine** with the same core abstraction: a search tree over arbitrary states, configurable actions/generators, normalized scores, `step` and `ask/tell` APIs, checkpointing, and AB-MCTS variants. TreeQuest’s public API is already generic in spirit: each node has a user-defined state, generation returns `(state, score)`, scores are expected in `[0,1]`, and multiple action types can represent different LLMs, prompts, or generation strategies. ([GitHub][1])

For your use case, the “state” is not a game position or coding answer. It is a **paper-claim audit state**: extracted claims, worker A’s constructive analysis, worker B’s adversarial analysis, evaluator E’s score, open EBP debts, Workbench2 risk flags, and next audit actions.

## 1. What you are porting conceptually

AB-MCTS is designed to balance two inference-time search directions: **go wider** by generating new candidate responses, or **go deeper** by refining promising existing ones. Sakana’s paper frames AB-MCTS as a generalization of repeated sampling plus multi-turn refinement, where each node can adaptively branch rather than using a fixed branching factor. ([arXiv][2]) The Sakana blog explains the same idea in practical terms: repeated sampling explores width, sequential refinement explores depth, and AB-MCTS decides dynamically when to do either. ([Sakana AI][3])

TreeQuest exposes this through a flexible library API: initialize a search tree, define generation functions, run `step`, or use `ask_batch` / `tell` for parallel generation and evaluation. It also supports AB-MCTS-A, AB-MCTS-M, Multi-LLM actions, checkpointing, resuming, and tree visualization. ([GitHub][1])

For Go, implement the **algorithmic contract**, not the Python dependency stack. In particular, AB-MCTS-M in Python uses PyMC mixed modeling, which you probably do **not** want to clone first. TreeQuest itself notes AB-MCTS-A uses node aggregation, while AB-MCTS-M uses mixed modeling and has extra dependencies. ([GitHub][1]) For a Go MVP, start with **AB-MCTS-A-style adaptive branching** using simple Beta/Gaussian posterior logic, then add a heavier mixed-model variant later only if the audit results justify it.

## 2. Target architecture in Go

Use a small set of packages:

```text
treequest-go/
  internal/
    mathx/              # beta, gaussian, UCB, Thompson sampling helpers
  treequest/
    tree.go             # Tree, Node, Edge, Trial, IDs
    algo.go             # Algorithm interfaces
    ask_tell.go         # ask, askBatch, tell semantics
    topk.go             # best-state extraction
    checkpoint.go       # JSON/gob checkpointing
    abmctsa.go          # MVP adaptive branching
    standard_mcts.go    # baseline
    repeated.go         # repeated sampling baseline
    sequential.go       # sequential refinement baseline
  llm/
    client.go           # provider-neutral LLM interface
    openai.go           # optional provider adapter
    anthropic.go        # optional provider adapter
    gemini.go           # optional provider adapter
  ebpaudit/
    state.go            # paper audit state
    actions.go          # constructive/adversarial/refine actions
    evaluator.go        # evaluator E scoring
    workbench.go        # Workbench2 schema mapping
    prompts.go          # A/B/E prompts
  cmd/
    treequest-ebp/
      main.go
```

Keep the **core tree search package domain-neutral**. EBP and Workbench2 belong in `ebpaudit`, not inside `treequest`.

This respects EBP’s tool-agnostic rule: EBP can be implemented in local files, Git repos, PTW, Lean, databases, notebooks, or other tools; no one implementation owns it.  It also respects Workbench2’s anti-bloat rule: use the smallest Workbench capable of exposing the real issue, with Flash, Core, and Extended levels rather than forcing every paper into heavy machinery. 

## 3. Core Go interfaces

The most important design is the generic search interface.

```go
package treequest

import "context"

type Score float64

type ScoredState[S any] struct {
	State S
	Score Score
	Meta  map[string]any
}

type Action[S any] interface {
	Label() string
	Generate(ctx context.Context, parent *S) (ScoredState[S], error)
}

type TrialID string
type NodeID string

type Trial[S any] struct {
	ID          TrialID
	ActionLabel string
	ParentID    NodeID
	ParentState *S
}

type Algorithm[S any] interface {
	InitTree() *Tree[S]
	Step(ctx context.Context, t *Tree[S], actions []Action[S]) (*Tree[S], error)
	Ask(t *Tree[S], actions []Action[S]) (*Tree[S], Trial[S], error)
	AskBatch(t *Tree[S], n int, actions []Action[S]) (*Tree[S], []Trial[S], error)
	Tell(t *Tree[S], trialID TrialID, result ScoredState[S]) (*Tree[S], error)
	TopK(t *Tree[S], k int) []NodeScore[S]
}
```

The `ask/tell` split is worth copying conceptually because it is perfect for LLM workflows. TreeQuest’s README says `ask_batch` returns trials with action, parent state, and trial id, while `tell` updates the tree with the result tied to the trial id; this makes evaluation order-independent and supports concurrency. ([GitHub][1]) That matters because worker A, worker B, and evaluator E will have variable latency.

## 4. The paper-audit state

For your EBP/Workbench use case, define a state like this:

```go
type PaperAuditState struct {
	PaperID        string
	ClaimID        string
	Depth          int

	SourceExcerpt  string
	ClaimText      string

	Constructive   *WorkerOutput
	Adversarial    *WorkerOutput
	Evaluation     *EvaluatorOutput

	EBP            EBPLedger
	Workbench      WorkbenchReport

	OpenQuestions  []string
	NextMoves      []string
	History        []AuditStep
}

type WorkerOutput struct {
	Role           string // "constructive_A" or "adversarial_B"
	Text           string
	ClaimRefs      []string
	ProposedDebts  []string
	RiskFlags      []string
	Confidence     string
}

type EvaluatorOutput struct {
	Score          float64 // normalized [0,1]
	Rationale      string
	EvidenceStatus map[string]string
	OverclaimFlags []string
	RetiredDebt    []string
	NewDebt         []string
	PromotionBlock bool
}

type EBPLedger struct {
	NeedMap                string
	NeedInvariant          string
	NeedToyCheck           string
	NeedNullModel          string
	NeedObstruction        string
	NeedFaithfulnessReview string
	ContainsFinalTruth     bool
	PromotionStatus        string // alive, unpromoted, promoted_for_scope, reopened
}

type WorkbenchReport struct {
	FunctionClass       string
	MaturityStage       string
	ClaimType           string
	Assumptions         []string
	Constraints         []string
	BridgePrinciples    []string
	InverseConstraints  []string
	RivalDifferential   []string
	ExplanationLevel    string
	RiskFlags           []string
	Incompleteness      []string
}
```

The evaluator should score the audit **not** by “which worker sounds smarter,” but by Workbench/EBP quality: claim typing, assumption separation, bridge clarity, inverse constraints, risk flags, incompleteness, evidence discipline, and overclaim control. Workbench2 explicitly says the Workbench is not a truth machine; it clarifies claims, assumptions, constraints, missing bridges, what would break the claim, and what remains unresolved. 

## 5. Actions for A/B/E AB-MCTS

Your actions should be semantically meaningful, not just model names.

```text
Action 1: A_initial_constructive
Action 2: B_initial_adversarial
Action 3: E_evaluate_A
Action 4: E_evaluate_B
Action 5: A_refine_after_E
Action 6: B_refine_after_E
Action 7: E_synthesize_pair
Action 8: E_file_new_debt
Action 9: E_select_next_smallest_move
```

But for the first version, keep it simpler:

```text
A_constructive
B_adversarial
E_evaluate
E_synthesize
```

A good AB-MCTS mapping:

```text
go wider  = generate another independent A/B audit path for the same claim
go deeper = refine a promising existing audit after evaluator feedback
```

In paper-audit language:

```text
wide branch:
  "Give me a fresh constructive reading of claim C under Core Workbench."

deep branch:
  "Improve this existing audit by repairing evaluator-flagged gaps."
```

This fits AB-MCTS directly. The paper says AB-MCTS decides at each node whether to generate a new candidate or refine an existing one, using external feedback signals. ([arXiv][2]) Your external feedback signal is evaluator E’s score plus EBP/Workbench violations.

## 6. Scoring: do not score truth; score audit quality

This is critical.

The evaluator E must not return “this paper is true” or “this claim is false.” It should return a normalized score for **audit usefulness**.

Suggested score components:

```text
0.20 source grounding
0.15 claim typing correctness
0.15 Workbench2 completeness
0.15 EBP debt visibility
0.10 adversarial strength
0.10 no-overclaim discipline
0.10 next smallest useful move quality
0.05 readability / compactness
```

Penalty triggers:

```text
-0.25 claims final truth
-0.20 treats author claim as established fact
-0.15 upgrades accommodation to derivation
-0.15 treats metaphor as ontology
-0.10 missing incompleteness section
-0.10 no rival differential
-0.10 no inverse constraint
```

This is directly aligned with Workbench2’s evidence discipline: do not convert “the paper claims X” into “X is established,” and keep separate what the source says, what the analyst verifies, and whether X is established.  It also matches the Workbench LLM rule: LLMs are clerks, not authorities; they may extract claims, classify, flag missing bridges, and draft reports, but may not decide final truth or certify a theory. 

## 7. AB-MCTS-A MVP algorithm in Go

Implement AB-MCTS-A first.

The key idea is to treat every real node as having two “choice families”:

```text
GEN: create a new child from this node, i.e. go wider
REFINE/SELECT CHILD: continue from an existing child, i.e. go deeper
```

Sakana’s paper describes introducing a GEN node as a child of every node to explicitly represent generating new child nodes, so selecting the GEN child expands the parent with a new child. ([arXiv][2]) You can implement this without literally storing GEN as a normal node; in Go, you can represent it as a virtual candidate during selection.

Basic flow:

```text
1. Selection:
   Start at root.
   At each node, estimate value of:
   - generating a new child from this node;
   - descending into each existing child.
   Use Thompson sampling or UCB-like selection.

2. Expansion:
   If GEN selected, create a Trial for one action at the selected parent.
   If child selected, continue selection from that child.

3. Generation:
   Run worker/evaluator action outside the algorithm.

4. Tell:
   Insert new child node with state and score.
   Backpropagate score statistics to ancestors.

5. TopK:
   Return best-scoring states, optionally with audit metadata.
```

For MVP, you can use Beta distributions if scores are bounded `[0,1]`:

```go
type Posterior struct {
	Alpha float64
	Beta  float64
}

func (p Posterior) Mean() float64 {
	return p.Alpha / (p.Alpha + p.Beta)
}

func (p *Posterior) Update(score float64) {
	if score < 0 { score = 0 }
	if score > 1 { score = 1 }
	p.Alpha += score
	p.Beta += 1 - score
}
```

Then implement Thompson sampling with a Beta sampler. If you do not want to add dependencies, start with UCB and add Thompson later. If you use Gonum, `distuv.Beta` can sample.

## 8. Concurrency model

Go’s concurrency is a major reason this port is attractive.

Use `ask_batch` to produce trials. Then process trials concurrently:

```go
trials := algo.AskBatch(tree, batchSize, actions)

var wg sync.WaitGroup
results := make(chan TrialResult, len(trials))

for _, tr := range trials {
	wg.Add(1)
	go func(tr Trial[PaperAuditState]) {
		defer wg.Done()
		action := actionByLabel[tr.ActionLabel]
		result, err := action.Generate(ctx, tr.ParentState)
		results <- TrialResult{TrialID: tr.ID, Result: result, Err: err}
	}(tr)
}

wg.Wait()
close(results)

for r := range results {
	if r.Err != nil {
		tree = algo.TellError(tree, r.TrialID, r.Err)
		continue
	}
	tree, _ = algo.Tell(tree, r.TrialID, r.Result)
}
```

Follow TreeQuest’s warning in spirit: large batch sizes can skew the search tree too wide, and their README recommends starting around `batch_size <= 5`. ([GitHub][1]) For LLM audits, I would start with `batch_size=3`: one constructive, one adversarial, one evaluator/synthesis action.

## 9. How the A/B/E loop should work

Your desired workflow:

```text
Paper claim C
  -> Worker A: constructive audit
  -> Evaluator E scores A
  -> Worker B: adversarial audit
  -> Evaluator E scores B
  -> AB-MCTS decides:
       new A/B branch?
       refine A?
       refine B?
       synthesize?
       escalate to Extended Workbench?
```

A useful state transition design:

```text
Root:
  raw claim extracted from paper

A child:
  constructive reading of claim

B child:
  adversarial reading of claim

E child:
  evaluator scoring of A or B

S child:
  synthesis that preserves useful parts and files remaining debt
```

But do not force all paths into A→E→B→E. AB-MCTS is valuable precisely because it can decide whether to generate another independent critique or refine a promising branch.

## 10. Prompt contracts

Use strict JSON contracts, because Go should parse and score outputs deterministically.

Constructive A prompt:

```text
You are Worker A, constructive analyst.
Given the paper excerpt and target claim, produce a Core Workbench analysis.
Your job is to steelman the claim without promoting it.
Return JSON only:
{
  "central_claim": "...",
  "claim_type": "...",
  "assumptions": [],
  "constraints": [],
  "bridge_principles": [],
  "what_it_explains_better": [],
  "open_debts": [],
  "next_smallest_move": "...",
  "confidence": "low|medium|high"
}
Rules:
- Do not decide truth.
- Separate author claim from established result.
- Do not call accommodation derivation.
```

Adversarial B prompt:

```text
You are Worker B, adversarial reviewer.
Assume the source, author, and Worker A may have overclaimed.
Find missing bridges, category errors, unsupported assumptions, rival explanations, and incompleteness.
Return JSON only:
{
  "main_objections": [],
  "risk_flags": [],
  "missing_bridges": [],
  "possible_null_models": [],
  "what_would_count_against_it": [],
  "downgrade_recommendations": [],
  "next_smallest_move": "...",
  "confidence": "low|medium|high"
}
```

Evaluator E prompt:

```text
You are Evaluator E.
Score the supplied Worker output for EBP/Workbench audit quality, not truth.
Return JSON only:
{
  "score": 0.0,
  "rationale": "...",
  "retired_debt": [],
  "new_debt": [],
  "evidence_status": {},
  "overclaim_flags": [],
  "promotion_block": true,
  "next_best_action": "go_wider|go_deeper|synthesize|stop"
}
Rules:
- LLMs are clerks, not authorities.
- No final-truth promotion.
- Mark missing fields explicitly.
```

## 11. Persistence and reproducibility

For PTW integration, every run should be reproducible enough to audit.

Store:

```text
run_id
paper_id
claim_id
model names
prompts
temperature
seed if supported
tree JSON
node states
scores
evaluator rationales
source spans
timestamp
cost/latency
final top_k states
```

Checkpoint format can start as JSON. Later you can use SQLite.

Minimal schema:

```text
runs
nodes
edges
trials
actions
llm_calls
scores
claims
source_spans
```

TreeQuest already supports checkpointing/resuming as a feature; your Go port should keep that as a first-class requirement, especially because paper audits can be expensive. ([GitHub][1])

## 12. CLI shape

Start with one CLI:

```bash
treequest-ebp audit \
  --paper paper.pdf \
  --claims claims.json \
  --mode core \
  --budget 40 \
  --batch-size 3 \
  --algo abmcts-a \
  --constructive-model gpt-5.5 \
  --adversarial-model claude-opus \
  --evaluator-model gemini-pro \
  --out out/audit_run.json
```

Then add:

```bash
treequest-ebp resume --run out/audit_run.json --budget +20
treequest-ebp topk --run out/audit_run.json --k 5
treequest-ebp render --run out/audit_run.json --format html
treequest-ebp export-workbench --run out/audit_run.json
```

## 13. MVP phases

### Phase 0 — Read and model TreeQuest behavior

Do not implement AB-MCTS-M yet.

Implement:

```text
Tree
Node
Trial
Action
Step
Ask
AskBatch
Tell
TopK
Checkpoint
RepeatedSampling baseline
SequentialRefinement baseline
StandardMCTS baseline
ABMCTSA baseline
```

### Phase 1 — EBP/Workbench audit adapter

Implement `PaperAuditState`, JSON prompt contracts, evaluator scoring, and Workbench2 report export.

### Phase 2 — A/B/E orchestration

Add named roles:

```text
ConstructiveWorker
AdversarialWorker
Evaluator
Synthesizer
```

The evaluator must score one worker output at a time, as you specified.

### Phase 3 — Claim extraction pipeline

Given a paper, extract claims first. The AB-MCTS loop should operate **per claim**, not on the whole paper at once.

### Phase 4 — Source-grounding

Every claim and audit statement should carry source spans. This is required to avoid source laundering, which Workbench2 explicitly flags as a risk when author rhetoric or LLM inference is treated as established result. 

### Phase 5 — PTW integration

Export:

```text
Core Workbench report
EBP ledger
open debts
risk flags
top-k audit paths
recommended next smallest move
```

## 14. What not to build first

Do **not** start with a dashboard. EBP explicitly says the first implementation should be embarrassingly small and warns against dashboards before artifacts, mandatory formalization at entry, or accounting becoming the work. 

Do **not** start with AB-MCTS-M unless you truly need mixed modeling. The Go-native path should first implement AB-MCTS-A because it is simpler, dependency-light, and adequate for proving the workflow.

Do **not** let evaluator E certify truth. E only scores audit quality and identifies debt movement. Workbench2 says LLMs may parse, extract, classify, flag, and draft, but may not decide final truth, certify a theory, or replace expert review. 

## 15. Suggested package-level API

```go
// treequest package
type Tree[S any] struct { ... }
type Node[S any] struct { ... }
type Algorithm[S any] interface { ... }
type ABMCTSA[S any] struct { Config ABMCTSAConfig }
type StandardMCTS[S any] struct { Config MCTSConfig }

func TopK[S any](t *Tree[S], k int) []NodeScore[S]
func SaveJSON[S any](path string, t *Tree[S]) error
func LoadJSON[S any](path string) (*Tree[S], error)

// ebpaudit package
type PaperAuditState struct { ... }
type ConstructiveAction struct { Client llm.Client }
type AdversarialAction struct { Client llm.Client }
type EvaluatorAction struct { Client llm.Client }
type SynthesizerAction struct { Client llm.Client }

func NewCoreWorkbenchActions(cfg Config) []treequest.Action[PaperAuditState]
func ExportWorkbenchReport(t *treequest.Tree[PaperAuditState]) WorkbenchReport
```

## 16. The key design insight

Your Go version should treat **LLM calls as actions** and **EBP/Workbench reports as states**.

That means TreeQuest-Go is general-purpose:

```text
state: any Go struct
action: any generator/refiner/evaluator
score: normalized quality signal
search: adaptive width/depth
```

Your physics-paper audit is just one adapter:

```text
state: PaperAuditState
actions: A constructive, B adversarial, E evaluator
score: EBP/Workbench audit quality
tree: competing and refining audit paths
top_k: best claim analyses, not truth verdicts
```

## Minimal first milestone

Build this first:

```text
treequest/
  tree.go
  algo.go
  repeated.go
  sequential.go
  abmctsa.go
  ask_tell.go
  topk.go
  checkpoint.go

ebpaudit/
  state.go
  scoring.go
  actions_fake.go

cmd/treequest-demo/
  main.go
```

Use fake deterministic workers before real LLMs. Example:

```text
A returns a constructive Workbench report.
B returns adversarial risk flags.
E returns a normalized score using simple string/rule checks.
```

Only after tests pass should you plug in real LLM APIs.

## EBP self-audit

**Claims made:** A Go port should implement the TreeQuest-style generic search abstraction first, then layer EBP/Workbench paper-audit adapters on top. AB-MCTS-A should be the MVP; AB-MCTS-M should be deferred. Evaluator E should score audit quality, not physical truth.

**needMap:** TreeQuest state/action/score/search maps to paper-claim audit state, A/B worker actions, E evaluator score, and top-k audit reports.

**needInvariant:** Preserve normalized scoring, ask/tell semantics, width/depth adaptivity, source-grounded evidence discipline, and no-final-truth promotion.

**needToyCheck:** Build deterministic fake-worker tests before using real LLMs.

**needNullModel:** Compare AB-MCTS against repeated sampling, sequential refinement, and standard MCTS baselines.

**needObstruction:** Main risks are evaluator unreliability, source laundering, overclaiming, cost explosion, and tree-shape distortion from overly large batches.

**needFaithfulnessReview:** The Go port should be checked against TreeQuest behavior, but the EBP/Workbench adaptation is a new application layer and must not be claimed as a faithful reproduction of Sakana’s benchmark results.

**Promotion status:** Architecture proposal only. Alive, unpromoted, ready for implementation planning or code scaffolding.

[1]: https://github.com/SakanaAI/treequest "GitHub - SakanaAI/treequest: A Tree Search Library with Flexible API for LLM Inference-Time Scaling · GitHub"
[2]: https://arxiv.org/pdf/2503.04412 "Wider or Deeper? Scaling LLM Inference-Time Compute with Adaptive Branching Tree Search"
[3]: https://sakana.ai/ab-mcts/ "Inference-Time Scaling and Collective Intelligence for Frontier AI"



## prompt

goals: 1) make treequest library general-purpose (not specific to ebp) and not tied to any LLM, 2) I want ebp use case here to be conceptual only in implementation so I think this should be another codebase apart from treequest 3) the output of ebp use case is a research paper with a flag that specifies how many turns the worker LLMs can ask its respective LLM for budget control, is this reasonable and good design?



Yes — that is a **good design correction**, and I would lock it in.

The clean architecture is:

```text
treequest-go        = general-purpose adaptive tree search library
ebp-treequest-audit = separate application that uses treequest-go
```

That separation is exactly what you want. TreeQuest itself is described as a flexible answer-tree search library with AB-MCTS, useful for but not limited to LLM inference-time scaling, and its ask/tell interface is explicitly designed so users can define custom generation and scoring outside the tree core. ([GitHub][1])

## 1. Goal 1: TreeQuest-Go should be fully general-purpose

This means **no EBP types, no Workbench types, no LLM client, no prompt code, no research-paper assumptions** inside the core library.

The core should only know:

```go
type State any
type Action any
type Score float64
type Tree
type Node
type Trial
type Algorithm
```

The library should not care whether a node state is:

```text
an ARC puzzle answer
a Go code patch
a chess-like search state
a proof attempt
a research-paper audit
a robotics plan
a molecule candidate
a workflow state
```

That means the core API should look like this conceptually:

```go
type Action[S any] interface {
	Label() string
	Generate(ctx context.Context, parent S) (ScoredState[S], error)
}
```

or, even cleaner, make the core not know generation at all:

```go
type Trial[S any] struct {
	ID       TrialID
	ParentID NodeID
	Parent   S
	Action   string
}
```

Then the application runs the action externally and calls:

```go
tree.Tell(trial.ID, result)
```

That matches TreeQuest’s ask/tell design: `ask` or `ask_batch` returns the parent state and action for the next expansion, and `tell` updates the tree after external generation/scoring. ([GitHub][2])

**Recommendation:** make `ask/tell` the primary API. A convenience `Step()` can exist, but it should be optional.

## 2. Goal 2: EBP use case should be a separate codebase

Yes. Strongly agree.

Suggested repo split:

```text
github.com/PithomLabs/treequest-go
github.com/PithomLabs/ebp-treequest-audit
```

or:

```text
github.com/PithomLabs/treequest
github.com/PithomLabs/workbench-treequest
```

The first repo is a reusable Go library. The second is your physics-paper audit application.

This also fits EBP itself: EBP is tool-agnostic, and no implementation owns the protocol. The uploaded EBP spec explicitly says EBP may be implemented in Markdown, Lean files, Git repos, local databases, PTW, Coq, Isabelle, notebooks, paper sidecars, or artifact registries, but none of those tools is required for the protocol to exist. 

It also fits Workbench2. Workbench is a visibility tool, not a truth machine, and its compact design says to use the smallest level capable of exposing the real issue.  So the audit app should use TreeQuest as one search strategy, not make TreeQuest itself “the Workbench.”

## 3. Goal 3: Output of the EBP app should be a research paper/report

Reasonable, with one important adjustment:

The **human-facing output** can be a research paper-style report.

The **machine-facing output** should also include structured artifacts.

So produce both:

```text
out/report.md or out/report.pdf
out/run.json
out/tree.json
out/claims.json
out/evidence.json
out/provenance.json
```

The research-paper output is excellent for reading and sharing. But you also need machine-readable provenance so PTW/Workbench can audit the run later.

The report should not say:

```text
This paper is true.
This claim is false.
This theory is solved.
```

It should say:

```text
This claim was extracted.
This worker argued constructively.
This worker argued adversarially.
The evaluator scored audit quality.
These debts remain.
These risk flags were triggered.
This is the strongest next useful check.
```

That matches Workbench2’s rule that the Workbench clarifies claims, assumptions, constraints, missing bridges, what would break the theory, and what remains unresolved, but does not decide truth. 

## 4. The “turn budget” flag is good design — but make it more precise

Your idea is good:

```bash
--worker-turns 3
```

But I would not make it the only budget control.

Use several budget controls:

```bash
--worker-turns 3
--evaluator-turns 1
--max-tree-steps 40
--max-llm-calls 120
--max-tokens 200000
--max-cost-usd 10.00
--max-wall-time 30m
```

Why? Because “turns” alone does not control cost. One turn can be short or huge. A failed JSON repair loop can consume calls. A large paper excerpt can explode tokens. AB-MCTS can go wide quickly if the budget is loose.

The clean design is:

```text
turn budget     = local conversation depth per agent branch
tree budget     = total AB-MCTS expansion steps
call budget     = total model invocations
token budget    = cost/latency guard
cost budget     = money guard
wall-time budget = operational guard
```

So yes, use turn budget, but as one budget dimension.

## 5. How to model “turns” correctly

For your EBP app, a “turn” should mean:

```text
one agent-internal refinement cycle for a selected node
```

Example:

```bash
--constructive-turns 2
--adversarial-turns 2
--evaluator-turns 1
```

This means:

```text
A can ask its model up to 2 times for a selected constructive expansion.
B can ask its model up to 2 times for a selected adversarial expansion.
E can evaluate in 1 pass.
```

I would also add:

```bash
--repair-turns 1
```

for JSON repair or schema correction, because schema repair should not silently consume the same intellectual budget as the worker’s actual analysis.

## 6. Keep agent turns outside treequest-go

Important design boundary:

```text
treequest-go does not know what a turn is.
```

The core library only sees:

```text
Ask -> Trial
Tell -> ScoredState
```

The EBP audit app manages:

```text
LLM turns
agent prompts
conversation memory
token limits
JSON repair
evaluator score
report synthesis
```

This keeps TreeQuest-Go useful outside LLMs.

In other words:

```text
TreeQuest budget:
  max nodes
  max depth
  max expansions
  max pending trials

EBP app budget:
  worker turns
  evaluator turns
  LLM calls
  tokens
  dollars
  report length
```

That boundary is the key to keeping the Go port general-purpose.

## 7. Recommended final repo boundaries

### Repo 1: `treequest-go`

Contains:

```text
tree structure
node statistics
algorithm interfaces
standard MCTS
repeated sampling baseline
sequential refinement baseline
AB-MCTS-A
ask/tell
checkpoint/resume
top-k
JSON export
tests with fake deterministic generators
```

Does **not** contain:

```text
OpenAI client
Anthropic client
Gemini client
EBP
Workbench
physics paper logic
prompt templates
PDF parsing
report writing
```

### Repo 2: `ebp-treequest-audit`

Contains:

```text
PDF/text ingestion
claim extraction
Workbench2 schemas
EBP ledger schemas
LLM provider adapters
A/B/E agents
turn budgets
TreeQuest adapter
research-paper report writer
PTW export
source provenance
```

Can import:

```go
import "github.com/PithomLabs/treequest-go/treequest"
```

## 8. Proposed CLI for EBP app

Something like:

```bash
ebp-treequest-audit run \
  --paper ./papers/example.pdf \
  --mode core \
  --claims auto \
  --constructive-model gpt-5.5 \
  --adversarial-model claude-opus \
  --evaluator-model gemini-pro \
  --constructive-turns 2 \
  --adversarial-turns 2 \
  --evaluator-turns 1 \
  --max-tree-steps 60 \
  --max-llm-calls 180 \
  --max-cost-usd 15 \
  --out ./out/example-audit
```

For budget presets:

```bash
--budget tiny     # flash, low cost
--budget standard # core, moderate
--budget deep     # extended, expensive
```

But always allow explicit overrides.

## 9. Research-paper output structure

The generated paper/report should look like:

```text
Title
Abstract
1. Source and Method
2. Extracted Claims
3. Constructive Analysis
4. Adversarial Analysis
5. Evaluator Assessment
6. EBP Debt Ledger
7. Workbench2 Report
8. Rival Differential
9. Incompleteness and Risk Flags
10. Strongest Next Tests
11. Limitations of This Audit
Appendix A: Tree Search Trace
Appendix B: Prompt/Model Provenance
Appendix C: Source Spans
```

Workbench2 specifically warns that LLMs are clerks, not authorities: they may parse, extract, classify, draft reports, flag missing bridges, and generate adversarial objections, but they may not decide final truth, certify a theory, or replace expert review.  That statement should appear in the report’s method section.

## 10. Is this good design?

Yes — with three refinements.

First, keep `treequest-go` completely domain-neutral. No LLM, no EBP, no Workbench.

Second, make the EBP use case a separate application/codebase. That application can use TreeQuest as an adaptive search engine for A/B/E audit paths.

Third, use `--worker-turns` as a budget flag, but do not rely on it alone. Add max calls, steps, tokens, cost, and wall-time so the tool is operationally safe.

This gives you the best of both worlds: a reusable Go TreeQuest library and a disciplined EBP/Workbench application that can generate research-paper-style audits without contaminating the core search library.

## EBP self-audit

**Claims made:**

1. Separating `treequest-go` from `ebp-treequest-audit` is good architecture.
2. Turn-budget control is reasonable but insufficient alone.
3. The EBP app should output both human-readable research reports and machine-readable provenance.
4. TreeQuest-Go should use ask/tell as its primary abstraction.

**needMap:**
TreeQuest core maps to generic state/action/score/tree search. EBP audit maps to a separate app layer using worker/evaluator agents and Workbench2 reports.

**needInvariant:**
Domain neutrality, provider neutrality, score normalization, ask/tell separation, provenance preservation, and no truth certification.

**needToyCheck:**
Build `treequest-go` tests with fake deterministic actions before connecting real LLMs or PDFs.

**needNullModel:**
Compare this architecture against a monorepo design and against a direct non-tree LLM pipeline. The separated design wins on reuse and contamination control; the monorepo wins only on short-term speed.

**needObstruction:**
Potential failure modes: budget leakage, evaluator bottleneck, LLM source laundering, schema repair loops, and core library contamination by app-specific logic.

**needFaithfulnessReview:**
Before implementation, define the exact TreeQuest-Go API contract and check it against Sakana’s ask/tell and AB-MCTS behavior without claiming benchmark equivalence.

**Promotion status:**
Architecture is strong but still **unpromoted** until tested with fake actions, baseline comparisons, and one small paper-audit proof of concept.

[1]: https://github.com/SakanaAI/treequest?utm_source=chatgpt.com "SakanaAI/treequest: A Tree Search Library with Flexible ..."
[2]: https://github.com/SakanaAI/treequest/releases?utm_source=chatgpt.com "Releases · SakanaAI/treequest"




## prompt

now analyze attached file and synthesize where it adds net value or if it is better, you can make it as your base, wdyt


Yes — the attached file **adds real net value**, but I would **not use it as the base unchanged**. I would use it as the **base after refactoring**, because it is strong on AB-MCTS-A mechanics and Go build order, but it violates your updated architecture goals by putting `llm` and `ebp` packages inside `treequest-go`. 

## Verdict

Use it as the **technical draft base**, not the **repo base**.

My recommendation:

```text
Keep 60–70%:
  AB-MCTS-A explanation
  Beta/Thompson sampling primitives
  generic Node/Tree sketch
  Go build order
  dummy first-run test
  checkpointing requirement
  ABMCTS-M defer decision

Rewrite or move 30–40%:
  pkg/llm
  pkg/ebp
  EBP state
  A/B/E prompts
  evaluator logic
  report generation
  Anthropic/OpenAI/Gemini providers
```

The file is strongest as a **bridge document** between Sakana’s TreeQuest and your Go implementation. It is weakest where it collapses the general-purpose library and your EBP application into one codebase.

## Where it adds net value

### 1. It correctly identifies AB-MCTS-A as the first implementation target

This is the most important practical call. The file correctly says to start with `ABMCTS-A` and defer `ABMCTS-M`, because the mixed-model version depends on heavier Bayesian/MCMC machinery that is not idiomatic as a first Go target. 

That matches the upstream picture: TreeQuest exposes AB-MCTS-A, AB-MCTS-M, Multi-LLM AB-MCTS, customizable generation/scoring, and checkpointing. ([GitHub][1]) The paper also distinguishes AB-MCTS-M and AB-MCTS-A as two methods for handling unbounded branching, both using Thompson Sampling but differing in node-selection implementation. ([arXiv][2])

So yes: **AB-MCTS-A first** is the right base.

### 2. It captures the core “wider vs deeper” search idea clearly

The file’s explanation of direction choice — generate a new child versus descend/refine an existing child — is the right conceptual center.  Sakana’s blog frames AB-MCTS the same way: repeated sampling is width, sequential refinement is depth, and AB-MCTS dynamically chooses between them. ([Sakana AI][3])

That is exactly the part worth preserving.

### 3. It gives a useful Go build order

The attached build order is sane:

```text
tree
bandit
abmcts_a
dummy generator
checkpoint
then application adapters
```

That is the right sequence. The dummy `SimpleState = string` first run is especially valuable because it prevents you from debugging LLM prompts and tree-search math at the same time. 

### 4. It correctly treats score as normalized `[0,1]`

This is important. TreeQuest-like search needs a generic reward signal. Your Go library should not know what the score means. It only knows that higher is better and that the value can update search statistics.

For EBP, the score means “audit quality.” For another user, it might mean “unit tests passed,” “puzzle solved,” “simulation accuracy,” or “human preference score.”

That abstraction is good.

## Where it conflicts with your goals

### 1. It puts `llm` and `ebp` inside `treequest-go`

This is the biggest flaw.

The attached architecture proposes:

```text
pkg/llm
pkg/ebp
cmd/evaluate
examples/bohmian_bfr
```

inside `treequest-go`. 

That directly conflicts with your clarified goal:

```text
treequest-go must be general-purpose
EBP use case should be conceptual/application-level only
LLMs should not be part of core treequest
```

So the corrected split should be:

```text
github.com/PithomLabs/treequest-go
  /treequest
  /treequest/algo
  /treequest/bandit
  /treequest/checkpoint
  /examples/simple

github.com/PithomLabs/ebp-treequest-audit
  /llm
  /agents
  /workbench
  /ebp
  /paper
  /report
  /cmd/ebp-treequest-audit
```

TreeQuest-Go should not import, mention, or assume:

```text
LLM
OpenAI
Anthropic
Gemini
EBP
Workbench
physics paper
constructive worker
adversarial worker
evaluator
```

Those are application concerns.

### 2. It uses “model” where the core should say “action”

The file uses `model_name`, `ModelBandits`, and `GenerateFns` keyed by model. That is faithful to the Multi-LLM use case, but too narrow for a general-purpose Go library.

In the core, rename:

```text
model -> action
model bandit -> action bandit
generate_fns -> action funcs
GeneratedBy -> ActionLabel
```

Why? Because in a non-LLM use case, an “action” might be:

```text
mutate candidate
run local solver
apply heuristic
sample random branch
call model A
call model B
run evaluator
repair JSON
```

So the generic library should expose:

```go
type ActionLabel string

type GenerateFunc[S any] func(ctx context.Context, parent S) (S, float64, error)
```

or better, for ask/tell:

```go
type Trial[S any] struct {
	ID          TrialID
	ParentID    NodeID
	ParentState S
	ActionLabel ActionLabel
}
```

Then the app decides whether `ActionLabel` means an LLM, heuristic, solver, or evaluator.

### 3. `Step()` should not be the primary API

The attached file centers `Step(tree, generate_fns)`. That is okay for demos, but not ideal for your final design.

For a general-purpose Go port, the primary API should be:

```text
Ask
AskBatch
Tell
```

Then `Step()` can be a convenience wrapper.

Why? Because TreeQuest’s own release notes emphasize `ask_batch` and `tell` for parallel generation and evaluation. ([GitHub][4]) This is also cleaner for budget control: the tree library can select trials, while the application handles LLM calls, timeouts, token budgets, retries, JSON repair, and scoring.

Correct API priority:

```text
Primary:
  Ask()
  AskBatch()
  Tell()

Convenience:
  Step()
```

### 4. The checkpoint design needs to be rewritten

The file proposes serializing the tree directly using `json.MarshalIndent`.  That will not be robust if the node struct contains:

```go
Parent *Node[S]
Children []*Node[S]
mu sync.Mutex
```

The parent/child pointers create cyclic graph risk, and mutexes are runtime-only state. The safer checkpoint format is a flat record list:

```go
type NodeRecord[S any] struct {
	ID          NodeID
	ParentID    *NodeID
	ChildIDs    []NodeID
	Depth       int
	State       S
	Score       float64
	ActionLabel string
	Wider       BetaParams
	Deeper      BetaParams
	ActionStats map[string]BetaParams
}

type Checkpoint[S any] struct {
	SchemaVersion string
	RootID        NodeID
	Nodes         []NodeRecord[S]
	PendingTrials []TrialRecord
	RNGState      optional
	Config        map[string]any
}
```

Then `Load()` reconstructs pointers or keeps ID-based adjacency internally.

This matters a lot. Checkpointing is a real TreeQuest feature upstream, so the Go port should not start with fragile serialization. ([GitHub][1])

### 5. Some code sketches are not Go-valid or not package-safe

The file is useful as pseudocode, but I would not copy the code directly.

Specific problems:

```go
func (a *ABMCTSA[S]) bestChild[S any](...)
```

Go methods cannot introduce a new method-level type parameter like that. It should be:

```go
func (a *ABMCTSA[S]) bestChild(parent *tree.Node[S]) *tree.Node[S]
```

Another issue: `n.mu` is unexported in package `tree`, but the algorithm code is in package `algo`. That means `algo` cannot access `n.mu` unless you expose methods on `Node` or place algorithm and node internals in the same package.

Better:

```go
func (n *Node[S]) AddChild(child *Node[S])
func (n *Node[S]) SnapshotChildren() []NodeID
func (n *Node[S]) UpdateWider(score float64)
func (n *Node[S]) UpdateDeeper(score float64)
func (n *Node[S]) UpdateAction(label ActionLabel, score float64)
```

The algorithm should not reach into mutex internals.

### 6. Parallel `Step()` is risky

The attached file suggests running multiple `Step()` calls in parallel.  That is risky because selection, expansion, RNG, and backpropagation can interleave badly.

Use `AskBatch()` instead.

Pattern:

```text
AskBatch selects N trials under lock.
Application runs N external generations concurrently.
Tell inserts results as they finish.
```

That is exactly why ask/tell is the better foundation.

### 7. Evaluator E should not live inside `GenerateFn` in the core design

The attached file says E is not a `GenerateFn`; E is called inside A and B, and its score is returned to AB-MCTS. 

That is acceptable for the **EBP app**, but should not leak into TreeQuest-Go.

For your specific goal — E evaluates A and B one at a time — I would model it in the application as:

```text
A produces candidate state
E evaluates A output
B produces adversarial state
E evaluates B output
optional: E synthesizes
Tell final scored state to TreeQuest
```

TreeQuest-Go should only receive:

```text
Trial result = state + score + metadata
```

## Should this become the base?

### For `treequest-go`: yes, but only Sections 1–6, 10–13, and 15

Use these parts as the base:

```text
1. What You Are Porting
2. AB-MCTS Algorithm Mechanics
4. Core Data Structures
5. Thompson Sampling in Go
6. ABMCTS-A Algorithm
10. Checkpointing, but rewrite implementation
11. Key Differences from Python
12. Non-Trivial Implementation Challenges
13. Recommended Build Order, but stop before LLM
15. Minimum Viable First Run
```

But rewrite them with these corrections:

```text
model -> action
LLM -> external generator
EBP -> removed
Step -> convenience only
Ask/Tell -> primary
direct JSON pointer tree -> flat checkpoint
Node mutex access -> package-safe methods
parallel Step -> AskBatch/Tell
```

### For `ebp-treequest-audit`: yes, but as a concept doc only

Use these parts as a second, separate app spec:

```text
7. EBP v2.1 + Workbench2 State
8. LLM Provider Interface
9. EBP Evaluation Pipeline
14. EBP Debt Item → Workbench2 Flag Mapping
```

But move them into the EBP app repo and revise:

```text
PromotionReady -> HumanReviewRequired / CandidateAuditComplete
EBPScore -> AuditQualityScore
LLMProvider -> app-level provider
ClaimEvalState -> app-level state, not treequest state requirement
```

I would avoid the field name `PromotionReady`. It is too strong if an LLM evaluator is involved. Under EBP/Workbench discipline, the app can say:

```text
audit_status: candidate_report_complete
human_review_required: true
promotion_blockers: [...]
```

Not:

```text
promotion_ready: true
```

EBP says promotion costs debt and no final-truth claim may be promoted; the Workbench says LLMs are clerks, not authorities.  

## Revised architecture I would use

### Repo 1: `treequest-go`

```text
treequest-go/
  go.mod

  treequest/
    types.go
    tree.go
    node.go
    stats.go
    checkpoint.go
    topk.go

  treequest/algo/
    repeated.go
    sequential.go
    standard_mcts.go
    abmcts_a.go

  treequest/bandit/
    beta.go
    thompson.go
    rng.go

  examples/
    simple_string/
    synthetic_bandit/
```

No `llm`. No `ebp`.

Core API:

```go
type Score float64
type NodeID string
type TrialID string
type ActionLabel string

type Trial[S any] struct {
	ID          TrialID
	ParentID    NodeID
	ParentState S
	ActionLabel ActionLabel
}

type Result[S any] struct {
	TrialID TrialID
	State   S
	Score   Score
	Meta    map[string]any
}

type Algorithm[S any] interface {
	Init(root S, actions []ActionLabel) *Tree[S]
	Ask(t *Tree[S]) (Trial[S], error)
	AskBatch(t *Tree[S], n int) ([]Trial[S], error)
	Tell(t *Tree[S], r Result[S]) error
	TopK(t *Tree[S], k int) []NodeScore[S]
}
```

### Repo 2: `ebp-treequest-audit`

```text
ebp-treequest-audit/
  cmd/ebp-treequest-audit/
  internal/paper/
  internal/claims/
  internal/agents/
  internal/llm/
  internal/workbench/
  internal/ebp/
  internal/report/
  internal/treeadapter/
```

This app imports `treequest-go`.

CLI:

```bash
ebp-treequest-audit run \
  --paper ./paper.pdf \
  --mode core \
  --worker-turns 2 \
  --evaluator-turns 1 \
  --max-tree-steps 60 \
  --max-llm-calls 180 \
  --max-cost-usd 15 \
  --out ./out/audit
```

Outputs:

```text
out/report.md
out/report.pdf optional
out/run.json
out/tree.json
out/claims.json
out/provenance.json
```

## My final answer

The attached file is **better than a blank start** and should become your **seed design**, but not your final base. Its algorithmic sections are valuable. Its architecture section must be split.

The clean decision is:

```text
Use the attachment as:
  TreeQuest-Go Technical Notes v0.1
  EBP TreeQuest Audit App Notes v0.1

Do not use it as:
  the monolithic repo structure
```

That gives you the right path:

```text
general-purpose TreeQuest-Go first
fake deterministic actions first
AB-MCTS-A first
ask/tell first
checkpointing right
EBP app second
LLM providers outside core
research-report output outside core
```

## EBP self-audit

**Claims made:** The attachment adds strong net value but should be refactored before becoming the implementation base. The core library should stay general-purpose; EBP/LLM logic should move to a separate application.

**needMap:** TreeQuest-Go maps to generic adaptive tree search. EBP paper audit maps to an external application using TreeQuest-Go.

**needInvariant:** Preserve domain neutrality, action neutrality, normalized scores, ask/tell semantics, checkpointability, and no truth certification.

**needToyCheck:** First toy check should be `S=string` with deterministic fake actions and a known reward function.

**needNullModel:** Compare against a monolithic repo and a simple non-tree A/B/E pipeline. Monolith is faster initially but worse for reuse; non-tree pipeline is simpler but loses adaptive width/depth search.

**needObstruction:** Current attached code has architectural leakage, checkpoint-cycle risk, package-boundary issues, unsafe parallel `Step`, and some Go-validity problems.

**needFaithfulnessReview:** Before coding, verify the exact AB-MCTS-A update semantics against Sakana TreeQuest’s implementation; do not claim behavioral parity until tests compare tree decisions on controlled seeds.

**Promotion status:** Use as a refactored design base, not as promoted implementation spec.

[1]: https://github.com/SakanaAI/treequest?utm_source=chatgpt.com "SakanaAI/treequest: A Tree Search Library with Flexible ..."
[2]: https://arxiv.org/html/2503.04412v1?utm_source=chatgpt.com "Wider or Deeper? Scaling LLM Inference-Time Compute ..."
[3]: https://sakana.ai/ab-mcts/?utm_source=chatgpt.com "Inference-Time Scaling and Collective Intelligence for ..."
[4]: https://github.com/SakanaAI/treequest/releases?utm_source=chatgpt.com "Releases · SakanaAI/treequest"




## prompt

analyze claude.md if it adds net new and if so, cherry pick and adopt them:



Yes — it **adds net new value**, mainly in four places. I would adopt those parts.

## What is genuinely new

### 1. The “entire surface area” framing is excellent

This line should be adopted:

```text
GenerateFn[S] is the entire surface area treequest-go should know about.
```

That is the clearest architectural boundary. It makes the repo split almost automatic:

```text
treequest-go does not know LLMs exist.
treequest-go does not know EBP exists.
treequest-go does not know papers, claims, debts, or evaluators exist.
```

It only knows:

```go
func(ctx context.Context, parent *S) (S, score, error)
```

or, in the ask/tell version:

```go
Ask -> Trial[S]
Tell -> Result[S]
```

I would adopt this as a **core design doctrine**.

### 2. The three-layer budget stack is a strong addition

This is better than just saying `--worker-turns`.

Adopt:

```text
1. Search-level budget
2. Worker-turn budget
3. Token/cost budget per turn
```

That is the right hierarchy.

In practice:

```bash
--max-tree-steps 60          # search-level budget
--worker-turns 2             # internal agent loop budget
--max-tokens-per-turn 8000   # per-turn guard
--max-llm-calls 180          # global guard
--max-cost-usd 15            # money guard
```

This should become the official budget model.

### 3. Budget exhaustion as an incompleteness event is important

This is the strongest net-new conceptual addition.

Adopt this rule:

```text
If a worker hits MaxTurns without converging, that must be surfaced as honest incompleteness, not hidden truncation.
```

That aligns perfectly with Workbench2’s rule that honest incompleteness should be visible rather than buried. Workbench2 explicitly requires reports to state unproven claims, open questions, missing bridges, uncontrolled approximations, and what would close the gap. 

So the EBP app state should include:

```go
type BudgetStatus struct {
	Worker             string `json:"worker"`
	MaxTurns           int    `json:"max_turns"`
	UsedTurns          int    `json:"used_turns"`
	BudgetExhausted    bool   `json:"budget_exhausted"`
	ExhaustionReason   string `json:"exhaustion_reason,omitempty"`
	EffectOnConfidence string `json:"effect_on_confidence,omitempty"`
}
```

Evaluator E should see this and penalize hidden incompleteness, not honest incompleteness.

### 4. Asymmetric budgets for A and B are worth supporting

This is a useful design refinement.

Constructive A and adversarial B do not necessarily have equal cognitive load. B may need more turns because it checks category errors, assumptions, bridge violations, inverse constraints, rival differentials, and incompleteness.

So instead of only:

```bash
--worker-turns 2
```

support:

```bash
--constructive-turns 2
--adversarial-turns 3
--evaluator-turns 1
```

Then optionally allow the shorthand:

```bash
--worker-turns 2
```

which sets both A and B equally unless overridden.

## What is not net new, but reinforces the design

The two-repo split is not new, but the argument is clearer. Adopt the wording.

The reporter-stage separation is also not new, but it is a useful clarification:

```text
Research-paper output is a Reporter concern, not a TreeQuest concern.
```

That should be adopted too.

So the final pipeline becomes:

```text
treequest-go:
  adaptive search only

ebp-paper-eval:
  paper ingestion
  claim extraction
  A/B/E agent loops
  budget enforcement
  Workbench/EBP scoring
  reporter output
```

## Cherry-picked adoption list

I would officially adopt these into the implementation plan:

```text
ADOPT-001: treequest-go knows only GenerateFn[S] / Ask-Tell.
ADOPT-002: EBP cannot live inside treequest-go by design.
ADOPT-003: LLMProvider belongs in ebp-paper-eval, not treequest-go.
ADOPT-004: Use a three-layer budget stack: search, worker turns, token/cost.
ADOPT-005: Budget exhaustion becomes explicit incompleteness metadata.
ADOPT-006: Evaluator E sees budget exhaustion and scores accordingly.
ADOPT-007: Support asymmetric A/B/E turn budgets.
ADOPT-008: Reporter stage is downstream of search and lives only in ebp-paper-eval.
```

## Revised CLI after adoption

```bash
ebp-paper-eval run \
  --paper ./paper.pdf \
  --mode core \
  --max-tree-steps 60 \
  --constructive-turns 2 \
  --adversarial-turns 3 \
  --evaluator-turns 1 \
  --max-tokens-per-turn 8000 \
  --max-llm-calls 180 \
  --max-cost-usd 15 \
  --out ./out/audit
```

Optional shorthand:

```bash
--worker-turns 2
```

Equivalent to:

```bash
--constructive-turns 2
--adversarial-turns 2
```

unless explicitly overridden.

## Revised state addition for `ebp-paper-eval`

```go
type AgentBudget struct {
	Role              string `json:"role"`
	MaxTurns          int    `json:"max_turns"`
	UsedTurns         int    `json:"used_turns"`
	MaxTokensPerTurn  int    `json:"max_tokens_per_turn,omitempty"`
	TokensUsed        int    `json:"tokens_used,omitempty"`
	BudgetExhausted   bool   `json:"budget_exhausted"`
	CompletionStatus  string `json:"completion_status"`
	EffectOnAudit     string `json:"effect_on_audit,omitempty"`
}

type ClaimEvalState struct {
	ClaimID      string        `json:"claim_id"`
	ClaimText    string        `json:"claim_text"`
	AgentBudgets []AgentBudget `json:"agent_budgets"`

	// Workbench / EBP app fields live here, not in treequest-go.
	AuditQualityScore float64  `json:"audit_quality_score"`
	OpenDebts         []string `json:"open_debts"`
	RiskFlags         []string `json:"risk_flags"`
	NextSmallestMove  string   `json:"next_smallest_move"`
}
```

## Final judgment

This adds **real net new value**. The biggest upgrades are:

```text
1. GenerateFn[S] as the hard boundary.
2. Three-layer budget stack.
3. Budget exhaustion as visible incompleteness.
4. Asymmetric worker budgets.
5. Reporter as downstream app layer.
```

I would adopt those immediately.

## EBP self-audit

**Claims:** The new text adds net value and should be partially adopted.
**needMap:** Generic TreeQuest core maps only to `GenerateFn[S]` / ask-tell; EBP paper evaluation maps to a separate consumer app.
**needInvariant:** Domain neutrality, budget visibility, honest incompleteness, and reporter separation must be preserved.
**needToyCheck:** Test budget exhaustion using fake workers before using real LLMs.
**needNullModel:** Compare symmetric versus asymmetric budgets in small trial runs.
**needObstruction:** Risk: budget metadata could become noise unless evaluator scoring actually uses it.
**needFaithfulnessReview:** Confirm that the final TreeQuest-Go API does not leak `LLM`, `EBP`, `claim`, `worker`, or `evaluator` concepts.
**Promotion status:** Adopted as design refinement, still unpromoted until implemented and tested.





## prompt

analyze attached kimi.md if it adds net new and if so, cherry pick and adopt them:



Yes — **kimi.md adds net new value**, but like the prior attachment, it should be **cherry-picked**, not adopted wholesale.

Its strongest additions are not the EBP-specific parts. Its strongest additions are the **general-purpose TreeQuest-Go library features**: baselines, backup strategies, batch executor, renderer, and clearer separation between `treequest` and `ebp-evaluator`. 

## Bottom-line verdict

Adopt it as:

```text
TreeQuest-Go Feature Expansion Notes v0.2
```

Do **not** adopt it as the final repo architecture unchanged.

It improves our previous plan in several places, but it also reintroduces a few issues we already decided to avoid: EBP-specific scoring language too close to the search core, evaluator E as a generic tree action without enough distinction, and AB-MCTS-M appearing too early.

## Net-new items worth adopting

### 1. Add baseline algorithms to `treequest-go`

This is genuinely useful.

Kimi proposes:

```text
standard_mcts.go
tree_of_thoughts.go
ucb.go / PUCT
backup.go
```

I would adopt this, but stage it carefully.

Final decision:

```text
Adopt:
- repeated sampling baseline
- sequential refinement baseline
- standard MCTS baseline
- Tree-of-Thoughts BFS baseline
- UCB1 / PUCT selectors
- backup strategy abstraction

Defer:
- AB-MCTS-M implementation
```

Why this matters: TreeQuest-Go should not only be “AB-MCTS-A in Go.” It should become a small **adaptive search toolkit**. Having baselines lets you test whether AB-MCTS-A actually adds value over simpler search. That is important for EBP hygiene: the null model for AB-MCTS-A is repeated sampling, sequential refinement, and standard MCTS.

### 2. Add a generic `batch.Executor`

This is a good addition if kept domain-neutral.

Kimi’s `batch.Executor` concept is useful: it runs trials concurrently and returns results. 

Adopt as:

```text
treequest/batch/executor.go
```

But it must not know LLMs.

Correct generic shape:

```go
type Executor[S any] struct {
	Workers int
}

type TrialRunner[S any] func(context.Context, Trial[S]) Result[S]

func (e *Executor[S]) Run(
	ctx context.Context,
	trials []Trial[S],
	run TrialRunner[S],
) []Result[S]
```

This keeps the executor useful for:

```text
LLM calls
simulators
code generators
unit-test runners
proof-search attempts
local heuristics
```

### 3. Add renderers: DOT and HTML

This is a strong net-new feature.

Adopt:

```text
treequest/render/dot.go
treequest/render/html.go
```

But make it optional and generic.

Minimum first renderer:

```text
DOT export first
HTML later
```

DOT is cheap and immediately useful for debugging. HTML can wait.

### 4. Add explicit backup strategies

This is valuable and was missing from our previous plan.

Kimi includes `tree/backup.go`.  Adopt the idea.

Why: different domains may want different node value semantics:

```text
max child score
mean child score
visit-weighted mean
last score
discounted depth score
risk-adjusted score
```

Generic interface:

```go
type BackupStrategy interface {
	Backup(parent NodeStats, childScore Score) NodeStats
}
```

For MVP:

```text
MeanBackup
MaxBackup
```

For AB-MCTS-A, keep its own bandit updates, but expose backup strategies for standard MCTS and Tree-of-Thoughts baselines.

### 5. Add early stopping, but in the app layer

Kimi proposes:

```go
EarlyStopPatience
MinScoreThreshold
```

Good idea, but location matters.

Adopt in `ebp-paper-eval`, not `treequest-go`.

TreeQuest-Go can provide generic helpers later, but early stopping is domain-sensitive. In EBP audit, a high score does not mean “truth”; it means “audit quality.” So early stopping should be based on:

```text
score plateau
budget exhaustion
no new debt movement
no improvement in risk flags
human-configured max steps
```

Adopt as:

```go
type StopConfig struct {
	EarlyStopPatience int
	MinAuditScore    float64
	StopOnNoDebtMovement bool
}
```

### 6. Add local model adapter in the EBP app

Kimi includes:

```text
llm/local.go
```

Adopt this for the EBP app. It is useful for Ollama, llama.cpp servers, or local OpenAI-compatible endpoints.

But again: it belongs only in:

```text
ebp-paper-eval/internal/llm
```

not in TreeQuest-Go.

### 7. Add report template ideas

Kimi’s report structure is useful. 

Adopt the broad sections:

```text
metadata
budget
best path
all explored paths
debt status
risk flags
verdict
```

But change terminology.

Do **not** use:

```text
Promoted: true
```

for an LLM-generated report.

Use:

```text
Audit status: complete / incomplete / budget-exhausted / needs-human-review
Promotion blockers: [...]
Human review required: true
```

EBP says promotion is debt-gated and no final-truth claim may be promoted; Workbench2 says LLMs are clerks, not authorities.  

## Items to reject or modify

### 1. Do not implement `ABMCTS-M` in Phase 1

Kimi lists:

```text
abmcts_m.go
```

inside the initial treequest structure. 

Reject for MVP.

Better:

```text
treequest/algo/abmcts_m.go      # not present yet
docs/abmcts_m_deferred.md       # explain why deferred
```

AB-MCTS-M is heavy, more statistically complex, and not needed to validate the Go port.

### 2. Do not make the tree purely immutable in the first Go implementation

Kimi says:

```text
Tree mutability: Immutable functional style
```

This sounds elegant, but it will add friction in Go.

Adopt instead:

```text
Internal mutable tree with safe checkpoint snapshots.
Public API may return updated tree for ergonomic compatibility.
```

So:

```go
func (a *ABMCTSA[S]) Tell(t *Tree[S], r Result[S]) error
```

is fine.

No need to clone the whole tree on every step unless testing reveals it is necessary.

### 3. Do not put `evaluator-E` as a core search concept

Kimi’s generic action list includes:

```text
worker-A
worker-B
evaluator-E
```

That is fine **inside the EBP app**, but not in TreeQuest-Go. 

TreeQuest-Go should see only:

```text
ActionLabel
Trial
Result
Score
```

In `ebp-paper-eval`, you can decide whether E is:

```text
a sub-step inside A/B generation
a separate action
a scoring function after each worker result
a synthesizer pass after top-k
```

For your stated workflow — E evaluates A and B one at a time — I still prefer:

```text
A or B generates candidate analysis
E evaluates that one worker output
the app returns scored Result to TreeQuest
```

That keeps TreeQuest neutral and keeps scoring disciplined.

### 4. Do not use `Promoted bool` in generated reports

Kimi’s report struct includes:

```go
Promoted bool
```

Reject or rename.

Use:

```go
HumanReviewRequired bool
AuditStatus string
PromotionBlockers []string
```

Reason: LLM-generated analysis should not be allowed to promote physics claims. It can suggest debt status, but human review remains required.

### 5. Do not use “score = promotion readiness”

Kimi sometimes frames score as promotion readiness. Modify this.

Use:

```text
AuditQualityScore
```

not:

```text
PromotionScore
EBPScore as promotion readiness
```

The score should reflect the quality and completeness of the audit, not whether the claim is true or promoted.

## Adopted refined repo structure

### `treequest-go`

```text
treequest-go/
  go.mod

  treequest/
    types.go
    node.go
    tree.go
    stats.go
    checkpoint.go
    topk.go

  treequest/algo/
    interface.go
    abmcts_a.go
    standard_mcts.go
    repeated.go
    sequential.go
    tree_of_thoughts.go

  treequest/bandit/
    beta.go
    thompson.go
    ucb.go
    puct.go

  treequest/batch/
    trial.go
    executor.go

  treequest/backup/
    mean.go
    max.go

  treequest/render/
    dot.go
    html.go

  examples/
    simple_string/
    synthetic_reward/
```

### `ebp-paper-eval`

```text
ebp-paper-eval/
  cmd/ebp-paper-eval/

  internal/paper/
    loader.go
    extractor.go

  internal/claims/
    claim.go
    source_span.go

  internal/llm/
    client.go
    openai.go
    anthropic.go
    gemini.go
    local.go

  internal/agents/
    constructive.go
    adversarial.go
    evaluator.go
    budget.go

  internal/workbench/
    flash.go
    core.go
    extended.go
    risk_flags.go

  internal/ebp/
    idea.go
    debt.go
    ledger.go

  internal/search/
    state.go
    actions.go
    treequest_adapter.go

  internal/report/
    markdown.go
    json.go
    provenance.go

  prompts/
    worker_a_constructive.md
    worker_b_adversarial.md
    evaluator_e.md
```

## Adopted design decisions from kimi.md

```text
ADOPT-KIMI-001: Add Standard MCTS baseline.
ADOPT-KIMI-002: Add Tree-of-Thoughts BFS baseline.
ADOPT-KIMI-003: Add UCB1 / PUCT selectors.
ADOPT-KIMI-004: Add generic batch executor.
ADOPT-KIMI-005: Add DOT renderer; HTML renderer later.
ADOPT-KIMI-006: Add backup strategy abstraction.
ADOPT-KIMI-007: Add local model adapter in ebp-paper-eval.
ADOPT-KIMI-008: Add early stopping config in ebp-paper-eval.
ADOPT-KIMI-009: Adopt report metadata/budget/best-path/all-paths layout.
ADOPT-KIMI-010: Keep prompt templates as files under prompts/.
```

## Rejected or modified items

```text
MODIFY-KIMI-001: ABMCTS-M is documented but deferred.
MODIFY-KIMI-002: Use internal mutable tree, not fully immutable tree.
MODIFY-KIMI-003: E is not a core TreeQuest concept.
MODIFY-KIMI-004: Score means audit quality, not promotion readiness.
MODIFY-KIMI-005: Replace Promoted bool with HumanReviewRequired + PromotionBlockers.
MODIFY-KIMI-006: LLMProvider remains only in ebp-paper-eval.
MODIFY-KIMI-007: EBP/Workbench packages remain outside treequest-go.
```

## Updated build order after cherry-picking

```text
Phase 1: treequest-go core
1. types, node, tree
2. checkpoint using flat node records
3. Beta Thompson sampler
4. AB-MCTS-A
5. ask/tell and askBatch/tell
6. TopK
7. deterministic fake-action tests

Phase 2: generic baselines and tooling
8. repeated sampling
9. sequential refinement
10. standard MCTS
11. Tree-of-Thoughts BFS
12. UCB1 / PUCT
13. backup strategies
14. batch executor
15. DOT renderer

Phase 3: ebp-paper-eval app
16. paper ingestion
17. claim extraction
18. LLM providers
19. A/B/E agents
20. budget stack
21. Workbench2 + EBP state
22. TreeQuest adapter
23. Markdown/JSON report

Phase 4: validation
24. fake-paper deterministic test
25. small real-paper smoke test
26. compare AB-MCTS-A vs repeated/sequential/standard MCTS
27. check budget exhaustion visibility
28. human review of generated report
```

## Final judgment

Kimi.md adds meaningful net new value. Its best contribution is that it expands the TreeQuest-Go plan from “AB-MCTS-A port” into a reusable **search toolkit** with baselines, selectors, batch execution, backup strategies, and rendering.

But it should be adopted with the same architectural discipline:

```text
TreeQuest-Go remains generic.
EBP/Workbench/LLM logic remains separate.
AB-MCTS-M waits.
Scores mean audit quality, not truth.
Budget exhaustion becomes visible incompleteness.
Reports require human review.
```

## EBP self-audit

**Claims:** Kimi.md adds net new implementation value, especially baselines, batch executor, renderers, backup strategies, and report layout. It should be cherry-picked, not adopted whole.

**needMap:** Generic search toolkit maps to `treequest-go`; EBP/Workbench paper analysis maps to `ebp-paper-eval`.

**needInvariant:** Preserve domain neutrality, LLM neutrality, score normalization, ask/tell semantics, budget visibility, and no truth/promotion certification.

**needToyCheck:** Test all adopted generic features with fake deterministic generators before connecting LLMs.

**needNullModel:** Baseline algorithms from Kimi are now adopted as null models against AB-MCTS-A.

**needObstruction:** Main risks are scope creep, premature AB-MCTS-M, evaluator-as-truth, and report language that implies promotion.

**needFaithfulnessReview:** Compare AB-MCTS-A behavior against upstream TreeQuest on controlled toy runs before claiming faithful port.

**Promotion status:** Kimi.md is accepted as a design-input source with cherry-picked refinements; implementation remains unpromoted pending tests.




## prompt

analyze attached deepseek.md if it adds net new and if so, cherry pick and adopt them:


Yes — **deepseek.md adds some net new value**, but I would adopt only a small subset. Its best contribution is **not** the AB-MCTS algorithm detail, because some of that drifts toward ordinary UCB MCTS and hard-coded A/B alternation. Its useful contribution is the idea of **generic resource/search statistics** and a possible **optional Agent/Evaluator adapter layer** for users who prefer classic MCTS abstractions. 

## Bottom-line verdict

Use `deepseek.md` as:

```text
Supplemental API Design Notes v0.3
```

Do **not** use it as the TreeQuest-Go base.

It is weaker than the previous Kimi design for the actual TreeQuest port, because it subtly changes the algorithm from TreeQuest-style AB-MCTS-A into a more ordinary alternating-agent MCTS with UCB selection. That may be useful as a **baseline**, but it should not define the core AB-MCTS implementation.

## What deepseek.md adds net new

### 1. Generic `SearchStats`

Adopt this.

Deepseek suggests returning usage statistics such as constructive calls, adversarial calls, evaluator calls, and budget information.  The wording is too LLM-specific, but the idea is good.

In `treequest-go`, make it generic:

```go
type SearchStats struct {
	Iterations       int            `json:"iterations"`
	TrialsAsked      int            `json:"trials_asked"`
	TrialsTold       int            `json:"trials_told"`
	NodesCreated     int            `json:"nodes_created"`
	MaxDepthReached   int            `json:"max_depth_reached"`
	ActionCounts      map[string]int `json:"action_counts"`
	ActionErrorCounts map[string]int `json:"action_error_counts"`
	StartedAtUnix     int64          `json:"started_at_unix"`
	FinishedAtUnix    int64          `json:"finished_at_unix"`
	StopReason        string         `json:"stop_reason"`
}
```

Then `ebp-paper-eval` can translate this into:

```text
Worker A calls used
Worker B calls used
Evaluator calls used
Budget exhausted
Turns used
Cost estimate
```

TreeQuest-Go should not call these “LLM calls.”

### 2. `State / Action / Agent / Evaluator` as an optional adapter

Adopt as an **adapter**, not as the core API.

Deepseek proposes:

```go
type State interface { Clone() State }
type Action interface { Apply(State) State }
type Agent interface { GenerateActions(ctx, state) ([]Action, error) }
type Evaluator interface { Evaluate(ctx, state) (float64, error) }
```

This is useful for some Go users, especially those coming from classic MCTS. But it is less flexible than the generic `GenerateFn[S]` / `Ask-Tell` contract because it forces a particular decomposition of “action” and “evaluation.”

So the core should remain:

```go
type Trial[S any] struct {
	ID          TrialID
	ParentID    NodeID
	ParentState S
	ActionLabel ActionLabel
}

type Result[S any] struct {
	TrialID TrialID
	State   S
	Score   Score
	Meta    map[string]any
}
```

Then optionally provide:

```go
package adapters

type Agent[S any, A any] interface {
	GenerateActions(ctx context.Context, state S) ([]A, error)
}

type Transition[S any, A any] interface {
	Apply(ctx context.Context, state S, action A) (S, error)
}

type Evaluator[S any] interface {
	Evaluate(ctx context.Context, state S) (treequest.Score, error)
}
```

This gives users the classic shape without contaminating the core.

### 3. Context deadline emphasis

Adopt.

Deepseek correctly stresses `context.Context` for cancellation, timeouts, and long-running searches.  We already had this implicitly, but it should become explicit in the API contract.

Every external call path should accept context:

```go
Ask(ctx, tree)
AskBatch(ctx, tree, n)
Tell(ctx, tree, result)
Run(ctx, root, runner)
```

For core pure tree operations, `ctx` may be optional. For anything that might block or use an executor, `ctx` is mandatory.

### 4. Error-handling policy

Adopt, with refinement.

Deepseek notes that external calls can fail and the search loop should handle errors gracefully.  Good.

In TreeQuest-Go, use a result status rather than throwing away failed trials silently:

```go
type Result[S any] struct {
	TrialID TrialID
	State   S
	Score   Score
	Err     error
	Meta    map[string]any
}
```

Then config decides:

```go
type ErrorPolicy string

const (
	ErrorPolicySkip      ErrorPolicy = "skip"
	ErrorPolicyPenalty   ErrorPolicy = "penalty"
	ErrorPolicyRetryable ErrorPolicy = "retryable"
)
```

For EBP, failed or budget-exhausted worker calls become visible incompleteness, not hidden failure.

### 5. Search-level budget belongs in TreeQuest-Go

Adopt carefully.

Deepseek suggests config fields like `MaxIterations`, `Timeout`, and `Seed`.  Good, but remove `MaxLLMCalls`.

TreeQuest-Go should support:

```go
type Budget struct {
	MaxIterations int
	MaxNodes      int
	MaxDepth      int
	MaxTrials     int
	Timeout       time.Duration
}
```

The EBP app supports:

```go
type LLMBudget struct {
	MaxTotalCalls       int
	MaxConstructiveTurns int
	MaxAdversarialTurns  int
	MaxEvaluatorTurns    int
	MaxTokensPerTurn   int
	MaxCostUSD         float64
}
```

This preserves the boundary.

## What to reject or modify

### 1. Reject hard-coded constructive/adversarial alternation in TreeQuest-Go

Deepseek describes AB-MCTS as alternating A and B by tree depth.  That may fit your EBP app, but it must not be built into the general library.

TreeQuest-Go should not know:

```text
constructive
adversarial
evaluator
A/B alternation
depth parity role assignment
```

Instead, the app supplies action labels:

```go
[]ActionLabel{"worker_a", "worker_b", "evaluator", "synthesizer"}
```

and the algorithm chooses actions generically.

### 2. Reject `MaxLLMCalls` in the core config

Deepseek puts `MaxLLMCalls` inside `ABMCTSConfig`.  This violates the “TreeQuest-Go does not know LLMs exist” boundary.

Rename generically if needed:

```go
MaxTrials
MaxExternalCalls
```

But I prefer:

```go
MaxTrials
```

because every `Tell` corresponds to one completed trial.

### 3. Reject UCB1 as AB-MCTS-A core

Deepseek’s search loop uses UCB1 selection.  That is fine for `standard_mcts.go`, but not for `abmcts_a.go`.

Adopt UCB1 as a baseline:

```text
treequest/algo/standard_mcts.go
treequest/bandit/ucb.go
```

But AB-MCTS-A should remain Thompson-sampling based, with GEN/CONT or wider/deeper decision logic.

### 4. Reject `State interface { Clone() State }` as mandatory

Mandatory clone interfaces are awkward in Go and reduce generic usability.

Use:

```go
type Tree[S any]
```

and let the application decide whether `S` is immutable, copied, pointer-based, or internally cloned.

If a helper needs cloning, allow optional config:

```go
type CloneFunc[S any] func(S) S
```

But do not require all states to implement `Clone()`.

### 5. Modify “Evaluator” location

Deepseek makes `Evaluator` part of the library-level abstraction. This is okay only as an optional adapter.

The core library should not require a separate evaluator because some applications return `(state, score)` directly from generation, while others evaluate externally, and others may evaluate asynchronously.

Core result remains:

```go
Result[S]{State: newState, Score: score}
```

The app can compute that score however it wants.

## Cherry-picked adoption list

```text
ADOPT-DEEPSEEK-001: Add generic SearchStats to TreeQuest-Go.
ADOPT-DEEPSEEK-002: Add optional Agent/Action/Evaluator adapter package, not core requirement.
ADOPT-DEEPSEEK-003: Make context cancellation explicit in executor and run helpers.
ADOPT-DEEPSEEK-004: Add explicit ErrorPolicy and failed-trial accounting.
ADOPT-DEEPSEEK-005: Add generic search-level budget: max iterations, nodes, depth, trials, timeout.
ADOPT-DEEPSEEK-006: Let EBP app render SearchStats into report metadata.
ADOPT-DEEPSEEK-007: Use UCB1 only as Standard MCTS baseline, not AB-MCTS-A core.
```

## Rejected or modified items

```text
REJECT-DEEPSEEK-001: No hard-coded constructive/adversarial alternation in treequest-go.
REJECT-DEEPSEEK-002: No MaxLLMCalls in the core library.
REJECT-DEEPSEEK-003: No mandatory State.Clone interface.
REJECT-DEEPSEEK-004: No required Evaluator interface in the core.
REJECT-DEEPSEEK-005: No UCB1-based AB-MCTS-A implementation.
REJECT-DEEPSEEK-006: No EBP, Lean, physics, paper, or LLM terminology in treequest-go.
```

## Updated TreeQuest-Go core config

After cherry-picking, I would use:

```go
type Config struct {
	Seed int64

	Budget Budget

	ScoreRange ScoreRange

	ErrorPolicy ErrorPolicy

	Backup BackupStrategy

	Selection SelectionStrategy
}

type Budget struct {
	MaxIterations int
	MaxNodes      int
	MaxDepth      int
	MaxTrials     int
	Timeout       time.Duration
}

type ScoreRange struct {
	Min float64 // default 0
	Max float64 // default 1
}

type SearchStats struct {
	Iterations       int
	TrialsAsked      int
	TrialsTold       int
	NodesCreated     int
	MaxDepthReached   int
	ActionCounts      map[ActionLabel]int
	ActionErrorCounts map[ActionLabel]int
	StopReason        string
}
```

## Updated EBP app budget config

```go
type AgentBudgetConfig struct {
	MaxTurns         int
	MaxTokensPerTurn int
	MaxCalls         int
	MaxCostUSD       float64
}

type EBPBudgetConfig struct {
	Constructive AgentBudgetConfig
	Adversarial  AgentBudgetConfig
	Evaluator    AgentBudgetConfig

	MaxTotalLLMCalls int
	MaxTotalTokens   int
	MaxTotalCostUSD  float64
}
```

Budget exhaustion becomes state metadata:

```go
type BudgetEvent struct {
	Actor            string
	LimitType        string
	LimitValue       string
	Used             string
	BudgetExhausted  bool
	EffectOnAnalysis string
}
```

## Final judgment

Deepseek.md adds **moderate net new value**. It is less faithful to TreeQuest-Go’s intended generic AB-MCTS-A core than Kimi’s proposal, but it contributes useful implementation details around stats, cancellation, errors, budgets, and optional classic MCTS-style interfaces.

Use it as a **supplemental design input**, not as the base.

## EBP self-audit

**Claims:** Deepseek.md adds moderate net value through SearchStats, context/error policy, generic budgets, and optional Agent/Evaluator adapters, but its A/B alternation and UCB-style loop should not define TreeQuest-Go.

**needMap:** Deepseek’s useful pieces map into generic `SearchStats`, `Budget`, `ErrorPolicy`, and optional adapters. EBP-specific A/B/E logic maps only to the separate app.

**needInvariant:** Preserve domain neutrality, action neutrality, normalized score, ask/tell separation, no LLM terminology in core, and no truth/promotion certification.

**needToyCheck:** Test SearchStats and ErrorPolicy with fake failing actions before LLM integration.

**needNullModel:** UCB1 MCTS becomes a baseline/null model for AB-MCTS-A, not a replacement.

**needObstruction:** Main risks are algorithm drift, hard-coded A/B role contamination, and LLM-budget leakage into the core library.

**needFaithfulnessReview:** Verify AB-MCTS-A against upstream TreeQuest behavior separately; Deepseek’s UCB loop should not be treated as faithful TreeQuest AB-MCTS.

**Promotion status:** Cherry-picked as supplemental design input; not accepted as the implementation base.



## prompt

analyze attached z.md if it adds net new and if so, cherry pick and adopt them:



Yes — **z.md adds real net new value**, and it is probably the **most useful supplemental design note so far**, but I would still **cherry-pick**, not adopt wholesale.

Its strongest contributions are:

```text
1. Observability hooks
2. Reward components + scalar normalization
3. Budget snapshots persisted with checkpoints
4. Declared budget + consumed budget in output
5. Sibling diversity hints
6. Budget exhaustion → dormant, not dead
7. Per-role / per-claim / per-paper budget scopes
8. Typed “annotated paper + JSON sidecar” output
```

The parts I would **not** adopt unchanged are the UCB-style AB-MCTS description, `any`-first core design, `LLMClient` terminology in core interfaces, and putting turn/token budgets too deeply into the generic library. 

## Bottom-line verdict

Use `z.md` as:

```text
TreeQuest-Go / EBP Runner Design Notes v0.4 — accepted with repairs
```

It adds more implementation-level maturity than the previous notes. It is especially good on **auditability**, **budget provenance**, and **output artifacts**.

But it should not become the implementation base because its AB-MCTS description leans toward **UCB-style sibling/refine selection**, while our chosen faithful MVP remains **AB-MCTS-A with Thompson sampling / Beta bandits**. UCB can be a baseline or alternate branching policy, not the default AB-MCTS-A core.

## Net-new items to adopt

### ADOPT-Z-001: Observability hooks

This is a strong addition.

Adopt this into `treequest-go`:

```go
type Hooks[S any] struct {
	OnSelect      func(SelectEvent[S])
	OnAsk         func(AskEvent[S])
	OnTell        func(TellEvent[S])
	OnExpand      func(ExpandEvent[S])
	OnEvaluate    func(EvaluateEvent[S])
	OnBackprop    func(BackpropEvent[S])
	OnBudgetEvent func(BudgetEvent)
	OnCheckpoint  func(CheckpointEvent)
}
```

This is better than `log.Printf`. The EBP app can use hooks to build provenance trails, while other apps can ignore them.

### ADOPT-Z-002: Reward components + scalar score

z.md proposes `Reward` with a scalar score plus component map and justification. This is excellent. 

Adopt:

```go
type Reward struct {
	Score         Score              `json:"score"`
	Components    map[string]float64 `json:"components,omitempty"`
	Justification string             `json:"justification,omitempty"`
	EvaluatorID   string             `json:"evaluator_id,omitempty"`
	Meta          map[string]any     `json:"meta,omitempty"`
}
```

Core algorithm uses only `Reward.Score`.

The EBP app can attach:

```text
clarity
source_grounding
bridge_strength
inverse_constraint
risk_penalty
incompleteness_visibility
```

This preserves generic search while allowing rich report provenance.

### ADOPT-Z-003: Normalizer interface

This is also worth adopting.

```go
type Normalizer interface {
	Normalize(Reward) (Score, error)
}
```

Default:

```go
type IdentityNormalizer struct{}
```

The core remains single-objective, but users can submit multi-component reward metadata and scalarize it before tree update.

### ADOPT-Z-004: Budget snapshots persisted with checkpoints

This is one of the best additions in z.md.

Adopt this rule:

```text
Checkpoint = graph/tree snapshot + algorithm stats + budget snapshot + config snapshot.
```

Without budget persistence, resume can overspend. z.md explicitly identifies checkpoint/budget drift as a pitfall. 

Checkpoint shape:

```go
type Checkpoint[S any] struct {
	SchemaVersion string       `json:"schema_version"`
	Tree          TreeSnapshot[S] `json:"tree"`
	Stats         SearchStats  `json:"stats"`
	Budget        BudgetSnapshot `json:"budget"`
	ConfigHash    string       `json:"config_hash"`
	CreatedAtUnix int64       `json:"created_at_unix"`
}
```

### ADOPT-Z-005: Declared budget and consumed budget in output

This is important for the EBP runner.

The report should include both:

```text
declared_budget
consumed_budget
```

Not just “turns used.”

Adopt for `ebp-paper-eval`:

```go
type BudgetProvenance struct {
	Declared BudgetConfig   `json:"declared"`
	Consumed BudgetSnapshot `json:"consumed"`
	Exhausted bool          `json:"exhausted"`
	StopReason string       `json:"stop_reason"`
}
```

This fits EBP’s “seamless accounting” spirit: the accounting should be mostly invisible during the run, but auditable afterward.

### ADOPT-Z-006: Per-role / per-claim / per-paper budget scopes

This is better than only global budgets.

Adopt in `ebp-paper-eval`, not the generic library:

```go
type PaperBudgetConfig struct {
	PerRole  map[string]AgentBudgetConfig `json:"per_role"`
	PerClaim *ClaimBudgetConfig           `json:"per_claim,omitempty"`
	PerPaper TotalBudgetConfig            `json:"per_paper"`
}
```

Recommended defaults:

```text
per-role: on
per-paper: on
per-claim: optional
```

This prevents one difficult claim from consuming the entire paper audit.

### ADOPT-Z-007: Budget exhaustion → dormant, not dead

This is a perfect EBP-aligned rule.

Adopt:

```text
If a claim exhausts its budget before convergence, mark it dormant / unresolved_due_to_budget, not failed.
```

In app terms:

```go
type ClaimAuditStatus string

const (
	ClaimAuditComplete        ClaimAuditStatus = "complete"
	ClaimAuditIncomplete      ClaimAuditStatus = "incomplete"
	ClaimAuditBudgetExhausted ClaimAuditStatus = "budget_exhausted"
	ClaimAuditDormant         ClaimAuditStatus = "dormant_unresolved"
)
```

Do **not** mark as false, rejected, or disproven.

### ADOPT-Z-008: Sibling diversity hints

This is a very useful implementation detail.

z.md notes that without diversity hints, LLM-generated siblings can become near-duplicates.  Adopt in the EBP app and generic request metadata.

Generic TreeQuest trial can expose sibling summaries through metadata or adapter request:

```go
type ExpansionContext[S any] struct {
	ParentState S
	Siblings    []NodeSummary[S]
	Depth       int
	Mode        ExpansionMode
	Meta        map[string]any
}
```

In EBP prompts:

```text
Produce an evaluation distinct from these prior sibling evaluations:
- sibling 1: ...
- sibling 2: ...
```

### ADOPT-Z-009: Annotated paper + JSON sidecar

This is a strong output design.

Adopt:

```text
paper_annotated.md
paper_report.json
tree_snapshot.json
```

The Markdown is for humans. The JSON sidecar is for PTW/automation/review.

The sidecar should include:

```text
budget declared/consumed
tree stats
best path
alternatives considered
risk flags
open debts
next smallest useful move
source spans
model/provider provenance
```

### ADOPT-Z-010: Typed hooks over structured logging

Adopt. This belongs in `treequest-go`.

The EBP app can translate hooks into:

```text
provenance events
report appendix
audit trace
debug output
```

### ADOPT-Z-011: Error semantics: recoverable vs fatal

Adopt.

```go
type ErrorKind string

const (
	ErrorRecoverable ErrorKind = "recoverable"
	ErrorFatal       ErrorKind = "fatal"
	ErrorBudget      ErrorKind = "budget"
	ErrorInvalid     ErrorKind = "invalid"
)

type TrialError struct {
	Kind    ErrorKind `json:"kind"`
	Message string    `json:"message"`
	Cause   error     `json:"-"`
}
```

This matters because LLM/API transient failures should not have the same semantics as malformed config or exhausted budget.

### ADOPT-Z-012: Determinism note

Adopt the distinction:

```text
TreeQuest-Go can seed internal search randomness.
It cannot seed external LLM behavior reliably.
External outputs must be recorded for reproducibility.
```

Good rule.

### ADOPT-Z-013: MaxRefineDepth and MinRewardDelta

Adopt, but as optional app/search config.

This prevents infinite refinement loops:

```go
type RefinementConfig struct {
	MaxRefineDepth int
	MinRewardDelta float64
}
```

For EBP, if refinements do not improve audit quality or debt movement, stop refining and go wider.

## Items to modify or reject

### MODIFY-Z-001: Do not use `LLMClient` in `treequest-go`

z.md says provider interfaces may include `LLMClient`, even if no concrete implementations are imported.  I would reject that name in the core.

Use:

```go
type Generator[S any] interface { ... }
type Evaluator[S any] interface { ... }
```

No `LLMClient` term in `treequest-go`.

`LLMClient` belongs only in `ebp-paper-eval`.

### MODIFY-Z-002: Do not make `Problem any` and `Solution any` the only core design

z.md recommends `any` for `Problem` and `Solution` because Python uses untyped `Any`.  I would not make that the main Go API.

Prefer generics:

```go
type Tree[S any]
```

where `S` is the whole user-defined node state.

Then optionally offer an adapter:

```go
type ProblemSolutionState[P any, Sol any] struct {
	Problem  P
	Solution Sol
	Meta     map[string]any
}
```

This gives both options:

```text
idiomatic Go generic state
optional Python-like problem/solution adapter
```

### MODIFY-Z-003: UCB sibling/refine should not define AB-MCTS-A MVP

z.md describes UCB-style sibling/refine selection and adaptive policy.  Useful, but not our default.

Adopt as:

```text
alternate branching policy / baseline
```

Not as:

```text
AB-MCTS-A faithful port
```

Default AB-MCTS-A remains Thompson/Beta based.

### MODIFY-Z-004: Budget subsystem should be split

z.md puts turns/tokens/wallclock budget in the library.  I would split it:

In `treequest-go`:

```go
type SearchBudget struct {
	MaxIterations int
	MaxNodes      int
	MaxDepth      int
	MaxTrials     int
	Timeout       time.Duration
}
```

In `ebp-paper-eval`:

```go
type ResourceBudget struct {
	Turns  int
	Tokens int
	CostUSD float64
	WallClock time.Duration
}
```

Optional generic budget interfaces can exist in TreeQuest-Go, but no role names like `A`, `B`, `E`, and no token semantics in core.

### MODIFY-Z-005: “Provider” package name

z.md uses `provider`. I would rename for clarity:

```text
treequest/runner or treequest/executor
```

Because “provider” sounds like model/provider/LLM.

If we keep it, it must be strictly generic.

### REJECT-Z-001: Do not ship a config-driven CLI in TreeQuest-Go v0.1

z.md proposes `cmd/treequest` and YAML/JSON config loaders.  Useful later, but not v0.1.

Core library first. CLI later.

### REJECT-Z-002: Do not make SQLite checkpointing early

File checkpoint first. SQLite backend later.

### REJECT-Z-003: Do not let library know A/B/E roles

A/B/E is EBP app logic only.

## Updated adopted architecture

### `treequest-go`

```text
treequest-go/
  treequest/
    types.go
    tree.go
    node.go
    stats.go
    reward.go
    normalizer.go
    checkpoint.go
    topk.go
    hooks.go

  treequest/algo/
    abmcts_a.go
    repeated.go
    sequential.go
    standard_mcts.go
    tree_of_thoughts.go

  treequest/bandit/
    beta.go
    thompson.go
    ucb.go

  treequest/batch/
    trial.go
    executor.go

  treequest/backup/
    mean.go
    max.go

  treequest/render/
    dot.go

  examples/
    toy_non_llm/
```

### `ebp-paper-eval`

```text
ebp-paper-eval/
  internal/paper/
  internal/claims/
  internal/llm/
  internal/agents/
  internal/workbench/
  internal/ebp/
  internal/search/
  internal/budget/
  internal/report/
  prompts/
  cmd/ebp-paper-eval/
```

## Final cherry-pick list

```text
ADOPT-Z-001: Observability hooks.
ADOPT-Z-002: Reward components + justification metadata.
ADOPT-Z-003: Normalizer interface.
ADOPT-Z-004: Budget snapshot persisted with checkpoint.
ADOPT-Z-005: Declared + consumed budget in output.
ADOPT-Z-006: Per-role / per-claim / per-paper budget scopes in EBP app.
ADOPT-Z-007: Budget exhaustion marks claim dormant/unresolved, not dead.
ADOPT-Z-008: Sibling diversity hints.
ADOPT-Z-009: Annotated paper + JSON sidecar.
ADOPT-Z-010: Typed hooks instead of logging.
ADOPT-Z-011: Recoverable vs fatal error semantics.
ADOPT-Z-012: Determinism distinction between seeded search and unseeded LLMs.
ADOPT-Z-013: Optional MaxRefineDepth and MinRewardDelta.
```

```text
MODIFY-Z-001: No LLMClient terminology in treequest-go.
MODIFY-Z-002: Prefer generics over any-only Problem/Solution.
MODIFY-Z-003: UCB sibling/refine becomes alternate policy, not AB-MCTS-A MVP.
MODIFY-Z-004: Split generic search budget from EBP resource budgets.
MODIFY-Z-005: Rename provider package or keep strictly generic.
```

```text
REJECT-Z-001: No config-driven CLI in treequest-go v0.1.
REJECT-Z-002: No SQLite checkpointing in v0.1.
REJECT-Z-003: No A/B/E roles inside treequest-go.
```

## Final judgment

`z.md` adds **high net new value** because it pushes the design from “algorithm port” toward a real auditable system: hooks, reward metadata, budget provenance, checkpoint discipline, diversity control, and output sidecars.

But it must be filtered through the locked architecture:

```text
TreeQuest-Go = generic adaptive search library.
EBP paper evaluator = separate app.
AB-MCTS-A = Thompson/Beta MVP.
UCB sibling/refine = optional baseline/policy.
Budgets in core = search limits only.
Turn/token/cost budgets = EBP app.
Reports = app layer.
```

## EBP self-audit

**Claims:** z.md adds high-value refinements but should be cherry-picked, not adopted whole.

**needMap:** z.md’s useful pieces map into hooks, rewards, normalizers, checkpoint-budget snapshots, budget provenance, and EBP output sidecars.

**needInvariant:** Preserve generic TreeQuest core, LLM neutrality, EBP separation, normalized scalar reward, source/provenance visibility, and no truth/promotion certification.

**needToyCheck:** Test observability hooks, budget snapshots, reward normalization, and checkpoint/resume with deterministic non-LLM examples.

**needNullModel:** UCB sibling/refine becomes a baseline or alternate policy against Thompson/Beta AB-MCTS-A.

**needObstruction:** Main risks are core contamination by LLM/provider/role terms, algorithm drift away from TreeQuest, and budget subsystem overreach.

**needFaithfulnessReview:** Compare AB-MCTS-A implementation against upstream TreeQuest separately before claiming faithful port.

**Promotion status:** Accepted as high-value supplemental design input; not promoted as implementation spec until repaired into the final architecture.




## prompt

analyze attached qwen.md if it adds net new and if so, cherry pick and adopt them:



Yes — **qwen.md adds net new value**, especially around **API ergonomics and boundary language**, but it is weaker on the actual AB-MCTS algorithm details. I would cherry-pick its Go/API architecture refinements, not its algorithmic recommendations wholesale. 

## Bottom-line verdict

Use `qwen.md` as:

```text
TreeQuest-Go API Boundary Notes v0.5 — accepted with repairs
```

It adds useful refinements in five areas:

```text
1. Consumer-defined interfaces
2. Ask/Tell as the primary public API
3. Functional options for configuration
4. Raw score vs normalized internal score separation
5. Clear library-vs-application budget boundary
```

But it also contains items I would **reject or modify**:

```text
1. UCT/UCB as the v0.1 default AB-MCTS core
2. Checkpointing fully pushed out of the core
3. Mandatory interface-heavy design over generics
4. Version folder layout like /v0
5. Raw score allowed as arbitrary non-numeric type
```

## Net-new items to adopt

### ADOPT-QWEN-001: Consumer-defined interfaces

This is a good Go design principle. Qwen emphasizes that the library should expose minimal contracts and let the consuming app define the concrete implementation. 

Adopt this as a design rule:

```text
treequest-go defines the smallest interfaces it needs.
Consumers define concrete generators, evaluators, scorers, and state types.
```

But I would avoid making everything an interface. In Go, the best version is usually:

```go
type GenerateFunc[S any] func(ctx context.Context, parent S) (S, Reward, error)
```

plus optional interfaces for users who want named structs:

```go
type Generator[S any] interface {
	Generate(ctx context.Context, req GenerateRequest[S]) (GenerateResult[S], error)
}
```

So: **adopt the principle, not an interface-heavy design.**

### ADOPT-QWEN-002: Ask/Tell as the primary API

This is a strong reinforcement. Qwen correctly argues that `Ask`/`Tell` is better for asynchronous evaluators, external services, batching, and budget control. 

Lock this in:

```go
trial, err := searcher.Ask(ctx)
result := runExternalWork(ctx, trial)
err = searcher.Tell(ctx, result)
```

And:

```go
trials, err := searcher.AskBatch(ctx, n)
results := executor.Run(ctx, trials, runner)
for _, r := range results {
	searcher.Tell(ctx, r)
}
```

`Step()` becomes only a helper:

```go
func Step[S any](ctx context.Context, s *Searcher[S], run TrialRunner[S]) error
```

### ADOPT-QWEN-003: Functional options pattern

This is genuinely useful and should be adopted.

Instead of huge config constructors:

```go
searcher := treequest.NewSearcher(
	treequest.WithSeed(42),
	treequest.WithMaxNodes(10_000),
	treequest.WithMaxDepth(30),
	treequest.WithAlgorithm(algo.NewABMCTSA()),
	treequest.WithNormalizer(treequest.IdentityNormalizer{}),
	treequest.WithHooks(hooks),
)
```

This is idiomatic Go and keeps the API flexible.

### ADOPT-QWEN-004: Terminology discipline

Qwen reinforces the point that “model” should be avoided in the core API. 

Adopt this vocabulary:

```text
ActionLabel
Generator
Evaluator
Scorer
Expansion
Trial
Result
Score
Reward
Utility
```

Avoid in `treequest-go`:

```text
LLM
model
prompt
response
worker
constructive
adversarial
evaluator E
EBP
claim
paper
debt
```

### ADOPT-QWEN-005: Raw score vs normalized score separation

This is useful, with one repair.

Qwen suggests user evaluators may return raw scores and the core can normalize to `[0,1]`.  Adopt the idea, but require the normalized score before tree update.

Use:

```go
type Reward struct {
	RawScore      any                `json:"raw_score,omitempty"`
	Score         Score              `json:"score"`
	Components    map[string]float64 `json:"components,omitempty"`
	Justification string             `json:"justification,omitempty"`
	Meta          map[string]any     `json:"meta,omitempty"`
}
```

Core algorithm uses only:

```go
Reward.Score
```

If `RawScore` exists, the user must supply:

```go
type Normalizer interface {
	Normalize(raw Reward) (Reward, error)
}
```

This avoids letting arbitrary raw score formats leak into AB-MCTS math.

### ADOPT-QWEN-006: Metadata as first-class but uninterpreted

Qwen emphasizes arbitrary metadata for debugging, provenance, interpretability, citations, and domain-specific traces.  Adopt.

Every node/result should allow:

```go
Meta map[string]any
```

But TreeQuest-Go never interprets it.

For EBP, metadata can hold:

```text
source spans
claim IDs
risk flags
budget events
LLM model IDs
worker role
prompt hash
evidence status
```

### ADOPT-QWEN-007: Budget boundary language

Qwen clearly separates search-intrinsic budgets from application budgets.  Adopt this distinction.

In `treequest-go`:

```go
type SearchBudget struct {
	MaxIterations int
	MaxNodes      int
	MaxDepth      int
	MaxTrials     int
	Timeout       time.Duration
}
```

In `ebp-paper-eval`:

```go
type ResourceBudget struct {
	MaxTurns          int
	MaxTokens         int
	MaxCostUSD        float64
	MaxLLMCalls       int
	MaxTokensPerTurn  int
	MaxWallClock      time.Duration
}
```

TreeQuest-Go tracks trials/nodes/depth. The EBP app tracks LLM turns/tokens/cost.

## Items to reject or modify

### MODIFY-QWEN-001: Do not make UCT the default AB-MCTS-A core

Qwen recommends a standard UCT-style exploration strategy for v0.1.  That is fine for `standard_mcts.go`, but not for the TreeQuest AB-MCTS-A port.

Final decision:

```text
AB-MCTS-A default = Thompson/Beta adaptive branching.
UCT/UCB = baseline / alternate selection strategy.
```

### MODIFY-QWEN-002: Do not push checkpointing entirely out of core

Qwen argues checkpointing should be application-level and the core should only provide marshal/snapshot functions.  Partly right.

Adopt this split:

```text
treequest-go provides:
  Snapshot()
  Restore()
  JSON marshal/unmarshal helpers
  checkpoint data structs

application provides:
  filesystem path
  database backend
  S3 backend
  persistence schedule
```

So checkpointing support belongs in the core as **state serialization**, but storage backends are optional/app-level.

### MODIFY-QWEN-003: Avoid arbitrary raw score as the only score

Qwen says raw score could be an integer, float, string, or complex struct.  This is okay for metadata, but not for algorithm input.

Final rule:

```text
Every Tell() must include a normalized Score in [0,1].
RawScore may exist only as metadata or normalizer input.
```

### MODIFY-QWEN-004: Do not use `/v0` folder layout

Qwen proposes a `v0/` directory.  I would not adopt that for a normal Go module.

Use semantic import versioning later if needed:

```text
github.com/PithomLabs/treequest-go
github.com/PithomLabs/treequest-go/v2
```

For now:

```text
treequest/
treequest/algo/
treequest/bandit/
```

is cleaner.

### MODIFY-QWEN-005: Do not let “policy” become core terminology

Qwen warns against ML-heavy words but also includes “PolicyFunction.” I would avoid `Policy` in the public core unless referring to a clearly generic “SelectionStrategy.”

Prefer:

```text
SelectionStrategy
ExpansionStrategy
BranchingStrategy
BackupStrategy
```

over:

```text
Policy
PolicyFunction
```

### REJECT-QWEN-001: Do not defer all parallelization too far

Qwen suggests single-threaded v0.1 and parallelization later.  I agree the core algorithm should be simple, but `AskBatch` should be present from the start because it shapes the API.

So:

```text
Do not implement complex internal parallel MCTS in v0.1.
Do implement AskBatch in v0.1.
Do implement generic batch executor in v0.1 or v0.2.
```

## Cherry-picked adoption list

```text
ADOPT-QWEN-001: Consumer-defined interface principle.
ADOPT-QWEN-002: Ask/Tell as primary API; Step only convenience wrapper.
ADOPT-QWEN-003: Functional options for configuration.
ADOPT-QWEN-004: Neutral terminology: action/generator/scorer/trial/result.
ADOPT-QWEN-005: Raw reward metadata + normalized internal score.
ADOPT-QWEN-006: Node/result metadata as first-class but uninterpreted.
ADOPT-QWEN-007: Clear boundary between search budgets and app/resource budgets.
ADOPT-QWEN-008: Idiomatic Go over Python API compatibility.
ADOPT-QWEN-009: Optional thin Python-like adapter later, not core.
```

```text
MODIFY-QWEN-001: UCT is baseline, not AB-MCTS-A default.
MODIFY-QWEN-002: Core provides snapshots; app handles storage.
MODIFY-QWEN-003: Every Tell must include normalized [0,1] score.
MODIFY-QWEN-004: No /v0 folder layout for now.
MODIFY-QWEN-005: Prefer Strategy terminology over Policy terminology.
MODIFY-QWEN-006: AskBatch belongs early even if full parallel MCTS waits.
```

```text
REJECT-QWEN-001: No mandatory State interface.
REJECT-QWEN-002: No arbitrary raw score without normalized Score.
REJECT-QWEN-003: No EBP/LLM/paper concepts in treequest-go.
REJECT-QWEN-004: No direct Python API compatibility as the primary design goal.
```

## Updated `treequest-go` API after adopting Qwen refinements

```go
searcher := treequest.NewSearcher[MyState](
	treequest.WithAlgorithm(algo.NewABMCTSA()),
	treequest.WithSeed(42),
	treequest.WithMaxNodes(10_000),
	treequest.WithMaxDepth(30),
	treequest.WithNormalizer(treequest.IdentityNormalizer{}),
	treequest.WithHooks(hooks),
)

trial, err := searcher.Ask(ctx)
if err != nil {
	return err
}

result := treequest.Result[MyState]{
	TrialID: trial.ID,
	State:   nextState,
	Reward: treequest.Reward{
		Score: 0.82,
		Components: map[string]float64{
			"clarity": 0.9,
			"rigor":   0.7,
		},
		Justification: "passed synthetic evaluator",
	},
	Meta: map[string]any{
		"generator": "heuristic-a",
	},
}

err = searcher.Tell(ctx, result)
```

## Updated v0.1 scope

```text
v0.1 treequest-go:
  - generic Tree[S]
  - Node/Trial/Result/Reward
  - Ask/Tell
  - AskBatch
  - AB-MCTS-A Thompson/Beta
  - repeated sampling baseline
  - sequential refinement baseline
  - basic standard MCTS baseline
  - functional options
  - normalized score enforcement
  - metadata fields
  - SearchBudget: nodes/depth/trials/iterations/timeout
  - Snapshot/Restore JSON
  - deterministic non-LLM examples
```

Deferred:

```text
v0.2+:
  - DOT/HTML rendering
  - PUCT
  - advanced backup strategies
  - config-driven CLI
  - SQLite/S3 checkpoint storage
  - Python compatibility adapter
  - AB-MCTS-M
```

## Final judgment

`qwen.md` adds **moderate-to-high net new value**, mostly on **API design discipline**, **Ask/Tell justification**, **functional options**, **neutral terminology**, and **budget boundary language**.

It does not add much new on the AB-MCTS algorithm itself, and some of its algorithmic recommendations should be treated as baseline MCTS rather than faithful TreeQuest AB-MCTS-A.

## EBP self-audit

**Claims:** qwen.md adds useful API and boundary refinements but should be cherry-picked, not adopted wholesale.

**needMap:** Qwen’s useful ideas map into the public API layer: Ask/Tell, functional options, neutral terminology, metadata, normalization, and budget boundaries.

**needInvariant:** Preserve generic core, action neutrality, normalized score, app-level LLM budgets, no EBP/LLM terms in core, and idiomatic Go.

**needToyCheck:** Test Ask/Tell, functional options, metadata, and normalization with deterministic non-LLM examples.

**needNullModel:** Keep UCT/UCB as a baseline/null model against AB-MCTS-A, not as the main algorithm.

**needObstruction:** Risks include interface overengineering, arbitrary score ambiguity, and drifting from faithful TreeQuest AB-MCTS-A into generic MCTS.

**needFaithfulnessReview:** Before claiming TreeQuest compatibility, compare against upstream algorithm behavior and document intentional Go API divergences.

**Promotion status:** Cherry-picked as API-design input; not accepted as final implementation spec.




## prompt


analyze attached xiaomi.md if it adds net new and if so, cherry pick and adopt them:


Yes — **xiaomi.md adds net new value**, but it is a **mixed-quality input**. It has useful practical details around `GenerateFn[S]`, immutable-style API compatibility, parallel step ergonomics, and the “soft per-worker cap” idea. But it also reintroduces a few things we already decided against: `ABMCTS-M` too early, immutable tree as default, evaluator embedded in worker functions, and “max turns” as a TreeQuest-level concept. 

## Bottom-line verdict

Use `xiaomi.md` as:

```text
TreeQuest-Go Ergonomics Notes v0.6 — cherry-picked, not adopted whole
```

Its strongest useful additions are:

```text
1. Public facade file: treequest.go
2. Actions[S] map as a convenience wrapper
3. Parent pointer as *S where nil means root expansion
4. ResultEntry / TopK naming clarity
5. ParallelStep convenience helper built on Ask/Tell
6. Soft per-worker cap warning instead of hard per-worker cap
7. Dry-run cost estimation for the EBP app
8. “Each turn = worker call + evaluator call” budget explanation
```

But these must be adopted under the locked separation:

```text
treequest-go = generic library
ebp-evaluator = separate app
TreeQuest core = no LLM, no EBP, no paper, no A/B/E roles
```

## Net-new items to adopt

### ADOPT-XIAOMI-001: Add a public facade file

The file proposes `treequest.go` as the public API facade.  This is good.

Adopt:

```text
treequest.go
node.go
tree.go
trial.go
algorithm.go
```

But keep internal complexity in subpackages.

Suggested structure:

```text
treequest-go/
  treequest.go       # public facade and common constructors
  types.go           # Score, Reward, Trial, Result, ActionLabel
  node.go
  tree.go
  checkpoint.go
  topk.go
  hooks.go

  algo/
    abmcts_a.go
    repeated.go
    sequential.go
    standard_mcts.go
```

This makes the library easier to use without flattening all internals.

### ADOPT-XIAOMI-002: `Actions[S]` as a convenience type

This is useful:

```go
type GenerateFn[S any] func(ctx context.Context, parent *S) (S, float64, error)
type Actions[S any] map[string]GenerateFn[S]
```

But I would modify it:

```go
type ActionLabel string

type GenerateFn[S any] func(ctx context.Context, req GenerateRequest[S]) (GenerateResult[S], error)

type Actions[S any] map[ActionLabel]GenerateFn[S]
```

Why? `parent *S` is simple, but it does not carry enough context for sibling diversity, depth, parent node ID, or metadata. We should still support a simple helper, but the core request should be richer.

Adopt both:

```go
type SimpleGenerateFn[S any] func(ctx context.Context, parent *S) (S, Score, error)
```

and adapter:

```go
func FromSimple[S any](fn SimpleGenerateFn[S]) GenerateFn[S]
```

### ADOPT-XIAOMI-003: `nil` parent means root expansion

This is a clean ergonomic convention. Adopt it for the simple adapter only:

```go
func(ctx context.Context, parent *S) (S, Score, error)
```

In the full Ask/Tell API, prefer explicit fields:

```go
type Trial[S any] struct {
	ID          TrialID
	ParentID    NodeID
	ParentState *S
	ActionLabel ActionLabel
	Depth       int
	Meta        map[string]any
}
```

`ParentState == nil` can still mean root expansion.

### ADOPT-XIAOMI-004: `ResultEntry[S]` / `TopK` naming

This is a small but good usability improvement.

Adopt:

```go
type ResultEntry[S any] struct {
	NodeID NodeID `json:"node_id"`
	State  S      `json:"state"`
	Reward Reward `json:"reward"`
	Depth int     `json:"depth"`
	Path []NodeID `json:"path,omitempty"`
}

func (s *Searcher[S]) TopK(k int) []ResultEntry[S]
```

This is clearer than overloading `Result`, which should mean a pending trial result.

### ADOPT-XIAOMI-005: `ParallelStep` convenience helper

Xiaomi’s `ParallelStep` helper is useful as a convenience wrapper over `AskBatch` / `Tell`.  Adopt as a helper, not the primary API.

Correct location:

```text
treequest/batch/parallel.go
```

Correct principle:

```text
Ask/Tell remains primary.
ParallelStep is sugar for simple users.
```

Signature:

```go
func ParallelStep[S any](
	ctx context.Context,
	searcher *Searcher[S],
	actions Actions[S],
	workers int,
) error
```

This is useful for examples and toy runs.

### ADOPT-XIAOMI-006: Soft per-worker caps in the EBP app

This is one of the best net-new suggestions.

Xiaomi argues that hard per-worker caps can fight AB-MCTS because Thompson Sampling should allocate more budget to the productive action.  I agree, with nuance.

Adopt in `ebp-evaluator`:

```text
--max-turns              hard total worker-turn budget
--max-turns-per-worker   soft warning cap by default
--hard-turns-per-worker  optional strict cap
```

Why this is good: if adversarial B is producing more useful improvements for a given paper, the search should be allowed to use B more. But users still need safety controls.

So:

```go
type RoleBudget struct {
	SoftMaxTurns int
	HardMaxTurns int
	UsedTurns int
	Warned bool
}
```

### ADOPT-XIAOMI-007: Clarify turn accounting

Xiaomi states that `--max-turns 50` roughly means 50 worker calls plus 50 evaluator calls if each worker output is scored by E.  This is important for user-facing docs.

Adopt this in the EBP app:

```text
One analysis turn = one worker generation plus one evaluator scoring pass.
```

But represent it explicitly:

```go
type TurnAccounting struct {
	WorkerCalls int `json:"worker_calls"`
	EvaluatorCalls int `json:"evaluator_calls"`
	TotalLLMCalls int `json:"total_llm_calls"`
}
```

CLI help should say:

```text
--max-turns 50 means up to 50 worker turns.
If evaluator scoring is enabled per turn, total LLM calls may be about 100 plus retries.
```

### ADOPT-XIAOMI-008: Dry-run cost estimation

This is genuinely useful and should be adopted in `ebp-evaluator`.

Add:

```bash
ebp-evaluator run --paper x.pdf --dry-run
```

Dry run should output:

```text
estimated claims
max worker calls
max evaluator calls
estimated tokens
estimated cost range
declared budget
```

This belongs only in the EBP app, not TreeQuest-Go.

### ADOPT-XIAOMI-009: Chain worker → evaluator outside TreeQuest core

Xiaomi says the cleaner approach is worker generates, E scores, then return `(state, score)`.  Adopt, but only in the EBP app.

That means TreeQuest sees:

```text
Action "constructive" returns a state and score.
Action "adversarial" returns a state and score.
```

The EBP app internally does:

```text
worker A/B call
evaluator E call
return scored result to TreeQuest
```

TreeQuest-Go does not know E exists.

## Items to modify or reject

### MODIFY-XIAOMI-001: Immutable-style tree should not be mandatory

Xiaomi recommends immutable-style tree updates returning new trees, mirroring Python.  I would not adopt as the default.

Final decision:

```text
Public API may return errors and mutate internal Searcher state.
Snapshots provide immutability for persistence.
```

Why: copy-on-write trees with generics and large states can become expensive and complex in Go. Use safe mutation internally, plus snapshot/restore.

Acceptable compromise:

```go
func (s *Searcher[S]) Ask(ctx context.Context) (Trial[S], error)
func (s *Searcher[S]) Tell(ctx context.Context, result Result[S]) error
func (s *Searcher[S]) Snapshot() (Snapshot[S], error)
```

Do not force:

```go
tree, err = algo.Tell(tree, trialID, result)
```

as the primary API.

### MODIFY-XIAOMI-002: `Step` is not primary

Xiaomi includes `Step` as part of the main `Algorithm` interface.  Keep `Step`, but as convenience.

Primary:

```text
Ask
AskBatch
Tell
```

Convenience:

```text
Step
ParallelStep
```

### MODIFY-XIAOMI-003: No `ABMCTS-M` implementation in v0.1

The file includes `abmcts_m.go` in the package layout.  Keep as future placeholder only.

v0.1:

```text
AB-MCTS-A only
```

v0.2+:

```text
AB-MCTS-M research spike
```

### MODIFY-XIAOMI-004: Checkpoint package yes, but flat snapshots

Xiaomi proposes checkpointing and `SaveTree/LoadTree`. Good. But avoid serializing pointer graphs directly.

Adopt:

```go
type Snapshot[S any] struct {
	SchemaVersion string
	Nodes []NodeRecord[S]
	RootID NodeID
	Stats SearchStats
	Config ConfigSnapshot
	CreatedAtUnix int64
}
```

Storage helpers can exist, but the core should expose snapshot first:

```go
func (s *Searcher[S]) Snapshot() (Snapshot[S], error)
func Restore[S any](snap Snapshot[S], opts ...Option[S]) (*Searcher[S], error)
```

### MODIFY-XIAOMI-005: EBP state vocabulary is too promotion-like

The proposed EBP `Claim.Status` includes `"supported"`, `"challenged"`, `"resolved"`.  I would change this.

Use:

```text
unreviewed
under_analysis
debt_visible
budget_exhausted
needs_human_review
candidate_report_complete
```

Avoid:

```text
supported
resolved
```

because LLM audit should not imply truth or closure.

### REJECT-XIAOMI-001: Hard “max turns” as TreeQuest search budget

Xiaomi maps search budget to `--max-turns`.  This is app-level terminology.

In TreeQuest-Go:

```text
MaxTrials
MaxNodes
MaxDepth
MaxIterations
```

In EBP app:

```text
MaxTurns
MaxWorkerCalls
MaxEvaluatorCalls
MaxTokens
MaxCost
```

### REJECT-XIAOMI-002: Worker/evaluator concepts in TreeQuest docs

The conceptual mapping is good for EBP docs, but not TreeQuest-Go docs.

TreeQuest-Go docs should use:

```text
generator
action
trial
result
score
```

not:

```text
worker A
worker B
evaluator E
```

## Adopted items

```text
ADOPT-XIAOMI-001: Add public facade file treequest.go.
ADOPT-XIAOMI-002: Add Actions[S] convenience map.
ADOPT-XIAOMI-003: Support SimpleGenerateFn with nil parent for root.
ADOPT-XIAOMI-004: Use ResultEntry[S] for TopK.
ADOPT-XIAOMI-005: Add ParallelStep helper built on AskBatch/Tell.
ADOPT-XIAOMI-006: Support soft per-worker caps in EBP app.
ADOPT-XIAOMI-007: Document turn accounting: worker call + evaluator call.
ADOPT-XIAOMI-008: Add dry-run cost estimation in EBP app.
ADOPT-XIAOMI-009: Chain worker output → evaluator scoring inside app action.
```

## Modified items

```text
MODIFY-XIAOMI-001: Immutable-style API becomes optional/snapshot-based, not mandatory.
MODIFY-XIAOMI-002: Step is convenience; Ask/Tell is primary.
MODIFY-XIAOMI-003: ABMCTS-M deferred.
MODIFY-XIAOMI-004: Checkpoint uses flat snapshots, not pointer graph serialization.
MODIFY-XIAOMI-005: EBP statuses avoid “supported/resolved”; use audit/debt statuses.
```

## Rejected items

```text
REJECT-XIAOMI-001: No max-turns concept inside treequest-go.
REJECT-XIAOMI-002: No worker/evaluator terminology in treequest-go docs.
REJECT-XIAOMI-003: No direct EBP state model in core examples.
REJECT-XIAOMI-004: No ABMCTS-M in v0.1 package layout except as docs/deferred.
```

## Updated v0.1 API after Xiaomi cherry-picks

```go
type ActionLabel string
type Score float64

type SimpleGenerateFn[S any] func(ctx context.Context, parent *S) (S, Score, error)
type Actions[S any] map[ActionLabel]GenerateFn[S]

type GenerateRequest[S any] struct {
	TrialID      TrialID
	ParentID     NodeID
	ParentState  *S
	ActionLabel  ActionLabel
	Depth        int
	Siblings     []NodeSummary[S]
	Meta         map[string]any
}

type GenerateResult[S any] struct {
	State  S
	Reward Reward
	Meta   map[string]any
}

type Trial[S any] struct {
	ID          TrialID
	ParentID    NodeID
	ParentState *S
	ActionLabel ActionLabel
	Depth       int
	Meta        map[string]any
}

type Result[S any] struct {
	TrialID TrialID
	State   S
	Reward  Reward
	Meta    map[string]any
	Err     error
}

type ResultEntry[S any] struct {
	NodeID NodeID
	State  S
	Reward Reward
	Depth int
	Path []NodeID
}
```

## Final judgment

`xiaomi.md` adds **moderate net new value**. Its best contributions are practical API ergonomics and EBP-app budgeting refinements, especially:

```text
Actions[S]
SimpleGenerateFn
ParallelStep helper
soft per-worker caps
dry-run cost estimation
explicit turn accounting
```

But it should not override the stronger locked decisions:

```text
Ask/Tell primary
generic core
no EBP/LLM terms in TreeQuest-Go
AB-MCTS-A first
app-level turn/token/cost budgets
snapshot-based checkpointing
```

## EBP self-audit

**Claims:** xiaomi.md adds moderate implementation value but must be cherry-picked.

**needMap:** Useful ideas map to API ergonomics, helper functions, and EBP app budget/reporting behavior.

**needInvariant:** Preserve generic core, action neutrality, Ask/Tell primacy, app-level LLM budgets, normalized scores, and no truth/promotion certification.

**needToyCheck:** Test `SimpleGenerateFn`, `Actions[S]`, `ParallelStep`, and `TopK` with deterministic non-LLM examples.

**needNullModel:** Compare soft per-worker caps against hard caps in toy A/B actions; soft caps should avoid fighting adaptive allocation.

**needObstruction:** Risks are immutable-tree overhead, ABMCTS-M scope creep, and misleading EBP statuses like “supported” or “resolved.”

**needFaithfulnessReview:** Ensure `ParallelStep` remains a wrapper over AskBatch/Tell and does not become a second divergent execution path.

**Promotion status:** Cherry-picked as ergonomic refinement; not adopted as final implementation spec.



## prompt

analyze attached stepfun.md if it adds net new and if so, cherry pick and adopt them:


Yes — **stepfun.md adds high net new value**, especially because it appears to be the first review that tries to mirror the **actual TreeQuest source architecture** rather than only proposing a clean-room Go design. I would cherry-pick several items and let them reshape the Go port plan. 

## Bottom-line verdict

Use `stepfun.md` as:

```text id="w55q05"
TreeQuest-Go Source-Faithfulness Notes v0.7 — high-value, cherry-picked
```

Its biggest contribution is this distinction:

```text id="5h7sja"
TreeQuest algorithm objects are stateless.
The evolving search state lives in AlgoStateT.
```

That is important. It suggests the Go port should not only be a generic search library; it should also preserve the source architecture where:

```text id="dlzisz"
Algorithm = stateless behavior
AlgoState = tree + trial store + algorithm-specific bookkeeping
```

That is the most valuable new idea in the file.

## What adds real net new value

### ADOPT-STEPFUN-001: Separate `StateT` from `AlgoStateT`

This is the strongest addition.

Earlier designs treated `Searcher[S]` as the main stateful object. Stepfun suggests a more source-faithful pattern:

```go id="13gef8"
type Algorithm[StateT any, AlgoStateT any] interface {
	InitTree() AlgoStateT
	Ask(state AlgoStateT, actions []ActionLabel) (AlgoStateT, Trial[StateT], error)
	AskBatch(state AlgoStateT, batchSize int, actions []ActionLabel) (AlgoStateT, []Trial[StateT], error)
	Tell(state AlgoStateT, trialID TrialID, result StateScore[StateT]) (AlgoStateT, error)
	StateScorePairs(state AlgoStateT) []StateScore[StateT]
}
```

This is worth adopting because it makes algorithm implementations cleaner:

```text id="7fq6ra"
ABMCTSAState = tree + ABMCTS-A bandit stats + trial store
StandardMCTSState = tree + queue/trial store
BestFirstState = tree + priority queue/trial store
```

The generic user state remains `StateT`.

The algorithm-specific runtime state becomes `AlgoStateT`.

### ADOPT-STEPFUN-002: TrialStore as a first-class concept

This is also important.

Stepfun identifies `Trial/TrialStore` as central for ask/tell, concurrency, resume, and checkpointing.  Adopt this.

Core types:

```go id="dxqvz3"
type TrialID string

type Trial[StateT any] struct {
	ID          TrialID
	Action      ActionLabel
	ParentNodeID NodeID
	ParentState *StateT
}

type TrialStore[StateT any] struct {
	Pending map[TrialID]Trial[StateT]
}
```

For queue algorithms:

```go id="a3t2re"
type TrialStoreWithNodeQueue[StateT any] struct {
	Trials TrialStore[StateT]
	Queue  []NodeID
}
```

This gives us better checkpoint/resume semantics than an ad hoc pending-trial map.

### ADOPT-STEPFUN-003: Source-faithful package list

Stepfun’s package list is compact and better aligned with the original library:

```text id="s9w3ms"
node.go
tree.go
algo.go
trial.go
abmctsa.go
abmctsm.go optional
ranker.go
types.go
```

Adopt the compact spine:

```text id="3z2rkd"
treequest-go/
  node.go
  tree.go
  algo.go
  trial.go
  types.go
  ranker.go
  abmctsa.go
  standard_mcts.go
  tree_of_thoughts_bfs.go
  best_first.go
  multiarmed_bandit_ucb.go
```

Then add later optional subpackages only when useful.

This is cleaner than over-modularizing too early.

### ADOPT-STEPFUN-004: Queue-based baseline algorithms

Stepfun reports that TreeQuest source includes queue-based algorithms:

```text id="qkqvo0"
StandardMCTS
TreeOfThoughtsBFS
BestFirstSearch
MultiArmedBanditUCB
```

This adds value because it gives us a more source-faithful baseline list. 

Adopt as v0.1/v0.2 priorities:

```text id="p6t4kk"
v0.1:
  ABMCTSA
  StandardMCTS
  TreeOfThoughtsBFS
  BestFirstSearch

v0.2:
  MultiArmedBanditUCB
  ABMCTSM deferred/research spike
```

### ADOPT-STEPFUN-005: `StateScorePairs` / ranker concept

Earlier plans used `TopK`. Stepfun points to a source-style `StateScorePairs` and `ranker.go`.  This is worth adopting.

Use both:

```go id="o4p7a4"
func StateScorePairs[StateT any](state AlgoStateT) []StateScore[StateT]
func TopK[StateT any](pairs []StateScore[StateT], k int) []StateScore[StateT]
```

This separates extraction from ranking.

### ADOPT-STEPFUN-006: `expand_idx` on Node

Stepfun mentions `Node[StateT]` with `state`, `score`, `parent`, `children`, and `expand_idx`.  We had not explicitly included `expand_idx`.

Adopt:

```go id="6dqc2r"
type Node[StateT any] struct {
	ID        NodeID
	State     StateT
	Score     Score
	ParentID  *NodeID
	Children  []NodeID
	ExpandIdx int
}
```

`ExpandIdx` can preserve child expansion order and help reproduce source behavior.

### ADOPT-STEPFUN-007: Keep `score in [0,1]` as a hard contract

Stepfun strongly states TreeQuest’s scoring contract: every node score must be in `[0,1]`.  Adopt as hard core behavior.

Earlier we allowed `RawScore` plus normalizer. Final rule:

```text id="xmacc0"
TreeQuest-Go Tell/Step accepts only normalized Score in [0,1].
RawScore may exist in metadata, but the algorithm only sees normalized Score.
```

Enforce:

```go id="dvzm0a"
func ValidateScore(s Score) error {
	if s < 0 || s > 1 || math.IsNaN(float64(s)) || math.IsInf(float64(s), 0) {
		return ErrInvalidScore
	}
	return nil
}
```

### ADOPT-STEPFUN-008: EBP mapping as actions, not core roles

Stepfun’s EBP mapping is clean:

```text id="6lsamh"
constructive = action
adversarial = action
E produces score after seeing node output
```

This matches the locked architecture. Adopt.

TreeQuest-Go sees:

```go id="tz5r3g"
actions := map[ActionLabel]GenerateFn[PartialReport]{
	"constructive": fnA,
	"adversarial":  fnB,
}
```

The EBP app wraps each action so Evaluator E scores the result before `Tell`.

### ADOPT-STEPFUN-009: Budget metadata belongs in run config + audit trail

Stepfun gives a good correction: putting budget-per-worker inside the “paper output” can conflate evaluation-run metadata with paper content. It recommends run config plus audit trail, and only embedding it in the artifact as a clearly separated metadata block if reproducibility requires it. 

Adopt this distinction.

For EBP output:

```text id="iwuk38"
paper_annotated.md:
  human-readable annotations
  small method/meta block

paper_report.json:
  full run config
  declared budget
  consumed budget
  model/provider provenance
  tree stats
  trial audit trail
```

Budget is not “content.” It is **provenance**.

## Items to modify or reject

### MODIFY-STEPFUN-001: `GenerateFn` should return error and use context

Stepfun’s source-faithful sketch uses:

```go id="d2z4cq"
type GenerateFn[StateT any] func(parentState *StateT) (StateT, float64)
```

Modify for Go robustness:

```go id="aljyl3"
type GenerateFn[StateT any] func(ctx context.Context, parentState *StateT) (StateScore[StateT], error)
```

or:

```go id="ejibwz"
type GenerateFn[StateT any] func(ctx context.Context, parentState *StateT) (StateT, Score, error)
```

Need context and errors for LLMs, simulations, file IO, and cancellation.

### MODIFY-STEPFUN-002: Algorithm interface should return errors

The source-style sketch omits errors. In Go, use errors.

```go id="wj0h9n"
Ask(state AlgoStateT, actions []ActionLabel) (AlgoStateT, Trial[StateT], error)
Tell(state AlgoStateT, trialID TrialID, result StateScore[StateT]) (AlgoStateT, error)
```

### MODIFY-STEPFUN-003: `Step` should be optional convenience

Stepfun mirrors Python with `Step`. Good for source parity, but Ask/Tell remains primary.

Final:

```text id="szagqi"
Core interface: Ask, AskBatch, Tell, StateScorePairs
Convenience helper: Step
```

### MODIFY-STEPFUN-004: `AlgoStateT` might be too advanced for the simplest public API

This is the only caution.

A fully generic `Algorithm[StateT, AlgoStateT]` is faithful, but may feel heavy to Go users.

So expose two layers:

```text id="lhzgr5"
low-level source-faithful API:
  Algorithm[StateT, AlgoStateT]

ergonomic wrapper:
  Searcher[StateT]
```

`Searcher[S]` internally owns an `Algorithm[S, AlgoState]` and its current state.

That gives both source-faithfulness and Go usability.

### REJECT-STEPFUN-001: No ABMCTS-M in initial port

Stepfun asks whether ABMCTS-M should be included and notes it depends on PyMC/numpyro/joblib.  We should lock the answer:

```text id="laew5f"
No ABMCTS-M in v0.1.
ABMCTS-A is sufficient for EBP use case.
ABMCTS-M becomes a documented deferred item.
No Python FFI for v0.1.
```

### REJECT-STEPFUN-002: Do not serialize arbitrary AlgoStateT blindly

Stepfun says resume/checkpointing is handled by serializing `AlgoStateT`. Good concept, but be careful.

Use explicit snapshots:

```go id="uaxef6"
type ABMCTSAStateSnapshot[StateT any] struct {
	Tree       TreeSnapshot[StateT]
	Trials     TrialStoreSnapshot[StateT]
	NodeStats  map[NodeID]ABMCTSANodeStats
	RNGState   optional
}
```

Do not rely on JSON encoding arbitrary maps containing runtime internals.

## Adopted items

```text id="3qzgzs"
ADOPT-STEPFUN-001: Separate StateT from AlgoStateT.
ADOPT-STEPFUN-002: TrialStore is first-class.
ADOPT-STEPFUN-003: Use source-faithful compact package spine.
ADOPT-STEPFUN-004: Include source-aligned baseline algorithms.
ADOPT-STEPFUN-005: Add StateScorePairs + ranker.go.
ADOPT-STEPFUN-006: Add expand_idx to Node.
ADOPT-STEPFUN-007: Enforce score in [0,1].
ADOPT-STEPFUN-008: Map EBP A/B as actions; E remains app-level scorer.
ADOPT-STEPFUN-009: Treat budgets as run metadata/provenance, not paper content.
```

## Modified items

```text id="x5fb3w"
MODIFY-STEPFUN-001: GenerateFn includes context and error.
MODIFY-STEPFUN-002: Algorithm methods return errors.
MODIFY-STEPFUN-003: Step is convenience, not primary.
MODIFY-STEPFUN-004: Offer ergonomic Searcher[S] wrapper over source-faithful Algorithm[S, AlgoStateS].
MODIFY-STEPFUN-005: Checkpoint uses explicit snapshots, not blind AlgoStateT serialization.
```

## Rejected items

```text id="4do907"
REJECT-STEPFUN-001: No ABMCTS-M in v0.1.
REJECT-STEPFUN-002: No Python FFI boundary for ABMCTS-M.
REJECT-STEPFUN-003: No blind JSON/gob serialization of arbitrary algorithm state.
```

## Updated architecture after stepfun.md

### Low-level source-faithful API

```go id="rz8h34"
type Algorithm[StateT any, AlgoStateT any] interface {
	InitTree(ctx context.Context) (AlgoStateT, error)

	Ask(
		ctx context.Context,
		state AlgoStateT,
		actions []ActionLabel,
	) (AlgoStateT, Trial[StateT], error)

	AskBatch(
		ctx context.Context,
		state AlgoStateT,
		batchSize int,
		actions []ActionLabel,
	) (AlgoStateT, []Trial[StateT], error)

	Tell(
		ctx context.Context,
		state AlgoStateT,
		trialID TrialID,
		result StateScore[StateT],
	) (AlgoStateT, error)

	StateScorePairs(state AlgoStateT) []StateScore[StateT]
}
```

### Ergonomic wrapper

```go id="1s1wkh"
type Searcher[StateT any, AlgoStateT any] struct {
	Algo  Algorithm[StateT, AlgoStateT]
	State AlgoStateT
}

func (s *Searcher[StateT, AlgoStateT]) Ask(ctx context.Context, actions []ActionLabel) (Trial[StateT], error) {
	next, trial, err := s.Algo.Ask(ctx, s.State, actions)
	if err != nil {
		return Trial[StateT]{}, err
	}
	s.State = next
	return trial, nil
}

func (s *Searcher[StateT, AlgoStateT]) Tell(ctx context.Context, trialID TrialID, result StateScore[StateT]) error {
	next, err := s.Algo.Tell(ctx, s.State, trialID, result)
	if err != nil {
		return err
	}
	s.State = next
	return nil
}
```

### EBP runner mapping

```go id="qzz0wc"
actions := map[treequest.ActionLabel]treequest.GenerateFn[PartialReport]{
	"constructive": func(ctx context.Context, parent *PartialReport) (treequest.StateScore[PartialReport], error) {
		draft := workerA.Generate(ctx, parent)
		score := evaluatorE.Score(ctx, draft)
		return treequest.StateScore[PartialReport]{
			State: draft,
			Score: score.AuditQualityScore,
			Meta: score.Provenance,
		}, nil
	},
	"adversarial": func(ctx context.Context, parent *PartialReport) (treequest.StateScore[PartialReport], error) {
		critique := workerB.Generate(ctx, parent)
		score := evaluatorE.Score(ctx, critique)
		return treequest.StateScore[PartialReport]{
			State: critique,
			Score: score.AuditQualityScore,
			Meta: score.Provenance,
		}, nil
	},
}
```

## Final judgment

`stepfun.md` adds **high net new value** because it redirects the design toward the actual TreeQuest source shape:

```text id="ap2fck"
Algorithm statelessness
AlgoStateT
TrialStore
queue-based algorithms
StateScorePairs
expand_idx
score in [0,1]
```

This should meaningfully update the final design.

It does **not** overturn the locked decisions:

```text id="f0hpzw"
TreeQuest-Go remains generic.
EBP remains separate.
Ask/Tell remains primary.
ABMCTS-A is first.
ABMCTS-M is deferred.
Budgets for A/B/E remain app-level provenance.
```

## EBP self-audit

**Claims:** stepfun.md adds high-value source-faithfulness details and should reshape the final Go architecture.

**needMap:** Source `StateT`/`AlgoStateT` maps to Go generics; `TrialStore` maps to first-class pending trial state; EBP A/B maps to action labels; E maps to app-level score production.

**needInvariant:** Preserve generic core, score `[0,1]`, ask/tell, source-faithful algorithm state, no LLM/EBP terms in core, and no ABMCTS-M in v0.1.

**needToyCheck:** Build source-faithful toy tests for `InitTree`, `AskBatch`, `Tell`, `TrialStore`, `StateScorePairs`, and `TopK`.

**needNullModel:** Baseline algorithms from source should be included as comparison points against ABMCTS-A.

**needObstruction:** Risks are overcomplicating public API with `AlgoStateT`, blind serialization, and accidentally treating EBP run metadata as paper content.

**needFaithfulnessReview:** This design still requires direct comparison against the actual Sakana TreeQuest source before claiming faithful port.

**Promotion status:** Adopted as high-value source-faithfulness input; implementation remains unpromoted pending code and tests.




## prompt

 What compatibility target should the first Go release use?
 
  › 1. AB-MCTS-A first (Recommended)  Deliver a pure-Go, useful core first, then add M and other algorithms
                                      behind the same interface.
    2. Full TreeQuest parity          Port A, M, standard MCTS, BFS, ranking, persistence, and visualization
                                      in the initial release.
    3. AB-MCTS-M first                Prioritize the mixed-model paper algorithm despite substantial
                                      Bayesian runtime/dependency work.



Use **Option 1: AB-MCTS-A first** as the first Go release target.

The first release should deliver a pure-Go, source-faithful, useful core around:

* generic `Node[StateT]` / `Tree[StateT]`
* `Algorithm[StateT, AlgoStateT]`
* `Trial` / `TrialStore`
* `Ask`, `AskBatch`, and `Tell`
* normalized score contract `[0,1]`
* AB-MCTS-A with Thompson Sampling
* `StateScorePairs` / `TopK`
* deterministic non-LLM toy tests
* snapshot/restore of explicit tree + trial + algorithm state

Do **not** target full TreeQuest parity in the first release. Full parity would mix too many concerns at once: AB-MCTS-A, AB-MCTS-M, standard MCTS, BFS, ranking, persistence, visualization, and mixed-model Bayesian machinery. That creates unnecessary implementation risk before the core API is stable.

Do **not** prioritize AB-MCTS-M first. AB-MCTS-M depends on heavy Bayesian mixed-model machinery in the Python ecosystem, while AB-MCTS-A is pure, practical, and enough to validate the Go port and the EBP/Workbench paper-audit use case.

The compatibility target should be:

> Source-faithful architecture, not full feature parity.

That means the Go port should preserve the important TreeQuest ideas — stateless algorithms, separate algorithm state, trial store, ask/tell, normalized scores, and AB-MCTS-A behavior — while using idiomatic Go APIs with context, errors, generics, and explicit snapshots.

Full parity can become a roadmap:

* v0.1: AB-MCTS-A core
* v0.2: baseline algorithms such as Standard MCTS, Tree-of-Thoughts BFS, Best-First, ranking helpers, DOT rendering
* v0.3+: richer checkpoint backends and visualization
* later/research: AB-MCTS-M, only after deciding whether to reimplement mixed models in Go or treat it as an optional external bridge

So the first release should optimize for correctness, usefulness, and clean architecture — not maximum parity.




## prompt


analyze attached codex.md if it adds net new and if so, cherry pick and adopt them:


Yes — **codex.md adds high net new value**, especially on **implementation hardening**. It is one of the strongest inputs so far because it adds several production-grade details we had not locked down sharply enough: **idempotent `Tell`, trial statuses, randomness injection, application-supplied state codecs, parity tests against Python, complete-state comparability between A and B, deterministic enforcement checks, and artifact-bundle output**. 

## Bottom-line verdict

Use `codex.md` as:

```text
TreeQuest-Go Production Hardening Notes v0.8 — high-value, cherry-picked
```

It does **not** overturn the existing locked direction:

```text
treequest-go = generic core
ebp-paper-evaluator = separate app
AB-MCTS-A first
AB-MCTS-M deferred
Ask/Tell primary
scores normalized to [0,1]
```

But it **does** upgrade the final spec.

## Highest-value additions to adopt

### ADOPT-CODEX-001: Idempotent and order-independent `Tell`

This is very important.

Codex says `Tell` should be order-independent and idempotent using trial IDs and statuses.  Adopt this as a hard design rule.

```go
type TrialStatus string

const (
	TrialPending   TrialStatus = "pending"
	TrialCompleted TrialStatus = "completed"
	TrialFailed    TrialStatus = "failed"
	TrialCanceled  TrialStatus = "canceled"
)

type Trial[S any] struct {
	ID          TrialID
	ParentID    NodeID
	ParentState *S
	Action      Action
	Status      TrialStatus
}
```

Rules:

```text
Tell on pending trial: accepted.
Tell on completed trial with same result hash: no-op.
Tell on completed trial with different result: error.
Tell on unknown trial: error.
Tell out of order: allowed.
```

This is essential for concurrency, retry logic, checkpoint/resume, and LLM latency variance.

### ADOPT-CODEX-002: Inject randomness through an interface

Adopt.

Codex says randomness should be injected so tests are reproducible.  This is better than directly using `math/rand` everywhere.

```go
type Rand interface {
	Float64() float64
	NormFloat64() float64
	ExpFloat64() float64
}
```

Or keep it minimal:

```go
type RandomSource interface {
	Float64() float64
}
```

For beta sampling, wrap a seeded `rand.Rand` or Gonum-compatible source.

This enables deterministic trace tests.

### ADOPT-CODEX-003: Application-supplied state codecs

This is a strong addition.

Codex says JSON checkpointing should use explicit schema versions and application-supplied state codecs.  Adopt this.

Why: generic `S any` may not always serialize cleanly. The library should not assume it can JSON-marshal arbitrary states.

```go
type StateCodec[S any] interface {
	MarshalState(S) ([]byte, error)
	UnmarshalState([]byte) (S, error)
}
```

Default helper:

```go
type JSONStateCodec[S any] struct{}
```

Checkpoint records store encoded state bytes plus codec metadata.

### ADOPT-CODEX-004: Parity tests against Python

This is one of the most important new items.

Codex proposes replaying fixed seeded traces against Python, covering selection, backpropagation, batching, duplicate `Tell`, out-of-order completion, serialization, and score validation.  Adopt this as a **faithfulness gate**.

Parity suite:

```text
- same seed, same generated scores, same action set
- selection path parity
- wider/deeper decision parity where applicable
- backpropagation parity
- AskBatch pending trial parity
- Tell order-independence
- duplicate Tell behavior
- score validation: NaN, Inf, <0, >1 rejected
- snapshot/restore continuation
```

This does not mean claiming perfect equivalence on all floating randomness. It means using controlled deterministic fixtures to prevent silent algorithm drift.

### ADOPT-CODEX-005: Both A and B must return the same complete state type

This is excellent and highly relevant.

Codex says constructive and adversarial actions must return the same complete `AssessmentState`; returning a paper from A and only a critique from B would make scores incomparable.  Adopt this as a hard EBP-app invariant.

```text
A returns complete AssessmentState.
B returns complete AssessmentState.
E evaluates complete candidate child state.
```

B can internally produce a patch, but before `Tell`, the app must apply it to produce a full child state.

This prevents apples-to-oranges scoring.

### ADOPT-CODEX-006: E evaluates one completed candidate at a time

Codex states E should receive only the candidate child plus immutable source and rubric.  Adopt.

This matches your original desired workflow:

```text
A/B worker produces candidate.
E evaluates that candidate one at a time.
TreeQuest receives state + normalized score.
```

No direct E↔worker chat is needed in the generic search loop.

### ADOPT-CODEX-007: Deterministic checks must enforce rules, not only LLM E

This is very important.

Codex says deterministic checks should enforce source citations, required claim fields, no-final-truth language, and debt consistency; the LLM should not be the sole enforcement mechanism.  Adopt.

For `ebp-paper-evaluator`, add validators:

```text
ValidateClaimFields
ValidateSourceCitations
ValidateNoFinalTruthLanguage
ValidateDebtConsistency
ValidateWorkbenchRequiredFields
ValidateScoreRange
ValidateBudgetProvenance
```

Evaluator E can judge quality, but deterministic checks enforce structural safety.

### ADOPT-CODEX-008: Artifact bundle, not altered paper

This is a strong output correction.

Codex says the output should be an artifact bundle and should not silently replace the research paper.  Adopt.

Final output bundle:

```text
out/
  source/
    original.pdf or source_hash.txt
  report/
    assessment.md
    annotated_paper.md
  data/
    claims.json
    debt_ledger.json
    workbench_report.json
    evaluator_scores.json
    provenance.json
    tree_snapshot.json
    budget_usage.json
```

Important rule:

```text
Never confuse generated assessment with the author’s original paper.
```

### ADOPT-CODEX-009: Preserve full evaluation outside TreeQuest as audit metadata

Adopt.

TreeQuest should only need:

```text
state + normalized score
```

The EBP app stores:

```text
rubric breakdown
evidence citations
confidence
rationale
prompt hashes
model IDs
retries
budget usage
```

This preserves the generic core while keeping the audit complete.

### ADOPT-CODEX-010: Retry/provenance accounting

Codex says record actual calls, tokens, model identifiers, prompt/configuration hashes, retries, and stop reason.  Adopt.

This should be a required provenance section in the EBP app.

## Items to modify

### MODIFY-CODEX-001: “Beta and Gaussian conjugate distributions” should be staged

Codex says AB-MCTS-A should include Beta and Gaussian conjugate distributions.  I would adopt Beta first and defer Gaussian until needed.

Final:

```text
v0.1: Beta Thompson Sampling
v0.2: optional Gaussian Thompson sampler if source parity or benchmarks need it
```

Do not let Gaussian support delay the MVP.

### MODIFY-CODEX-002: UCB belongs to baseline algorithms, not AB-MCTS-A MVP

Codex mentions Thompson sampling, UCB, GEN-versus-CONT selection, and backpropagation together.  Keep the separation:

```text
AB-MCTS-A core: Thompson/Beta
Standard MCTS / MAB baseline: UCB
```

### MODIFY-CODEX-003: “Concurrency-safe orchestration” belongs mostly outside the algorithm

Codex says use context cancellation but keep goroutines and worker pools in the caller rather than the search algorithm.  Adopt that. The core only needs:

```text
AskBatch
order-independent Tell
trial status
snapshot/restore
```

Worker pools remain app/helper layer.

### MODIFY-CODEX-004: Snapshot should include app codecs, but core should not own storage

Codex’s checkpointing suggestion is good, but storage remains outside core.

Core provides:

```go
Snapshot(codec StateCodec[S]) Snapshot
Restore(snapshot, codec)
```

App decides:

```text
file
SQLite
object storage
git artifact
```

## Rejected or deferred

```text
REJECT/DEFER-CODEX-001: No AB-MCTS-M in first release.
REJECT/DEFER-CODEX-002: No external inference service for AB-MCTS-M in v0.1.
REJECT/DEFER-CODEX-003: No visualization inside core package; snapshot-based visualization later.
REJECT/DEFER-CODEX-004: No app-specific provider adapters in treequest-go.
```

## Adopted items list

```text
ADOPT-CODEX-001: Idempotent, order-independent Tell.
ADOPT-CODEX-002: Trial statuses.
ADOPT-CODEX-003: Randomness injection interface.
ADOPT-CODEX-004: Application-supplied state codecs.
ADOPT-CODEX-005: Python parity/replay tests.
ADOPT-CODEX-006: A and B return the same complete AssessmentState.
ADOPT-CODEX-007: E evaluates one completed candidate child at a time.
ADOPT-CODEX-008: Deterministic validators enforce citations, required fields, no-final-truth language, and debt consistency.
ADOPT-CODEX-009: Output is artifact bundle, not altered paper.
ADOPT-CODEX-010: Full evaluation metadata stays outside TreeQuest.
ADOPT-CODEX-011: Provenance records calls, tokens, model IDs, prompt/config hashes, retries, and stop reason.
```

## Updated final architecture after codex.md

### `treequest-go` v0.1

```text
treequest-go/
  types.go          # Action, NodeID, TrialID, Score, StateScore
  node.go
  tree.go
  trial.go          # Trial, TrialStatus, TrialStore
  algo.go           # Algorithm[StateT, AlgoStateT]
  abmctsa.go        # AB-MCTS-A, Beta Thompson Sampling
  ranker.go         # StateScorePairs, TopK
  codec.go          # StateCodec[S], JSONStateCodec[S]
  snapshot.go       # explicit schema-versioned snapshots
  rng.go            # injectable randomness
  validation.go     # score validation, trial validation
```

### `ebp-paper-evaluator`

```text
ebp-paper-evaluator/
  internal/state/
    assessment_state.go

  internal/actions/
    constructive.go
    adversarial.go

  internal/evaluator/
    evaluator_e.go
    rubric.go
    scalarize.go

  internal/validate/
    source_citations.go
    final_truth.go
    debt_consistency.go
    required_fields.go

  internal/provenance/
    calls.go
    prompt_hash.go
    budget_usage.go

  internal/output/
    artifact_bundle.go
    annotated_paper.go
    report.go
```

## Updated EBP app invariant

```text
For every selected TreeQuest trial:

1. Worker A or B receives parent AssessmentState.
2. Worker returns structured patch or full candidate.
3. App applies patch to produce a complete AssessmentState.
4. Deterministic validators run.
5. Evaluator E scores that complete candidate.
6. App normalizes score to [0,1].
7. TreeQuest Tell receives only complete AssessmentState + normalized score.
8. Full evaluator metadata is saved in provenance.
```

## Final judgment

`codex.md` adds **high-value production hardening**. It should be incorporated into the final design more strongly than most previous notes.

The strongest additions are:

```text
idempotent Tell
trial statuses
randomness injection
state codecs
Python parity tests
complete-state comparability
deterministic EBP validators
artifact bundle output
```

These are not cosmetic. They materially improve correctness, reproducibility, concurrency, and epistemic safety.

## EBP self-audit

**Claims:** codex.md adds high-value hardening and should be cherry-picked into the final architecture.

**needMap:** Codex additions map to trial semantics, reproducibility, checkpointing, deterministic EBP validation, and artifact-bundle output.

**needInvariant:** Preserve generic core, app-level EBP logic, normalized score, idempotent ask/tell, full-state comparability, and no truth/promotion certification.

**needToyCheck:** Test duplicate `Tell`, out-of-order `Tell`, invalid scores, snapshot/restore, fake A/B complete-state generation, and deterministic validators.

**needNullModel:** Python replay traces become the source-faithfulness null model; simple non-tree A/B pipelines remain the app-level baseline.

**needObstruction:** Main risks are overcomplicating v0.1, treating LLM E as sole validator, and losing audit metadata outside TreeQuest.

**needFaithfulnessReview:** Use parity tests against upstream Python before claiming TreeQuest compatibility.

**Promotion status:** Accepted as high-value production-hardening input; implementation remains unpromoted pending tests.



## prompt

Yes — **gemini.md adds low-to-moderate net new value**, mostly as **confirmation and simplification**, not as a major new architecture source. It reinforces several decisions already locked in, and it adds a few small practical refinements worth adopting. But it should **not** override the stronger source-faithfulness and production-hardening decisions from `stepfun.md` and `codex.md`. 

## Bottom-line verdict

Use `gemini.md` as:

```text
TreeQuest-Go Simplification / Consensus Notes v0.9 — cherry-picked lightly
```

Its best contribution is that it gives a **minimal implementation path**:

```text
treequest-go first
generic Node/SearchTree/BetaSampler first
mock S=string harness first
EBP app second
```

That is useful because previous notes have become feature-rich. Gemini helps keep the first milestone small.

## What it adds net new

### ADOPT-GEMINI-001: Minimal dependency core

Gemini recommends the core library depend only on math/tree logic, with `gonum/distuv` for Beta sampling and no LLM dependencies. 

Adopt this as the v0.1 dependency rule:

```text
treequest-go v0.1 dependencies:
- standard library
- gonum, if we choose not to hand-roll Beta sampling
```

No OpenAI, Anthropic, Gemini, EBP, Workbench, PDF, Markdown, or CLI framework in the core.

### ADOPT-GEMINI-002: Start with mock `S = string` harness

This is simple but important.

Gemini explicitly recommends testing with `S = string` and simulated `rand.Float64()` scores before touching LLMs.  Adopt.

First toy test:

```text
State type: string
Actions: "a", "b"
Generator: deterministic or seeded pseudo-random expansion
Score: seeded [0,1]
Goal: verify tree growth, Thompson updates, TopK, Ask/Tell, Tell idempotency
```

This belongs in `examples/toy_string` and `internal` tests.

### ADOPT-GEMINI-003: Budget exhaustion flag in EBP state

Gemini reinforces the “honest incompleteness” rule:

```text
BudgetExhausted: true
```

should be visible in the EBP app state if a worker hits the turn limit. 

Adopt this field in `ebp-paper-evaluator`, not `treequest-go`:

```go
type BudgetExhaustion struct {
	Actor        string `json:"actor"`
	LimitType   string `json:"limit_type"`
	LimitValue  int    `json:"limit_value"`
	Used        int    `json:"used"`
	Effect      string `json:"effect"`
}
```

Evaluator E must see it, and the report must surface it as incompleteness, not failure.

### ADOPT-GEMINI-004: Three-layer budget explanation for docs

Gemini cleanly states the three-layer stack:

```text
search-level budget
worker-turn budget
token budget
```

Adopt this for documentation. It is not a new mechanism, but it is a good explanation for users.

Final version:

```text
TreeQuest core:
- max trials
- max nodes
- max depth
- max iterations

EBP app:
- worker turns
- evaluator calls
- token limits
- cost limits
- wall-clock limits
```

## What to modify

### MODIFY-GEMINI-001: `Step` should not be primary

Gemini centers the implementation around `Step`.  Keep `Step` as a convenience helper only.

Final decision remains:

```text
Primary API:
- Ask
- AskBatch
- Tell

Convenience:
- Step
- ParallelStep
```

### MODIFY-GEMINI-002: Avoid “model” terminology in core

Gemini uses `ModelBandits`, `GeneratedBy`, and “chosenModel.”  Replace with neutral terms:

```text
ModelBandits  -> ActionBandits
GeneratedBy   -> Action
chosenModel   -> chosenAction
GenerateFns   -> Actions
```

Core library should not imply LLMs.

### MODIFY-GEMINI-003: Avoid pointer-heavy node serialization

Gemini’s node sketch uses:

```go
Parent *Node[S]
Children []*Node[S]
mu sync.Mutex
```

Useful for in-memory exploration, but not ideal for checkpointing. Keep our locked decision:

```text
runtime tree may use internal pointers if needed;
snapshot format must be flat NodeRecord IDs;
do not JSON-marshal pointer graphs or mutexes.
```

### MODIFY-GEMINI-004: EBP package naming

Gemini’s separate app structure uses `pkg/ebp` and `pkg/llm`, which is fine, but we should keep the more precise app name:

```text
ebp-paper-evaluator
```

or:

```text
ebp-paper-eval
```

The core library remains `treequest-go`.

## Rejected items

```text
REJECT-GEMINI-001: Do not make Step the main public design.
REJECT-GEMINI-002: Do not use model/LLM terminology in treequest-go.
REJECT-GEMINI-003: Do not serialize runtime pointer nodes directly.
REJECT-GEMINI-004: Do not let worker-turn budget become a TreeQuest core concept.
```

## Adopted items

```text
ADOPT-GEMINI-001: Minimal dependency core: stdlib + optional gonum.
ADOPT-GEMINI-002: First toy harness with S=string and fake scores.
ADOPT-GEMINI-003: Explicit BudgetExhausted metadata in EBP state.
ADOPT-GEMINI-004: Three-layer budget explanation for docs.
ADOPT-GEMINI-005: Implement treequest-go before ebp-paper-evaluator.
```

## Final judgment

`gemini.md` is useful as a **simplicity anchor**. It does not add many new architectural ideas, but it helps prevent the plan from becoming too heavy too early.

The final adoption is:

```text
Use Gemini to simplify v0.1.
Use Stepfun for source-faithfulness.
Use Codex for production hardening.
Use Z for observability/provenance.
Use Qwen for API ergonomics.
Use Kimi/Xiaomi for baselines and helper APIs.
```

## EBP self-audit

**Claims:** gemini.md adds modest net value as a simplification and consensus document.

**needMap:** Useful ideas map to minimal dependencies, toy harness, EBP budget-exhaustion metadata, and documentation.

**needInvariant:** Preserve generic core, no LLM/EBP terms in TreeQuest-Go, Ask/Tell primacy, normalized score, app-level budgets, and explicit incompleteness.

**needToyCheck:** Implement `S=string` deterministic fake search before any EBP or LLM integration.

**needNullModel:** Use the mock harness as the first null against accidental LLM-specific assumptions.

**needObstruction:** Main risks are reverting to Step-first API, model terminology, pointer-graph checkpointing, and overloading the core with app budgets.

**needFaithfulnessReview:** Gemini is supportive but not sufficient for source parity; still rely on direct TreeQuest source comparison and parity tests.

**Promotion status:** Lightly adopted as simplification guidance; not a final implementation spec.



## prompt

give prompt to write implementation plan based on recommendations above

