TreeQuest-Go can help **after** the current `simple_triple_review` baseline, but I would not put it inside the v0.1 “simple” path. The simple path has now earned its purpose: three independent reviewers, strict JSON, evaluator scoring, model suitability, and lexical agreement diagnostics. TreeQuest-Go becomes useful when you want to move from **one-shot evaluation** to **budget-aware iterative search**.

## The simplest answer

TreeQuest-Go can help by turning the evaluator from:

```text
3 reviewers → compare outputs → report
```

into:

```text
3 reviewers → compare outputs → identify weak spots
→ search possible refinements
→ test which refinement improves grounding/agreement/debt coverage
→ produce a better candidate review bundle
```

But it should remain an **advanced mode**, not part of `simple_triple_review`.

## Where TreeQuest-Go helps most

### 1. Low agreement diagnostics

Right now you found this situation:

```text
agreement_score = 0
but claims are semantically close
```

Example:

```text
Reviewer 2: quantum and classical energy components periodically interchange dominance

Reviewer 3: dBB Hamilton-Jacobi decomposition separates total energy... components periodically interchange dominance
```

Lexically, Jaccard says weak overlap. Semantically, a human sees they are close.

TreeQuest can explore candidate actions like:

```text
normalize_claim
merge_claim_pair
split_claim
request_claim_rewrite
ask_reviewer_to_ground_claim
ask_third_model_to adjudicate similarity
```

Then the evaluator can score whether the action improves:

```text
claim alignment
evidence grounding
debt coverage
overclaim safety
report clarity
```

Important guardrail: TreeQuest should only produce **candidate alignments**, not declare semantic truth.

### 2. Budget-aware refinement

Without TreeQuest, every improvement is manually designed. With TreeQuest, the evaluator can search under a budget:

```text
You have 10 LLM calls.
Which next action is most useful?
```

Possible actions:

```text
ask reviewer 2 to rewrite claims in canonical EBP form
ask reviewer 3 to attach exact evidence quotes
ask an adjudicator model whether claim A and claim B are equivalent
ask for missing null models
ask for strongest obstruction
ask for overclaim audit
ask for claim-debt remapping
```

TreeQuest helps choose which branch to explore rather than running every possible follow-up.

### 3. Reviewer/model selection

Your `model_suitability.json` now records which models are schema-compliant. TreeQuest could use that as search data.

For example:

```text
poolside/laguna-xs.2:free → schema_compliant
poolside/laguna-m.1:free → schema_compliant
nex-agi/nex-n2-pro:free → schema_compliant
```

TreeQuest can test reviewer pools:

```text
Which 3-model set maximizes parseability + grounding + agreement diversity under cost?
```

This would be useful for a later `model_pool_sweep` mode.

### 4. Better consensus construction

Simple mode currently compares claims lexically. TreeQuest could search over candidate consensus structures:

```text
consensus version A: conservative merge only
consensus version B: merge paraphrases with adjudicator support
consensus version C: keep claims separate but link as possible paraphrases
consensus version D: split broad claims into subclaims first
```

The reward function should prefer:

```text
more exact evidence quotes
less overclaiming
more explicit debt mapping
clearer obstruction ledger
no fake agreement
no fake promotion
```

### 5. Finding the next best EBP action

TreeQuest is especially useful for “what should we do next?” decisions.

Given a paper review bundle, it can search next actions like:

```text
extract equations
map claims to exact paper spans
generate null models
generate obstruction list
request human faithfulness review checklist
compare reviewer disagreement
produce Lean obligation stubs
produce Go toy-check proposal
```

This fits TreeQuest better than simple one-shot review.

## Where TreeQuest-Go should not help

I would not use TreeQuest-Go to:

```text
decide whether the paper is true
promote an EBP claim
replace human faithfulness review
replace strict JSON validation
silently merge claims as semantically equivalent
hide model disagreement
```

TreeQuest should be a **search assistant**, not an authority.

## Best architecture

Keep two modes:

```text
Mode 1: simple_triple_review
Purpose: stable v0.1 baseline
TreeQuest: false
Output: candidate assessment bundle

Mode 2: treequest_refine_review
Purpose: advanced iterative refinement
TreeQuest: true
Input: simple_triple_review bundle
Output: refinement tree + candidate improved report
```

The clean flow:

```text
local paper
   ↓
simple_triple_review
   ↓
candidate bundle
   ↓
treequest_refine_review
   ↓
search over refinement actions
   ↓
improved candidate bundle + search trace
```

This preserves your architectural boundary: `treequest-go` remains generic, and `ebp-paper-evaluator` defines the domain-specific state, actions, and reward function.

## Concrete v0.2 ticket idea

```text
EBP-EVAL-TQ-0001: TreeQuest-Guided Review Refinement
```

Scope:

```text
Input:
  existing simple_triple_review artifact bundle

Actions:
  normalize_claims
  propose_claim_merges
  request_evidence_grounding
  request_null_models
  request_obstructions
  request_overclaim_audit
  request_debt_remap
  generate_human_review_questions

Reward dimensions:
  parseability preserved
  evidence quote coverage
  EBP debt coverage
  obstruction quality
  null model coverage
  no-overclaim discipline
  agreement clarity
  next-step usefulness

Output:
  tree_snapshot.json
  refinement_trace.json
  candidate_refined_report.md
  no_promotion_audit.json
```

Non-goals:

```text
no paper truth claim
no EBP promotion
no human faithfulness review
no automatic semantic convergence claim
```

## My recommendation

Freeze `simple_triple_review` first as v0.1. Then add TreeQuest as a separate advanced refinement mode.

The current simple mode answers:

```text
What did three independent reviewers say?
```

TreeQuest mode should answer:

```text
Given what they said, what is the best next refinement path under limited budget?
```

## EBP/PTW self-audit

**needMap:** TreeQuest maps review artifacts into searchable refinement states/actions.

**needInvariant:** Must preserve strict parsing, provenance, candidate-only language, and no-promotion constraints.

**needToyCheck:** Start with synthetic bundles before real papers.

**needNullModel:** Compare TreeQuest-guided refinement against simple fixed-step refinement.

**needObstruction:** Main risk is semantic laundering: making weak agreement look stronger than it is.

**needFaithfulnessReview:** Still human/source-level; TreeQuest can generate checklists, not retire faithfulness debt.

**Promotion status:** TreeQuest can improve candidate review quality, but it cannot prove the paper, validate physics, or promote EBP claims.
