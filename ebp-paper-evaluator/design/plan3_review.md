## Review verdict

`Pasted text(18).txt` is a **strong and implementation-ready plan** for the simplified scope.

I would mark it:

```text
accept
```

The plan correctly reframes `EBP-EVAL-DECL-0001.1` as a **scope-reduction and hardening ticket**, not as a URL/PDF/arXiv hardening ticket. It removes remote document ingestion entirely, keeps `treequest-go` unchanged, preserves declarative policy evaluation, and focuses the repair on local file safety, JSON extraction, prompt boundaries, and mock determinism. 

## What is accepted

The following design choices are correct:

```text
PASS: CLI-only scope
PASS: local .txt/.md only
PASS: no URL/arXiv/PDF/pdftotext/browser/OCR
PASS: no document-ingestion network/subprocess imports
PASS: OpenRouter remains only an LLM provider, not paper ingestion
PASS: balanced JSON extraction replaces greedy extraction
PASS: paper chunks/spans are explicitly untrusted
PASS: policy stays declarative and separate from evidence
PASS: mock routing uses role/task metadata, not prompt substrings
PASS: non-EBP policy fixture remains part of the genericity proof
PASS: artifact bundle remains intact
PASS: EBP-safe no-promotion language remains enforced
```

The strongest part is that the plan resolves the SSRF issue by **eliminating the feature**, which is cleaner than hardening a feature you do not want.

## Minor implementation cautions

### 1. Path safety needs careful ordering

The path handling is good, but during implementation make sure the file is opened only after containment and symlink checks, and then `Stat()` the opened handle. The plan already mentions TOCTOU limits, which is honest and enough for this scope.

### 2. Be careful with “policy instructions in system prompt”

This is right for trusted shipped policy. For user-supplied policy, keep the existing trusted/untrusted enforcement. Do not let arbitrary policy markdown silently become authoritative unless the user explicitly selected/trusted it.

### 3. Do not over-test by parsing Go imports in brittle ways

`TestDocumentPackage_NoHTTPClient` is useful, but implement it carefully. A source scan for `net/http`, `os/exec`, `pdftotext`, `arxiv`, and URL-ingestion identifiers is fine. Do not make the test so brittle that harmless comments break builds unless that is intentional.

### 4. Preserve local file privacy in artifacts

`source_ref.json` can include `ResolvedPath`, but consider whether full local absolute paths should be written by default. Safer default:

```text
input_path: user-provided relative path
resolved_path: optional, or redacted unless --include-local-paths is set
```

This is not a blocker, just a privacy improvement.

## Recommended implementation status

```json
{
  "plan_status": "accepted",
  "ready_for_implementation": true,
  "scope": "CLI-only local .txt/.md ingestion plus JSON/prompt-boundary hardening",
  "treequest_go_changes": "forbidden",
  "remote_ingestion": "removed_from_scope",
  "acceptance_gate": [
    "go test ./...",
    "go test -race ./...",
    "go vet ./...",
    "local EBP mock run",
    "local non-EBP mock run",
    "artifact/secret/language audit"
  ]
}
```

## Best next ticket name

Use:

```text
EBP-EVAL-DECL-0001.1: Local-Text-Only CLI Simplification and Boundary Hardening
```

That title is better than mentioning SSRF, because the real design decision is: **no remote ingestion at all**.

## EBP/PTW self-audit

**Claims:** The plan is accepted as an implementation plan, not as evidence that the repair is already implemented.

**needMap:** Satisfied. The plan maps old URL/PDF/arXiv-capable ingestion to local-only ingestion, JSON hardening, prompt boundaries, and mock-role routing.

**needInvariant:** Satisfied. Key invariants are unchanged `treequest-go`, local-only paper input, immutable source hashing, verified evidence spans, policy-owned business rules, and no EBP promotion.

**needToyCheck:** Specified. Local EBP and non-EBP mock runs are required.

**needNullModel:** Satisfied by the non-EBP policy fixture proving policy-driven behavior.

**needObstruction:** Addressed. The major obstructions are path traversal, symlink escape, malformed LLM JSON, prompt injection, mock routing manipulation, and regression to remote ingestion.

**needFaithfulnessReview:** Correctly remains `not_assessed`.

**Promotion status:** `implementation_plan_accepted_for_local_text_repair`; not implemented or validated yet.
