Based on thorough inspection of the latest real-provider smoke-test bundle at `/home/chaschel/Desktop/physics/sakana/out/my-paper-triple-review0/` (and comparison runs), here is the adversarial review.

---

## 1. Verdict

**`reject_for_now`**

Zero parseable reviews were produced across every real-provider smoke run, including the latest. The schema-contract hardening ticket did not translate into observable schema compliance by any of the five free models exercised, and the prompt ledger does not record the temperature, response-format, or schema-example metadata needed to verify the hardening was applied. The evaluator correctly reports honest degraded status, but this does not establish real-provider operational readiness for a `triple_review_complete` outcome.

---

## 2. Real-Provider Run Summary

| Field | Value |
|---|---|
| `bundle_path` | `/home/chaschel/Desktop/physics/sakana/out/my-paper-triple-review0/` |
| `models` | `google/gemma-4-31b-it:free`, `poolside/laguna-m.1:free`, `nex-agi/nex-n2-pro:free` |
| `returned_response_count` | 3 |
| `parseable_review_count` | 0 |
| `call_run_status` | All three reviewers: `response_returned` |
| `parse_run_status` | All three reviewers: `review_parse_failed` (inferred from `score.json`; no `reviewer_N_parsed.json`) |
| `assessment_status` | `CANDIDATE_EBP_ASSESSMENT_HUMAN_REVIEW_REQUIRED` (per-reviewer) / bundle `triple_review_complete` |

Consensus `scoring_summary.json` confirms: `parseable_reviewers: 0`, `returned_reviewers: 3`, `run_completeness: 1`.

**Classification:** `not_usable` — the run is complete but yields no parseable signal.

---

## 3. Per-Reviewer Table

| reviewer | model | call_status | parse_status | score | failure_category | usable_for_schema_contract? |
|---|---|---|---|---|---|---|
| reviewer_1 | google/gemma-4-31b-it:free | response_returned | review_parse_failed | 0.0 | **schema_incompatible_output** (Markdown fence; extra fields; `ebp_debts` mapped to `recommended_next_steps` with `debt_item`) | **No** |
| reviewer_2 | poolside/laguna-m.1:free | response_returned | review_parse_failed | 0.0 | **schema_incompatible_output** (Markdown fence; unknown keys `evidence_type`, `translation_rule`, `evidence`, `severity`, `check`, etc.) | **No** |
| reviewer_3 | nex-agi/nex-n2-pro:free | response_returned | review_parse_failed | 0.0 | **schema_incompatible_output** (completely different envelope: `id`, `ebp_profile`, `debt_status` maps, `obstructions_identified`; no fence, but wrong schema entirely) | **No** |

All three emitted rich prose-structured JSON that defied the flattened-string-list and flat-array schema contract.

---

## 4. Critical Blockers

1. **Zero parseable reviews** — `parseable_review_count = 0` across all five smoke runs (six bundles including the `my-paper-triple-review0` repetition). This is an outright blocker for real-provider release candidacy.
2. **Markdown JSON fences present in 2/3 outputs** — boundary violation in raw artifacts (`reviewer_1_raw.txt`, `reviewer_2_raw.txt`), indicating the no-fence prompt rule was not obeyed.
3. **Prompt ledger lacks temperature, `response_format`, `json_mode`, schema example, document/policy/profile IDs** — the hardening ticket’s core enforcement metadata is not observable in artifacts, so verification of contract application is impossible.
4. **No per-reviewer `call_run_status` / `parse_run_status` / `assessment_status` fields anywhere** — the bundle uses only bundle-level provenance. The per-reviewer breakdown exists only inside `score.json` as `call_status` / `parse_status` strings, not as the distinct top-level status taxonomy mandated.

---

## 5. High-Priority Repairs

1. **Verify the hardened prompt actually ships to the provider.** Add prompt-ledger fields: `temperature`, `response_format` (e.g., `json_object` or `provider_response_format_unsupported`), `json_mode_requested`, `document_hash`, `policy_hash`, `profile_id`. Without these, the audit gap is uncloseable.
2. **Add `reviewer_N_parsed.json` and `reviewer_N_error.json` artifacts** so parse failures are inspectable rather than inferred solely from `score.json`.
3. **Enforce no-Markdown-fence stripping in the parser** even when models wrap output in fences, and record count of fence-stripped parses to monitor provider disobedience.
4. **Add explicit `ebp_debts` as `[]string` schema field** in the prompt example; reviewers are currently mapping debt semantics into heterogeneous structures.

---

## 6. Medium-Priority Improvements

