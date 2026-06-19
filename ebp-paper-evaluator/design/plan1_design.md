## Proposed design: **Declarative Policy Paper Evaluator**

Yes — I would redesign the evaluator around a simple principle:

```text id="6jyqoh"
The user supplies:
1. a research paper
2. a declarative policy / knowledge base, usually EBP 2.1 markdown
3. optional model/provider settings

The evaluator supplies:
1. ingestion
2. claim/evidence extraction
3. multi-LLM assessment
4. deterministic policy validation
5. artifact bundle output
```

The Go code should not “know” EBP business rules directly. It should know how to load a **policy bundle**, build a **paper evidence index**, ask LLM roles to evaluate against that policy, and run generic validators derived from the policy.

`plan1.md` already points in this direction: it says users should provide arXiv/PDF/text instead of manually entering claims; claim discovery should happen before AB-MCTS; EBP should be applied per extracted claim; deterministic failures must affect the actual TreeQuest reward; and faithfulness review may be recorded as `not_assessed` rather than falsely retired. 

---

# 1. UX target

The user experience should be this simple:

```bash id="ehzfad"
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

No manual claim input should be required for normal users.

Manual claims can remain as expert/debug overrides:

```bash id="6v2s0c"
--claim-file ./debug_claims.json
```

but the main workflow should be:

```text id="t2qj64"
paper + policy markdown → automatic assessment bundle
```

---

# 2. Core architecture

```text id="78py71"
Input Layer
  paper: arXiv / URL / PDF / text
  policy: EBP 2.1 markdown
  optional profile config

        ↓

Document Ingestion Layer
  normalize paper
  split sections/chunks
  extract evidence spans
  hash source
  build retrieval index

        ↓

Policy Layer
  load policy markdown
  compile to PolicyIR
  expose policy text + structured checklist/rubric/status rules

        ↓

Metadata Consensus Layer
  all 3 LLMs ingest paper + policy
  internal agreement on claims / evidence / policy metadata
  output canonical PaperProfile

        ↓

Claim Evaluation Layer
  one TreeQuest search per claim
  Worker A constructive candidate
  Worker B adversarial candidate
  Evaluator E scores candidate against policy
  deterministic validators adjust/cap reward

        ↓

Artifact Bundle
  assessment_report.md
  claim_ledger.json
  evidence_ledger.json
  policy_snapshot.md
  policy_ir.json
  budget_usage.json
  provenance.json
  tree_snapshot.json
```

The important shift is this:

```text id="mmwhfg"
EBP is no longer hardcoded as Go business logic.
EBP is loaded as a declarative policy.
```

---

# 3. Policy-as-data design

Create a new package:

```text id="1aacw7"
ebp-paper-evaluator/pkg/policy
```

with these types:

```go id="fm0swd"
type PolicyBundle struct {
    ID          string
    Name        string
    Version     string
    SourcePath  string
    SourceHash  string

    Markdown    string
    IR          PolicyIR
}

type PolicyIR struct {
    ClaimTypes        []PolicyEnum
    FunctionClasses   []PolicyEnum
    DebtItems         []DebtItem
    StatusRules       []StatusRule
    HardFailures      []HardFailure
    RubricDimensions  []RubricDimension
    RequiredOutputs   []RequiredOutput
    ReportLanguage    ReportLanguagePolicy
}

type DebtItem struct {
    ID          string // needMap, needInvariant, etc.
    Label       string
    Description string
    Automated   bool
    Required    bool
    DefaultStatus string // remaining, retired, not_assessed, not_applicable
}

type RubricDimension struct {
    ID          string
    Description string
    Weight      float64
    EvidenceRequired bool
}

type HardFailure struct {
    ID          string
    Description string
    RewardCap float64
    BlocksReadiness bool
}
```

For EBP 2.1, the policy markdown would compile into items like:

```text id="f8lgwv"
needMap
needInvariant
needToyCheck
needNullModel
needObstruction
needFaithfulnessReview
```

But the Go code should not hardcode those names. It should only say:

```text id="m66w1s"
Load debt items from policy.
Require remaining/retired/not_assessed statuses to match policy rules.
```

That makes the evaluator generic.

---

# 4. How to make Markdown declarative without making it brittle

Use a two-layer policy file:

```text id="05wabz"
1. Human-readable Markdown policy
2. Optional embedded machine-readable policy block
```

Example:

````markdown
# Elephant Bridge Protocol v2.1

Ideas enter free. Promotion costs debt.

## Debt Checklist

- needMap
- needInvariant
- needToyCheck
- needNullModel
- needObstruction
- needFaithfulnessReview

## Automated Profile

Faithfulness review is not assessed in automated mode.

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

This keeps the policy human-readable while giving the evaluator stable machine-readable rules.

For existing EBP 2.1 markdown without structured blocks, add a one-time command:

```bash id="k5worl"
ebp-paper-evaluator compile-policy \
  --policy ./ebp_v2_1.md \
  --out ./policies/ebp_v2_1.policy.json
