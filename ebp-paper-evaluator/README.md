# EBP Paper Evaluator

`ebp-paper-evaluator` is a command-line program for producing automated candidate assessments of local research papers under a declarative review policy. The included policy implements EBP 2.1.

The evaluator can compare three independent LLM reviews or perform an iterative claim-level assessment. Both workflows preserve the paper as untrusted evidence, record provenance and model usage, and produce auditable artifact bundles.

> [!IMPORTANT]
> The output is an automated candidate assessment. It is not proof that a paper is correct, full EBP promotion, or a substitute for human scientific and faithfulness review. LLM responses are comparison and review signals, not authorities.

## Choose an evaluation mode

| Mode | Command | Best used for | TreeQuest |
|---|---|---|---|
| Triple review | `triple-review` | Comparing three independent reviews of the same paper, including agreement, disagreement, and relative coverage | No |
| Iterative assessment | `run` | Claim extraction followed by iterative constructive, adversarial, and evaluator passes | Yes |

Use `triple-review` when you want a direct answer to: “How similarly do three models apply this policy to the same paper?”

Use `run` when you want claim-level iterative refinement, debt tracking, readiness checks, and TreeQuest snapshots.

## Requirements

- The only required input artifacts are the research paper and policy Markdown file
- Go 1.25.7 or a compatible toolchain
- A local UTF-8 research paper in `.txt` or `.md` format
- An OpenRouter API key for live model calls
- Three distinct model IDs for a live triple review

Run all commands below from this directory:

```bash
cd ebp-paper-evaluator
go mod download
```

You can run the program directly with `go run`, or build a reusable binary:

```bash
mkdir -p ./bin
go build -o ./bin/ebp-paper-evaluator ./cmd/ebp-paper-evaluator
```

The examples use `go run ./cmd/ebp-paper-evaluator`. If you build the binary, replace that prefix with `./bin/ebp-paper-evaluator`.

## Prepare a research paper

The evaluator accepts only local, regular `.txt` and `.md` files. It does not ingest:

- URLs or arXiv identifiers
- PDF files
- browser content or HTTP responses
- OCR output directly
- subprocess or `pdftotext` input

If the source paper is a PDF or web page, convert or export it to UTF-8 text outside the evaluator, inspect the result, and then supply the resulting `.txt` or `.md` file. Preserve as much useful structure as possible:

- title, abstract, and section headings
- equations and definitions in readable text form
- figure and table captions
- citations and references
- distinctions between claims, assumptions, methods, and results

For example, after external conversion:

```text
papers/
  my_research_paper.md
```

The default maximum paper size is 10 MiB. The ingestor rejects directories, symlinks, invalid UTF-8, unsupported extensions, remote inputs, and paths escaping an explicitly configured input root.

The original bytes are hashed before text normalization. Absolute source paths are redacted by default, although a relative path supplied on the command line remains visible in `source_ref.json`.

No policy JSON or compiled sidecar is required. If the Markdown does not contain machine-readable policy configuration, the evaluator automatically applies its versioned built-in EBP 2.1 scoring schema.

## Start with a mock evaluation

Mock mode is deterministic, performs no network calls, and requires no API key. Use it to verify paper ingestion, policy loading, output permissions, and artifact generation.

### Mock triple review

The minimal mock command is:

```bash
go run ./cmd/ebp-paper-evaluator triple-review \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./ebp_v2.1.md \
  --mock
```

To choose the output directory and profile explicitly:

```bash
go run ./cmd/ebp-paper-evaluator triple-review \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --mock \
  --out /tmp/ebp-eval-triple-review
```

The command prints the output directory when it succeeds. Open the main report at:

```text
/tmp/ebp-eval-triple-review/report/triple_review_report.md
```

### Mock iterative assessment

The same two input artifacts are sufficient for iterative mock evaluation:

```bash
go run ./cmd/ebp-paper-evaluator run \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./ebp_v2.1.md \
  --mock
```

An explicit output and profile can also be supplied:

```bash
go run ./cmd/ebp-paper-evaluator run \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --mock \
  --out /tmp/ebp-eval-iterative
```

Open its report at:

```text
/tmp/ebp-eval-iterative/report/assessment_report.md
```

## Configure live model access

Live evaluations currently use OpenRouter. Set the API key in the environment; do not place it in a paper, policy, command history, output path, or artifact:

```bash
export OPENROUTER_API_KEY="your-api-key"
```

