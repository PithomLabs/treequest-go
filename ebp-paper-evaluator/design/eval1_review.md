## Evaluation

The diagnosis is basically correct: the infrastructure worked, but the **real-provider reviewer contract failed**. The issue is not primarily that the LLMs “do not know physics.” The issue is that the program asked them for strict JSON, but the prompt showed them a weak schema example with `null` slices, so the models guessed their own structure. The uploaded investigation found that **12 of 12 returned responses failed parsing**, mostly because models returned strings where Go expected `[]string`, arrays of objects where Go expected `[]string`, or different claim keys than `ReviewerClaim` expects. It also identified the likely root cause: `systemPrompt` marshals `ReviewerOutput{}` directly, producing a schema-like example where arrays appear as `null`, leaving the LLMs without a concrete valid JSON shape. 

So the problem is best classified as:

```text
real_provider_schema_contract_failure
```

not:

```text
physics_reasoning_failure
```

The LLMs may have produced useful EBP-style prose, but the evaluator could not use it because the JSON contract was underspecified and strict decoding rejected the output.

---

# Main problems

## 1. Empty struct schema produces bad guidance

This is the biggest issue.

```go
schema, _ := json.Marshal(ReviewerOutput{})
```

When slices are nil, the JSON example becomes:

```json
{
  "main_claims": null,
  "maps_identified": null,
  "limitations": null
}
```

To an LLM, that does not communicate:

```json
"maps_identified": ["string", "string"]
```

or:

```json
"main_claims": [
  {
    "claim_id": "claim_1",
    "claim_text": "...",
    "evidence_quotes": [
      {
        "quote": "...",
        "section_hint": "..."
      }
    ],
    "ebp_debts": ["needMap"],
    "status": "candidate_unreviewed"
  }
]
```

So the model guessed.

## 2. The schema is stricter than the natural LLM response

The models naturally returned richer objects for maps, invariants, obstructions, and debts. That is understandable. A physicist-like response may want to say:

```json
{
  "domain": "Hamiltonian configuration space",
  "codomain": "observable predictions",
  "notes": "requires formal map"
}
```

But the current schema expects:

```json
"map from Hamiltonian configuration space to observable predictions requires formal review"
```

That is fine for v0.1 simplicity, but the prompt must make it explicit.

## 3. “response returned” is being confused with “usable review”

A run with three raw responses but zero parseable reviewer outputs should not look like a successful triple review. It should be a diagnostic artifact, not a completed candidate assessment.

Add a second status axis:

```json
{
  "call_status": "triple_review_complete",
  "parse_status": "no_parseable_reviews",
  "assessment_status": "assessment_unavailable_schema_parse_failed"
}
```

Right now, all-zero scores can look like “the reviewers were bad.” The better interpretation is “the evaluator could not parse the reviews.”

## 4. Provider/model reliability is now a real concern

Three calls failed at provider/API level, and all successful calls failed parse. That means real-provider mode needs model eligibility and schema compliance checks before being trusted for serious runs.

---

# Recommended repair ticket

Use this as the next repair:

```text
EBP-EVAL-SIMPLE-0003: Real-Provider Reviewer Schema Contract Hardening
```

This should probably happen **before** the binary-boundary cleanup if your next goal is real-provider usability. The TreeQuest binary-boundary issue is architectural hygiene. This schema issue blocks real model runs from producing usable artifacts.

---

# How to mitigate

## 1. Replace empty schema with a canonical filled example

Do not generate the prompt schema from `ReviewerOutput{}`.

Create a deliberate function:

```go
func reviewerOutputExample(reviewerID, modelID string, policy policy.PolicyIR) ReviewerOutput
```

It should initialize every slice with at least one valid example item.

Example:

```go
func reviewerOutputExample(reviewerID, modelID string) ReviewerOutput {
	return ReviewerOutput{
		ReviewerID:   reviewerID,
		ModelID:      modelID,
		PaperSummary: "Brief 2-3 sentence summary of the paper.",
		MainClaims: []ReviewerClaim{
			{
				ClaimID:   "claim_1",
				ClaimText: "Precise candidate claim extracted from the paper.",
				EvidenceQuotes: []EvidenceQuote{
					{
						Quote:       "Exact quote copied from the paper.",
						SectionHint: "Abstract or Section 1",
					},
				},
				EBPDebts: []string{
					"needMap",
					"needInvariant",
				},
				Status: "candidate_unreviewed",
			},
		},
		MapsIdentified: []string{
			"Map from proposed mathematical structure to observable physical quantity.",
		},
		InvariantsIdentified: []string{
			"Claimed invariant, conservation law, symmetry, or preserved quantity.",
		},
		ToyChecksIdentified: []string{
			"Suggested finite toy check or calculation.",
		},
		NullModelsIdentified: []string{
			"Alternative baseline explanation or simpler rival model.",
		},
		ObstructionsIdentified: []string{
			"Technical blocker, missing derivation, or circularity risk.",
		},
		FaithfulnessLimits: []string{
			"Human faithfulness review was not performed.",
		},
		OverclaimWarnings: []string{
			"Do not treat this as proof or EBP promotion.",
		},
		RecommendedNextSteps: []string{
			"Extract exact equations and map them to EBP debts.",
		},
		OverallAssessment: "Candidate EBP assessment only.",
		Limitations: []string{
			"Automated candidate assessment only.",
			"Human faithfulness review was not performed.",
			"No EBP promotion is claimed.",
		},
	}
}
```

Then marshal that.

