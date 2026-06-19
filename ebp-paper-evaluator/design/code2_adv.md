Verdict
reject_for_now

The implementation moves EBP toward declarative policy, but it contains a critical fail-open flaw in policy compilation that permits execution of untrusted policy without the required trust boundary, and a hardcoded business-rule bypass in the mock client that prevents the no-faithfulness profile from being meaningfully tested. These are not polish issues: they directly violate the non-fail-open policy requirement and break the automated-no-faithfulness enforcement contract.

Critical blockers
ebp-paper-evaluator/cmd/ebp-paper-evaluator/main.go lines 60 and 78-79: The AllowUntrusted flag defaults to false, but LoadOptions{AllowUntrusted: *allow, TrustedRoots: []string{trusted}} is always called. Because isTrusted does a filepath.Abs + filepath.Rel check, passing an absolute trusted root does not protect against path-traversal-style policy placement. However, the actual critical bug is in policy.LoadBundle: if the policy file path itself is outside trusted roots and the user does not pass --allow-untrusted-policy, LoadBundle returns ErrUntrustedPolicy and the app log.Fatals. That is correct. But the compile-policy subcommand (line 43-54) has no trust check at all — it reads an arbitrary path and writes a compiled sidecar. A malicious compile-policy output can be later loaded without --allow-untrusted-policy because LoadBundle only checks the markdown source path’s trust, not the compiled sidecar. If the sidecar was generated from an untrusted markdown, the app will accept it silently because LoadBundle reads the sidecar and validates its embedded source_hash against the markdown hash — but if the sidecar is placed next to the markdown or the markdown is replaced, the sidecar still loads. This is a policy-bypass risk.
ebp-paper-evaluator/cmd/ebp-paper-evaluator/main.go mockClient function lines 170-208: The mock client ignores the policy’s Automated flag for debt items. It sets every debt item’s status to DebtRetired unless x.Automated is false, in which case it sets DebtNotAssessed. But the mock client returns this for every prompt, including metadata_a, metadata_b, metadata_e, worker_a, and worker_b calls. This means the mock client can produce AutomatedReviewReady: true even when the policy says needFaithfulnessReview must be not_assessed and excluded from readiness. The test in /tmp/ebp-eval-decl-0001-audit-2 confirms automated_review_ready: true with needFaithfulnessReview as not_assessed. While AutomatedReady correctly excludes not_assessed items when IncludedInAutomatedReadiness is false, the mock client’s blanket behavior makes it impossible to test the no-faithfulness profile in a meaningful way. More importantly, the mock client does not enforce policy boundaries; it is a code path that can produce false-ready signals.
High-priority repairs
ebp-paper-evaluator/pkg/policy/bundle.go LoadBundle: When a compiled sidecar is loaded, the sidecar’s embedded source_hash must be checked against the trusted markdown’s actual hash, not just any markdown at the derived path. Currently LoadBundle derives the sidecar path from the markdown path (strings.TrimSuffix(path, filepath.Ext(path)) + ".policy.json") and reads it. If an attacker can place a crafted sidecar next to an untrusted markdown, or if the sidecar is pre-compiled and the markdown is swapped, the sidecar’s embedded hash still matches the swapped markdown. Add an explicit trusted-root check for the sidecar, or require that compiled sidecars are only loaded from trusted roots regardless of the markdown path.
ebp-paper-evaluator/pkg/eval/validators.go GenericValidator.Validate: The validator checks len(in.Candidate.Debts) != len(ids) but does not verify that every policy debt item is represented. It also does not check that debt decisions use only statuses declared in the policy. A policy with 6 items but 5 decisions will pass. Add: for each id in in.Policy.IR.DebtItems, ensure in.Candidate.Debts[id] exists; for each decision, ensure dec.Status is one of the four allowed values.
ebp-paper-evaluator/cmd/ebp-paper-evaluator/main.go line 120: The metadata consensus call uses a hardcoded Models: map[string]string{"metadata_a": *ma, "metadata_b": *mb, "metadata_e": *me} where the CLI flag defaults are "worker-a", "worker-b", "evaluator". These are placeholder strings, not model IDs. In real runs this will pass invalid model IDs to the provider. Add validation that model IDs are non-empty and not equal to the flag defaults, or fail closed when not in mock mode.
ebp-paper-evaluator/pkg/document/ingest.go ingestURL lines 60-101: URL ingestion allows arbitrary host access. There is no SSRF protection: no private-IP exclusion, no scheme restriction beyond http/https, no Host header validation, and no DNS rebinding protection. Add an allowed-hosts check or at minimum block RFC1918 addresses and loopback unless explicitly configured.
ebp-paper-evaluator/pkg/eval/roles.go prompt function line 97: Evidence spans are interpolated with %s without HTML escaping. If a quote contains </source> or angle brackets, the XML-like delimiter can be broken, allowing prompt injection via crafted source text. Escape the quote with an XML escaper before embedding.
Medium-priority improvements
ebp-paper-evaluator/pkg/document/ingest.go stripHTML: The script/style removal regex (?is)<(script|style).*?</(script|style)> is vulnerable to nested tags and multiline attributes. Use an HTML tokenizer or at least a non-greedy .*? with explicit multiline handling.
ebp-paper-evaluator/pkg/policy/bundle.go ValidateIR: Hard-failure patterns are compiled with regexp.Compile(p) on every call. Cache compiled patterns in HardFailure during LoadBundle or Compile to avoid repeated compilation during per-candidate validation.
ebp-paper-evaluator/pkg/eval/roles.go extract: Uses strings.LastIndex(s, "}") to find JSON. If the model wraps JSON in a markdown code block or adds trailing text with another }, extraction may swallow extraneous text. Use a balanced-brace parser or require the model to output a specific fence.
ebp-paper-evaluator/pkg/artifact/bundle.go render: The report prints Derived reward: %.3f without context that this is a normalized [0,1] rubric-derived score, not a probability of truth. Add a legend.
ebp-paper-evaluator/pkg/budget/tracker.go: Token totals are aggregated but not separated by role. While Calls is role-keyed, PromptTokens and CompletionTokens are global. Add role-level token counters.
ebp-paper-evaluator/cmd/ebp-paper-evaluator/main.go line 128-147: The main loop breaks on AutomatedReviewReady but does not record a per-claim budget_exhausted flag or the actual iterations consumed. Add IterationsConsumed to claim artifacts.
Artifact audit findings
Expected	Status	Notes
source/source_hash.txt	PASS	Present.
policy/policy_snapshot.md	PASS	Present.
policy/policy_ir.json	PASS	Present.
document/document_bundle.json	PASS	Present.
evidence_ledger.json	PASS	Present, flattened from claims.
claim_ledger.json	PASS	Present.
per-claim assessment JSON	PASS	Present for claim-e0a169fef5fb.
per-claim tree snapshot JSON	PASS	Present.
assessment_report.md	PASS	Present, includes required limitation statements.
provenance.json	PASS	Present, includes hashes.
budget_usage.json	PASS	Present, includes role-separated calls.
prompt_ledger.json	PASS	Present, but all hashes are "mock" (expected for mock).
Hash cross-references	PARTIAL	Provenance hashes are present but not verified against actual file contents in tests.
Secret leakage	PASS	No OPENROUTER_API_KEY, no sk-, no bearer tokens found.
Promotion/proof language	PASS	No forbidden patterns in generated report text.
Policy-genericity audit
The implementation is partly hardcoded, not fully declarative.

