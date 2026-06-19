You are a senior Go architect and algorithm engineer. Write a detailed implementation plan for a Go port of SakanaAI TreeQuest / AB-MCTS based on the recommendations below.

Project goal:

Build two separate codebases:

1. `treequest-go`

   * A general-purpose Go library for adaptive tree search.
   * It must not depend on LLMs, EBP, Workbench, papers, prompts, OpenAI/Anthropic/Gemini SDKs, PDFs, or any domain-specific logic.
   * It should use idiomatic Go: generics, context, errors, explicit state, explicit snapshots, deterministic tests.

2. `ebp-paper-evaluator`

   * A separate downstream application that imports `treequest-go`.
   * It uses EBP 2.1 / Workbench2 to evaluate physics or research papers.
   * It may use LLM workers A/B and evaluator E, but that logic must live outside `treequest-go`.

Core compatibility target:

Use **AB-MCTS-A first** as the first release target.

Do not target full TreeQuest parity in v0.1. Do not implement AB-MCTS-M first. AB-MCTS-M should be deferred because the upstream implementation depends on heavy Python Bayesian inference stacks such as PyMC / NumPyro / joblib. The first release should prioritize a pure-Go, source-faithful, useful AB-MCTS-A core.

Important architectural decisions already accepted:

1. Preserve TreeQuest source-shape where useful:

   * Algorithms are stateless behavior.
   * Evolving search state lives in `AlgoStateT`.
   * Use `Algorithm[StateT, AlgoStateT]`.
   * Use `Trial` and `TrialStore` as first-class concepts.
   * Support `Ask`, `AskBatch`, and `Tell`.
   * Keep `Step` only as a convenience helper.
   * Support `StateScorePairs` and `TopK`.

2. Public API:

   * `Ask` / `Tell` is primary.
   * `AskBatch` must exist early.
   * `Tell` must be order-independent and idempotent.
   * `Tell` must use durable `TrialID`s and `TrialStatus`.
   * Duplicate `Tell` with the same result hash should be a no-op.
   * Duplicate `Tell` with a different result should be an error.
   * Unknown trial IDs should be an error.
   * Out-of-order completion should be allowed.

3. Scoring:

   * Every score accepted by the core must be normalized to `[0,1]`.
   * Reject NaN, Inf, values below 0, and values above 1.
   * Raw score, rubric breakdown, rationale, citations, and evaluator metadata may exist only as metadata outside the algorithm math.
   * Core algorithm only uses scalar normalized score.

4. Randomness and reproducibility:

   * Inject randomness through an interface or seeded source.
   * Do not call global random functions directly in algorithm code.
   * Support deterministic seeded tests.
   * Prepare replay/parity tests against the Python implementation using fixed traces.

5. Checkpointing and snapshots:

   * Use explicit schema-versioned snapshots.
   * Do not serialize pointer graphs or mutexes.
   * Use flat node records by ID.
   * Include tree state, trial store, algorithm-specific state, stats, and config hash.
   * Use application-supplied `StateCodec[S]` because generic state `S any` may not be JSON-marshalable.
   * Core provides snapshot/restore structures and helpers.
   * Application chooses file/database/object storage.

6. Core terminology:

   * Use neutral names: `Action`, `ActionLabel`, `Generator`, `Trial`, `Result`, `Score`, `Reward`, `StateScore`, `SelectionStrategy`, `BackupStrategy`.
   * Avoid core terms like LLM, model, prompt, response, worker, constructive, adversarial, evaluator E, EBP, claim, paper, debt, promotion.
   * Action names are opaque strings.

7. v0.1 `treequest-go` scope:

   * `Node[StateT]`
   * `Tree[StateT]`
   * `Algorithm[StateT, AlgoStateT]`
   * `Trial`, `TrialStore`, `TrialStatus`
   * `StateScore[StateT]`
   * `Ask`, `AskBatch`, `Tell`
   * `StateScorePairs`
   * `TopK`
   * `ABMCTSA`
   * Beta Thompson sampling
   * GEN-versus-CONT / wider-versus-deeper selection
   * Action selection via Thompson sampling
   * Score validation
   * Idempotent `Tell`
   * Snapshot/restore with codec
   * Deterministic toy examples
   * Minimal dependency core: standard library plus optional Gonum for distributions

8. v0.1 should not include:

   * AB-MCTS-M
   * OpenAI/Anthropic/Gemini clients
   * EBP/Workbench logic
   * PDF parsing
   * prompt templates
   * visualization as a core dependency
   * SQLite/S3 checkpoint backend
   * Python FFI
   * CLI-heavy framework unless clearly optional

9. v0.2+ roadmap:

   * Standard MCTS baseline
   * Tree-of-Thoughts BFS baseline
   * Best-First Search
   * Multi-Armed Bandit UCB baseline
   * DOT renderer from snapshots
   * richer checkpoint storage helpers
   * optional Gaussian sampler if needed
   * AB-MCTS-M research spike later

10. EBP paper evaluator app:

* It must be separate.
* It defines a complete `AssessmentState`.
* Both constructive and adversarial actions must return the same complete `AssessmentState` type.
* A and B may internally produce patches, but the app must apply patches before calling evaluator E.
* Evaluator E assesses one completed candidate state at a time.
* TreeQuest receives only complete state + normalized score.
* Full evaluator metadata is stored outside TreeQuest as provenance.
* Deterministic validators must enforce citations, required claim fields, no-final-truth language, and debt consistency.
* LLM E is not the sole enforcement mechanism.
* Budget exhaustion is explicit incompleteness, not failure.
* Output is an artifact bundle, not a modified paper replacement.

11. EBP app budget model:

* TreeQuest core budget: max trials, max nodes, max depth, max iterations, timeout.
* EBP app budget: worker turns, evaluator calls, tokens, cost, wall-clock, retries.
* Every worker expansion normally consumes one worker call and one evaluator call.
* Record actual calls, tokens, model IDs, prompt/config hashes, retries, and stop reason.
* Include declared budget and consumed budget in output provenance.
* Support dry-run cost estimation.
* Support soft per-role caps and optional hard per-role caps.

12. EBP output artifact bundle:

* immutable source hash or original source reference
* annotated paper or assessment report
* structured claim ledger
* debt ledger
* Workbench report
* evaluator scores and rationale
* provenance
* budget usage
* search tree/checkpoint reference
* generated report must not be confused with the original author’s paper

Task:

Write a full implementation plan with these sections:

1. Executive summary
2. Final architecture: two repositories
3. `treequest-go` package layout
4. `treequest-go` public API design
5. Core type definitions with Go code sketches
6. AB-MCTS-A algorithm implementation plan
7. TrialStore and idempotent `Tell` design
8. Score validation and reward metadata plan
9. Randomness/reproducibility plan
10. Snapshot/restore and `StateCodec[S]` plan
11. v0.1 milestone breakdown with tests
12. v0.2+ roadmap
13. Python parity/replay test strategy
14. `ebp-paper-evaluator` package layout
15. EBP worker A / worker B / evaluator E flow
16. Deterministic EBP validators
17. Budget and provenance model
18. Output artifact bundle design
19. Risks and mitigations
20. Explicit non-goals
21. Acceptance criteria for v0.1
22. EBP/PTW self-audit: needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview, promotion status

Writing requirements:

* Be specific and implementation-oriented.
* Use Go code sketches where useful.
* Do not include any LLM, EBP, paper, prompt, or provider logic inside `treequest-go`.
* Do not overclaim that the Go port is faithful until parity tests exist.
* Treat the plan as a workbench implementation plan, not a proof or final-truth claim.
* Keep AB-MCTS-M deferred.
* Keep all executable implementation guidance in Go.



Additional requirement:

The separate `ebp-paper-evaluator` application must support OpenRouter through the Go package:

```text
github.com/revrost/go-openrouter
```

Important boundary rule:

* `github.com/revrost/go-openrouter` must **not** be imported by `treequest-go`.
* OpenRouter support belongs only in the downstream `ebp-paper-evaluator` application.
* `treequest-go` must remain provider-independent and LLM-free.

Implement an app-layer LLM abstraction such as:

```go
type LLMClient interface {
    Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}
```

Then implement an OpenRouter adapter backed by `github.com/revrost/go-openrouter`.

The `ebp-paper-evaluator` should support at least:

* OpenRouter API key from environment variable, for example `OPENROUTER_API_KEY`
* configurable model IDs for Worker A, Worker B, and Evaluator E
* per-role temperature / max-token settings
* structured JSON output mode where supported
* retry handling
* token/cost/provenance accounting
* prompt/config hashing
* model ID recording in provenance
* optional streaming later, but non-streaming request/response is enough for v0.1

Example app-layer config shape:

```go
type ProviderConfig struct {
    Provider string `json:"provider"` // "openrouter"
    APIKeyEnv string `json:"api_key_env"`
    BaseURL string `json:"base_url,omitempty"`

    WorkerAModel string `json:"worker_a_model"`
    WorkerBModel string `json:"worker_b_model"`
    EvaluatorModel string `json:"evaluator_model"`

    WorkerATemperature float64 `json:"worker_a_temperature"`
    WorkerBTemperature float64 `json:"worker_b_temperature"`
    EvaluatorTemperature float64 `json:"evaluator_temperature"`

    MaxTokensPerCall int `json:"max_tokens_per_call"`
}
```

CLI requirement:

```bash
ebp-paper-evaluator run \
  --provider openrouter \
  --worker-a-model <model-id> \
  --worker-b-model <model-id> \
  --evaluator-model <model-id> \
  --max-turns 50 \
  --out ./out/audit
```

Security requirement:

* Never store raw API keys in output artifacts.
* Record only the provider name, model IDs, prompt/config hashes, call counts, token usage, retries, and stop reason.

Testing requirement:

* Add a fake `LLMClient` for deterministic tests.
* Add OpenRouter adapter tests behind an integration-test flag or environment check.
* Unit tests must not require a real OpenRouter API key.
* The `treequest-go` test suite must not import OpenRouter or any LLM provider package.

Note: `github.com/revrost/go-openrouter` is an unofficial Go client for the OpenRouter API, so the implementation plan should include a fallback boundary: keep the adapter isolated behind `LLMClient` so it can be replaced later by the official OpenRouter Go SDK or another OpenAI-compatible client without changing the EBP evaluator workflow.


