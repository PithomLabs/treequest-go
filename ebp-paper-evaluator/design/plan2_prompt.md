You are a senior Go architect and EBP/PTW implementation planner.

Write a detailed implementation plan for the ticket:

# EBP-EVAL-DECL-0001: Declarative Policy Bundle and Paper-Ingestion Refactor

## Background

We have two existing Go codebases:

1. `treequest-go`

   * A generic Go adaptive tree-search library.
   * It must remain completely independent of LLMs, OpenRouter, EBP, Workbench, papers, prompts, providers, HTTP clients, and domain-specific logic.
   * It already implements the repaired v0.1 AB-MCTS-A scope: Ask/AskBatch/Tell, TrialStore, deterministic action selection, score validation, snapshot/restore, race-safe state handling, and deterministic tests.
   * Do not modify `treequest-go` except through its existing generic interfaces.

2. `ebp-paper-evaluator`

   * A downstream application that imports `treequest-go`.
   * It currently evaluates research claims using Worker A, Worker B, and Evaluator E.
   * It supports OpenRouter through an app-layer adapter using `github.com/revrost/go-openrouter`.
   * It currently contains too much hardcoded EBP-specific logic.
   * The goal of this ticket is to make the evaluator generic by moving EBP/business-policy rules into declarative policy files.

## User requirements

Design the evaluator so that it is user-friendly and generic:

1. All 3 LLM roles must ingest the given research paper:

   * Worker A
   * Worker B
   * Evaluator E

2. All 3 LLM roles must evaluate against EBP 2.1 markdown loaded as declarative policy.

3. How the LLMs agree on common EBP 2.1 metadata, such as claims, claim types, debt items, evidence spans, and readiness status, is an implementation detail.

4. The evaluator should not hardcode EBP business rules in Go wherever avoidable.

   * Policy and knowledge base should be declarative.
   * EBP 2.1 markdown should be the default policy bundle.
   * The evaluator should later be able to support another policy by changing policy files, not rewriting Go business logic.

5. Users should normally provide:

   * an arXiv ID,
   * URL,
   * PDF,
   * or text file,
     not manually enter claims.

6. Manual claim input should remain only as an expert/debugging override.

7. The original paper must remain immutable.

   * Output is an assessment bundle, not a rewritten paper.

8. Faithfulness review may be excluded from the automated profile, but if excluded it must be marked `not_assessed`, never falsely retired.

9. The report must say something like:

   * “Automated EBP profile excluding faithfulness review.”
   * “Faithfulness review was not performed.”
   * “This is an automated candidate assessment, not full EBP promotion.”

10. Do not use overclaiming language:

    * no “proved”
    * no “verified truth”
    * no “promoted research artifact”
    * no “final”
    * no “solved”
    * no “validated physics”

Use safe language such as:
`CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED`.

## Design target

The desired command should feel like this:

```bash
ebp-paper-evaluator run \
  --paper ./paper.pdf \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --provider openrouter \
  --worker-a-model <model> \
  --worker-b-model <model> \
  --evaluator-model <model> \
  --out ./out/audit
```

Optional debug override:

```bash
ebp-paper-evaluator run \
  --paper ./paper.pdf \
  --policy ./policies/ebp_v2_1.md \
  --claim-file ./debug_claims.json \
  --out ./out/audit
```

## Required architecture

Design around this high-level pipeline:

```text
paper input + policy markdown
  ↓
DocumentBundle
  ↓
PolicyBundle / PolicyIR
  ↓
MetadataConsensus
  ↓
PaperProfile with claim records and evidence spans
  ↓
one TreeQuest search per claim
  ↓
Worker A / Worker B / Evaluator E evaluate complete candidate states
  ↓
deterministic policy validators
  ↓
policy-derived RewardBuilder
  ↓
aggregate paper-level report and artifact bundle
```

## Required packages

Propose a concrete Go package layout, likely including:

