 # EBP-EVAL-DECL-0001: Declarative Policy Bundle and Paper-Ingestion Refactor

  ## 1. Verdict and scope

  Implement this ticket entirely in /home/chaschel/Documents/go/treequest/ebp-paper-evaluator.

  Do not change treequest-go. Continue using its existing generic GenerateFn, AB-MCTS-A state, Ask/AskBatch/Tell, score, and snapshot interfaces.

  The ticket replaces the current hardcoded, single-claim EBP workflow with:

  paper + declarative policy
  → normalized document
  → policy compilation
  → automatic metadata consensus
  → per-claim TreeQuest searches
  → policy-driven validation and reward
  → aggregate candidate assessment bundle

  EBP 2.1 becomes the shipped default policy bundle. Go code implements generic policy mechanics, document processing, orchestration, validation primitives, and artifact persistence—not EBP debt names or readiness rules.

  ## 2. Architecture summary

  Use these boundaries:

  CLI
   ├── document ingestion
   ├── policy loading/compilation
   ├── metadata consensus
   ├── per-claim evaluation orchestration
   │    ├── Worker A
   │    ├── Worker B
   │    ├── Evaluator E
   │    ├── generic validators
   │    ├── policy-derived reward
   │    └── unchanged treequest-go
   └── artifact writer

  Core invariants:

  - The original source bytes are never modified.
  - Every claim references verified evidence spans from the normalized document.
  - All A/B/E calls include the same document hash, policy source hash, compiled IR hash, profile ID, and claim context.
  - Workers produce complete candidate states, not standalone fragments.
  - Evaluator E returns structured judgments, not the TreeQuest reward.
  - Generic validators run before Tell.
  - RewardBuilder derives the normalized reward from policy rubric weights and validator caps.
  - Domain metadata remains opaque to treequest-go.

  ## 3. Package layout

  ebp-paper-evaluator/
    cmd/
      ebp-paper-evaluator/
        main.go
        run.go
        compile_policy.go

    policies/
      ebp_v2_1.md
      ebp_v2_1.policy.json
      profiles/
        automated-no-faithfulness.json

    pkg/
      document/
        types.go
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
        canonical_json.go

      metadata/
        types.go
        extract_claims.go
        consensus.go
        dedupe.go
        verify_spans.go
        manual_claims.go

      eval/
        state.go
        roles.go
        prompt_context.go
        orchestrator.go
        evaluator.go
        reward.go
        validators.go
        validator_debt.go
        validator_language.go
        validator_evidence.go
        validator_outputs.go
        validator_profile.go

      llm/
        client.go
        openrouter.go
        mock.go
        recorded.go

      artifact/
        bundle.go
        report.go
        provenance.go
        sanitize.go

      budget/
        tracker.go
        client.go

      runctx/
        context.go
        prompt_log.go

  Remove the current pkg/ebp package after its responsibilities have migrated. Do not create an EBP-specific replacement Go package.

  ## 4. Data model changes

  ### Document model

  type DocumentBundle struct {
        ID       string         `json:"id"`
        Title    string         `json:"title"`
        Abstract string         `json:"abstract"`
        Sections []Section      `json:"sections"`
        Chunks   []Chunk        `json:"chunks"`
        Source   SourceMetadata `json:"source"`
        Hash     string         `json:"hash"`
        Summary  string         `json:"summary"`
  }

  type Section struct {
        ID         string      `json:"id"`
        Title      string      `json:"title"`
        Ordinal    int         `json:"ordinal"`
        Text       string      `json:"text"`
        Pages      []int       `json:"pages,omitempty"`
        SourceRefs []SourceRef `json:"source_refs"`
  }

  type Chunk struct {
        ID         string   `json:"id"`
        SectionID  string   `json:"section_id"`
        Text       string   `json:"text"`
        Start      int      `json:"start"`
        End        int      `json:"end"`
        Pages      []int    `json:"pages,omitempty"`
        TokenCount int      `json:"token_count"`
        SourceHash string   `json:"source_hash"`
  }

  type SourceMetadata struct {
        Kind        string `json:"kind"`
        Input       string `json:"input"`
        ResolvedURL string `json:"resolved_url,omitempty"`
        ArXivID     string `json:"arxiv_id,omitempty"`
        MediaType   string `json:"media_type"`
        OriginalHash string `json:"original_hash"`
        RetrievedAt string `json:"retrieved_at,omitempty"`
  }

  type EvidenceSpan struct {
        ID         string `json:"id"`
        Section    string `json:"section"`
        Page       int    `json:"page,omitempty"`
        Start      int    `json:"start"`
        End        int    `json:"end"`
        Quote      string `json:"quote"`
        SourceHash string `json:"source_hash"`
  }

  Start and End are UTF-8 byte offsets within normalized section text. Page is zero when the source format has no page mapping.

  ### Policy model

  type PolicyBundle struct {
        ID         string   `json:"id"`
        Name       string   `json:"name"`
        Version    string   `json:"version"`
        SourcePath string   `json:"source_path"`
        SourceHash string   `json:"source_hash"`
        IRHash     string   `json:"ir_hash"`
        Markdown   string   `json:"markdown,omitempty"`
        IR         PolicyIR `json:"ir"`
  }

  type PolicyIR struct {
        ClaimTypes       []PolicyEnum       `json:"claim_types"`
        FunctionClasses  []PolicyEnum       `json:"function_classes"`
        MaturityStages   []PolicyEnum       `json:"maturity_stages"`
        DebtItems        []DebtItem         `json:"debt_items"`
        StatusRules      []StatusRule       `json:"status_rules"`
        HardFailures     []HardFailure      `json:"hard_failures"`
        RubricDimensions []RubricDimension  `json:"rubric_dimensions"`
        RequiredOutputs  []RequiredOutput   `json:"required_outputs"`
        RoleInstructions RoleInstructions   `json:"role_instructions"`
        ReportLanguage   ReportLanguagePolicy `json:"report_language"`
  }

  ### Debt state

  type DebtItem struct {
        ID            string     `json:"id"`
        Label         string     `json:"label"`
        Description   string     `json:"description"`
        Automated     bool       `json:"automated"`
        Required      bool       `json:"required"`
        DefaultStatus DebtStatus `json:"default_status"`
  }

  type DebtStatus string

  const (
        DebtRemaining     DebtStatus = "remaining"
        DebtRetired       DebtStatus = "retired"
        DebtNotApplicable DebtStatus = "not_applicable"
        DebtNotAssessed   DebtStatus = "not_assessed"
  )

  type DebtDecision struct {
        ItemID       string         `json:"item_id"`
        Status       DebtStatus     `json:"status"`
        Rationale    string         `json:"rationale"`
        EvidenceSpan []string       `json:"evidence_span_ids"`
        Confidence   float64        `json:"confidence"`
        DecidedBy    string         `json:"decided_by"`
  }

  ### Paper metadata

  type MetadataConsensus interface {
        BuildPaperProfile(
                ctx context.Context,
                doc document.DocumentBundle,
                policy policy.PolicyBundle,
                roles RoleClients,
        ) (PaperProfile, error)
  }

  type PaperProfile struct {
        DocumentID string         `json:"document_id"`
        PolicyID   string         `json:"policy_id"`
        Claims     []ClaimRecord  `json:"claims"`
        Metadata   map[string]any `json:"metadata,omitempty"`
  }

  type ClaimRecord struct {
        ID            string         `json:"id"`
        Text          string         `json:"text"`
        ClaimType     string         `json:"claim_type"`
        FunctionClass string         `json:"function_class"`
        MaturityStage string         `json:"maturity_stage"`
        EvidenceSpans []EvidenceSpan `json:"evidence_spans"`
        Confidence    float64        `json:"confidence"`
        SourceHash    string         `json:"source_hash"`
  }

  ### Assessment state

  type AssessmentState struct {
        DocumentID           string                    `json:"document_id"`
        DocumentHash         string                    `json:"document_hash"`
        PolicyID             string                    `json:"policy_id"`
        PolicySourceHash     string                    `json:"policy_source_hash"`
        PolicyIRHash         string                    `json:"policy_ir_hash"`
        ProfileID            string                    `json:"profile_id"`
        Claim                metadata.ClaimRecord      `json:"claim"`
        Debts                map[string]DebtDecision   `json:"debts"`
        ConstructiveAnalysis RoleAnalysis              `json:"constructive_analysis"`
        AdversarialAnalysis  RoleAnalysis              `json:"adversarial_analysis"`
        EvaluatorJudgment    EvaluatorJudgment         `json:"evaluator_judgment"`
        Validation           ValidationResult          `json:"validation"`
        Reward               float64                   `json:"reward"`
        AutomatedReviewReady bool                      `json:"automated_review_ready"`
        GenerationDepth      int                       `json:"generation_depth"`
        BudgetExhausted      bool                      `json:"budget_exhausted"`
  }

  type EvaluatorJudgment struct {
        DimensionScores map[string]float64       `json:"dimension_scores"`
        DebtDecisions   map[string]DebtDecision `json:"debt_decisions"`
        Flags           []string                `json:"flags"`
        Rationale       string                  `json:"rationale"`
  }

  ## 5. PolicyBundle and PolicyIR implementation

  policy.LoadBundle performs:

  1. Read Markdown without modifying it.
  2. Compute SHA-256 over the exact source bytes.
  3. Parse policy metadata and optional policy-json.
  4. Load a compiled sidecar when configured.
  5. Validate the IR.
  6. Canonicalize IR JSON.
  7. Compute SHA-256 over canonical IR JSON.
  8. Return an immutable PolicyBundle.

  API:

  func LoadBundle(path string, opts LoadOptions) (PolicyBundle, error)
  func ValidateIR(ir PolicyIR) error
  func CanonicalIR(ir PolicyIR) ([]byte, error)
  func HashBytes(data []byte) string

  Executable runs require an IR. It may come from:

  - an embedded policy-json fence; or
  - an explicitly supplied or adjacent compiled .policy.json.

  Raw Markdown remains available as human-readable policy knowledge and prompt context. A prose-only policy without executable IR may be inspected but cannot run an evaluation.

  No EBP debt IDs, forbidden phrases, dimensions, status rules, or report statements are defined in Go constants.

  ## 6. Policy markdown compiler design

  compile-policy uses a deterministic compiler:

  ebp-paper-evaluator compile-policy \
    --policy ./policies/ebp_v2_1.md \
    --out ./policies/ebp_v2_1.policy.json

  Compilation steps:

  1. Read Markdown bytes.
  2. Locate exactly zero or one policy-json fenced block.
  3. Reject multiple blocks.
  4. Decode with json.Decoder.DisallowUnknownFields.
  5. Validate references and constraints.
  6. Canonicalize the IR.
  7. Write a compiled envelope containing source hash, IR hash, compiler schema version, and IR.
  8. Write atomically through a temporary file and rename.

  type CompiledPolicy struct {
        SchemaVersion string   `json:"schema_version"`
        SourceHash    string   `json:"source_hash"`
        IRHash        string   `json:"ir_hash"`
        IR            PolicyIR `json:"ir"`
  }

  Validation rules:

  - IDs are non-empty and unique within their namespace.
  - Debt defaults use known DebtStatus values.
  - Rubric weights are finite, non-negative, and sum to 1.0 ± 1e-9.
  - Reward caps are finite and within [0,1].
  - Status rules reference declared debts and outputs.
  - Required outputs have unique JSON field names and declared types.
  - Regex-based language rules compile successfully.
  - Role instructions exist for metadata A, metadata B, metadata E, Worker A, Worker B, and Evaluator E.
  - Report-required statements are non-empty.
  - Compiled source hash must match the loaded Markdown.

  The shipped EBP Markdown contains the executable block. Compiled output is committed for review and reproducible runs.

  ## 7. EvaluationProfile design

  type EvaluationProfile struct {
        ID            string                  `json:"id"`
        Name          string                  `json:"name"`
        Description   string                  `json:"description"`
        DebtOverrides map[string]DebtOverride `json:"debt_overrides"`
        StatusLabel   string                  `json:"status_label"`
        RequiredNotes []string                `json:"required_notes"`
  }

  type DebtOverride struct {
        DefaultStatus                DebtStatus `json:"default_status"`
        IncludedInAutomatedReadiness bool       `json:"included_in_automated_readiness"`
  }

  policy.ApplyProfile validates every override against an existing debt ID and returns an effective policy view:

  type EffectivePolicy struct {
        Bundle  PolicyBundle
        Profile EvaluationProfile
        Debts   map[string]EffectiveDebtRule
  }

  Profiles cannot remove a debt record. They may change its initial status and whether it participates in scoped automated readiness.

  Unknown profiles, unknown debt IDs, or invalid overrides fail before any model call.

  ## 8. Document ingestion design

  Expose one input contract:

  type Ingestor interface {
        Supports(input PaperInput) bool
        Ingest(ctx context.Context, input PaperInput) (DocumentBundle, error)
  }

  type PaperInput struct {
        Value string
        Kind  InputKind // auto, arxiv, url, pdf, text
  }

  document.Ingest dispatches in this order:

  1. Explicit arXiv ID.
  2. arXiv URL.
  3. HTTP/HTTPS URL.
  4. Local PDF by media sniffing.
  5. Local UTF-8 text or Markdown.

  For arXiv:

  - Normalize versioned and unversioned IDs.
  - Prefer official arXiv HTML/source-derived text when available.
  - Fall back to the arXiv PDF.
  - Record resolved URL, arXiv ID, retrieval timestamp, and byte hash.
  - Enforce response-size, redirect, timeout, and content-type limits.

  For PDF:

  - Use a page-aware Go PDF extraction adapter.
  - Preserve page numbers during extraction.
  - Detect extraction failure or implausibly empty output.
  - Return a typed error suggesting text/HTML input when the document is scanned or unsupported.
  - OCR is outside this ticket.

  For text:

  - Validate UTF-8.
  - Preserve original bytes for hashing.
  - Detect headings and abstract where possible.
  - Use stable fallback section names when headings are absent.

  Normalization:

  - Normalize line endings and Unicode compatibility characters conservatively.
  - Join PDF line wraps without merging paragraphs.
  - remove repeated page headers/footers only when detected across multiple pages;
  - preserve equations as text tokens rather than deleting them;
  - retain a normalized-to-source page mapping.

  Chunking:

  - Chunk within section boundaries.
  - Prefer paragraph boundaries.
  - Apply configurable token targets and overlap.
  - Never allow a chunk to reference multiple source hashes.
  - Generate stable chunk IDs from source hash, section ID, and offsets.

  ## 9. Evidence span and source provenance design

  Evidence verification is deterministic:

  func VerifySpan(doc DocumentBundle, span EvidenceSpan) error
  func ResolveNormalizedQuote(doc DocumentBundle, quote string, hint SpanHint) ([]EvidenceSpan, error)

  Rules:

  - SourceHash must equal DocumentBundle.Hash.
  - Section must exist.
  - Offsets must be within section text.
  - section.Text[start:end] must equal Quote.
  - A normalized quote may be resolved only when the match is unique within the hinted section/page.
  - Ambiguous matches are rejected.
  - Page metadata must agree with the section source map when available.
  - Span IDs are hashes of document hash, section ID, start, end, and quote.

  Maintain document/evidence_ledger.json with every accepted span and its referencing claim IDs.

  The source artifact contains either:

  - an immutable byte-for-byte copy under source/original.<ext> when --copy-source is enabled; or
  - source/source_ref.json containing the original location, retrieval metadata, media type, and hashes.

  ## 10. MetadataConsensus design

  Implement metadata.ConsensusV1 behind the required interface.

  type ConsensusV1 struct {
        ExtractorA llm.LLMClient
        CriticB    llm.LLMClient
        ResolverE  llm.LLMClient
        Verifier   SpanVerifier
        Deduper    Deduper
  }

  Flow:

  1. Worker A receives policy context and batches of section-aware chunks, then emits CandidateClaim records.
  2. Worker B receives the same document and policy hashes, source context, and A candidates; it marks unsupported, duplicated, missing, or overbroad candidates and may add candidates.
  3. Evaluator E receives verified candidate evidence and consolidates canonical records.
  4. VerifySpans rejects records without valid source evidence.
  5. Deduper merges equivalent records.
  6. Stable claim IDs are assigned from document hash plus canonical claim text and primary span.
  7. Return PaperProfile.

  All three metadata calls are logged with role, model ID, prompt hash, document hash, policy hashes, chunks/spans provided, token usage, and finish reason.

  The public CLI never exposes this protocol. Its contract remains Paper + Policy → PaperProfile.

  ## 11. Claim extraction and deduplication design

  type CandidateClaim struct {
        Text              string         `json:"text"`
        ClaimType         string         `json:"claim_type"`
        FunctionClass     string         `json:"function_class"`
        MaturityStage     string         `json:"maturity_stage"`
        EvidenceProposals []SpanProposal `json:"evidence_proposals"`
        Confidence        float64        `json:"confidence"`
        SourceRole        string         `json:"source_role"`
  }

  Extraction rules:

  - Claim classifications must reference IDs declared in PolicyIR.
  - Claims require at least one verified span.
  - Metadata E cannot introduce an uncited claim.
  - Unsupported candidates are retained only in metadata diagnostics, not PaperProfile.Claims.
  - Manual claim files pass through the same span verification and policy-enum validation.
  - Manual claims without evidence fail unless --debug-allow-unverified-claims is explicitly set; such runs cannot become automated-review-ready.

  Deduplication uses two passes:

  1. Deterministic normalization and token similarity to merge exact and near-exact duplicates.
  2. Evaluator E’s declared merge groups, accepted only when their evidence spans overlap or their normalized token similarity exceeds a configured policy threshold.

  Merged claims retain all supporting spans and source candidate IDs. Conflicting classifications produce a metadata flag and require E to select a declared policy value.

  ## 12. A/B/E prompt-context design

  type SharedRunContext struct {
        DocumentID       string
        DocumentHash     string
        Title            string
        Abstract         string
        SectionMap       []SectionSummary
        GlobalSummary    string
        PolicyID         string
        PolicySourceHash string
        PolicyIRHash     string
        PolicyMarkdown   string
        PolicyIR         PolicyPromptView
        Profile          policy.EvaluationProfile
  }

  type ClaimPromptContext struct {
        Shared         SharedRunContext
        Claim          metadata.ClaimRecord
        RelevantSpans []document.EvidenceSpan
        NearbyChunks  []document.Chunk
        DebtHistory   map[string]DebtDecision
        CurrentState  AssessmentState
  }

  Every A/B/E request includes:

  - paper title, hash, abstract, section map, and summary;
  - claim evidence spans and nearby context;
  - policy source and IR hashes;
  - relevant policy prose excerpts;
  - executable checklist, rubric, statuses, and output schema;
  - active profile and required limitation statements.

  Do not repeatedly paste the complete paper. The role receives a structured paper view derived from the same DocumentBundle; this satisfies paper ingestion while keeping prompts bounded.

  Policy Markdown is indexed by heading during compilation. Prompt construction selects:

  - global doctrine;
  - claim classification rules;
  - relevant debt/rubric sections;
  - report-language requirements.

  Role instructions come from PolicyIR.RoleInstructions, not Go string literals.

  ## 13. Per-claim TreeQuest evaluation flow

  For each ClaimRecord:

  1. eval.NewInitialState creates all debt decisions from the effective policy.
  2. Create a new AB-MCTS-A tree using the existing generic API.
  3. Register opaque action labels constructive and adversarial.
  4. The selected action clones the complete parent state.
  5. Worker A or B receives ClaimPromptContext and returns a complete structured candidate.
  6. Evaluator E assesses that complete candidate and returns EvaluatorJudgment.
  7. Generic validators produce ValidationResult.
  8. RewardBuilder computes the normalized reward.
  9. Readiness is computed from policy status rules and profile participation.
  10. Call TreeQuest Tell through the existing generation interface with only the complete state and normalized reward.
  11. Persist evaluator judgment, validation, reward derivation, and prompt provenance in application artifacts.
  12. Continue until per-claim iterations, depth, token/cost budget, or policy readiness stop condition is reached.
  13. Store the best candidate and tree snapshot for that claim.

  Run claims sequentially initially for deterministic budget attribution. Add bounded claim-level concurrency later without changing public interfaces.

  ## 14. Policy-driven validator design

  type Validator interface {
        ID() string
        Validate(ctx context.Context, in ValidationInput) ValidationResult
  }

  type ValidationInput struct {
        Policy    policy.PolicyBundle
        Profile   policy.EvaluationProfile
        Document  document.DocumentBundle
        Claim     metadata.ClaimRecord
        Candidate AssessmentState
  }

  type ValidationResult struct {
        Passed           bool             `json:"passed"`
        Flags            []ValidationFlag `json:"flags"`
        RewardCaps       []RewardCap      `json:"reward_caps"`
        ReadinessBlocked bool             `json:"readiness_blocked"`
  }

  Generic validators:

  - DebtPartitionValidator: uses declared debt IDs and allowed statuses.
  - ForbiddenLanguageValidator: reads policy patterns and target fields.
  - EvidenceSpanValidator: verifies claim and decision evidence against the document.
  - RequiredOutputValidator: checks fields declared in RequiredOutputs.
  - ProfileStatusValidator: enforces profile defaults and readiness inclusion.
  - EnumValidator: checks claim metadata against policy enums.
  - MalformedJudgmentValidator: rejects missing, unknown, non-finite, or out-of-range rubric scores.
  - HardFailureValidator: maps generic flags to policy hard failures and reward caps.

  No validator contains needMap, needFaithfulnessReview, or a fixed debt count.

  Validation flag IDs are declared in policy or generic structural namespaces. Unknown model-returned flags remain advisory and cannot trigger a reward cap unless mapped by policy.

  ## 15. Policy-derived RewardBuilder design

  type RewardBuilder struct{}

  type RewardBreakdown struct {
        Dimensions  map[string]float64 `json:"dimensions"`
        WeightedBase float64           `json:"weighted_base"`
        AppliedCaps []RewardCap        `json:"applied_caps"`
        DerivedReward float64          `json:"derived_reward"`
  }

  func (RewardBuilder) Build(
        ir policy.PolicyIR,
        judgment EvaluatorJudgment,
        validation ValidationResult,
  ) (RewardBreakdown, error)

  Algorithm:

  weighted base = Σ(dimension score × policy weight)
  derived reward = min(weighted base, every applicable validator cap)

  Rules:

  - All required dimensions must be present exactly once.
  - Unknown dimensions fail judgment validation.
  - Scores must be finite and within [0,1].
  - Weights come exclusively from policy.
  - Caps come from validated hard-failure definitions.
  - Missing required output or malformed judgment uses the policy-declared cap.
  - The returned TreeQuest score is DerivedReward.
  - Evaluator E never returns or controls the TreeQuest reward directly.

  If validation itself cannot complete safely, do not call Tell; mark the trial failed in application orchestration and record the failure.

  ## 16. Faithfulness-review automated profile

  The shipped automated-no-faithfulness profile contains:

  {
    "id": "automated-no-faithfulness",
    "name": "Automated profile excluding faithfulness review",
    "debt_overrides": {
      "needFaithfulnessReview": {
        "default_status": "not_assessed",
        "included_in_automated_readiness": false
      }
    },
    "status_label": "CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED",
    "required_notes": [
      "Automated EBP profile excluding faithfulness review.",
      "Faithfulness review was not performed.",
      "This is an automated candidate assessment, not full EBP promotion."
    ]
  }

  Enforcement:

  - The debt remains present.
  - Its status starts and remains not_assessed.
  - Models cannot change it to retired under this profile.
  - It does not participate in AutomatedReviewReady.
  - PromotionReady is removed from application state and artifacts.
  - Required notes are inserted from the profile and checked by artifact tests.
  - Formalization-dependent claims may receive an advisory manual_faithfulness_recommended flag from policy.

  ## 17. Artifact bundle design

  out/audit/
    source/
      original.pdf | source_ref.json
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
      prompt_ledger.json

  provenance.json records:

  - document/source hashes;
  - policy source and IR hashes;
  - profile ID;
  - model IDs by role;
  - prompt hashes;
  - application and policy schema versions;
  - TreeQuest module version;
  - run configuration;
  - stop reason per claim;
  - timestamps;
  - manual override flags.

  Artifact writing uses restrictive permissions, atomic writes, and a staging directory renamed only after all files succeed.

  artifact.Sanitize rejects:

  - configured secret environment values;
  - common credential patterns;
  - authorization headers;
  - raw provider request headers.

  The report title and status clearly identify an automated candidate assessment. It includes the three required profile statements and never presents the generated report as the source paper.

  ## 18. CLI changes

  Replace the flat command with subcommands:

  ebp-paper-evaluator run \
    --paper ./paper.pdf \
    --policy ./policies/ebp_v2_1.md \
    --profile automated-no-faithfulness \
    --provider openrouter \
    --worker-a-model <model> \
    --worker-b-model <model> \
    --evaluator-model <model> \
    --out ./out/audit

  Debug override:

  ebp-paper-evaluator run \
    --paper ./paper.pdf \
    --policy ./policies/ebp_v2_1.md \
    --profile automated-no-faithfulness \
    --claim-file ./debug_claims.json \
    --out ./out/audit

  Compiler:

  ebp-paper-evaluator compile-policy \
    --policy ./policies/ebp_v2_1.md \
    --out ./policies/ebp_v2_1.policy.json

  Important flags:

  - --paper: required.
  - --paper-kind: optional auto|arxiv|url|pdf|text.
  - --policy: defaults to shipped EBP policy.
  - --compiled-policy: optional explicit sidecar.
  - --profile: defaults to automated-no-faithfulness.
  - --claim-file: debug override.
  - --max-claims, --iterations-per-claim, --max-depth.
  - token, call, cost, and timeout budgets.
  - --copy-source.
  - --mock and --mock-fixture.

  Remove normal-operation flags for claim text, claim type, maturity stage, function class, title, and abstract.

  ## 19. Migration from current implementation

  1. Preserve existing tests as behavioral references before moving code.
  2. Split pkg/llm/client.go into provider-neutral, OpenRouter, and mock files without changing LLMClient.
  3. Move budget tracking from pkg/ebp/budget.go into pkg/budget.
  4. Replace ClaimEvalState with policy-neutral eval.AssessmentState.
  5. Replace RemainingDebt and RetiredDebt slices with a debt-decision map keyed by PolicyIR IDs.
  6. Replace hardcoded prompts in worker_a.go, worker_b.go, and evaluator.go with generic role prompt builders driven by policy instructions.
  7. Replace RunDeterministicValidators with the validator registry.
  8. Replace evaluator scalar parsing with EvaluatorJudgment parsing.
  9. Add RewardBuilder; send only its derived score to TreeQuest.
  10. Wire the existing extractor concept into MetadataConsensus, then remove the “top 1–3” limitation and noncanonical hardcoded classifications.
  11. Replace the current behavior that stores the whole paper in PaperAbstract.
  12. Replace the single tree in main.go with one tree per verified claim.
  13. Replace SaveArtifactBundle with the hierarchical artifact writer.
  14. Replace PromotionReady with AutomatedReviewReady.
  15. Update mock responses to use policy-defined dimensions, debt decisions, and evidence-span IDs.
  16. Delete pkg/ebp only after all callers and tests migrate.
  17. Keep the root treequest-go module and its packages untouched.

  During migration, do not maintain two authoritative readiness or reward paths. Switch tests and orchestration to the policy-driven implementation before removing the old files.

  ## 20. Test plan

  ### Policy tests

  - Markdown bytes load and source hash is stable.
  - Embedded policy-json parses.
  - Compiled IR hash is stable across map ordering.
  - Duplicate debt IDs fail.
  - Unknown statuses fail.
  - Invalid, non-finite, and out-of-range caps fail.
  - Invalid rubric weights fail.
  - Missing required outputs fail.
  - Source-hash mismatch with compiled sidecar fails.
  - Profile marks faithfulness not_assessed.
  - Changing a policy fixture changes validator behavior without Go changes.

  ### Document tests

  - Plain-text ingestion.
  - UTF-8 rejection and normalization behavior.
  - PDF page-aware fallback fixture.
  - arXiv HTML/source path with an HTTP test server.
  - PDF fallback when HTML is unavailable.
  - Section detection.
  - stable source hashing;
  - stable chunk IDs;
  - evidence offset and quote verification;
  - ambiguous normalized quote rejection.

  ### Metadata tests

  - A/B/E mocks receive identical document and policy hashes.
  - Every accepted claim has a valid span.
  - Unsupported claim is rejected.
  - Duplicate claims merge and preserve spans.
  - Unknown classifications fail.
  - Manual claim file works only as an explicit override.
  - Unverified manual claims block readiness.

  ### Evaluation tests

  - A and B return complete cloned states.
  - B can operate safely when selected before an A refinement.
  - E receives one complete candidate.
  - E cannot set TreeQuest reward directly.
  - Validators run before reward construction.
  - Hard failures apply policy caps.
  - Invalid debt partitions block readiness.
  - Profile-excluded debt cannot be marked retired.
  - No fixed debt count or EBP debt ID exists in evaluation Go code.
  - Budget exhaustion remains visible.

  ### Artifact tests

  - Required hierarchy and files are written.
  - Policy Markdown and IR snapshots are present.
  - Evidence and claim ledgers are present.
  - Each claim has an assessment and tree snapshot.
  - Required automated-profile statements appear.
  - Source paper bytes remain unchanged.
  - Credential-pattern scan finds no secrets.
  - Report status is CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED when scoped criteria are met.

  ### Integration tests

  Run from both modules:

  go test ./...
  go test -race ./...

  Run a deterministic mock evaluation:

  ebp-paper-evaluator run \
    --paper ./testdata/paper.txt \
    --policy ./testdata/policy.md \
    --profile automated-no-faithfulness \
    --mock \
    --out ./test-output/audit

  Assert that document, policy, claims, tree, report, budget, prompt, and provenance artifacts exist and cross-reference matching hashes.

  ## 21. Risks and mitigations

  - Markdown cannot reliably imply executable rules: require embedded machine-readable IR or a hash-matched compiled sidecar.
  - PDF extraction damages equations or column order: prefer arXiv HTML/source, retain pages, detect weak extraction, and report typed ingestion errors.
  - Models invent source quotations: require deterministic offset-and-quote verification.
  - Context size grows with paper length: use section summaries, relevant spans, nearby chunks, and policy excerpt selection.
  - Policy becomes executable attack content: use typed JSON, strict decoding, no scripts, no templates with filesystem or process access.
  - Policy changes silently alter results: hash source and IR and snapshot both in every run.
  - A/B/E metadata diverges: resolve behind MetadataConsensus and accept only policy-valid, source-backed canonical records.
  - Model-produced high scores bias search: evaluator supplies dimensions only; deterministic RewardBuilder owns reward.
  - Profile exclusion is mistaken for debt retirement: preserve not_assessed and enforce required report language.
  - Multi-claim cost becomes unbounded: enforce maximum claims and global/per-role token, call, time, and cost budgets.
  - URL ingestion creates SSRF risk: restrict schemes, validate redirects, bound response sizes, and optionally allowlist arXiv.
  - Artifacts leak credentials: sanitize structured output and test against real environment-variable values without serializing them.

  ## 22. Explicit non-goals

  - No changes to treequest-go.
  - No AB-MCTS-M implementation.
  - No source-parity claim for the Go TreeQuest port.
  - No OCR for scanned PDFs.
  - No browser renderer for JavaScript-only paper pages.
  - No automatic human faithfulness review.
  - No rewriting or modifying the source paper.
  - No general-purpose policy scripting language.
  - No model-provider logic outside pkg/llm.
  - No scientific truth determination.
  - No EBP promotion status for automated runs.

  ## 23. Acceptance criteria

  The ticket is accepted when:

  - The desired run and compile-policy commands work.
  - A user can supply arXiv, URL, PDF, or text without entering claims.
  - Manual claims are available only through the debug override.
  - All A/B/E calls include matching document, policy source, and IR hashes.
  - EBP rules are loaded from the shipped policy bundle.
  - Changing policy fixtures changes debt, validator, reward, and report behavior without Go changes.
  - No EBP debt IDs or fixed debt count remain in generic evaluation code.
  - Every accepted claim has verified evidence provenance.
  - Each claim receives an independent TreeQuest search.
  - Workers return complete candidate states.
  - Evaluator E returns dimensions and debt judgments, not the TreeQuest reward.
  - Validators execute before RewardBuilder and Tell.
  - Faithfulness is not_assessed under the automated profile.
  - The artifact bundle matches the required hierarchy.
  - The report includes all required limitation statements.
  - Source and policy hashes appear in provenance.
  - Source bytes remain unchanged.
  - No credentials appear in artifacts.
  - go test ./... and go test -race ./... pass.
  - The mock end-to-end run produces CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED or a debt-visible scoped status without any EBP promotion statement.

  ## 24. EBP/PTW self-audit

  - needMap: Satisfied at planning level. The plan maps paper ingestion, policy compilation, metadata consensus, per-claim search, validation, reward, and artifacts to concrete packages and interfaces.
  - needInvariant: Satisfied at planning level. The preserved invariants are immutable source bytes, stable hashes, verified evidence spans, complete candidate states, policy-owned business rules, normalized TreeQuest rewards, and an
    unchanged generic TreeQuest boundary.

  - needToyCheck: Specified but not executed. The deterministic mock run and fixture tests provide the required implementation-level toy checks.
  - needNullModel: Addressed. The current hardcoded single-claim evaluator is the baseline; acceptance requires policy changes to alter behavior without Go changes.
  - needObstruction: Addressed at planning level. PDF degradation, invented evidence, policy ambiguity, context growth, SSRF, score manipulation, cost growth, and secret leakage have explicit mitigations.
  - needFaithfulnessReview: not_assessed. This ticket does not perform human faithfulness review.
  - promotion status: CANDIDATE_IMPLEMENTATION_PLAN_HUMAN_REVIEW_REQUIRED.

  This ticket may improve the evaluator architecture. It does not prove any paper claim. It does not establish source-faithful TreeQuest parity. It does not perform human faithfulness review. It does not promote EBP claims. It produces
  automated candidate assessments only.
