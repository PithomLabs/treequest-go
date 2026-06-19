1. Verdict

`reject_for_now`

The implementation has a critical concurrency defect in `TrialStore`/`AllRewards` that can corrupt tree state under concurrent `Ask`/`Tell` usage, a score-clamping violation in the EBP evaluator that breaks the normalized-score contract with the core algorithm, and a structural gap where debt consistency is enforced solely by an LLM with no deterministic validator. These are not polish issues; they are silent-correctness and boundary-violation bugs that must be fixed before acceptance.

2. Critical blockers

- `pkg/algo/abmcts_a.go`: `TrialStore.Pending`, `TrialStore.Finished`, and `AllRewards` are mutated in `AskBatch` and `Tell` without any mutex. `SearchTree` is concurrent-safe, but the mutable algo state is not. Concurrent `Tell` on the same or different trial IDs will race on map writes and slice appends. Fix: add a `sync.Mutex` to `ABMCTSAAlgoState` (or `TrialStore`) and hold it for all mutations, or document that the algorithm is strictly single-threaded and enforce it.
- `ebp-paper-evaluator/pkg/ebp/evaluator.go` lines 80–86: `RunEvaluator` silently clamps `evalResp.Score` to `[0,1]` before returning it. This means invalid LLM output is laundered into a valid `tree.Score`, and `validateScore` in the core never rejects it. The core’s score-validation contract is bypassed at the app boundary. Fix: return an error (or pass the raw score and let the core reject it); do not clamp.
- `pkg/algo/abmcts_a.go` `selectAction` (line 170): action selection iterates over `node.ActionBandits` (a `map`) and breaks ties by map iteration order. In Go, map iteration is randomized. Identical bandit values will produce different chosen actions across runs, breaking determinism. Fix: iterate over the `actions` slice parameter instead of the map, or add a stable tiebreaker.
- `ebp-paper-evaluator/pkg/ebp/validators.go`: There is no deterministic validator for debt consistency (`RemainingDebt` ∪ `RetiredDebt` must equal the full EBP checklist, no overlap). `PromotionReady` is currently computed as `len(RemainingDebt) == 0 && !FinalTruthFlag` in `RunEvaluator`, making the LLM the sole gate for promotion. This violates the requirement that deterministic validators enforce critical structural rules. Fix: add a `ValidateDebtConsistency` check in `RunDeterministicValidators` that compares retired+remaining against the canonical six-item checklist and sets `PromotionReady = false` on mismatch.

3. High-priority repairs

- `pkg/algo/abmcts_a.go` `NewBetaSampler` (line 25): The second PCG seed is hardcoded to `12345`. This is not a correctness bug, but it means two different user seeds can collapse to the same effective sequence if the first seed differs only in ways that collide with the fixed second half. Document this or derive the second seed from the first (e.g., via a mixer) so the seed space is fully utilized.
- `ebp-paper-evaluator/pkg/ebp/budget.go` `RecordCall`: `PromptTokens`, `CompletionTokens`, and `TotalTokens` are aggregated globally and not separated by worker vs. evaluator calls. The review checklist requires worker and evaluator tokens to be tracked separately. Add `WorkerPromptTokens`, `WorkerCompletionTokens`, `EvaluatorPromptTokens`, `EvaluatorCompletionTokens` (or equivalent grouped fields).
- `ebp-paper-evaluator/pkg/ebp/bundle.go` line 124: `getPromotionStatusString` returns `"PROMOTED_RESEARCH_ARTIFACT"`. This is overclaiming language. Per the EBP-safe language requirement, the status should be candidate-oriented (e.g., `"CANDIDATE_PASS"` or `"ALIVE_WITH_DEBT"` only). `PromotionReady` should not be rendered as a final-truth promotion in an audit artifact.
- `ebp-paper-evaluator/pkg/ebp/evaluator.go`: `parseJSONResponse` regex `(?s)\{.*\}` is greedy and will match from the first `{` to the last `}` in the content. If the LLM wraps the JSON in a markdown code block and adds commentary after the closing brace, the regex will swallow the trailing text and likely fail `json.Unmarshal`. Use a non-greedy match or extract the first balanced JSON object explicitly.
- `ebp-paper-evaluator/cmd/evaluate/main.go`: The declared budget (`maxTurns`, `maxIterations`) is never persisted into `ProvenanceLog` or `budget_usage.json`. Add `MaxTurnsDeclared`, `MaxIterationsDeclared` to provenance so consumed vs. declared budgets are both visible.

4. Medium-priority improvements

- `pkg/algo/abmcts_a.go` `Step` (line 334): `lastNode = nodes[len(nodes)-1]` assumes `GetNodes()` returns nodes in insertion order. This is true only because `ExpandIdx` is monotonically increasing and `GetNodes` sorts by it. That is correct, but the dependency is implicit. Add a comment or a `LastInsertedNode()` helper on `SearchTree` to make the intent explicit.
- `pkg/tree/checkpoint.go` `RestoreTree`: The restored `Node` mutexes are zero-value `sync.Mutex`es. If a snapshot is restored and the tree is then used in a concurrent context, the mutexes work fine. However, if a snapshot is taken from a live tree while nodes are locked, the snapshot itself is safe because `SaveSnapshot` takes `RLock` and then per-node `Lock`. No issue here, but document that snapshots are point-in-time and not atomic with respect to in-flight `Tell` calls.
- `ebp-paper-evaluator/pkg/ebp/worker_a.go` and `worker_b.go`: Both workers call `RunDeterministicValidators` but only on their own output (`ConstructiveAnalysis` or `AdversarialReview`). `AdversarialReview` is never validated for final-truth language. `RunDeterministicValidators` should also scan `AdversarialReview` and `EvaluatorReasoning`, because adversarial critiques can themselves contain final-truth overclaims.
- `ebp-paper-evaluator/pkg/ebp/state.go` `Clone`: `Clone` is a value receiver, so it copies the struct but the slices are deep-copied. This is correct. However, `Clone` is not used in `GenerateA`/`GenerateB` when `parent == nil`; a zero-value `ClaimEvalState` is created instead. This is fine but inconsistent.
- `treequest-go/pkg/algo/abmcts_a_test.go`: The `QueueSampler` falls back to `0.5` when exhausted. This masks bugs where the test asks for more samples than the queue provides. Consider making it panic or return an error so tests fail fast if the trace is under-specified.