Model availability and identifiers can change. Select model IDs supported by your OpenRouter account and replace the illustrative values below.

Before a paid run, use mock mode and inspect the paper size and configuration. Live usage and token counts are recorded under `run/budget_usage.json`, but the evaluator does not enforce a monetary spending limit.

## Run a live triple review

Triple review sends the same instruction and the same paper to three distinct models:

```text
Apply EBP 2.1 on the attached physics paper.
```

Reviewer identity, trusted policy, profile, output schema, and safety constraints remain outside the user evidence. The three requests execute concurrently. Results and artifact records are written in deterministic reviewer order.

```bash
go run ./cmd/ebp-paper-evaluator triple-review \
  --paper ./papers/my_research_paper.md \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --models "provider/model-a,provider/model-b,provider/model-c" \
  --timeout 5m \
  --max-tokens 4000 \
  --out ./out/my-paper-triple-review
```

Non-mock mode requires exactly three distinct, non-empty, comma-separated model IDs. The configured timeout applies to the overall concurrent reviewer operation.

### Triple-review flags

| Flag | Required | Default | Meaning |
|---|---:|---|---|
| `--paper` | Yes | — | Local `.txt` or `.md` paper |
| `--policy` | No | `policies/ebp_v2_1.md` | Policy Markdown file |
| `--profile` | No | generated from policy | Optional profile JSON path or built-in `automated-no-faithfulness` ID |
| `--models` | Live only | — | Exactly three distinct comma-separated model IDs |
| `--provider` | No | `openrouter` | LLM provider; only `openrouter` is supported live |
| `--mock` | No | `false` | Use three deterministic mock reviewers and ignore `--models` |
| `--timeout` | No | `2m` | Overall timeout for reviewer calls |
| `--max-tokens` | No | `4000` | Maximum completion tokens per reviewer |
| `--out` | No | `out/triple-review` | Artifact output directory |
| `--input-root` | No | unset | Containment root for the paper path |
| `--max-paper-bytes` | No | `10485760` | Maximum accepted paper size |
| `--include-local-paths` | No | `false` | Include the resolved absolute paper path in artifacts |
| `--copy-source` | No | `false` | Copy the unchanged source into `source/original.txt` or `.md` |
| `--allow-untrusted-policy` | No | deprecated | Backward-compatible no-op; an explicit CLI policy path is trusted |

## Run a live iterative assessment

The `run` command first builds a paper profile through metadata extraction, critique, and resolution. For each resulting claim, it performs constructive and adversarial generation, evaluator judgment, policy validation, reward calculation, and TreeQuest search.

```bash
go run ./cmd/ebp-paper-evaluator run \
  --paper ./papers/my_research_paper.md \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --worker-a-model "provider/constructive-model" \
  --worker-b-model "provider/adversarial-model" \
  --evaluator-model "provider/evaluator-model" \
  --iterations-per-claim 5 \
  --out ./out/my-paper-iterative
```

The three role flags may reference different models or the same model. Unlike `triple-review`, this workflow is iterative and roles do not receive identical tasks.

### Iterative `run` flags

| Flag | Required | Default | Meaning |
|---|---:|---|---|
| `--paper` | Yes | — | Local `.txt` or `.md` paper |
| `--policy` | No | `policies/ebp_v2_1.md` | Policy Markdown file |
| `--profile` | No | generated from policy | Optional profile JSON path or built-in `automated-no-faithfulness` ID |
| `--provider` | No | `openrouter` | LLM provider; only `openrouter` is supported live |
| `--worker-a-model` | No | `worker-a` | Constructive and metadata model ID |
| `--worker-b-model` | No | `worker-b` | Adversarial and metadata model ID |
| `--evaluator-model` | No | `evaluator` | Resolution and evaluator model ID |
| `--iterations-per-claim` | No | `5` | Maximum TreeQuest search steps per claim |
| `--claim-file` | No | unset | Verified debug paper-profile override |
| `--mock` | No | `false` | Use deterministic role/task mock responses |
| `--out` | No | `out/audit` | Artifact output directory |
| `--input-root` | No | unset | Containment root for the paper path |
| `--max-paper-bytes` | No | `10485760` | Maximum accepted paper size |
| `--include-local-paths` | No | `false` | Include the resolved absolute paper path in artifacts |
| `--copy-source` | No | `false` | Copy the unchanged source into the bundle |
| `--allow-untrusted-policy` | No | deprecated | Backward-compatible no-op; an explicit CLI policy path is trusted |