```text
ebp-paper-evaluator/
  pkg/
    document/
      ingest.go
      normalize.go
      chunk.go
      evidence.go
      arxiv.go
      pdf.go
      text.go

    policy/
      bundle.go
      markdown.go
      compile.go
      ir.go
      profile.go
      validate_policy.go

    metadata/
      extract_claims.go
      consensus.go
      dedupe.go
      verify_spans.go

    eval/
      state.go
      roles.go
      orchestrator.go
      reward.go
      validators.go

    llm/
      client.go
      openrouter.go
      mock.go

    artifact/
      bundle.go
      report.go
      provenance.go
```

Modify the package layout if needed, but preserve the separation of concerns.

## Required data structures

Include Go code sketches for the main types.

At minimum include:

```go
type DocumentBundle struct {
    ID       string
    Title    string
    Abstract string
    Sections []Section
    Chunks   []Chunk
    Source   SourceMetadata
    Hash     string
    Summary  string
}

type EvidenceSpan struct {
    ID         string
    Section    string
    Page       int
    Start      int
    End        int
    Quote      string
    SourceHash string
}
```

Policy types:

```go
type PolicyBundle struct {
    ID         string
    Name       string
    Version    string
    SourcePath string
    SourceHash string
    Markdown   string
    IR         PolicyIR
}

type PolicyIR struct {
    ClaimTypes       []PolicyEnum
    FunctionClasses  []PolicyEnum
    DebtItems        []DebtItem
    StatusRules      []StatusRule
    HardFailures     []HardFailure
    RubricDimensions []RubricDimension
    RequiredOutputs  []RequiredOutput
    ReportLanguage   ReportLanguagePolicy
}
```

Debt/status types:

```go
type DebtItem struct {
    ID            string
    Label         string
    Description   string
    Automated     bool
    Required      bool
    DefaultStatus string
}

type DebtStatus string

const (
    DebtRemaining     DebtStatus = "remaining"
    DebtRetired       DebtStatus = "retired"
    DebtNotApplicable DebtStatus = "not_applicable"
    DebtNotAssessed   DebtStatus = "not_assessed"
)
```

Profile types:

```go
type EvaluationProfile struct {
    ID            string
    Name          string
    DebtOverrides map[string]DebtOverride
}

type DebtOverride struct {
    DefaultStatus                string
    IncludedInAutomatedReadiness bool
}
```

Metadata consensus types:

```go
type MetadataConsensus interface {
    BuildPaperProfile(
        ctx context.Context,
        doc DocumentBundle,
        policy PolicyBundle,
        roles RoleClients,
    ) (PaperProfile, error)
}

type PaperProfile struct {
    DocumentID string
    PolicyID   string
    Claims     []ClaimRecord
    Metadata   map[string]any
}

type ClaimRecord struct {
    ID            string
    Text          string
    ClaimType     string
    FunctionClass string
    EvidenceSpans []EvidenceSpan
    Confidence    float64
    SourceHash    string
}
```

Validation and reward types:

```go
type ValidationInput struct {
    Policy    PolicyBundle
    Document  DocumentBundle
    Claim     ClaimRecord
    Candidate AssessmentState
}

type ValidationResult struct {
    Passed           bool
    Flags            []ValidationFlag
    RewardCaps       []RewardCap
    ReadinessBlocked bool
}

type EvaluatorJudgment struct {
    DimensionScores map[string]float64
    DebtDecisions   map[string]DebtDecision
    Flags           []string
    Rationale       string
}
```

## Policy markdown requirements

Design the evaluator to load EBP 2.1 markdown as the declarative policy source.

The policy system should support:

1. Raw human-readable markdown.
2. Optional embedded machine-readable fenced block, for example:

````markdown
```policy-json
{
  "debt_items": [
    {
      "id": "needMap",
      "required": true,
      "automated": true,
      "default_status": "remaining"
    },
    {
      "id": "needFaithfulnessReview",
      "required": true,
      "automated": false,
      "default_status": "not_assessed"
    }
  ],
  "hard_failures": [
    {
      "id": "final_truth_language",
      "reward_cap": 0.2,
      "blocks_readiness": true
    },
    {
      "id": "invented_source_quote",
      "reward_cap": 0.0,
      "blocks_readiness": true
    }
  ]
}
```
````

