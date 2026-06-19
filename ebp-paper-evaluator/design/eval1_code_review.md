Excellent. I would mark this as:

```text
EBP-EVAL-SIMPLE-0004: completed_reported_validated_for_code_and_test_scope
```

with one caveat:

```text
real_provider_operational_readiness: pending smoke-test result review
```

You said the real-provider smoke test was run, but the walkthrough does not include the actual smoke-test outcome: model IDs, returned response count, parseable review count, parse statuses, bundle path, and report status. So I would not yet say “real-provider mode is accepted.” I would say the **schema-contract repair is accepted**, and the real-provider smoke output should be reviewed next.

## What is accepted

```text
PASS: zero-value ReviewerOutput{} schema replaced
PASS: populated schema example added
PASS: arrays no longer appear as null in prompt example
PASS: active policy debt IDs used in schema example
PASS: generic debt fallback avoids hardcoded EBP fallback
PASS: explicit prompt contract added
PASS: strict parsing preserved
PASS: malformed real-provider fixtures fail clearly
PASS: parseability-aware run status added
PASS: all-zero parse failures no longer look like EBP judgment
PASS: degraded parseability is reported honestly
PASS: low temperature added
PASS: JSON response_format support added
PASS: go test ./... PASS
PASS: go test -race ./... PASS
PASS: go vet ./... PASS
```

This directly fixes the core defect: the model was not given a usable output contract. The evaluator now gives a concrete example and keeps the gate strict.

## Remaining check before calling real-provider mode ready

Please inspect or report the smoke-test bundle with:

```text
model IDs
returned_response_count
parseable_review_count
call_run_status
parse_run_status
assessment_status
per-reviewer parse_status
per-reviewer score
whether report contains schema-contract warning or candidate assessment
bundle path
```

Acceptance target:

```text
parseable_review_count >= 2
assessment_status = degraded_candidate_assessment_available
or
assessment_status = candidate_assessment_available
```

If `parseable_review_count == 0`, then the code repair may still be valid, but the selected free models are not yet schema-compliant enough for this task.

## Updated status language

Use:

```text
EBP-EVAL-SIMPLE-0004 repaired the real-provider reviewer schema contract.

The evaluator now provides a populated JSON schema example, explicit array/type rules, strict first-object decoding, parseability-aware statuses, and diagnostic reporting for schema failures.

This improves real-provider parseability without weakening validation.

Real-provider operational acceptance still requires review of the smoke-test bundle and parseable-review count.
```

## Best next step

```text
EBP-EVAL-SIMPLE-0005: Real-Provider Smoke Artifact Review
```

Purpose:

```text
Review the actual smoke-test output bundle after schema-contract hardening, confirm parseable_review_count, model reliability, artifact safety, report wording, and whether at least 2 of 3 reviewers produce parseable candidate assessments.
```

## EBP/PTW self-audit

**needMap:** Satisfied. The broken bridge was prompt schema → reviewer JSON → strict Go parser; this ticket repairs that bridge.

**needInvariant:** Satisfied. Strict parsing remains; malformed structures still fail instead of being silently accepted.

**needToyCheck:** Satisfied for test scope through observed malformed-output fixtures and no-parseability status tests.

**needNullModel:** Next useful comparison is old zero-value schema versus new populated schema on the same models.

**needObstruction:** Remaining obstruction is provider/model schema compliance under real calls.

**needFaithfulnessReview:** Still `not_assessed`.

**Promotion status:** `schema_contract_hardening_completed_reported_validated`; no physics truth, EBP promotion, human faithfulness review, or TreeQuest parity claim.