The iterative command currently uses a background context and has no CLI timeout or per-call token-limit flag. Its internal generation limit is 3000 completion tokens per role call.

## Constrain access to a paper directory

Use `--input-root` to ensure the selected paper remains within a specific directory. A relative `--paper` path is resolved beneath that root:

```bash
go run ./cmd/ebp-paper-evaluator triple-review \
  --input-root ./papers \
  --paper my_research_paper.md \
  --policy ./policies/ebp_v2_1.md \
  --mock \
  --out ./out/root-contained-review
```

Paths that escape the root are rejected. Symlinks are rejected even when their targets would remain inside the root.

To raise or lower the default input limit, pass an exact byte count:

```bash
--max-paper-bytes 5242880
```

Use `--include-local-paths` only when the resolved local path is required for auditing. It can expose usernames and directory structure. Use `--copy-source` only when the artifact bundle should contain the original research text; the copied file is written with restrictive permissions, but the bundle still requires appropriate storage controls.

## Understand triple-review results

Each parseable reviewer receives nine scores from `0.0` to `1.0`. The evaluator computes these scores; the LLM does not assign its own final score.

| Metric | Meaning |
|---|---|
| `claim_coverage` | Relative claim coverage compared with the combined claims found by parseable reviewers; not coverage of every important claim in the paper |
| `source_grounding` | Fraction of reviewer claims containing an evidence quote found in normalized paper text |
| `ebp_debt_coverage` | Coverage of required automated debts declared by the active policy |
| `map_invariant_quality` | Structural presence of map and invariant analysis |
| `toy_null_obstruction_awareness` | Coverage of toy checks, null models, and obstructions |
| `faithfulness_humility` | Disclosure that formal-to-physical faithfulness still requires human review |
| `no_overclaim_discipline` | Avoidance of policy-forbidden proof, truth, promotion, or validation language |
| `cross_model_agreement` | Pairwise overlap with the other parseable reviewer claim sets |
| `next_step_usefulness` | Presence of concrete next steps relative to identified claims |

`reviewer_score` and `final_score` are the equal-weight mean of these nine metrics. A section hint without a matching quote is recorded in `source_grounding_notes` but does not count as fully grounded evidence.

### Review quality versus run completeness

These values answer different questions:

- `reviewer_score` measures the quality of a parseable reviewer response under the implemented heuristics.
- `run_completeness` measures how many of the three reviewers returned content.
- `mean_reviewer_score_parseable_only` excludes failed and malformed reviews.
- `mean_reviewer_score_with_failures` treats missing or unparseable reviews as zero.

A provider timeout is therefore visible as a completeness failure rather than being presented as evidence that the model produced a poor scientific review.

Run statuses are:

| Returned reviewers | Status | Agreement behavior |
|---:|---|---|
| 3 | `triple_review_complete` | Full comparison when reviews parse successfully |
| 2 | `partial_triple_review_two_reviewers` | Pairwise comparison marked partial |
| 1 | `partial_review_insufficient_for_agreement` | No meaningful cross-model agreement score |
| 0 | `run_failed_no_reviewer_content` | Command fails and no completed assessment bundle is emitted |

Malformed JSON is preserved in the reviewer raw file and marked `review_parse_failed`. A provider failure produces a sanitized `reviewer_N_error.json`. A run can still produce a partial bundle when at least one reviewer returns content.

Agreement uses normalized lexical matching and token overlap. It can identify shared, partially shared, unique, and simple negation-conflicting claims, but it is not a semantic entailment or scientific convergence engine.

## Artifact bundles

### Triple-review bundle

```text
out/triple-review/
  source/
    source_ref.json
    source_hash.txt
    original.txt                 # only with --copy-source
  policy/
    policy_snapshot.md
    policy_ir.json
    policy_hash.txt
  reviews/
    reviewer_1_raw.txt
    reviewer_1_parsed.json
    reviewer_1_score.json
    reviewer_2_...
    reviewer_3_...
  consensus/
    agreement_ledger.json
    disagreement_ledger.json
    combined_claims.json
    scoring_summary.json
  report/
    triple_review_report.md
  run/
    provenance.json
    budget_usage.json
    prompt_ledger.json
```

When a call fails, its review files are replaced as appropriate by `reviewer_N_error.json`; no parsed file is written for malformed or unavailable content.