1. Probe provider-native JSON mode (`response_format: json_object`) for OpenRouter-compatible models and record the actual flag in the prompt ledger.
2. Add a post-parse schema-diff report to the consensus directory that records exact unknown keys / wrong-type keys per reviewer.
3. Separate `call_run_status`, `parse_run_status`, `assessment_status`, `returned_response_count`, and `parseable_review_count` as top-level fields in `consensus/scoring_summary.json` so reviewers do not have to drill into per-reviewer `score.json`.
4. Emit a `provider_response_format_unsupported` status (instead of generic parse failure) when the raw output is non-JSON prose.

---

## 7. Schema-Contract Audit

| Check | Result |
|---|---|
| Arrays are arrays, not `null` | **Partial failure.** Reviewer_3 (0318 bundle) emitted all-null arrays; current bundle outputs are object arrays / wrong shape, not `null`. |
| `main_claims` uses expected keys (`claim`, `status`) | **Partial.** Keys vary by model; reviewer_2 adds `evidence_type`; reviewer_3 uses `id`, `candidate_assessment`, `debt_status`. |
| `evidence_quotes` uses `quote` and `section_hint` | **Not used.** No reviewer produced this structure. |
| `ebp_debts` is flat `[]string` | **Failure.** No reviewer emitted flat string array; debt is embedded in object keys or strings inside other objects. |
| String-list fields are arrays of strings | **Failure.** Maps, invariants, toys, null-models, obstructions are arrays of objects with heterogeneous schemas. |
| No arrays of objects for `maps_identified` | **Failure.** All three emitted object arrays. |
| No unknown fields / no renamed keys | **Failure.** Extensive unknown keys present. |
| No Markdown fences | **Failure.** Reviewers 1 and 2 wrapped output in triple-backtick fences. |
| No LLM-provided score field | **Pass.** No self-scoring in raw outputs. |
| No proof/promotion language | **Pass.** Language is appropriately hedged. |

**Classification:** `schema_contract_failure`.

---

## 8. Prompt-Ledger Audit

| Check | Result |
|---|---|
| All three user messages byte-identical | **Cannot verify** — user message text is not stored in ledger. |
| User message begins with “Apply EBP 2.1…” | **Cannot verify** — not in ledger. |
| `<untrusted_paper>` wrapper present | **Cannot verify** — not in ledger. |
| Reviewer identity absent from user content | **Cannot verify** — not in ledger. |
| Reviewer identity in system/metadata | **Cannot verify** — not in ledger. |
| Prompt stores model ID, role/task, document hash, policy hash, profile ID | **Partial.** Role/task/model_id/prompt_hash stored; document_hash, policy_hash, profile_id missing. |
| `response_format` / JSON mode recorded | **Failed** — not present in any artifact. |
| Low temperature recorded | **Failed** — not present. |

**Classification:** `prompt_ledger_incomplete`.

---

## 9. Scoring/Report Audit

| Check | Result |
|---|---|
| All scores evaluator-computed | **Pass.** All zero; no LLM self-scores present. |
| Final score = equal-weight mean of nine metrics | **Pass.** Zeroed consistently. |
| Malformed/failed reviewers score `0.0` | **Pass.** |
| Failed reviewer score not confused with quality | **Pass.** Status `review_quality_unscorable`. |
| Run completeness separate from quality | **Pass.** `run_completeness: 1`, scores `0`. |
| `parseable_review_count == 0` → report does not present all-zero scores as meaningful | **Pass.** Report clearly labels `CANDIDATE_EBP_ASSESSMENT_HUMAN_REVIEW_REQUIRED`. |
| Partial parseability → report says degraded | **Pass.** |
| Agreement computed only from parseable reviews | **Pass.** Agreement `0.0` with `unavailable` status; fewer than 2 parseable. |
| Cross-model agreement `0.0` if < 2 parseable | **Pass.** |
| Lexical agreement limitation stated | **Pass.** |
| Relative claim coverage not paper-truth coverage | **Pass.** Stated explicitly in report. |

---

## 10. Artifact/Secret/Privacy Audit

| Check | Result |
|---|---|
| Required directories present (`source/`, `policy/`, `reviews/`, `consensus/`, `report/`, `run/`) | **Pass.** |
| Required files present | **Pass.** |
| Hash consistency | **Pass.** `source_hash.txt` matches `source_ref.json`; `policy_hash.txt` matches IR; prompt hashes present per reviewer. |
| Budget records per reviewer | **Pass.** `budget_usage.json` covers all three. |
| Secrets / API keys / Authorization / Bearer / `sk-` / `x-api-key` | **Pass.** Zero matches across all artifacts. |
| Raw local absolute paths | **Pass.** No absolute paths found. |

---

## 11. Model Suitability Audit

