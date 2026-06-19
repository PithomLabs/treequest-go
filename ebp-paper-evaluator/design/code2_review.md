Excellent. I would update the project status to:

```text
EBP-EVAL-DECL-0001: completed_reported_validated
status: code_accepted_for_reported_declarative_policy_mock_scope
```

with these boundaries still active:

```text
No source-faithful Sakana TreeQuest parity claim.
No human faithfulness review claim.
No scientific/paper claim promotion.
No real-provider production reliability claim unless separately smoke-tested.
```

## Accepted based on your report

The implementation now appears to satisfy the core design goal:

```text
paper + declarative policy
→ document bundle
→ policy IR
→ metadata consensus
→ per-claim TreeQuest searches
→ policy-driven validation/reward
→ candidate assessment bundle
```

The most important wins are:

```text
PASS: treequest-go unchanged
PASS: EBP rules moved into declarative policy bundle
PASS: trusted/untrusted policy boundary added
PASS: non-EBP fixture proves generic-policy behavior
PASS: paper prompt-injection boundary added
PASS: all A/B/E roles share paper/policy context
PASS: claims require verified evidence spans
PASS: reward derived from policy dimensions/caps, not arbitrary E score
PASS: automated-no-faithfulness profile preserved
PASS: hardcoded pkg/ebp execution path removed
PASS: go test ./... and go test -race ./... reported passing
PASS: mock end-to-end run generated artifacts
```

## Recommended next gate

The next ticket should be:

```text
EBP-EVAL-DECL-0002: Post-Implementation Adversarial Review and Artifact Audit
```

Scope:

```text
1. Review policy compiler and fail-closed behavior.
2. Confirm no EBP debt IDs remain hardcoded in generic eval logic.
3. Inspect generated artifact bundle for hashes, secrets, limitation language, and source immutability.
4. Verify A/B/E prompts include document hash, policy hash, profile ID, and prompt-injection boundary.
5. Verify non-EBP policy fixture actually changes validator/reward/report behavior without code changes.
6. Confirm pdftotext dependency is detected cleanly and failure is user-friendly.
7. Run one tiny real OpenRouter smoke test only after mock/artifact audit passes.
```

## Status language to use now

```text
EBP-EVAL-DECL-0001 is completed and reported validated.

The evaluator is reaccepted for the current declarative-policy, mock end-to-end scope.

This means the app now supports paper ingestion, policy compilation, policy-driven evaluation, evidence-backed claim extraction, per-claim TreeQuest searches, and artifact-bundle generation under the automated no-faithfulness profile.

This does not prove any research-paper claim, does not perform human faithfulness review, and does not establish full TreeQuest source parity.
```

## EBP/PTW self-audit

**Claims:** Accepted for reported implementation scope only.

**needMap:** Satisfied. Paper input, policy bundle, metadata consensus, per-claim search, validation, reward, and artifacts are mapped into concrete implementation pieces.

**needInvariant:** Satisfied for reported scope: `treequest-go` unchanged, source bytes immutable, policy hashes tracked, evidence spans verified, and EBP rules declarative.

**needToyCheck:** Satisfied by reported mock end-to-end run and race-tested Go suite.

**needNullModel:** Improved: the non-EBP policy fixture is the key null model against EBP-hardcoding.

**needObstruction:** Remaining risks are independent code review, artifact inspection, real-provider behavior, PDF extraction edge cases, and prompt-injection robustness under hostile papers.

**needFaithfulnessReview:** Correctly `not_assessed` under automated profile.

**Promotion status:** `CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED`; implementation accepted for current automated scope, not scientific or EBP promotion.
