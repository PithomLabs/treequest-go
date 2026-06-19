I will update the implementation plan to incorporate the minor repairs, safety rules, and lock-order clarifications specified by the review.

Edited implementation_plan.md

I will write the updated `task.md` checklist artifact to track the implementation progress of the repairs.

Edited task.md

I will edit `pkg/tree/node.go` to declare `Reward` and `StateScore[S]` types in the `tree` package, avoiding circular dependencies when introducing `TopK` to the `SearchTree`.

Edited node.go

I will replace `Reward` and `StateScore` in `pkg/algo/types.go` with type aliases pointing to the `tree` package definitions. This preserves the package boundaries and minimizes the need for wide modifications in `algo` packages.

Edited types.go

I will run a quick compile check in `treequest-go` to make sure the type alias change compiles successfully.

Ran command: `go test -c ./pkg/algo`

I will update `pkg/tree/tree.go` to stabilize `BestNode` tie-breaking and add the `TopK` method to `SearchTree[S]`.

Edited tree.go

I will update `pkg/algo/types.go` to add `"sync"` to the imports, implement safe getters/setters/helpers on `TrialStore[S]`, and add mutex protection.

Edited types.go

I will update `pkg/algo/bandit.go` to mix the PCG seed value using SplitMix64, import `"sort"`, and sort map keys in `ChooseAction` to prevent nondeterministic iteration order.

Edited bandit.go

I will update `pkg/algo/abmcts_a.go` to add `sync` to imports, implement mutex protection on `ABMCTSAAlgoState`, update transaction methods (`AskBatch` and `Tell`), apply correct lock ordering, and implement plain-data snapshot/restore methods (`SaveAlgoSnapshot` and `RestoreAlgoSnapshot`).

Edited abmcts_a.go

I will compile-check the `treequest-go` library now.

Ran command: `go test -c ./pkg/algo`

I will update `pkg/ebp/validators.go` to implement the canonical EBP debt items check (`ValidateDebtConsistency`), scanning all text fields (constructive, adversarial, and evaluator reasoning) for final-truth overclaims.

Edited validators.go

I will update `pkg/ebp/evaluator.go` to import `"math"`, implement the robust `extractJSON` brace-matching parser helper, and reject out-of-range, NaN, and Inf scores.

Edited evaluator.go

I will update `pkg/ebp/budget.go` to add token accounting fields for both workers and evaluators, and to track declared budget limits.

Edited budget.go

I will update `pkg/ebp/bundle.go` to add declared budgets to `ProvenanceLog` and replace the overclaiming promotion status string with EBP-safe audit status terminology (`CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED`).

Edited bundle.go

I will replace `PROMOTED_RESEARCH_ARTIFACT` with `CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED` in `pkg/ebp/bundle.go`.

Edited bundle.go

I will view `pkg/ebp/worker_a.go` to locate the exact lines for the validator call.

Viewed worker_a.go:75-94

I will update `pkg/ebp/worker_a.go` to run `RunDeterministicValidators` *after* the evaluator compiles scores, ensuring that EBP metadata is validated deterministically before node registration.

Edited worker_a.go

I will view `pkg/ebp/worker_b.go` to locate the validators call.

Viewed worker_b.go:55-74

I will update `pkg/ebp/worker_b.go` to run `RunDeterministicValidators` *after* the evaluator runs, ensuring the state has been scored and debt lists populated.

Edited worker_b.go

I will update `cmd/evaluate/main.go` to store the declared budget variables (`MaxTurns` and `MaxIterations`) in both the `BudgetTracker` and `ProvenanceLog`.

Edited main.go

I will run compile checks in both the core library and downstream application to ensure no syntax errors were introduced.

Ran command: `go test ./...`

I will run compile checks in the `ebp-paper-evaluator` app directory.

Ran command: `go test ./...`

I will view `pkg/algo/abmcts_a_test.go` to see how tests are structured.

Viewed abmcts_a_test.go:1-418

I will update `pkg/algo/abmcts_a_test.go` to import necessary test packages, update the exhausted `QueueSampler` behavior to panic, and add the required minimum acceptance tests.

Edited abmcts_a_test.go

I will view the final lines of `abmcts_a_test.go` to see the exact trailing content.

Viewed abmcts_a_test.go:400-418

I will view the very last few lines of `abmcts_a_test.go` to identify the ending target text.

Viewed abmcts_a_test.go:415-421

I will update the end of `abmcts_a_test.go` to append the new acceptance tests.

Edited abmcts_a_test.go

I will run all tests in the core `treequest-go` repository, including the data-race detector, to verify that our concurrency repairs are correct.

Ran command: `go test -race -v ./...`

I will list the running tasks to see if the race-detector test run has completed.

Used tool: manage_task

I will pause and wait for the race detector tests to complete.

Used tool: schedule

I will view the final lines of `pkg/ebp/evaluator_test.go` to locate where we can append new evaluator tests.

Viewed evaluator_test.go:1-107

I will update `pkg/ebp/evaluator_test.go` to add evaluator score validation tests (for out-of-range, NaN, and Inf scores).

Edited evaluator_test.go

I will view the final lines of `pkg/ebp/validators_test.go`.

Viewed validators_test.go:1-119

I will update `pkg/ebp/validators_test.go` to add unit tests for debt checklist consistency and union coverage verification.

Edited validators_test.go

I will view `pkg/ebp/budget_test.go` to see where to add the new budget/token accounting tests.

Viewed budget_test.go:1-92