```

The compiled `PolicyIR` should be reviewable and pinned by hash.

---

# 5. All 3 LLMs ingest the paper

I would interpret “ingest the paper” as:

```text id="e1z121"
All three roles receive the same paper identity, source hash, section map, abstract, global summary, claim candidates, and retrieval access to evidence spans.
```

Not:

```text id="0uejj3"
Paste the full paper into every prompt every time.
```

That would be expensive and unreliable, especially for long PDFs.

Instead, build a shared `DocumentBundle`:

```go id="xn3bcb"
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

Then every LLM role gets a common run preamble:

```text id="qdnkwq"
You are evaluating this paper under this policy.

Paper:
- title
- source hash
- abstract
- section map
- global summary

Policy:
- EBP 2.1 markdown
- compiled checklist/rubric/status rules

You may only cite evidence spans supplied by the evaluator.
Do not invent claims.
Do not claim final truth.
```

For each claim, the LLM gets:

```text id="2ipkv4"
claim text
claim source quote
section/page/context
relevant nearby chunks
policy excerpt
current state
```

This satisfies “all 3 LLMs ingest the given research paper” while keeping the system practical.

---

# 6. Metadata agreement should be hidden implementation detail

The user should not care how A/B/E agree on claims. Internally, use a `MetadataConsensus` interface:

```go id="1v5ni1"
type MetadataConsensus interface {
    BuildPaperProfile(
        ctx context.Context,
        doc DocumentBundle,
        policy PolicyBundle,
        roles RoleClients,
    ) (PaperProfile, error)
}
```

Output:

```go id="6n3s33"
type PaperProfile struct {
    DocumentID string
    PolicyID   string
    Claims     []ClaimRecord
    Metadata   PolicyMetadata
}

type ClaimRecord struct {
    ID           string
    Text         string
    ClaimType    string
    FunctionClass string
    EvidenceSpans []EvidenceSpan
    Confidence   float64
    SourceHash   string
}
```

Implementation can be changed later without changing the UX.

Possible first implementation:

```text id="o6jlha"
1. Worker A extracts candidate claims from section chunks.
2. Worker B looks for missing, unsupported, duplicated, or over-broad claims.
3. Evaluator E consolidates into canonical ClaimRecord entries.
4. Deterministic verifier rejects claims with no evidence span.
5. Deduplicator merges semantically similar claims.
```

Later, you could replace that with embeddings, voting, smaller models, or deterministic extraction. The public contract remains:

```text id="6eo692"
Paper + policy → PaperProfile
```

---

# 7. Generic validator engine

Replace EBP-specific validators with policy-driven validators.

Current validator idea:

```go id="k86qp1"
ValidateDebtConsistency(state)
ValidateFinalTruth(state)
ValidateCitations(state)
```

Better generic design:

```go id="8706w9"
type Validator interface {
    Validate(ctx context.Context, input ValidationInput) ValidationResult
}

type ValidationInput struct {
    Policy PolicyBundle
    Document DocumentBundle
    Claim ClaimRecord
    Candidate AssessmentState
}

type ValidationResult struct {
    Passed bool
    Flags []ValidationFlag
    RewardCaps []RewardCap
    ReadinessBlocked bool
}
```

Then validators use `PolicyIR`:

```text id="v82el2"
DebtPartitionValidator:
  reads PolicyIR.DebtItems

ForbiddenLanguageValidator:
  reads PolicyIR.HardFailures / ReportLanguage

CitationValidator:
  reads DocumentBundle.EvidenceSpans

RequiredOutputValidator:
  reads PolicyIR.RequiredOutputs

RewardCapValidator:
  reads PolicyIR.HardFailures
```

So if tomorrow you use a different policy markdown, the validators still work.

---

# 8. Reward should be policy-rubric-derived, not arbitrary LLM score

`plan1.md` correctly warns that the evaluator should not rely too heavily on unconstrained LLM scoring and that deterministic failures should affect the actual TreeQuest reward. 

Design:

```go id="q9w270"
type EvaluatorJudgment struct {
    DimensionScores map[string]float64
    DebtDecisions   map[string]DebtDecision
    Flags           []string
    Rationale       string
}

type RewardBuilder struct{}

func (b RewardBuilder) Build(
    policy PolicyIR,
    judgment EvaluatorJudgment,
    validation ValidationResult,
) tree.Reward
```

Reward calculation:

```text id="1g75uy"
base score = weighted rubric dimensions from PolicyIR
then apply deterministic caps
then force readiness false if hard failures exist
```

Example:

```text id="uow9n2"
source_support: 0.9
map_quality: 0.7
obstruction_handling: 0.6

base = weighted average
final = min(base, all reward caps)
```

