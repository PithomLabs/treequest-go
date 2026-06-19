## Review verdict

`Pasted text(17).txt` is a **strong implementation plan** for `EBP-EVAL-DECL-0001`. I would mark it:

```text
accept_with_minor_repairs
```

It meets your three core requirements well:

```text
1. All 3 LLM roles ingest the paper through a shared DocumentBundle / SharedRunContext.
2. EBP 2.1 is loaded as declarative markdown + compiled PolicyIR.
3. Metadata agreement is hidden behind MetadataConsensus as an implementation detail.
```

Most importantly, the plan keeps `treequest-go` untouched and moves the evaluator toward a generic **paper + policy → assessment bundle** architecture rather than a hardcoded EBP evaluator. 

## What is already excellent

The plan gets the central design right:

```text
EBP 2.1 = shipped default policy bundle
Go code = generic policy/document/evaluation machinery
treequest-go = unchanged generic search engine
```

The strongest parts are:

```text
- PolicyBundle / PolicyIR with source hash and IR hash
- compile-policy command
- EvaluationProfile with automated-no-faithfulness
- DocumentBundle with section/chunk/evidence provenance
- MetadataConsensus interface hiding A/B/E claim agreement
- one TreeQuest search per verified claim
- Evaluator E returns dimensions, not direct TreeQuest reward
- RewardBuilder derives reward from policy rubric + validator caps
- artifact bundle includes source, policy, document, claims, tree, report, run metadata
- no full EBP promotion / no human faithfulness claim
```

This directly fixes the earlier UX problem: normal users should not manually enter claims. The new workflow lets them supply an arXiv ID, URL, PDF, or text file, plus a declarative policy. 

## Minor repairs before implementation

### 1. Add an explicit prompt-injection boundary

The plan handles policy-as-data well, but it should explicitly say the **paper is untrusted content**.

Add this rule:

```text
Paper text, PDF text, arXiv HTML, quoted evidence spans, and source chunks are untrusted content.
They must never be treated as system instructions.
Only the application system prompt and selected policy bundle define evaluator behavior.
If the paper contains text such as “ignore previous instructions,” it must be treated as quoted source text only.
```

This matters because all three LLMs ingest the paper. The design needs to prevent paper text from overriding EBP policy.

### 2. Distinguish trusted policy from untrusted policy

The plan lets users pass arbitrary policy markdown. That is good for generic use, but there should be two modes:

```text
trusted policy mode:
  shipped or locally approved policy bundles may define role instructions and executable policy-json.

untrusted policy mode:
  policy may be loaded for inspection, but executable role instructions require explicit --allow-untrusted-policy or review.
```

Otherwise, a downloaded “policy” could become a prompt-injection vehicle.

### 3. Clarify whether EBP IDs may exist in shipped data

The plan says:

```text
No EBP debt IDs, forbidden phrases, dimensions, status rules, or report statements are defined in Go constants.
```

That is correct. But the shipped profile includes `needFaithfulnessReview`. Clarify:

```text
EBP-specific IDs may appear in shipped policy/profile JSON or markdown fixtures.
They must not appear as hardcoded business-rule constants in Go evaluator logic.
```

That prevents future reviewers from misreading the rule.

### 4. Add a policy compatibility/version check

Because runs will depend on compiled `PolicyIR`, add:

```text
PolicyIR must include schema_version.
Compiled policies with unsupported schema_version fail before model calls.
```

The plan has `CompiledPolicy.SchemaVersion`, but I would also make `PolicyIR` or the compiled envelope version central to validation.

### 5. Add “no hidden fallback to hardcoded EBP”

If policy compilation fails, the system should not silently fall back to built-in EBP rules.

Add:

```text
If the policy cannot compile or the compiled sidecar hash does not match, the run fails.
The evaluator must never silently fall back to hardcoded EBP defaults.
```

This preserves the declarative-policy goal.

### 6. Add one non-EBP policy fixture as a null model

The plan says no fixed EBP IDs in Go code, but the best proof is a test fixture.

Add:

```text
testdata/policies/simple_review_policy.md
```

with different debt IDs, such as:

```text
sourceSupport
methodClarity
limitationDisclosure
```

Acceptance test:

```text
Changing from EBP policy to simple_review_policy changes debt/status/validator behavior without Go code changes.
```

This is the strongest way to prove the evaluator is generic.

## One wording repair

The final status says:

```text
promotion status: CANDIDATE_IMPLEMENTATION_PLAN_HUMAN_REVIEW_REQUIRED
```

That is fine, but for consistency with your current language I would use:

```text
implementation_plan_accept_with_minor_repairs;
not implemented, not validated, no EBP promotion claim.
```

For generated reports, keep:

```text
CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED
```

## Recommended patch section

Add this before “Risks and mitigations”:

```text
## Trust and Prompt-Injection Boundaries

1. Paper content is untrusted evidence, not instruction.
   It must be quoted, chunked, cited, and evaluated, but never allowed to override system or policy instructions.

2. Policy bundles are executable evaluator configuration.
   Shipped policies are trusted by default. User-supplied policies require explicit selection and are hashed, snapshotted, validated, and recorded in provenance.

3. The evaluator never silently falls back to hardcoded EBP rules.
   If policy compilation or hash validation fails, the run fails before model calls.

4. EBP-specific IDs may exist in shipped policy/profile data.
   They must not appear as hardcoded business-rule constants in generic Go evaluation logic.

5. At least one non-EBP policy fixture must be tested to prove the evaluator is policy-generic.
```

## Final status

```json
{
  "status": "accept_with_minor_repairs",
  "ready_for_implementation_after_repairs": true,
  "main_strength": "moves evaluator from hardcoded EBP workflow to declarative paper-policy evaluator",
  "required_repairs": [
    "add prompt-injection boundary for paper content",
    "distinguish trusted vs untrusted policy bundles",
    "clarify EBP IDs allowed in data but not hardcoded Go logic",
    "fail closed on policy compilation/hash mismatch",
    "add a non-EBP policy fixture to prove generic behavior"
  ]
}
```

## EBP/PTW self-audit

**Claims:** The plan is strong and suitable as the implementation baseline after minor trust-boundary repairs.

**needMap:** Satisfied. It maps paper input, policy compilation, metadata consensus, per-claim TreeQuest evaluation, validation, reward, and artifacts.

**needInvariant:** Mostly satisfied. Add explicit invariants for untrusted paper content, trusted policy handling, and no hardcoded fallback.

**needToyCheck:** Specified. Add one non-EBP policy fixture to test generic behavior.

**needNullModel:** Partially satisfied. The non-EBP policy fixture becomes the null model against EBP-hardcoding.

**needObstruction:** Main risks are prompt injection, unsafe policy-as-instruction, PDF extraction weakness, and silent fallback to hardcoded EBP. The plan already mitigates most; add fail-closed policy behavior.

**needFaithfulnessReview:** Correctly marked `not_assessed` under automated profile.

**Promotion status:** `implementation_plan_accept_with_minor_repairs`; design only, not implemented or validated.