## 2. Add explicit schema rules in the system prompt

Add direct instructions like:

```text
Return exactly one JSON object.
Do not use Markdown fences.
Do not return prose outside JSON.
All array fields must be arrays, never null.
Use [] when there are no items.
The following fields must be arrays of strings:
maps_identified
invariants_identified
toy_checks_identified
null_models_identified
obstructions_identified
faithfulness_limits
overclaim_warnings
recommended_next_steps
limitations

Do not return arrays of objects for those fields.
Do not add unknown fields.
Do not rename keys.
main_claims must use claim_id, claim_text, evidence_quotes, ebp_debts, status.
evidence_quotes must use quote and section_hint.
```

This is more important than the one-line user instruction. The user instruction remains simple; the system schema contract must be precise.

## 3. Initialize empty arrays, never nil arrays

Even internally, when producing examples or default JSON, prefer:

```go
[]string{}
```

not nil slices.

If you ever need a schema-like JSON example, it must show:

```json
"maps_identified": []
```

not:

```json
"maps_identified": null
```

## 4. Add parseability-aware run status

Add fields like:

```json
{
  "call_run_status": "triple_review_complete",
  "parse_run_status": "no_parseable_reviews",
  "assessment_status": "assessment_unavailable_schema_parse_failed",
  "parseable_review_count": 0,
  "returned_response_count": 3
}
```

Then change report behavior:

```text
If parseable_review_count == 0:
  do not present final_score as meaningful
  do not present agreement as meaningful
  report should say: no parseable reviewer outputs
```

This prevents users from misreading all-zero scores as an EBP judgment.

## 5. Add regression tests using the observed failures

Create fixtures from the actual failure modes:

```text
testdata/reviewer_outputs/limitations_as_string.json
testdata/reviewer_outputs/maps_as_objects.json
testdata/reviewer_outputs/wrong_claim_keys.json
testdata/reviewer_outputs/debts_as_object.json
```

Tests:

```text
TestReviewerSchemaExample_HasNoNullArrays
TestReviewerSchemaExample_ContainsClaimShape
TestReviewerSchemaExample_ListsStringArrayFields
TestReviewerParse_ObservedMalformedOutputsFailClearly
TestTripleReview_AllReturnedButNoneParseable_StatusIsNoParseableReviews
TestTripleReview_ReportDoesNotTreatAllZeroParseAsEBPScore
```

## 6. Use provider JSON mode when available

If OpenRouter/model supports structured outputs or JSON mode, use it.

Add capability detection/config:

```json
{
  "response_format": {
    "type": "json_object"
  }
}
```

But do not rely on this alone. Keep strict parsing.

## 7. Use lower temperature for reviewer calls

For schema compliance:

```text
temperature = 0.0 or 0.1
```

This is not about creativity. It is about reliable structure.

## 8. Optional: one-shot schema repair

This is optional, but useful for real-provider mode.

If parse fails, you may send a second prompt:

```text
Convert the following raw reviewer response into the exact JSON schema.
Do not add new claims.
Do not change scientific meaning.
Only reformat.
If a field is missing, use [] or an empty string as appropriate.
```

Track it explicitly:

```json
{
  "repair_attempted": true,
  "repair_succeeded": true,
  "original_parse_status": "review_parse_failed",
  "final_parse_status": "review_parse_repaired"
}
```

I would not enable this silently at first. Use:

```text
--repair-malformed-json
```

or keep it as a later patch.

## 9. Do not loosen the decoder casually

Avoid this quick fix:

```text
Accept arbitrary maps and objects everywhere.
```

That would undermine the evaluator’s auditability.

Strict decoding is doing its job. The problem is the prompt contract, not the decoder.

---

# Priority order

## Must fix

```text
1. Replace empty zero-value schema with populated canonical JSON example.
2. Add explicit “arrays never null” and “string arrays only” schema rules.
3. Add parseability-aware run status.
4. Add tests proving schema example has no null arrays.
5. Add tests for all-returned-but-none-parseable status.
```

## Should fix

```text
6. Use JSON mode where supported.
7. Lower temperature for reviewer calls.
8. Add observed-failure fixtures.
9. Improve report wording for parse failures.
```

## Later

```text
10. Optional schema-repair retry.
11. Consider richer structured map/invariant types in v0.2.
12. Add model eligibility scoring for real-provider runs.
```

---

# Recommended final status

```text
EBP-EVAL-SIMPLE-0002 remains accepted_with_repairs.

New finding:
real-provider runs expose a reviewer schema-contract defect.

Functional mock/e2e mode remains valid.
Real-provider mode is not ready for stable use until EBP-EVAL-SIMPLE-0003 repairs schema prompting and parseability reporting.
```

## EBP/PTW self-audit

**Claims:** The problem is schema-contract failure, not evidence that the evaluator cannot assess physics.

**needMap:** Update map: prompt schema → LLM reviewer output → strict parser → scoring. The schema-to-output bridge is currently weak.

**needInvariant:** Preserve strict JSON validation. Do not weaken parsing to hide schema failures.

**needToyCheck:** Add observed malformed real-provider outputs as regression fixtures.

**needNullModel:** Compare repaired schema prompt against old empty-schema prompt on the same models.

**needObstruction:** Main obstruction is LLM schema drift under real-provider conditions.

**needFaithfulnessReview:** Still `not_assessed`.

**Promotion status:** `real_provider_schema_contract_repair_required`; no paper claim, EBP promotion, or physics truth claim is made.