3. A compile command:

```bash
ebp-paper-evaluator compile-policy \
  --policy ./policies/ebp_v2_1.md \
  --out ./policies/ebp_v2_1.policy.json
```

4. Policy hashing:

   * policy markdown hash
   * compiled PolicyIR hash
   * include both in provenance

5. Policy validation:

   * unknown status values fail
   * duplicate debt item IDs fail
   * rubric weights must be valid
   * hard-failure reward caps must be in `[0,1]`
   * required output fields must be declared

## All 3 LLM roles ingesting paper and policy

Design the system so all 3 roles receive a shared run context:

```text
Paper:
- title
- source hash
- abstract
- section map
- global summary
- relevant evidence spans

Policy:
- EBP 2.1 markdown
- PolicyIR checklist/rubric/status rules
- evaluation profile
```

Do not paste the full paper into every prompt repeatedly. Instead:

* ingest the paper once
* build sections/chunks/evidence spans
* provide relevant source spans and nearby context per claim
* include paper hash and policy hash in every LLM call
* log prompt hashes and model IDs in provenance

The design must still satisfy “all 3 LLMs ingest the paper” by ensuring each role receives the shared document context and can evaluate against the same source/evidence structure.

## Metadata consensus design

Treat claim extraction and shared EBP metadata as an implementation detail behind `MetadataConsensus`.

First implementation can be:

1. Worker A extracts candidate claims from chunks.
2. Worker B adversarially checks for unsupported, duplicated, missing, or overbroad claims.
3. Evaluator E consolidates canonical `ClaimRecord` entries.
4. Deterministic verifier rejects claims without evidence spans.
5. Deduplicator merges semantically similar claims.

The user-facing contract must remain:

```text
Paper + Policy → PaperProfile
```

not:

```text
User manually enters claims.
```

## Claim extraction requirements

Implement:

```text
arXiv ID/URL/PDF/text
  → normalized document
  → section-aware chunks
  → candidate claims
  → verification and deduplication
  → one search per claim
  → aggregate report
```

Requirements:

* prefer arXiv HTML/source text when available
* PDF extraction is fallback
* text input remains supported
* section/page/evidence provenance must survive normalization
* every claim must have a quote or normalized quote and location
* reject invented or unsupported claims
* deduplicate claims across sections
* do not send the full paper on every LLM call

## Evaluation flow per claim

For each extracted claim:

1. Initialize assessment state from `PolicyIR` and `EvaluationProfile`.
2. Worker A produces a complete constructive `AssessmentState`.
3. Worker B produces a complete adversarial `AssessmentState`.
4. Evaluator E scores one complete candidate at a time against:

   * claim
   * evidence spans
   * policy markdown
   * PolicyIR
   * debt/status history
   * current analyses
5. Deterministic validators run.
6. RewardBuilder derives final normalized score from policy rubric and validator caps.
7. TreeQuest receives only complete candidate state + normalized score.
8. Persist full evaluator metadata outside `treequest-go`.

## Reward design

Do not accept arbitrary LLM scalar scores as the final TreeQuest reward.

Implement:

```text
base score = weighted rubric dimensions from PolicyIR
final score = base score capped by deterministic validator failures
```

Example dimensions can include:

* source support
* claim type correctness
* function class correctness
* map quality
* invariant quality
* toy-check quality
* null-model quality
* obstruction handling
* bridge validity
* adversarial issues resolved
* uncertainty disclosure

Hard failures should cap or zero the final reward:

* invented quotation
* unsupported claim
* invalid debt partition
* final-truth language
* malformed required output
* missing evidence span

## Validator design

Validators must be policy-driven, not EBP-hardcoded.

Implement generic validators such as:

* DebtPartitionValidator
* ForbiddenLanguageValidator
* Citation/EvidenceSpanValidator
* RequiredOutputValidator
* RewardCapValidator
* ProfileStatusValidator

Validators should read from `PolicyIR`, `EvaluationProfile`, `DocumentBundle`, and `ClaimRecord`.

