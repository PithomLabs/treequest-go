## Review verdict

`code2_plan.md` is a solid repair plan. I would mark it:

```text
accept_with_minor_repairs
```

It directly addresses the adversarial review’s real blockers: concurrency protection, score clamping removal, deterministic action selection, debt-consistency validation, token attribution, declared budget persistence, snapshot restoration of trial state, and missing acceptance tests. 

## What is accepted

The repair scope correctly targets:

```text
treequest-go:
- mutex protection for TrialStore and AllRewards
- deterministic action selection
- TopK
- snapshot/restore of full algorithm state
- seed-space repair
- pending/finished trial snapshot testing

ebp-paper-evaluator:
- no score clamping
- out-of-range/NaN/Inf score rejection
- deterministic debt-consistency validator
- broader final-truth scanning
- separate worker/evaluator token accounting
- safer JSON extraction
- declared budget persistence
- API-key leakage tests
```

The test list is also well aligned with the adversarial review and should be treated as the minimum gate before restoring any code-acceptance status. 

## Minor repairs before implementation

### 1. Replace `EBP_VERIFIED_WITHOUT_DEBT`

The plan proposes replacing `PROMOTED_RESEARCH_ARTIFACT` with:

```text
EBP_VERIFIED_WITHOUT_DEBT
```

I would **not** use “verified” either. It still sounds too strong for an LLM-assisted audit.

Use one of these instead:

```text
CANDIDATE_REPORT_COMPLETE
DEBT_CHECKS_CURRENTLY_CLEAR
NO_OPEN_DEBT_REPORTED
HUMAN_REVIEW_REQUIRED
```

Best default:

```text
CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED
```

This preserves EBP discipline: even if the debt ledger is empty under current checks, that is not “verified truth.”

### 2. Clarify lock ordering to avoid deadlocks

The plan adds mutexes to both `TrialStore` and `ABMCTSAAlgoState`. Good, but it must define a lock order.

Add this rule:

```text
Lock order:
1. ABMCTSAAlgoState.mu
2. TrialStore.mu
3. SearchTree.mu / node locks

Never acquire these in reverse order.
```

Or simplify:

```text
Hold ABMCTSAAlgoState.mu for the entire Ask/AskBatch/Tell transaction and avoid nested TrialStore locks.
```

Without a lock-order rule, the fix could introduce deadlocks while removing races.

### 3. Be careful with `sync.RWMutex` plus custom JSON serialization

The plan says `TrialStore` should get a mutex and custom JSON serialization “if needed.” Make that explicit:

```text
TrialStore runtime type may contain sync.RWMutex.
TrialStoreSnapshot must not contain sync.RWMutex.
Snapshot code must copy Pending and Finished under lock into plain maps.
Restore must rebuild a fresh TrialStore with a zero-value mutex.
```

### 4. Make `AllRewards` snapshot semantics explicit

The plan says `SaveAlgoSnapshot` should include global rewards. Add:

```text
AllRewards must be copied under ABMCTSAAlgoState.mu.
Restored AllRewards order must be deterministic if used by selection/ranking.
```

If `AllRewards` is a slice, preserve append order. If it is a map, sort by `ExpandIdx` or node ID when deriving rankings.

### 5. Require `go test -race`

The plan mentions `go test -race` only inside one test description. Make it a verification command:

```bash
go test ./...
go test -race ./...
```

For `ebp-paper-evaluator`, add:

```bash
go test ./...
```

and keep any real OpenRouter tests behind an explicit integration flag.

### 6. Define canonical six EBP debt items

The debt-consistency validator depends on “canonical six EBP debt checklist items,” but the plan should name them:

```text
needMap
needInvariant
needToyCheck
needNullModel
needObstruction
needFaithfulnessReview
```

Validator rule:

```text
RemainingDebt ∩ RetiredDebt must be empty.
RemainingDebt ∪ RetiredDebt must equal the canonical checklist.
Unknown debt names fail validation.
Duplicate debt names fail validation.
```

## Updated repair status

```json
{
  "status": "accept_with_minor_repairs",
  "ready_for_implementation_after_repairs": true,
  "required_wording_repair": "Replace EBP_VERIFIED_WITHOUT_DEBT with candidate/human-review-safe language",
  "required_engineering_clarifications": [
    "define mutex lock order",
    "snapshot TrialStore without serializing mutexes",
    "copy AllRewards under lock",
    "make go test -race a required gate",
    "name canonical six EBP debt items"
  ]
}
```

## Recommended final patch to the plan

Add this section before “Verification Plan”:

```text
## Repair Safety Rules

1. No EBP output status may use “promoted,” “verified,” “proved,” “resolved,” or “final.”
   Use `CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED` for the best-case generated audit status.

2. Mutex lock order is:
   ABMCTSAAlgoState.mu → TrialStore.mu → SearchTree.mu / node locks.
   Code must not acquire locks in reverse order.

3. Snapshot records must not serialize mutexes, pointers, runtime locks, or live channels.
   TrialStoreSnapshot and AlgoStateSnapshot must be plain data records.

4. Canonical EBP debt checklist:
   needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview.

5. Acceptance requires:
   go test ./...
   go test -race ./...
   mock dry-run artifact generation
   no API-key leakage in artifacts
```

## EBP/PTW self-audit

**Claims:** `code2_plan.md` addresses the adversarial review blockers and is suitable for implementation after minor repairs.

**needMap:** Satisfied. Each adversarial finding maps to a concrete code repair and test.

**needInvariant:** Mostly satisfied. Add lock-order and no-“verified” language rules.

**needToyCheck:** Strong. The test list includes concurrency, idempotency, score validation, snapshot restore, debt consistency, and attribution.

**needNullModel:** Not the focus of this repair ticket; acceptable because this is a blocker repair, not an algorithm-comparison phase.

**needObstruction:** Remaining risks are lock-order deadlocks, overclaiming status language, and incomplete snapshot semantics.

**needFaithfulnessReview:** Still pending after this repair. This ticket restores code acceptance only if tests pass; it does not establish Sakana TreeQuest parity.

**Promotion status:** `TREEQUEST_REPAIR_0001_plan_accept_with_minor_repairs`; code remains `reject_for_now_pending_repairs`.
