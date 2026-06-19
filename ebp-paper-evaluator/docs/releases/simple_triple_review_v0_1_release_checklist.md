# Release Checklist - simple_triple_review v0.1 Release Freeze

This checklist documents the checks performed to audit the simple triple review v0.1 release baseline.

## Boundaries and Strict Parsing
- [x] Local `.txt`/`.md` ingestion only (no remote, symlinks, or directories)
- [x] URL/arXiv/PDF/HTTP/OCR/pdftotext ingestion absent from `simple_triple_review`
- [x] Strict JSON parsing preserved (rejects unknown fields, non-flat structures, or invalid values)
- [x] Reviewer user messages identical across all three reviewer LLMs
- [x] Reviewer identity kept outside the untrusted paper context

## Provenance and Configuration Checks
- [x] Prompt hashes recorded in `Provenance` (`user_message_hash`, `system_prompt_hash`)
- [x] Output schema example hash recorded in `Provenance` (`schema_example_hash`)
- [x] Review temperature (`0.1`) recorded in `Provenance`
- [x] Review `response_format` (`json_object`) recorded in `Provenance`
- [x] `treequest_used` is explicitly `false` in `Provenance`
- [x] Faithfulness limits and assessments marked `not_assessed` (human review required)
- [x] No EBP promotion or physics proof claims are made by the tool

## Artifact and Output Emitted
- [x] `model_suitability.json` emitted with suitability status, error category, and notes
- [x] `agreement_diagnostics.json` emitted with lexical similarity, disclaimers, and closest-pair diagnostic logs
- [x] `real_provider_status` emitted in provenance and scoring summary

## Real-Provider Smoke Baseline Verification
- [x] Real-provider smoke bundle path recorded (`/home/chaschel/Desktop/physics/sakana/out/my-paper-triple-review`)
- [x] Source paper hash recorded (`5b8a165ea00a3fcccc8f6b2a0675bea23a40b6aef7106c8d5bb898bc2da3934d`)
- [x] Policy source/IR hashes recorded (`cb3b34bf...` and `5b961b55...`)
- [x] Provenance hashes and example schema hash recorded
- [x] Reviewer model IDs recorded (`poolside/laguna-xs.2:free`, `poolside/laguna-m.1:free`, `nex-agi/nex-n2-pro:free`)
- [x] `parseable_review_count = 3` successfully verified and recorded

## Verification Runs
- [x] Mock EBP run passes successfully
- [x] Mock non-EBP run passes successfully
- [x] Latest real-provider smoke reviewed and validated
- [x] Linter checks (`go vet ./...`) pass
- [x] Unit tests (`go test ./...`) pass
- [x] Race detector tests (`go test -race ./...`) pass

## Release Integrity
- [x] No secrets (credentials, API key strings) leak in artifacts
- [x] Release-freeze ticket adds no new runtime features beyond documentation/checklist/regression tests (no feature drift)
