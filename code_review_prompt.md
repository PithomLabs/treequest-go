You are an adversarial senior Go code reviewer. Review the completed implementation of two related codebases:

1. `treequest-go`

   * A generic Go port of SakanaAI TreeQuest / AB-MCTS.
   * Must remain completely independent of LLMs, OpenRouter, EBP, Workbench, papers, prompts, provider SDKs, HTTP clients, and domain logic.
   * v0.1 target is AB-MCTS-A only: pure-Go, Beta Thompson Sampling, Ask/AskBatch/Tell, TrialStore, score validation, snapshots, and deterministic tests.
   * AB-MCTS-M is deferred.

2. `ebp-paper-evaluator`

   * A downstream application that imports `treequest-go`.
   * Implements EBP 2.1 / Workbench2 paper-evaluation workflow.
   * Uses Worker A constructive, Worker B adversarial, and Evaluator E scoring.
   * May use OpenRouter through `github.com/revrost/go-openrouter`, but this dependency must never appear in `treequest-go`.

Your job is to look for architectural violations, hidden coupling, algorithmic drift, incorrect AB-MCTS-A behavior, brittle concurrency, idempotency bugs, serialization bugs, budget/provenance errors, and EBP overclaiming.

Context from reported implementation:

* `plan3.md` was updated to mark the plan accepted for implementation, but not source-faithful until tests and parity fixtures pass.
* `treequest-go` generic core and `ebp-paper-evaluator` app were implemented.
* `GenerateA` and `GenerateB` were updated to receive both `workerClient` and `evaluatorClient` so worker calls and evaluator calls are attributed separately.
* CLI compile issues were fixed.
* Unit tests were added for:

  * deterministic validators
  * evaluator JSON parsing, including markdown-wrapped JSON and fallback defaults
  * budget accounting
  * LLM client prompt hashing and mock responses
* `go test -v ./...` reportedly passes for the EBP packages.
* Mock dry run reportedly succeeds:

  * `go run cmd/evaluate/main.go --mock=true`
  * completed 10 MCTS iterations
  * generated `assessment_report.md`, `budget_usage.json`, `claim_ledger.json`, `provenance.json`, and `tree_snapshot.json`

Important review boundaries:

* Do not claim source-faithful Sakana TreeQuest parity unless Python replay/parity fixtures are present and reviewed.
* Do not treat mock dry-run success as real OpenRouter validation.
* Do not treat EBP evaluator output as proof of physics claims.
* Do not allow generated assessment artifacts to be confused with the original paper.
* Do not allow LLM E to be the sole enforcement mechanism; deterministic validators must enforce critical structural rules.

Review checklist:

## A. `treequest-go` boundary review

Check whether `treequest-go` imports or references any forbidden concepts:

* OpenRouter
* OpenAI
* Anthropic
* Gemini
* LLM
* prompt
* response
* worker
* evaluator E
* constructive
* adversarial
* EBP
* Workbench
* claim
* paper
* debt
* promotion
* HTTP/network provider code

Expected result: none of these should appear in the generic core except possibly in comments explicitly saying they are out of scope. Prefer no references at all.

## B. Public API review

Verify that the core API follows the locked design:

* `Ask`
* `AskBatch`
* `Tell`
* `Trial`
* `TrialStore`
* `TrialStatus`
* `StateScore`
* `Score`
* `StateCodec[S]`
* snapshot/restore
* `Algorithm[S, AS]` or an equivalent source-faithful pattern
* `Step` only as a convenience wrapper, not the primary execution path

Flag any Step-first design, global mutable search loop, or API that forces LLM-style generation.

## C. TrialStore and idempotent `Tell`

Review carefully:

* Are trial IDs durable and unique?
* Are pending/completed/failed/canceled statuses represented?
* Does `AskBatch` create pending trials?
* Does `Tell` reject unknown trial IDs?
* Does `Tell` allow out-of-order completion?
* Does duplicate `Tell` with the same result hash become a no-op?
* Does duplicate `Tell` with a different result return an error?
* Is `ResultHash` derived consistently when the application does not supply it?
* Are pending trials included in snapshots?
* Does restore preserve enough trial state to continue safely?
* Are race conditions possible if `Tell` is called concurrently?

Create specific failing test suggestions for any gap.

## D. Score validation

Check that all scores entering tree math are validated:

* reject score < 0
* reject score > 1
* reject NaN
* reject +Inf / -Inf
* do not silently clamp invalid scores
* do not accept raw rubric values directly into the algorithm
* use only normalized scalar score for bandit/backprop math

Flag any path where invalid scores can mutate the tree.

## E. AB-MCTS-A algorithm review

Review whether the implementation actually matches the intended v0.1 target:

* Beta Thompson Sampling
* wider-vs-deeper / GEN-vs-CONT decision
* action selection via action bandits
* reward backpropagation
* action-bandit update
* node stats update
* no AB-MCTS-M code in v0.1
* no Python FFI
* no UCB pretending to be AB-MCTS-A unless explicitly baseline/deferred

Identify exact algorithmic uncertainties that require upstream Python parity traces.

Pay special attention to:

* first-node / cold-start behavior
* leaf behavior
* whether wider/deeper updates are applied to the correct nodes
* whether action priors are initialized for new actions
* whether zero-visit or nil-bandit cases are safe
* whether deterministic seeded sampler tests are meaningful
* whether `ExpandIdx` is maintained and restored

## F. Randomness and reproducibility

Check:

* no global randomness inside algorithm code
* sampler is injected or seedable
* deterministic sampler tests exist
* deterministic fake `S=string` or equivalent toy tests exist
* tests are stable under repeated runs
* randomness state is handled or intentionally documented for snapshot/restore

Flag any nondeterminism hidden in map iteration, especially action selection over maps.

## G. Snapshot / restore review

Check:

* snapshots are flat records by ID
* no pointer graphs serialized
* no mutexes serialized
* `StateCodec[S]` is used for generic state
* schema version exists
* root ID is stored
* parent/child links restore correctly
* bandit parameters restore correctly
* trial store restores correctly
* best-path / TopK works after restore
* malformed snapshot errors are safe and clear
* API keys or prompt bodies are not accidentally stored in generic `treequest-go` snapshots

Propose restoration tests if absent.

## H. Package structure and dependency review

Run or inspect equivalent of:

```bash
go list -deps ./...
go test ./...
go vet ./...
```

Check that:

* `treequest-go` depends only on stdlib plus optional Gonum.
* `treequest-go` does not import network libraries unless strictly necessary for stdlib reasons.
* OpenRouter dependency exists only in `ebp-paper-evaluator`.
* mocks do not leak into production runtime paths.
* package boundaries do not create circular or domain coupling.

## I. `ebp-paper-evaluator` flow review

Check that:

* Worker A and Worker B both produce the same complete `ClaimEvalState` type.
* Patch-only worker outputs are applied before Evaluator E scores.
* Evaluator E scores one completed candidate at a time.
* TreeQuest receives only complete state + normalized score.
* evaluator metadata is preserved outside TreeQuest.
* worker client calls and evaluator client calls are tracked separately.
* mock mode cannot accidentally call real providers.
* real OpenRouter mode requires explicit API key and model config.
* API keys are never written to provenance or logs.

## J. Deterministic EBP validators

Review deterministic validators for:

* final-truth / overclaim language
* missing citations
* debt consistency
* required fields
* budget exhaustion visibility
* Workbench required sections

Check whether the validators are strict enough or mostly cosmetic. The LLM evaluator must not be the only gate.

Suggest additional tests such as:

* final-truth phrase hidden in markdown
* missing citation with plausible-looking bracket
* debt retired without evidence
* BudgetExhausted true but not reflected in report
* evaluator score high despite validator failure
* malformed JSON with fallback behavior that hides failure

## K. Budget and provenance review

Check:

* worker calls and evaluator calls are counted separately
* tokens are tracked separately from calls
* cost estimate is documented as estimate, not exact billing truth
* retries are counted
* stop reason is recorded
* prompt/config hashes are recorded
* raw API keys are never stored
* model IDs are recorded
* declared budget and consumed budget both appear
* dry run estimates cost without calling providers
* budget exhaustion becomes explicit incompleteness, not failure or truth judgment

## L. Output artifact bundle review

Check generated files:

* `assessment_report.md`
* `budget_usage.json`
* `claim_ledger.json`
* `provenance.json`
* `tree_snapshot.json`

Verify:

* generated report is clearly labeled as generated assessment, not original paper
* original source hash or source reference is preserved
* budget usage is complete and separate
* provenance has no secrets
* tree snapshot is generic and does not contain provider secrets
* claim ledger avoids saying claims are proven, resolved, or true unless human review actually says so
* report uses EBP-safe language: alive, debt-visible, needs review, budget-exhausted, candidate report, not final truth

## M. Tests that must exist or be added

Check for existing tests and propose missing ones:

`treequest-go`:

* invalid score rejection
* NaN/Inf rejection
* duplicate `Tell` same hash
* duplicate `Tell` different hash
* out-of-order `Tell`
* unknown trial ID
* snapshot/restore with pending trials
* snapshot/restore with completed trials
* deterministic sampler trace
* action map deterministic ordering
* `TopK`
* `StateCodec[S]` failure handling
* toy fake `S=string` AB-MCTS-A run

`ebp-paper-evaluator`:

* mock mode does not use real provider
* OpenRouter adapter behind integration flag only
* worker/evaluator attribution
* budget exhaustion appears in output
* final-truth validator
* citation validator
* debt consistency validator
* evaluator malformed JSON fallback does not hide failure
* no API key in provenance
* artifact bundle creation

## N. Required output format

Return the review in this exact structure:

1. Verdict

   * one of:

     * `accept`
     * `accept_with_repairs`
     * `reject_for_now`
   * Include a one-paragraph reason.

2. Critical blockers

   * Must-fix before accepting code.
   * Use file/function names when possible.

3. High-priority repairs

   * Important but not necessarily blocking.

4. Medium-priority improvements

   * Cleanup, maintainability, ergonomics.

5. Tests to add

   * Include concrete test names.

6. Architecture boundary audit

   * Pass/fail table:

     * generic core isolation
     * no LLM/provider deps in core
     * OpenRouter app-layer only
     * AB-MCTS-A first
     * AB-MCTS-M deferred
     * Ask/Tell primary
     * idempotent Tell
     * snapshot safety
     * normalized score contract
     * EBP output safety

7. EBP/PTW audit

   * needMap
   * needInvariant
   * needToyCheck
   * needNullModel
   * needObstruction
   * needFaithfulnessReview
   * promotion status

8. Final recommended next ticket

   * Give the smallest next implementation or review ticket.

Review standard:

Be adversarial. Do not be polite at the expense of correctness. Do not accept mock dry-run success as proof of source-faithful TreeQuest parity. Do not accept LLM output as a validator. Do not allow overclaiming.