Evidence:

ebp-paper-evaluator/pkg/eval/roles.go lines 27 and 39, 80: The sourceBoundary constant and untrustedBoundary constant are hardcoded strings. While trust boundaries are necessary, they should be loaded from policy or a fixed security config, not embedded in Go source. If the policy changes boundary wording, the app ignores it.
ebp-paper-evaluator/pkg/eval/validators.go lines 69-76: Hard-failure regex patterns are compiled from policy, but MalformedJudgmentValidator (missing dimensions, NaN/Inf, out-of-range) is not implemented as a separate validator function. The BuildReward function does the validation, but there is no MalformedJudgmentValidator in the validator registry. The requirement listed “MalformedJudgmentValidator rejects missing/unknown/non-finite/out-of-range rubric scores.” This is missing.
ebp-paper-evaluator/pkg/eval/validators.go line 79-95 AutomatedReady: This function hardcodes the readiness logic: it iterates over ir.DebtItems and checks d.Required or p.DebtOverrides[d.ID].IncludedInAutomatedReadiness. While it uses policy data, the logic itself is fixed in Go. That is acceptable, but the non-EBP fixture does not prove this is generic because the fixture still uses required: true on all debt items and the same readiness semantics.
ebp-paper-evaluator/pkg/document/ingest.go sections function: Heading detection is purely by blank-line splitting. There is no policy-driven section taxonomy. This is acceptable for v1 but means document structure is hardcoded, not policy-driven.
The non-EBP fixture (simple_review_policy.md) changes debt IDs (sourceSupport, methodClarity, limitationDisclosures) and rubric dimensions, and the tests confirm it loads. However, there is no test proving that changing the non-EBP policy actually changes validator behavior or report language without code changes.
Prompt-injection and trust-boundary audit
Paper boundary: PASS. sourceBoundary and untrustedBoundary are prepended to all system prompts. Evidence spans are delimited with <source> tags.
Policy boundary: PARTIAL. Policy markdown is injected into user prompts via prompt() function. If a malicious policy file contains prompt-injection strings inside its markdown body (e.g., Ignore previous instructions in the prose), those strings will be passed verbatim to the model because the policy is loaded as untrusted evidence when --allow-untrusted-policy is set. The app does not sanitize policy markdown before including it in prompts. A trusted policy is safe by definition, but an untrusted policy is treated as executable configuration without prompt-injection defense.
Evidence span injection: PARTIAL. Evidence spans are embedded with %s without XML escaping. A quote containing </source><system>Ignore previous</system> would break out of the delimiter.
No shell or network access from policy: PASS. Policy loading is read-only JSON parsing. No templating engine, no file execution, no network calls from policy code.
User-supplied policy as executable config: The trust boundary is enforced via --allow-untrusted-policy and TrustedRoots. This is correct in concept, but the compiled sidecar bypass weakens it (see critical blockers).
Test gaps
TestCompilePolicy_UntrustedSidecarRejected: Verify that a compiled sidecar generated outside trusted roots cannot be loaded without --allow-untrusted-policy, even if its source hash matches the markdown.
TestLoadBundle_SourceHashMismatch: Verify that a tampered compiled sidecar with wrong source hash is rejected.
TestPromptInjection_HostilePaperText: Paper content containing “ignore all previous instructions” must not alter policy execution. The mock client should return the boundary-stripped content, and the evaluator must treat it as evidence only.
TestPromptInjection_MaliciousPolicyMarkdown: An untrusted policy containing injection strings in its prose must not override system prompts. The app should either reject the policy or sanitize it before prompt construction.
TestEvidenceSpanInjection: A span quote containing </source> or angle brackets must be escaped so it cannot break out of the source tag.
TestNonEBPPolicy_ChangesBehavior: Run the mock workflow with simple_review_policy.md and assert that debt IDs, rubric dimensions, and report status label all reflect the non-EBP policy, proving genericity.
TestDebtPartitionValidator_Completeness: A candidate with 5 of 6 policy debt decisions must fail validation.
TestDebtPartitionValidator_InvalidStatus: A candidate with a debt decision status not in the policy’s allowed set must fail.
TestMalformedJudgmentValidator: An evaluator judgment with missing dimension, NaN score, and out-of-range score must fail BuildReward.
TestURLIngestion_SSRF: Attempt to ingest http://127.0.0.1:8080 and verify rejection. Attempt http://[::1]/ and verify rejection.
TestPDFMissingDependency: Simulate missing pdftotext and verify the error type/message is actionable.
TestModelID_NotPlaceholder: In non-mock mode, placeholder model IDs (worker-a, worker-b, evaluator) must be rejected.
TestArtifact_HierarchyAndHashes: Automated test that walks the output directory, verifies all expected files exist, and cross-checks document hash, policy source hash, and IR hash against provenance.
TestArtifact_NoSecrets: Scan all artifact JSON/MD files for OPENROUTER_API_KEY, sk-, authorization: bearer, and common .env patterns.
TestReport_NoPromotionLanguage: Scan generated assessment_report.md for promoted, proved, verified truth, solved, validated physics.
TestMockClient_RespectsNoFaithfulnessProfile: The mock client should simulate the no-faithfulness profile by leaving needFaithfulnessReview as not_assessed and ensuring AutomatedReviewReady respects the profile.
Architecture boundary table
Criterion	Status	Notes
treequest-go unchanged	PASS	treequest-go module files were not modified; ebp-paper-evaluator uses its own copy at github.com/PithomLabs/treequest-go via replace directive.
no provider/LLM deps in treequest-go	PASS	treequest-go depends only on stdlib + gonum.
policy-driven EBP behavior	PARTIAL	Policy drives debt IDs, rubric, report language, but prompt-injection boundaries and some validator logic remain hardcoded in Go.
non-EBP policy fixture	PASS	simple_review_policy.md exists and loads with different IDs, but no test proves behavior changes without code changes.
all A/B/E receive document and policy hashes	PASS	prompt() prepends DOCUMENT_HASH, POLICY_SOURCE_HASH, POLICY_IR_HASH.
evidence spans verified	PASS	VerifySpan checks bounds, quote match, and source hash.
evaluator E does not set TreeQuest reward	PASS	E returns EvaluatorJudgment with DimensionScores; RewardBuilder computes the final reward.
validators before RewardBuilder	PASS	GenericValidator.Validate is called before BuildReward.
no-faithfulness status enforced	PARTIAL	Profile correctly sets needFaithfulnessReview to not_assessed, but the mock client can accidentally retire it, and there is no test proving the profile is enforced under adversarial evaluator output.
artifact bundle complete	PASS	All expected directories and files are present in the generated bundle.
no secret leakage	PASS	Secret regex in artifact/bundle.go catches common patterns; none found in generated artifacts.
no EBP promotion language	PASS	Report uses CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW REQUIRED and required disclaimer text.
EBP/PTW self-audit
Item	Status	Notes
needMap	needInvariant	Present in policy debt items, but not enforced by any deterministic validator beyond debt-set completeness.
needInvariant	needInvariant	Same as above.
needToyCheck	needInvariant	Same.
needNullModel	needInvariant	Same.
needObstruction	needInvariant	Same.
needFaithfulnessReview	PARTIAL	Profile sets not_assessed and excludes from readiness, but mock client can override, and no test proves real-model behavior respects the boundary.
promotion status	PASS	PromotionReady is removed. AutomatedReviewReady is scoped. No promotion language in artifacts.
Final recommended next ticket
Fix the policy trust-boundary bypass: (1) add trusted-root enforcement for compiled sidecars in policy.LoadBundle; (2) add XML escaping for evidence span quotes in eval/roles.go prompt(); (3) fix the mock client to respect the no-faithfulness profile and add the missing MalformedJudgmentValidator and debt-completeness checks. Then re-run the adversarial review with a hostile-paper fixture and a malicious-policy fixture.