| model | rating | rationale |
|---|---|---|
| `google/gemma-4-31b-it:free` | `not_schema_compliant` | Emitted rich but incorrect schema; JSON fence wrapper. |
| `poolside/laguna-m.1:free` | `not_schema_compliant` | Heterogeneous unknown keys; fence wrapper. |
| `nex-agi/nex-n2-pro:free` | `not_schema_compliant` | Completely divergent schema (EBP profile object, `debt_status` maps); no fence but shape mismatch. |
| `nvidia/nemotron-3-super-120b-a12b:free` (0318 run) | `not_schema_compliant` | Emitted pure `null` arrays for structured fields. |
| `nvidia/nemotron-3-ultra-550b-a55b:free` (0245 run) | `provider_unreliable` | Call failed outright (`reviewer_call_failed`). |

None of the five tested models are schema-compliant under the hardened contract.

---

## 12. Before/After Comparison to Pre-Hardening Runs

| Metric | Pre-hardening (20260620_0245) | Post-hardening (my-paper-triple-review0) |
|---|---|---|
| `run_status` | `partial_triple_review_two_reviewers` | `triple_review_complete` |
| `run_completeness` | 0.67 | 1.0 |
| `returned_reviewers` | 2 | 3 |
| `parseable_reviewers` | 0 | 0 |
| Main failure mode | reviewer_3 call failed; reviewers 1-2 schema-incompatible | all three schema-incompatible; JSON fences persisted |
| Main failure mode after | same schema noncompliance | no improvement in parseability |

**The populated schema-example and prompt rules produced no measurable parseability improvement.** The dominant failure mode remained schema incompatibility and Markdown fence wrapping.

---

## 13. Architecture Boundary Table

| boundary | pass/fail | evidence |
|---|---|---|
| `treequest_used` = false | **Pass** | `provenance.json` |
| local `.txt` / `.md` only | **Pass** | `source_ref.json` shows `local_text` |
| Prompt schema populated (`reviewerOutputExample`) | **Cannot verify** | not present in prompt ledger; cannot confirm what was sent |
| no null-array schema | **Partial** | latest raw outputs are object arrays, not null; earlier (0318) had all-null arrays |
| explicit array/type rules | **Cannot verify** | prompt content not stored in artifacts |
| JSON mode requested or `provider_response_format_unsupported` recorded | **Fail** | absent from all artifacts |
| low temperature recorded | **Fail** | absent |
| strict parsing preserved | **Partial** | scoring shows parse failures, but no `DisallowUnknownFields` record |
| parseability-aware status | **Pass** | per-reviewer `call_status` / `parse_status` in `score.json` |
| no misleading all-zero EBP score | **Pass** | report labels `HUMAN_REVIEW_REQUIRED` |
| no proof/promotion language | **Pass** | report and raw outputs hedged |
| `faithfulness` = `not_assessed` | **Pass** | profile is `automated-no-faithfulness`; reviewer_3 lists `not_assessed_automated_profile` |
| no secret leakage | **Pass** | scanned all artifacts |

---

## 14. EBP/PTW Self-Audit

| aspect | status |
|---|---|
| `needMap` | Present in outputs; not parsed/confirmed due to parse failure. |
| `needInvariant` | Present in outputs; not parsed/confirmed. |
| `needToyCheck` | Present in outputs; not parsed/confirmed. |
| `needNullModel` | Present in outputs; not parsed/confirmed. |
| `needObstruction` | Present in outputs; not parsed/confirmed. |
| `needFaithfulnessReview` | Correctly marked `not_assessed` in automated profile. |
| promotion status | **Not established.** Zero parseable reviews means no candidate can be assessed, let alone promoted. |

---

## 15. Final Recommended Next Ticket

**Ticket: `ebp-eval-simple-0006-real-provider-parseability-recovery`**

Scope (smallest repair to unblock):
- Inspect the actual prompt template sent to providers and confirm the hardened `reviewerOutputExample`, no-fence rule, flat-`ebp_debts`, and `DisallowUnknownFields` are present in code.
- Add prompt-ledger emission of `temperature`, `response_format`, `document_hash`, `policy_hash`, `profile_id`, and a hash of the full user message.
- Add a relaxed fallback parser that:
  1. Strips ` ```json ... ``` ` fences,
  2. Maps common alternative keys to canonical keys (e.g., `debt_item` → accept but normalize),
  3. Normalizes object arrays of strings to flat string arrays when unambiguous,
  4. Otherwise emits `provider_response_format_unsupported` rather than generic parse failure.
- Re-run against at least two of the tested models and require `parseable_review_count >= 2` before declaring real-provider readiness.

Without these, the hardening work is unverifiable and the run remains `reject_for_now`.