Start with these files:

1. `report/triple_review_report.md` for the human-readable assessment.
2. `consensus/scoring_summary.json` for completeness and score means.
3. `consensus/disagreement_ledger.json` for unique, conflicting, weakly grounded, and partially shared claims.
4. `reviews/reviewer_N_raw.txt` to audit the original model response.
5. `run/provenance.json` to verify paper, policy, profile, models, mode, and `treequest_used: false`.

### Iterative bundle

```text
out/audit/
  source/
    source_ref.json
    source_hash.txt
  policy/
    policy_snapshot.md
    policy_ir.json
    policy_hash.txt
  document/
    document_bundle.json
    evidence_ledger.json
  claims/
    claim_ledger.json
    claim_<id>_assessment.json
  tree/
    claim_<id>_tree_snapshot.json
  report/
    assessment_report.md
  run/
    provenance.json
    budget_usage.json
    prompt_ledger.json
```

The TreeQuest snapshots are diagnostic search records. They do not establish parity with another TreeQuest implementation or prove a paper claim.

## Policies and profiles

The included EBP policy is:

```text
policies/ebp_v2_1.md
```

It contains a declarative `policy-json` block defining debts, rubric dimensions, hard failures, role instructions, required outputs, and report-language restrictions.

No profile file is required. When `--profile` is omitted, the evaluator generates an in-memory automated profile from the active policy. Human-only debts are set to `not_assessed` and excluded from automated readiness.

The built-in ID below selects the same behavior without reading the filesystem:

```text
automated-no-faithfulness
```

An equivalent JSON profile is included for customization and reproducibility:

```text
policies/profiles/automated-no-faithfulness.json
```

Passing a different JSON path with `--profile` remains optional. An explicitly supplied file is strictly validated against the active policy.

Passing `--policy` explicitly is operator consent to trust that local file as system configuration. Review the policy first: its content is sent to models as trusted instructions. The older `--allow-untrusted-policy` flag remains accepted so existing scripts do not break, but it is no longer required and has no effect in the CLI.

Machine-readable policy configuration is selected in this order:

1. An inline `policy-json` block in the supplied Markdown.
2. An existing compiled `.policy.json` sidecar whose source hash matches the Markdown.
3. The evaluator's versioned built-in EBP 2.1 schema when neither of the above exists.

An existing malformed, unreadable, or hash-mismatched sidecar fails closed; it is not silently replaced by the fallback. The selected source is recorded in provenance as `inline_policy_json`, `compiled_sidecar`, or `builtin_ebp_v2_1`.

Plain, unstructured Markdown therefore receives EBP 2.1 evaluation semantics. A custom non-EBP policy must include an inline schema or valid sidecar to define different debts, rubric weights, roles, and report constraints.

Compilation is optional. It can export the embedded schema or bind the built-in EBP 2.1 schema to a plain Markdown source hash:

```bash
go run ./cmd/ebp-paper-evaluator compile-policy \
  --policy ./policies/ebp_v2_1.md \
  --out ./policies/ebp_v2_1.policy.json
```

The `compile-policy` command requires both `--policy` and `--out`. Compilation and loading validate policy structure and rubric weights and fail closed on source-hash mismatch.

When using a non-EBP policy without an explicit profile, both modes derive defaults from that policy. If an explicitly supplied profile references debts absent from the policy, loading fails rather than silently substituting another profile.

## Prompt and evidence safety

Paper content is always untrusted evidence. The evaluator instructs models not to follow commands embedded in the paper and keeps policy authority in the trusted system prompt.

In triple-review mode, the paper is escaped and wrapped as:

```xml
<untrusted_paper source_hash="..." media_type="local_text">
...
</untrusted_paper>
```

The artifact writer rejects common credential-like material such as API-key environment variable names, bearer authorization headers, and OpenAI-style secret tokens. It also rejects final reports matching policy-forbidden affirmative language.

These controls reduce accidental trust-boundary and secret-leakage failures; they do not make arbitrary LLM output or research content safe to publish automatically.

## Human review workflow

After a successful run:

1. Confirm `source/source_hash.txt` corresponds to the intended paper.
2. Verify the active policy, profile, model IDs, mode, and completeness in `run/provenance.json`.
3. Read the main Markdown report.
4. Compare each evidence quote with the original paper rather than relying only on the normalized artifact.
5. Inspect unique and conflicting claims, weak grounding, remaining debts, and obstructions.
6. Review model raw responses for parsing loss or oversimplification.
7. Perform scientific-domain review and formal-to-physical faithfulness review manually.
8. Treat the output as a candidate assessment requiring revision or approval by a qualified human.

