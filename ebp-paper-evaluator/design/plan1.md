 # Automatic EBP Paper-Evaluation Pipeline

  ## Summary

  The current implementation partially applies EBP 2.1, but automatic paper ingestion is not yet wired into
  the CLI and the evaluator relies too heavily on an unconstrained LLM score.

  Users should provide an arXiv identifier, URL, PDF, or text file—not manually enter claims. Claim
  discovery and classification should be a preprocessing stage before AB-MCTS.

  Faithfulness review may be skipped, but it must be recorded as not_assessed, never falsely marked retired.
  Outputs should say “automated EBP profile excluding faithfulness review,” not claim complete EBP
  promotion.

  ## Current Behavior and Gaps

  - /home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/ebp/evaluator.go asks E to classify five
    debts, detect final-truth language and bridge violations, and return a scalar score.

  - /home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/ebp/validators.go checks debt
    partitioning, citations, and final-truth phrases.

  - A and B each invoke E after producing one candidate, which correctly matches TreeQuest’s GenerateFn →
    score → Tell flow.

  - /home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/ebp/extractor.go can extract claims, but /
    home/chaschel/Documents/go/treequest/ebp-paper-evaluator/cmd/evaluate/main.go does not use it. The CLI
    still initializes one manually supplied claim.

  - A paper text file is currently placed entirely into PaperAbstract and repeated in prompts. This will
    become expensive and unreliable for full papers.

  - Extraction is limited to the “top 1–3” claims and uses classifications that do not match the canonical
    Workbench 2 taxonomy.

  - Deterministic validation runs after E supplies the reward but does not recompute that reward. A
    validator-rejected candidate can therefore retain a high TreeQuest score.

  - Documentation and mock data still mention six debts while runtime validation uses five.

  ## Implementation Changes

  ### Paper ingestion and automatic claim discovery

  Create an ingestion stage before TreeQuest:

  arXiv ID/URL, PDF, HTML, or text
      → normalized document
      → section-aware chunks
      → candidate claims
      → verification and deduplication
      → one search per claim
      → aggregate paper report

  Define document provenance explicitly:

  type Document struct {
      ID       string
      Title    string
      Abstract string
      Sections []Section
      Source   SourceMetadata
      Hash     string
  }

  type EvidenceSpan struct {
      Section   string
      Page      int
      Start     int
      End       int
      Quote     string
      SourceHash string
  }

  - Prefer arXiv HTML/source-derived text when available.
  - Use PDF extraction as a fallback; raw PDF text is often disrupted by columns, equations, headers,
    references, and footnotes.

  - Plain text remains a supported input but should pass through normalization and section detection.
  - Do not send the complete paper on every LLM call. Retrieve the claim’s source span plus nearby context
    and relevant sections.

  - Extract claims chunk-by-chunk, then run a second consolidation pass to deduplicate and verify each claim
    against an exact source span.

  - Require every extracted claim to contain a verbatim or normalized quotation and location. Reject
    unsupported invented claims.

  - Remove required --claim-* inputs. Retain them only as optional debugging overrides.

  ### Applying EBP 2.1

  For every extracted claim:

  1. Classify it using canonical Workbench 2 claim types, function classes, and maturity stages.
  2. Initialize the five enabled automated debts:
      - needMap
      - needInvariant
      - needToyCheck
      - needNullModel
      - needObstruction

  3. Let A produce a constructive complete assessment state.
  4. Let B produce an adversarial complete assessment state.
  5. Let E assess one resulting candidate at a time using the claim, evidence spans, current analyses, and
     debt history.

  6. Run deterministic checks before calculating the final TreeQuest reward.
  7. Derive reward from an explicit rubric rather than accepting E’s arbitrary scalar directly.

  Recommended score dimensions:

  - source support;
  - claim-type and function-class correctness;
  - map and invariant quality;
  - toy-check quality;
  - null-model comparison;
  - obstruction handling;
  - bridge-principle validity;
  - adversarial issues resolved;
  - uncertainty and incompleteness disclosure.

  Hard failures such as invented quotations, invalid debt state, or final-truth language must cap the final
  reward and block promotion.

  Debt retirement should require:

  - an evaluator decision;
  - cited source or analysis evidence;
  - deterministic structural validation where possible.

  ### Faithfulness-review policy

  Represent debt status explicitly:

  type DebtStatus string

  const (
      DebtRemaining   DebtStatus = "remaining"
      DebtRetired     DebtStatus = "retired"
      DebtNotApplicable DebtStatus = "not_applicable"
      DebtNotAssessed DebtStatus = "not_assessed"
  )

  Set needFaithfulnessReview to not_assessed under an automated-no-faithfulness evaluation profile.

  - Do not include it in the automated readiness calculation.
  - Do not place it in RetiredDebt.
  - Include a visible report limitation: “Faithfulness review was not performed.”
  - Rename PromotionReady to something scoped, such as AutomatedReviewReady.
  - Reserve full PromotionReady for a workflow that performs every applicable EBP obligation.

  If a claim depends on a formalization-to-physics mapping, add a flag such as
  manual_faithfulness_recommended; this does not stop the automated run.

  ## Test Plan

  - Import arXiv HTML, ordinary PDF, two-column PDF, and plain text fixtures.
  - Verify section and evidence-span provenance survives normalization.
  - Ensure users can complete an evaluation without supplying claims.
  - Reject extracted claims with no supporting source span.
  - Test claim deduplication and coverage across multiple sections.
  - Verify only canonical Workbench classifications are accepted.
  - Confirm faithfulness is not_assessed, never retired.
  - Confirm reports clearly identify the reduced automated profile.
  - Ensure deterministic failures alter the actual TreeQuest reward.
  - Verify each claim gets an independent tree and the final report aggregates all claim results without
    rewriting the original paper.

  ## Assumptions

  - Automated claim extraction is mandatory for normal users.
  - Manual claims remain an expert/debugging override only.
  - Faithfulness review is deliberately excluded from automated readiness.
  - The original paper remains immutable; the output is an assessment bundle with annotations, claim ledger,
    evidence, limitations, budget usage, and provenance.