5. Tests to add

`treequest-go`:
- `TestTrialStore_ConcurrentTell`: Spawn multiple goroutines calling `Tell` on distinct trials and assert no data race (`go test -race`) and correct final counts.
- `TestTell_UnknownTrialID`: Call `Tell` with a trial ID that was never `Ask`ed; expect `ErrUnknownTrial`.
- `TestTell_OutOfOrder`: Call `AskBatch` with batchSize 2, then `Tell` the second trial before the first; ensure both succeed.
- `TestTell_DuplicateDifferentHash`: Submit same trial ID with two different results; expect `ErrResultMismatch`.
- `TestSnapshot_RoundTripWithPendingTrials`: Serialize and deserialize an `ABMCTSAAlgoState` that contains pending trials. Currently impossible because `TrialStore` is not serializable; this test will expose the gap and drive the fix.
- `TestSelectAction_DeterministicTiebreak`: Use a sampler that returns identical values for all actions and verify the chosen action follows the input `actions` slice order, not map order.
- `TestTopK`: Add `TopK(k int) []StateScore[S]` to `SearchTree` and test that it returns the k highest-scoring non-root nodes.

`ebp-paper-evaluator`:
- `TestRunEvaluator_RejectsOutOfRangeScore`: Mock a response with `score: 1.5` and verify `RunEvaluator` returns an error (after fixing the clamping bug).
- `TestRunEvaluator_RejectsNaNScore`: Mock `score: NaN` and verify error.
- `TestRunDeterministicValidators_DebtConsistency`: Construct a state with `RemainingDebt=["needMap"]`, `RetiredDebt=["needMap"]` (overlap) and verify the validator rejects it or flags it.
- `TestRunDeterministicValidators_AllSixDebtItems`: Verify that if the union of remaining+retired does not equal the full checklist, `PromotionReady` is forced to `false`.
- `TestBudgetTracker_SeparateTokenAccounting`: Record worker and evaluator calls and verify `WorkerPromptTokens`, `EvaluatorPromptTokens`, etc. (after adding them).
- `TestSaveArtifactBundle_NoAPIKey`: Assert that `provenance.json`, `claim_ledger.json`, and `budget_usage.json` contain no strings matching `sk-`, `OPENROUTER_API_KEY`, or similar.
- `TestMockMode_NoRealProviderCall`: Verify that when `--mock=true`, no network call is attempted (e.g., by using a custom `net.Dial` hook or by asserting `OpenRouterClient` is never instantiated).
- `TestGenerateA_And_GenerateB_Attribution`: Verify that `workerClient.Generate` and `evaluatorClient.Generate` are called the expected number of times and that `BudgetClient` counts them separately.

6. Architecture boundary audit

| Criterion | Status | Notes |
|---|---|---|
| generic core isolation | **PASS** | `treequest-go` imports only stdlib + gonum. |
| no LLM/provider deps in core | **PASS** | Zero references to OpenRouter, OpenAI, Anthropic, prompts, workers, evaluators, EBP, HTTP. |
| OpenRouter app-layer only | **PASS** | `github.com/revrost/go-openrouter` appears only in `ebp-paper-evaluator/pkg/llm`. |
| AB-MCTS-A first | **PASS** | `ABMCTSA` is the only algorithm implementation. |
| AB-MCTS-M deferred | **PASS** | No AB-MCTS-M code present. |
| Ask/Tell primary | **PASS** | `Step` is a convenience wrapper, not the primary interface. |
| idempotent Tell | **PASS with caveat** | Logic is correct; missing mutex makes concurrent use unsafe. |
| snapshot safety | **PARTIAL FAIL** | Tree snapshots are safe, but `TrialStore` is not serialized/restored, and `TopK` is missing. |
| normalized score contract | **PARTIAL FAIL** | Core validates scores, but EBP app clamps invalid scores before they reach the core. |
| EBP output safety | **PARTIAL FAIL** | Report contains overclaiming language (`PROMOTED_RESEARCH_ARTIFACT`); debt consistency has no deterministic gate. |

7. EBP/PTW audit

| PTW Item | Status | Notes |
|---|---|---|
| needMap | **needInvariant** | Not enforced by deterministic validator. Solely LLM-driven. |
| needInvariant | **needInvariant** | Same. |
| needToyCheck | **needInvariant** | Same. |
| needNullModel | **needInvariant** | Same. |
| needObstruction | **needInvariant** | Same. |
| needFaithfulnessReview | **needInvariant** | Same. |
| promotion status | **needObstruction** | `PromotionReady` is computed from LLM debt list + final-truth flag. No deterministic check that all six debt items are actually resolved or that the debt lists are internally consistent. |

8. Final recommended next ticket

Fix the critical blockers first: (1) add a mutex to `ABMCTSAAlgoState`/`TrialStore` and protect all map mutations in `AskBatch`/`Ask`/`Tell`; (2) remove the score clamp in `RunEvaluator` and return an error for out-of-range scores; (3) stabilize `selectAction` by iterating over the `actions` slice instead of the `ActionBandits` map. Then add the missing deterministic debt-consistency validator and re-run the review.