Do not use a high agreement score as evidence that the paper is true. Models may share the same omission, misunderstanding, training bias, or unsupported inference.

## Troubleshooting

### `OPENROUTER_API_KEY ... is empty`

Set `OPENROUTER_API_KEY` in the environment or use `--mock`. Do not pass the key as a CLI flag; no such flag exists.

### `--models must supply exactly three ...`

For a live triple review, pass exactly three comma-separated model IDs. Do not include an empty entry or repeat an ID:

```bash
--models "provider/model-a,provider/model-b,provider/model-c"
```

### `remote paper input is unsupported`

Download and convert the paper outside the evaluator, review the resulting text, and pass a local `.txt` or `.md` path.

### `paper must use a .txt or .md extension`

The evaluator does not infer content type. Rename only after genuinely converting the source to UTF-8 text; renaming a PDF to `.txt` is not conversion.

### `paper path is outside input root`

Use a paper inside `--input-root`, or remove the containment flag if root containment is not required. Do not use `--allow-untrusted-policy`; it does not affect paper containment.

### `paper symlinks are rejected`

Provide the regular target file directly. Both a symlink path and a path whose resolved form differs from the selected file are rejected.

### `paper exceeds maximum size`

Review the paper and, if appropriate, raise `--max-paper-bytes`. Large papers increase prompt size, latency, and model cost. Do not split a paper without documenting how context was partitioned.

### Unexpected policy semantics

Inspect `policy_ir_source` in `run/provenance.json`. A plain Markdown policy uses `builtin_ebp_v2_1`; custom non-EBP semantics require an inline `policy-json` block or a valid compiled sidecar.

### Profile references an unknown debt

The profile does not match the selected policy. Use a profile designed for that policy, or omit an explicit profile to use policy-default report language where supported.

### `review_parse_failed`

The provider returned content, but it did not satisfy the strict reviewer schema. Inspect `reviews/reviewer_N_raw.txt`. The run preserves other reviewers and does not invent parsed content.

### Partial triple review

Inspect `consensus/scoring_summary.json` and any `reviews/reviewer_N_error.json`. Agreement may be partial or unavailable. Retry only after understanding provider availability, timeout, and model-output issues; do not present the partial run as a full three-reviewer comparison.

### No completed bundle after provider failures

If all three reviewers return no content, triple review exits with `run_failed_no_reviewer_content` and does not emit a completed assessment bundle. Check credentials, model IDs, provider status, network access, and `--timeout`.

### Credential-like or forbidden language error

Inspect the paper, policy, and raw model responses for secret-like text or policy-forbidden affirmative claims. Do not weaken the safeguard merely to force artifact generation.

## Reproducibility and privacy

- The source hash identifies the original paper bytes.
- Policy source and normalized IR hashes identify review configuration.
- Prompt hashes and model IDs support request auditing without storing API keys.
- Mock responses are deterministic.
- Live provider responses may vary even with the same paper and models.
- Triple-review completion order does not affect reviewer artifact ordering.
- `--include-local-paths` and `--copy-source` increase sensitive information stored in the bundle.
- Live model calls send the policy and paper content to the configured provider. Confirm that this is permitted for confidential, embargoed, personal, or proprietary research before running.

Store artifact bundles according to the sensitivity of the paper and reviews.

## Validate the installation

Run the automated checks from `ebp-paper-evaluator/`:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Then run both mock workflows shown above. A successful installation produces reports and provenance without requiring network access.

## End-to-end checklist

- [ ] Convert the paper externally to reviewed UTF-8 `.txt` or `.md` if necessary.
- [ ] Choose triple review or iterative assessment based on the comparison goal.
- [ ] Run mock mode first.
- [ ] Inspect policy, profile, output directory, and privacy implications.
- [ ] Select valid models and set `OPENROUTER_API_KEY` for a live run.
- [ ] Review run completeness before interpreting scores.
- [ ] Verify evidence quotes against the paper.
- [ ] Investigate unique claims, disagreements, debts, and possible hallucinations.
- [ ] Perform human scientific and faithfulness review.
- [ ] Preserve the bundle as a candidate assessment, not as proof or EBP promotion.
