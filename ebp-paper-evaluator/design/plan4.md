  # EBP-EVAL-SIMPLE-0001: Simple Triple Review

  ## Summary

  Add triple-review to ebp-paper-evaluator as an independent, concurrent three-
  reviewer pipeline. It will reuse local document ingestion, policy/profile
  loading, LLM clients, strict JSON extraction, budget tracking, and artifact
  safety while importing or invoking no TreeQuest APIs.

  ## Implementation Changes

  - Add pkg/simple orchestration, prompt construction, strict reviewer types,
    deterministic scoring, lexical agreement analysis, reporting, and artifact
    writing.

  - Extend the CLI with:
      - --paper, --input-root, --max-paper-bytes
      - --policy, --profile, --allow-untrusted-policy
      - --provider, --models, --mock
      - --timeout, --out, --include-local-paths, --copy-source

  - Non-mock mode requires exactly three distinct, non-empty comma-separated model
    IDs. Mock mode uses reviewer slots reviewer_1..3 mapped to mock_reviewer_1..3.

  - Launch one goroutine per reviewer under a shared timeout. Collect
    independently, then sort all results and prompt-ledger records by reviewer ID
    before writing.

  - Give every reviewer the identical user message: the exact one-line instruction
    followed by the same HTML-escaped <untrusted_paper> payload. Put reviewer
    identity, trusted policy/profile, schema, and safety rules in system/metadata.

  - Strictly decode the first balanced JSON object with unknown fields rejected.
    Validate expected reviewer/model IDs, required fields, candidate status,
    score-independent schema, and limitation language.

  - Preserve malformed content as raw with review_parse_failed. Preserve provider
    failures as sanitized reviewer_call_failed records.

  - Run statuses:
      - 3 returned responses: triple_review_complete
      - 2: partial_triple_review_two_reviewers
      - 1: partial_review_insufficient_for_agreement
      - 0: return run_failed_no_reviewer_content and do not emit a completed
        assessment bundle.

  ## Scoring and Agreement

  - Compute, never accept from the LLM, nine bounded scores:
      - claim_coverage: reviewer claim clusters divided by the combined parseable
        claim clusters.

      - source_grounding: proportion of claims containing at least one exact
        normalized-paper quote.

      - ebp_debt_coverage: recognized required automated policy debts covered by
        the response.

      - map_invariant_quality: structural completeness of nonblank map and
        invariant findings.

      - toy_null_obstruction_awareness: mean coverage of toy checks, null models,
        and obstructions.

      - faithfulness_humility: explicit limits plus required no-human-faithfulness
        language.

      - no_overclaim_discipline: absence of policy hard-failure/forbidden
        affirmative language.

      - cross_model_agreement: mean pairwise claim-set similarity; zero when fewer
        than two parseable reviews exist.

      - next_step_usefulness: bounded nonblank, debt-relevant next steps relative
        to identified claims.

  - Use the equal-weight mean for final_score; malformed or failed reviews receive
    0.0 with failure status, kept distinct from run completeness.

  - Normalize claim text by case-folding, punctuation removal, whitespace
    collapse, and stable tokenization. Match exact normalized claims or token
    Jaccard similarity of at least 0.60.

  - Build deterministic claim clusters and compute shared claims, reviewer-unique
    claims, shared/unique debts, shared obstructions, mean pairwise agreement,
    weakly grounded claims, and a disagreement ledger.

  - Flag conflicts only when substantially overlapping claims have opposing
    explicit negation polarity; do not imply broader semantic contradiction
    detection.

  - Mark agreement as full, partial, insufficient, or unavailable according to
    parseable-review count.

  ## Artifacts and Interfaces

  - Add public simple.Runner, configuration/result types, injectable LLM client,
    clock, and artifact writer dependencies so CLI and concurrency behavior are
    testable.

  - Produce the requested source/, policy/, reviews/, consensus/, report/, and
    run/ layout. Parsed files exist only for valid parses; malformed responses
    retain raw and score files; call failures produce reviewer_N_error.json.

  - Include sanitized provenance with mode: "simple_triple_review",
    treequest_used: false, model mapping, hashes, profile, run/comparison status,
    and per-reviewer outcomes.

  - Keep faithfulness not_assessed, include all required limitation sentences
    verbatim, apply policy forbidden-language checks, redact paths by default, and
    reject credential-like artifact content.

  - Use a fixed injected clock in determinism tests so reviewer completion order
    cannot change the bundle digest.

  - Add testdata/papers/local_physics_paper.txt; preserve existing fixtures and
    legacy run behavior.

  ## Test and Validation Plan

  - Add the requested CLI, prompt-boundary, parsing, scoring, agreement, artifact,
    and regression tests.

  - Add concurrent completion, deterministic reviewer ordering, isolated reviewer
    failure, completion-order-independent bundle digest, per-reviewer budget
    accounting, identical user messages, reviewer identity exclusion from user
    content, and 3/2/1/0 completion tests.

  - Verify pkg/simple has no TreeQuest import or API surface and provenance always
    records treequest_used: false.

  - Run:
      - go test ./...
      - go test -race ./...
      - go vet ./...
      - Mock EBP triple review
      - Mock non-EBP triple review

  - Preserve the existing dirty worktree and do not modify treequest-go or
    unrelated user changes.

  ## Assumptions

  - “Independent” means distinct configured model IDs and independent concurrent
    requests.

  - Non-EBP policies remain supported; policy debts drive debt coverage while the
    fixed nine comparison metrics remain available.

  - No embeddings, external semantic service, scientific-truth scoring, EBP
    promotion, or human faithfulness assessment are introduced.
