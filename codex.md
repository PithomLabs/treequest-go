 # Go TreeQuest Port and EBP Integration

  ## Summary

  Build two separate codebases:

  1. treequest-go: general-purpose, provider-independent tree search.
  2. ebp-paper-evaluator: physics-paper workflow using TreeQuest, EBP 2.1, Workbench 2, and configurable LLM
     adapters.

  Start with AB-MCTS-A. It is practical to implement in pure Go. Add AB-MCTS-M later because the upstream
  implementation depends on PyMC/JAX/NumPyro mixed-model inference.

  ## TreeQuest Go Design

  - Represent node state with Go generics: Tree[S any], Node[S], Trial[S].
  - Keep algorithms independent of generators, evaluators, prompts, HTTP clients, and LLM providers.
  - Preserve the upstream stateless ask/tell model:
      - Ask selects a parent and action and creates a durable trial.
      - The caller performs arbitrary external work.
      - Tell submits the resulting state and normalized [0,1] score.

  - Make Tell order-independent and idempotent using trial IDs and statuses.
  - Inject randomness through an interface so searches are reproducible in tests.
  - Use context.Context for cancellation, but keep goroutines and worker pools in the caller rather than the
    search algorithm.

  - Support JSON checkpointing with explicit schema versions and application-supplied state codecs.
  - Keep action names opaque; they can mean LLMs, prompts, mutations, simulations, or non-AI operations.

  Suggested public shape:

  type Action string
  type NodeID int64
  type TrialID string

  type Trial[S any] struct {
      ID          TrialID
      ParentID    NodeID
      ParentState *S
      Action      Action
      Status      TrialStatus
  }

  type Result[S any] struct {
      State S
      Score float64
  }

  type Search[S any] interface {
      Ask(actions []Action) (Trial[S], error)
      AskBatch(actions []Action, n int) ([]Trial[S], error)
      Tell(id TrialID, result Result[S]) error
      TopK(k int) ([]ScoredState[S], error)
      Snapshot() Snapshot[S]
  }

  Implement in stages:

  1. Tree, trials, ask/tell, ranking, validation, checkpointing.
  2. AB-MCTS-A with Beta and Gaussian conjugate distributions, Thompson sampling, UCB, GEN-versus-CONT
     selection, and reward backpropagation.

  3. Batch behavior and concurrency-safe orchestration.
  4. Standard MCTS, best-first search, and Tree-of-Thought BFS if API parity is wanted.
  5. AB-MCTS-M behind the same interface, using either a validated native Go statistical implementation or
     an optional external inference service.

  6. Optional visualization from snapshots; keep it outside the core package.

  Parity tests should replay fixed seeded traces against the Python implementation, covering selection,
  backpropagation, batching, duplicate Tell, out-of-order completion, serialization, and score validation.

  ## EBP Paper Evaluator

  The separate application should define:

  type AssessmentState struct {
      SourceHash       string
      Claims           []ClaimAssessment
      UnresolvedDebt   []Debt
      WorkbenchReport  Report
      Revision         int
      Provenance       Provenance
  }

  Use two TreeQuest actions:

  - constructive: strengthen mappings, evidence, bridge principles, tests, and debt retirement.
  - adversarial: locate counterexamples, hidden assumptions, overclaims, missing bridges, and unpaid debt.

  Both actions must return the same complete AssessmentState type. Returning a paper from A and only a
  critique from B would make their scores incomparable. A structured patch is acceptable internally, but
  apply it before passing a complete child state to evaluator E.

  For each selected trial:

  1. Pass the parent state and selected role to that worker.
  2. Parse and validate its structured result.
  3. Give evaluator E only that candidate child plus the immutable source and rubric.
  4. Have E return a rubric breakdown, evidence citations, confidence, and scalar search reward.
  5. Validate and normalize the scalar reward before Tell.
  6. Preserve the full evaluation outside TreeQuest as audit metadata.

  Use EBP promotion/debt rules and Workbench checks in the application, not the library. Deterministic
  checks should enforce source citations, required claim fields, no-final-truth language, and debt
  consistency; the LLM should not be the sole enforcement mechanism.

  ## Budget and Output Design

  A worker-turn flag is reasonable, but it should be an input policy rather than a flag embedded only in the
  output.

  Use a budget configuration containing:

  - maximum TreeQuest expansions;
  - maximum worker calls per role;
  - maximum evaluator calls;
  - per-call and global token limits;
  - optional wall-clock and monetary limits;
  - maximum tree depth and batch size.

  Every worker expansion normally consumes one worker call and one evaluator call. Record actual calls,
  tokens, model identifiers, prompts/configuration hashes, retries, and stop reason in the output
  provenance.

  The output should be an artifact bundle, not silently replace the research paper:

  - immutable original paper or source hash;
  - annotated paper or generated assessment report;
  - structured claim and debt ledger;
  - Workbench analysis;
  - evaluator scores and rationale;
  - search-tree/checkpoint reference;
  - configured budget and actual usage.

  This separation makes runs auditable and resumable while preventing an altered paper from being confused
  with the author’s original.

  ## Assumptions

  - AB-MCTS-A is the first production target; AB-MCTS-M is deferred.
  - Scores remain scalar and normalized to [0,1]; EBP rubric dimensions are scalarized by the application.
  - Evaluator E assesses one completed candidate at a time and does not communicate directly with workers.
  - The EBP application keeps provider adapters replaceable and may assign different models to A, B, and E.
  - Upstream behavior follows the TreeQuest repository (https://github.com/SakanaAI/treequest) and Sakana
    AB-MCTS description (https://sakana.ai/ab-mcts/).