## Faithfulness policy

Implement `automated-no-faithfulness` as a profile, not as hardcoded logic.

For EBP 2.1:

* `needFaithfulnessReview` should be `not_assessed`
* it should not be placed in `RetiredDebt`
* it should not be included in automated readiness calculation
* report must visibly say faithfulness review was not performed
* full `PromotionReady` should not be used for the automated run
* use a scoped field such as `AutomatedReviewReady`

## Output artifact bundle

Design output bundle:

```text
out/audit/
  source/
    original or source_ref.json
    source_hash.txt

  policy/
    policy_snapshot.md
    policy_ir.json
    policy_hash.txt

  document/
    document_bundle.json
    evidence_ledger.json

  claims/
    claim_ledger.json
    claim_<id>_assessment.json

  tree/
    claim_<id>_tree_snapshot.json

  report/
    assessment_report.md

  run/
    provenance.json
    budget_usage.json
```

Requirements:

* no API keys
* no raw secrets
* original paper remains immutable
* generated assessment clearly labeled
* policy hash included
* source hash included
* budget and model provenance included
* no EBP promotion claim

## Required test plan

Include tests for:

Policy:

* policy markdown loads and hashes
* embedded `policy-json` parses
* duplicate debt IDs rejected
* invalid reward caps rejected
* automated-no-faithfulness marks `needFaithfulnessReview` as `not_assessed`
* changing policy fixture changes validator behavior without code changes

Document:

* plain text ingestion
* PDF ingestion fallback
* arXiv/source-text path, if implemented
* section detection
* evidence span provenance
* source hash stability

Metadata consensus:

* all A/B/E mock clients receive same document hash and policy hash
* extracted claims require evidence spans
* unsupported claim rejected
* duplicate claim merged
* manual claim file works only as debug override

Evaluation:

* Worker A and B return complete assessment states
* Evaluator E scores complete candidate only
* deterministic validators run before final reward is sent to TreeQuest
* reward is capped by hard failure
* final-truth language blocks readiness
* faithfulness review is not_assessed, not retired
* no hardcoded six-debt assumption outside policy fixtures

Artifacts:

* policy snapshot written
* policy IR written
* evidence ledger written
* claim ledger written
* no API key leakage
* generated report says automated candidate assessment
* report says faithfulness review was not performed

Integration:

* `go test ./...`
* `go test -race ./...`
* mock run with paper + policy completes
* output bundle includes document, policy, claims, tree, report, run metadata

## Required implementation-plan structure

Return the implementation plan in this exact structure:

1. Verdict and scope
2. Architecture summary
3. Package layout
4. Data model changes
5. PolicyBundle and PolicyIR implementation
6. Policy markdown compiler design
7. EvaluationProfile design
8. Document ingestion design
9. Evidence span and source provenance design
10. MetadataConsensus design
11. Claim extraction and deduplication design
12. A/B/E prompt-context design
13. Per-claim TreeQuest evaluation flow
14. Policy-driven validator design
15. Policy-derived RewardBuilder design
16. Faithfulness-review automated profile
17. Artifact bundle design
18. CLI changes
19. Migration from current implementation
20. Test plan
21. Risks and mitigations
22. Explicit non-goals
23. Acceptance criteria
24. EBP/PTW self-audit

## EBP/PTW self-audit requirements

In the final section classify:

* needMap
* needInvariant
* needToyCheck
* needNullModel
* needObstruction
* needFaithfulnessReview
* promotion status

Use strict language:

* This ticket may improve the evaluator architecture.
* It does not prove any paper claim.
* It does not establish source-faithful TreeQuest parity.
* It does not perform human faithfulness review.
* It does not promote EBP claims.
* It produces automated candidate assessments only.

## Review standard

Be implementation-specific. Give concrete Go types, files, functions, CLI commands, tests, migration steps, and acceptance criteria. Keep `treequest-go` generic. Keep policy as declarative data. Keep EBP 2.1 as the default policy bundle, not hardcoded business logic.
