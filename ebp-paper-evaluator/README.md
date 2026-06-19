# EBP Paper Evaluator

`ebp-paper-evaluator` is a CLI-only declarative paper-policy evaluator. It imports the unchanged generic `treequest-go` library.

Paper input is restricted to local UTF-8 `.txt` and `.md` regular files. URL, arXiv, PDF, browser, OCR, remote-fetch, and subprocess-based document ingestion are not supported.

```bash
go run ./cmd/ebp-paper-evaluator run \
  --paper ./papers/quantum_gravity.txt \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --mock \
  --out ./out/audit
```

Constrain paper lookup to a local directory:

```bash
go run ./cmd/ebp-paper-evaluator run \
  --input-root ./papers \
  --paper quantum_gravity.txt \
  --policy ./policies/ebp_v2_1.md \
  --mock \
  --out ./out/audit
```

Local symlinks, root escapes, directories, invalid UTF-8, unsupported extensions, and oversized files are rejected. Original bytes are hashed before normalization. Absolute local paths are redacted from artifacts unless `--include-local-paths` is supplied. `--copy-source` optionally copies the unchanged source bytes into the bundle.

Paper content is untrusted evidence and is isolated from trusted policy/system instructions in every A/B/E prompt. Claims are extracted through metadata consensus; `--claim-file` is a verified debugging override.

Compile a policy:

```bash
go run ./cmd/ebp-paper-evaluator compile-policy \
  --policy ./policies/ebp_v2_1.md \
  --out ./policies/ebp_v2_1.policy.json
```

User policies outside trusted local policy roots require `--allow-untrusted-policy`. Policy compilation and hash validation fail closed without hardcoded EBP fallback.

The automated no-faithfulness profile records faithfulness review as `not_assessed`. Generated artifacts are automated candidate assessments requiring human review, not rewritten papers.
