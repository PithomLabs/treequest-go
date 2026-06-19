## Review verdict

The StepFun review should be accepted as a **blocking review**.

I would downgrade `EBP-EVAL-DECL-0001` from:

```text id="izek39"
code_accepted_for_reported_declarative_policy_mock_scope
```

to:

```text id="ut5mzo"
reject_for_now_pending_EBP_EVAL_DECL_0001_SECURITY_REPAIR
```

The review is balanced: it confirms the architecture is sound, but identifies three real blockers: unsafe greedy JSON extraction in evaluator and metadata paths, missing SSRF protections in URL ingestion, and weakened prompt trust boundaries because full policy markdown is embedded in user-role content. 

## What remains accepted

The good news: the high-level redesign is still validated.

```text id="jdqmn4"
PASS: treequest-go unchanged
PASS: EBP behavior moved to declarative PolicyIR
PASS: non-EBP policy fixture exists
PASS: evidence spans verified
PASS: evaluator E does not directly set TreeQuest reward
PASS: validators run before RewardBuilder
PASS: no-faithfulness profile enforced
PASS: artifact bundle complete
PASS: no secret leakage found
PASS: no EBP promotion language found
```

So the architecture is not wrong. The implementation needs a hardening repair pass.

## Critical blockers to repair

### 1. Replace greedy JSON extraction

Blockers:

```text id="4x6lar"
pkg/eval/roles.go extract
pkg/metadata/consensus.go extractObject
```

Current pattern:

```text id="4xcz6i"
first "{" through last "}"
```

This is unsafe with multiple JSON objects, trailing commentary, malformed output, or nested structures. It can corrupt metadata or evaluator state. 

Required repair:

```text id="3gjre7"
Use a first-complete-object balanced JSON extractor.
Fail closed on malformed, multiple ambiguous, or unterminated JSON.
Run json.Decoder with DisallowUnknownFields where applicable.
Post-validate required fields are non-nil.
```

### 2. Add SSRF protections to URL ingestion

Blocker:

```text id="b85wbz"
pkg/document/ingest.go ingestURL
```

It accepts arbitrary HTTP/HTTPS URLs without blocking private/internal addresses. That can hit loopback, link-local, Kubernetes, or cloud metadata endpoints. 

Required repair:

```text id="1bp3zw"
Allow only http/https.
Resolve DNS before request.
Reject loopback, private, link-local, multicast, unspecified, and cloud metadata IPs.
Recheck IPs after redirects.
Limit redirects.
Limit response size.
Enforce content-type allowlist.
Use request timeout.
```

### 3. Strengthen prompt trust boundary

The review says policy markdown is embedded in user-role message content. That weakens the authority separation between policy and untrusted paper text. 

Required repair:

```text id="sgd1k6"
Move trusted policy instructions into system/developer-level prompt context where possible.
Keep user-role content for paper evidence, claim context, and candidate state.
Wrap all paper chunks/spans in explicit untrusted evidence delimiters.
In metadata prompts, wrap chunks just like eval prompts.
```

If the provider API only supports system/user roles, then:

```text id="dbrxxc"
system = application rules + trusted policy summary/instructions
user = untrusted paper evidence + candidate state
```

## High-priority follow-ups

The review also flags several important non-blockers:

```text id="3kxqum"
- mock behavior should use explicit role metadata, not substring matching against prompts
- PDF ingestion should preflight exec.LookPath("pdftotext")
- URL title extraction should not blindly use filepath.Base(input)
- evaluator judgment parsing should require DimensionScores and DebtDecisions to be non-nil
- untrusted policy enforcement should be actively checked by caller, not merely computed
- regexes from policy should be compiled during policy validation, not inside each validation loop
```

## New repair ticket

Use this as the next ticket:

```text id="c1xyc3"
EBP-EVAL-DECL-0001.1: JSON Extraction, SSRF, and Prompt Trust-Boundary Hardening
```

### Required scope

```text id="c0oqul"
1. Replace evaluator JSON extraction with balanced first-object extraction.
2. Replace metadata consensus JSON extraction with the same shared extractor.
3. Add required-field post-unmarshal validation.
4. Add SSRF protections to URL ingestion.
5. Add URL redirect/IP revalidation.
6. Add content-type allowlist and clear URL-ingestion errors.
7. Move trusted policy instructions out of user message.
8. Wrap metadata chunks in explicit untrusted-evidence delimiters.
9. Replace mock substring routing with explicit role/request metadata.
10. Add pdftotext LookPath preflight.
```

### Required tests

```text id="2y62zo"
- TestExtractEvaluatorJSON_FirstObjectOnly
- TestExtractMetadataJSON_FirstObjectOnly
- TestExtractJSON_MalformedFailsClosed
- TestExtractJSON_MultipleObjectsFailsOrChoosesFirstSafely
- TestURLIngest_BlocksPrivateIPs
- TestURLIngest_BlocksMetadataEndpoint
- TestURLIngest_RevalidatesRedirectTarget
- TestPromptInjection_HostilePaperDoesNotOverrideSystem
- TestMetadataPrompt_WrapsChunksAsUntrustedEvidence
- TestPDFIngest_MissingPdftotext
- TestMockClient_UsesExplicitRoleNotPromptSubstring
```

## Status language to use now

```text id="qlgt3p"
EBP-EVAL-DECL-0001 architecture remains accepted, but the implementation is rejected-for-now by post-implementation adversarial review.

Current blockers are JSON extraction hardening, URL-ingestion SSRF protection, and prompt trust-boundary repair.

No production acceptance, no real-provider reliability claim, no EBP promotion claim, and no paper-claim proof should be made until EBP-EVAL-DECL-0001.1 is repaired and re-reviewed.
```

## EBP/PTW self-audit

**Claims:** StepFun’s review identifies true blockers and should override the previous implementation acceptance.

**needMap:** Still satisfied architecturally. The pipeline map remains sound.

**needInvariant:** Failing in current implementation: trust boundary and safe-ingestion invariants are not yet strong enough.

**needToyCheck:** Insufficient. Need hostile JSON, hostile paper, SSRF, and prompt-boundary tests.

**needNullModel:** Partially satisfied by the non-EBP fixture; not affected by this review.

**needObstruction:** Main obstructions are adversarial LLM output, malicious URLs, and prompt injection through paper/policy content.

**needFaithfulnessReview:** Still correctly `not_assessed`; no human faithfulness review is claimed.

**Promotion status:** `implementation_reject_for_now_pending_EBP_EVAL_DECL_0001_1`.
