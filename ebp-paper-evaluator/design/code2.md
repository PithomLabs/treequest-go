Implemented EBP-EVAL-DECL-0001 in ebp-paper-evaluator; treequest-go was not modified.

  Key changes:

  - Declarative policy compiler, schema validation, hashes, trusted/untrusted policy enforcement.
  - Shipped /home/chaschel/Documents/go/treequest/ebp-paper-evaluator/policies/ebp_v2_1.md and no-faithfulness
    profile.

  - Non-EBP policy fixture proving generic behavior.
  - Text, URL, arXiv and PDF ingestion; PDF requires pdftotext.
  - Verified evidence spans and automatic A/B/E metadata consensus.
  - Explicit paper prompt-injection boundary.
  - Policy-driven debt validation, readiness, reward dimensions and caps.
  - One unchanged TreeQuest search per claim.
  - New run and compile-policy CLI commands.
  - Hierarchical assessment bundle with document/policy hashes, evidence, claims, trees, prompt ledger, budget
    and provenance.

  - Removed the obsolete hardcoded pkg/ebp execution path.

  Validation completed:

  go test ./...       PASS
  go test -race ./... PASS
  mock end-to-end run PASS

  Mock artifacts were generated at /tmp/ebp-eval-decl-0001-audit-2.

  Status: CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED.