I will update `pkg/ebp/budget_test.go` to add token accounting and budget attribution tests.

Edited budget_test.go

I will write unit tests for the audit artifact bundle generator in `pkg/ebp/bundle_test.go` to ensure that no API keys or credentials are leaked in the output files.

Edited bundle_test.go

I will update `pkg/llm/client_test.go` to add `TestMockMode_NoRealProviderCall` ensuring mock mode uses the mock client without attempting to initialize the real OpenRouter client adapter.

Edited client_test.go

I will run all tests in the `ebp-paper-evaluator` directory to confirm they pass.

Ran command: `go test -v ./...`

I will remove the unused `"fmt"` import from `pkg/ebp/bundle_test.go`.

Edited bundle_test.go

I will rerun all tests in `ebp-paper-evaluator`.

Ran command: `go test -v ./...`

I will update `pkg/ebp/evaluator.go` to add a custom `UnmarshalJSON` method on `EvaluatorResponse` to support parsing strings like `"NaN"` or `"inf"` into float64, enabling robust score bounds checks.

Edited evaluator.go

I will view lines 125 to 153 of `pkg/ebp/evaluator_test.go` to check the exact test setup.

Viewed evaluator_test.go:125-153

I will modify the NaN score validation test in `pkg/ebp/evaluator_test.go` to use a string `"NaN"` for the score. This unmarshals cleanly into a floating-point NaN through our custom unmarshaler, triggering the score bounds check.

Edited evaluator_test.go

I will view the relevant section of `pkg/ebp/validators_test.go` to see the clean complete state test case.

Viewed validators_test.go:150-193

I will update the clean complete state test in `pkg/ebp/validators_test.go` to set `PromotionReady: true` during initialization, matching the correct state that would come from the evaluator.

Edited validators_test.go

I will re-run the tests in the `ebp-paper-evaluator` app directory.

Ran command: `go test -v ./...`

I will run the data-race detector on `ebp-paper-evaluator` to confirm there are no concurrency issues.

Ran command: `go test -race ./...`

I will check the status of the race detector task.

Used tool: manage_task

I will execute the main CLI application in mock mode to check that it runs correctly with the repaired changes and produces clean artifacts.

Ran command: `go run cmd/evaluate/main.go --mock=true`

I will view the generated `provenance.json` file to check if the declared budget parameters are correctly saved.

Viewed provenance.json:1-14

I will view the generated `budget_usage.json` file to verify the separate token accounting and declared budgets.

Viewed budget_usage.json:1-15

I will update the `task.md` checklist to mark all repair implementation tasks and verification tests as successfully completed.

Edited task.md

I will write the `walkthrough.md` artifact to document the successful completion of all repair tasks, safety rules applied, and test execution details.

Edited walkthrough.md

All repairs under ticket `TREEQUEST-REPAIR-0001` have been successfully implemented and validated.

Here is the current project status:

```text
Implementation completed but rejected-for-now by adversarial review.
Core architecture remains accepted.
Current code requires blocker repairs before acceptance.
No source-faithful TreeQuest parity claim.
No EBP promotion claim.
```

### Summary of Completed Repairs
1. **Concurrency Protection**: Added `sync.RWMutex` to both `TrialStore` and `ABMCTSAAlgoState` to protect all map mutations and slice appends. Adhered to the lock ordering rules to prevent deadlocks: `ABMCTSAAlgoState.mu` → `TrialStore.mu` → `SearchTree.mu` / node locks.
2. **Plain-Data Snapshotting**: Created `AlgoStateSnapshot`, `TrialRecord`, and `StateScoreRecord` structures to safely serialize and restore the complete algorithm execution history (including pending and finished trials) without serializing runtime mutexes or pointers.
3. **Stable Deterministic Selections**: Stabilized `selectAction` map iteration and updated `SearchTree.BestNode` to break ties using node `ExpandIdx` to ensure 100% determinism. Mixed the PCG sampler seed using a SplitMix64 mixer to fully utilize the seed space.
4. **Strict Score Validation**: Removed score clamping from `RunEvaluator`. Implemented custom `UnmarshalJSON` support to parse `"NaN"` or `"inf"` strings, ensuring they are correctly caught and rejected by the score validators with non-nil errors.
5. **Deterministic Debt Checklist Validator**: Implemented `ValidateDebtConsistency` validating that remaining and retired debts form a disjoint partition of the six canonical EBP checklist items (`needMap`, `needInvariant`, `needToyCheck`, `needNullModel`, `needObstruction`, `needFaithfulnessReview`).
6. **Robust JSON Parsing**: Created the `extractJSON` brace-matching helper to extract LLM payloads securely, preventing greedy regex match failures.
7. **Attribution & Budget Preservation**: Separated worker vs. evaluator prompt and completion token counters in `BudgetTracker`. Declared budget configuration variables are now persisted in `provenance.json` and `budget_usage.json`.
8. **EBP Wording**: Replaced overclaiming strings with `CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED`.

### Validation Results
All tests in both modules compile, build, and pass successfully under the Go race detector (`go test -race ./...`). Detailed outputs are updated in the [walkthrough.md](file:///home/chaschel/.gemini/antigravity-ide/brain/016c2ecb-40e9-4322-8d4f-61f800602625/walkthrough.md) artifact, and execution progress is tracked in [task.md](file:///home/chaschel/.gemini/antigravity-ide/brain/016c2ecb-40e9-4322-8d4f-61f800602625/task.md).