Hard failures:

```text id="07mo5j"
invented quote → reward cap 0.0
final-truth language → reward cap 0.2
invalid debt partition → reward cap 0.3
missing evidence span → reward cap 0.0
```

The LLM can judge, but policy and validators govern.

---

# 9. Faithfulness review as policy profile

Do not hardcode “five debts” versus “six debts.”

Instead, define an evaluation profile:

```go id="8t3vs3"
type EvaluationProfile struct {
    ID string
    Name string
    DebtOverrides map[string]DebtOverride
}

type DebtOverride struct {
    DefaultStatus string
    IncludedInAutomatedReadiness bool
}
```

For automated EBP:

```json id="3n1nlh"
{
  "id": "automated-no-faithfulness",
  "debt_overrides": {
    "needFaithfulnessReview": {
      "default_status": "not_assessed",
      "included_in_automated_readiness": false
    }
  }
}
```

Report language:

```text id="8bmu9r"
Automated EBP profile excluding faithfulness review.
Faithfulness review was not performed.
This is an automated candidate assessment, not full EBP promotion.
```

This matches `plan1.md`: faithfulness may be skipped, but it must be recorded as `not_assessed`, never falsely retired. 

---

# 10. Proposed package layout

```text id="0e31mp"
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

Keep `treequest-go` untouched as the generic search engine.

---

# 11. Internal flow

```text id="685w87"
Run starts
  ↓
Load policy markdown
  ↓
Compile / load PolicyIR
  ↓
Ingest paper
  ↓
All 3 LLM roles receive shared DocumentBundle + PolicyBundle
  ↓
MetadataConsensus builds PaperProfile
  ↓
For each claim:
    initialize AssessmentState from PolicyIR debt defaults
    run TreeQuest search
    Worker A/B produce complete candidate states
    Evaluator E scores against policy rubric
    deterministic validators run
    RewardBuilder derives final reward
    TreeQuest Tell receives final normalized score
  ↓
Aggregate all claim results
  ↓
Write artifact bundle
```

---

# 12. What changes from current plan

The current `plan1.md` still sounds like EBP is partly encoded into runtime behavior: five enabled debts, canonical Workbench classifications, readiness calculation, etc. The new design moves those into:

```text id="y4i1bv"
policy markdown
compiled PolicyIR
evaluation profile
```

So the evaluator becomes:

```text id="yjzx4n"
generic paper-policy evaluator
```

not:

```text id="hn13qk"
hardcoded EBP evaluator
```

EBP 2.1 becomes the default policy bundle, not the program’s hidden business logic.

---

# 13. Minimal next implementation ticket

```text id="9fdm7r"
EBP-EVAL-DECL-0001: Declarative Policy Bundle and Paper-Ingestion Refactor
```

Scope:

```text id="u4akto"
1. Add PolicyBundle and PolicyIR types.
2. Load EBP 2.1 markdown as policy input.
3. Add optional embedded policy-json block support.
4. Add EvaluationProfile for automated-no-faithfulness.
5. Move hardcoded debt checklist into policy fixture.
6. Add DocumentBundle ingestion abstraction.
7. Add MetadataConsensus interface.
8. Make A/B/E prompts receive DocumentBundle + PolicyBundle.
9. Make validators read PolicyIR instead of hardcoded EBP names.
10. Make RewardBuilder derive score from policy rubric + validator caps.
```

Acceptance tests:

```text id="5w1tt9"
- policy markdown loads and hashes
- PolicyIR contains debt items from policy, not Go constants
- automated-no-faithfulness marks needFaithfulnessReview not_assessed
- paper text/PDF input creates DocumentBundle
- all A/B/E mock clients receive same policy hash and document hash
- claim extraction rejects claims without evidence span
- reward is capped by policy hard failures
- changing policy fixture changes validator behavior without code changes
```

---

## EBP/PTW self-audit

**Claims:** The proposed redesign makes the evaluator generic by moving EBP rules into declarative policy bundles.

**needMap:** Satisfied at design level: paper + policy → document bundle + policy IR → claim assessments → artifact bundle.

**needInvariant:** Key invariant: `treequest-go` remains generic; policy-specific rules live in policy data, not core algorithm code.

**needToyCheck:** Pending. Needs mock policy, mock paper, and mock A/B/E clients.

**needNullModel:** A non-EBP sample policy should be tested to prove the evaluator is not EBP-hardcoded.

**needObstruction:** Biggest risk is pretending arbitrary markdown is safely machine-readable. Mitigation: compile to `PolicyIR`, hash it, validate it, and optionally require structured fenced blocks.

**needFaithfulnessReview:** Automated profile should mark it `not_assessed`, not retired.

**Promotion status:** `design_proposal_only`; not implemented, not validated.
