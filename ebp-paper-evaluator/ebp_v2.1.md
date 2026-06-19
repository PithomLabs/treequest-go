# Elephant Bridge Protocol v2.1 — Revised Minimal Operational Kernel

## Ideas Enter Free. Promotion Costs Debt.

**Document status:** Working specification  
**Version:** 2.1 revised / operational patch 2.1.1 folded in  
**Purpose:** Define the smallest practical form of the Elephant Bridge Protocol that preserves imagination while preventing unearned promotion.  
**Primary contract language:** Lean-style structures, inductive types, definitions, theorem obligations, counterexamples, and executable checks.  
**Schema policy:** No YAML-first or JSON-first schema. Prose is canonical at entry. Lean-shaped structures are the promotion and debt-payment language.  
**Tool policy:** Tool-agnostic. Lean, PTW, GitHub, local files, databases, Coq, Isabelle, notebooks, and future systems may implement the protocol, but none owns it.

---

## 0. Executive Summary

EBP v2.1 is the minimal operational form of the Elephant Bridge Protocol.

Its core doctrine is:

```text
Ideas enter free.
Promotion costs debt.
Debt does not kill.
Debt is forever payable.
New evidence creates new debt.
No final-truth claim may be promoted.
Accounting must never become the work.
```

The protocol exists to protect two things simultaneously:

1. **The theorist's imagination.** Any idea may enter as a one-line claim.
2. **The search's epistemic integrity.** No idea may be promoted until its unpaid obligations are visible and sufficiently retired.

EBP v2.1 is not a theory of everything, not a theory generator, not a truth engine, not a replacement for peer review, not a Lean supremacy doctrine, and not a new gatekeeping institution.

It is a **notebook with a conscience**.

A notebook preserves imagination.  
A conscience prevents overclaiming.

The shortest summary is:

```text
Door: wide open.
Throne: guarded by debt.
```

Ideas enter through the door.  
Only debt-paid ideas approach the throne.

---

## 1. Why v2.1 Exists

EBP v1 preserved the original Blind Men and the Elephant spirit: each theory may touch a real part of the whole, but no theory should be accepted wholesale or rejected wholesale without extracting the features that survive.

EBP v2 corrected a deeper problem: even the protocol's own favorite assumptions can become dogma. TBA, E8, optimality, Bohmian decomposition, holography, relational amplitude spaces, and Lean formalization may be valuable tools or candidate lenses, but none should be sacred.

EBP v2.1 corrects the operational problem: a protocol designed to fight confusion can itself become bureaucracy if it demands too much structure too early.

Therefore v2.1 separates **entry** from **promotion**.

```text
Entry is cheap.
Promotion is expensive.
```

An idea may enter with no map, no invariant, no Lean object, no null model, no obstruction, and no proof.

But it may not be promoted while those debts remain unpaid.

This is the central anti-bureaucracy move.

---

## 2. Core Doctrine

### 2.1 One Entry Point

```text
The only mandatory entry is an Idea.
```

The smallest idea record is:

```text
owner + claim
```

Optional:

```text
source
born timestamp
notes
```

No map required.  
No invariant required.  
No classification required.  
No Lean required.  
No review required.  
No null model required.  
No obstruction required.

### 2.2 Debt Does Not Kill

```text
Debt does not kill.
Debt only blocks promotion.
```

An idea with unpaid debt is still alive. It may be dormant, unfinished, vague, or immature, but it is not rejected merely because it is unpaid.

### 2.3 Debt Is Forever Payable

```text
Debt is forever payable.
```

An idea from 2024 may sit dormant until 2029, then be repaired and promoted. There is no expiration date and no shame in incompleteness.

### 2.4 Promotion Costs Debt

```text
Promotion means current operational debt has been retired.
```

Promotion does not mean the idea is true. It means the idea is mature enough to be treated as a serious candidate artifact.

### 2.5 New Evidence Creates New Debt

```text
A promoted idea can become unpromoted if new evidence creates new debt.
```

A new obstruction, counterexample, stronger null model, or faithfulness problem reinstates debt. The idea remains alive, but promotion is suspended until the new debt is addressed.

### 2.6 No Final-Truth Claim May Be Promoted

```text
Final-truth language blocks promotion until repaired.
```

Examples of final-truth language:

```text
This proves the final theory of everything.
This is the true structure of nature.
This solves quantum gravity.
This proves E8 is physical reality.
This settles the problem.
```

Allowed language:

```text
This is a candidate bridge.
This is a toy-supported map.
This is a formal obligation.
This is a promoted research artifact under current debt conditions.
This survives the current null models.
```

### 2.7 Accounting Must Never Become the Work

```text
The accounting exists only to prevent accidental promotion.
```

If accounting becomes the main activity, the protocol has failed.

### 2.8 Lean Is an Instrument, Not the Door

```text
Lean is not required for entry.
Lean is one of the strongest ways to retire debt.
```

The canonical entry artifact is prose. Lean becomes important when an idea tries to pay debt through definitions, theorem obligations, executable toy checks, counterexamples, or no-go results.

---

## 3. The Minimum Pipeline

The whole pipeline is:

```text
Capture -> Clarify -> Debt -> Test -> Promote
```

The theorist should experience it as:

```text
I can throw an idea into the workbench, and it quietly tracks what the idea still owes.
```

### 3.1 Capture

One line:

```text
Boundary capacity may unify area and entropy.
```

The idea enters alive.

### 3.2 Clarify

The theorist may add prose, sketches, equations, metaphors, sources, or rough maps.

Clarification is creative, not clerical.

### 3.3 Debt

The system automatically attaches default debt.

Debt is not punishment. It is the idea's unpaid homework.

### 3.4 Test

The theorist retires one debt at a time by supplying a map, invariant, toy check, null model, obstruction response, or faithfulness review.

### 3.5 Promote

Promotion is allowed only when current debt is retired and the claim contains no final-truth language.

---

## 4. Minimal Lean Kernel

EBP v2.1 is Lean-first in the sense that its operational law can be represented as Lean-shaped definitions.

It is not Lean-first in the sense of requiring every idea to begin in Lean.

### 4.1 Timestamp

Use a minimal timestamp abstraction. Do not require real date libraries in the protocol kernel.

```lean
abbrev Timestamp := Nat
```

### 4.2 Debt Items

```lean
inductive DebtItem
| needMap
| needInvariant
| needToyCheck
| needNullModel
| needObstruction
| needFaithfulnessReview
deriving DecidableEq, Repr
```

### 4.3 Idea

```lean
structure Idea where
  owner : String
  claim : String
  source : Option String
  born : Timestamp
  debt : List DebtItem
  containsFinalTruthClaim : Bool
deriving Repr
```

### 4.4 Full Starting Debt

```lean
def fullDebt : List DebtItem :=
  [ DebtItem.needMap
  , DebtItem.needInvariant
  , DebtItem.needToyCheck
  , DebtItem.needNullModel
  , DebtItem.needObstruction
  , DebtItem.needFaithfulnessReview
  ]
```

### 4.5 Fresh Idea

```lean
def freshIdea
  (owner claim : String)
  (source : Option String)
  (born : Timestamp)
  : Idea :=
{
  owner := owner,
  claim := claim,
  source := source,
  born := born,
  debt := fullDebt,
  containsFinalTruthClaim := false
}
```

### 4.6 Alive and Promoted

```lean
def ideaAlive (_ : Idea) : Bool :=
  true


def ideaPromoted (i : Idea) : Bool :=
  i.debt.isEmpty && !i.containsFinalTruthClaim
```

This is the whole operational philosophy:

```text
Every idea is alive.
Only debt-free, non-final-truth ideas are promoted.
```

### 4.7 Retire Debt

```lean
def retireDebt (d : DebtItem) (i : Idea) : Idea :=
  { i with debt := i.debt.filter (fun x => x != d) }
```

### 4.8 Add Debt Without Duplicates

```lean
def addDebt (d : DebtItem) (i : Idea) : Idea :=
  if i.debt.contains d then i else { i with debt := d :: i.debt }
```

### 4.9 File Obstruction

```lean
def fileObstruction (i : Idea) : Idea :=
  addDebt DebtItem.needObstruction i
```

### 4.10 Mark Final-Truth Language

```lean
def markFinalTruthClaim (i : Idea) : Idea :=
  { i with containsFinalTruthClaim := true }
```

### 4.11 Repair Final-Truth Language

```lean
def clearFinalTruthClaim (i : Idea) : Idea :=
  { i with containsFinalTruthClaim := false }
```

This represents revision, not punishment.

---

## 5. The Door and the Throne

EBP v2.1 has two regimes.

### 5.1 The Door

The door is wide open.

```text
Any idea may enter.
No premature formalism.
No punishment for incompleteness.
No gatekeeping of imagination.
No Lean requirement.
No profile requirement.
No taxonomy requirement.
```

The door protects creativity.

### 5.2 The Throne

The throne is guarded by debt.

```text
No unpaid claim is promoted.
No final-truth language survives.
No metaphor becomes a map without work.
No uniqueness claim escapes null models.
No formal theorem escapes faithfulness review.
```

The throne protects rigor.

### 5.3 Operational Summary

```text
Gentle at the door.
Brutal at the throne.
```

---

## 6. Debt Items and How to Retire Them

Debt retirement is not bureaucracy. It is one useful move at a time.

```text
One useful move retires one debt.
```

### 6.1 `needMap`

An idea owes a map when it claims a relation between structures, theories, regimes, or descriptions.

Examples:

```text
E8 structure -> gauge symmetry
boundary area -> information capacity
spin-network punctures -> entropy contribution
CFT state count -> boundary distinguishability
```

Minimal retirement condition:

```text
State domain, codomain, and translation rule.
```

Lean shape:

```lean
structure CandidateMap where
  Domain : Type
  Codomain : Type
  map : Domain -> Codomain
```

A candidate map does not prove the bridge. It only makes the claim inspectable.

### 6.2 `needInvariant`

An idea owes an invariant when it claims that something survives translation, emergence, approximation, duality, reconstruction, or comparison.

Examples:

```text
capacity
entropy scaling
spectrum
symmetry
causal order
correlation structure
anomaly cancellation
code distance
```

Minimal retirement condition:

```text
State the invariant sharply enough that it can be preserved, violated, or compared.
```

Lean shapes:

```lean
structure CandidateInvariant (X : Type) where
  holds : X -> Prop
```

or:

```lean
structure NumericalInvariant (X : Type) where
  value : X -> Nat
```

### 6.3 `needToyCheck`

An idea owes a toy check when it has a map and invariant but no finite test.

Toy checks are wind tunnels.

```text
Toy success does not prove reality.
Toy failure can kill bad structure early.
```

Exact preservation:

```lean
def preservesInvariant
  {A B : Type}
  (f : A -> B)
  (IA : A -> Nat)
  (IB : B -> Nat)
  : Prop :=
  forall a : A, IB (f a) = IA a
```

Scaling or ordering preservation:

```lean
def preservesOrdering
  {A B : Type}
  (f : A -> B)
  (IA : A -> Nat)
  (IB : B -> Nat)
  : Prop :=
  forall a1 a2 : A, IA a1 <= IA a2 -> IB (f a1) <= IB (f a2)
```

Exact preservation and scaling preservation must never be confused.

### 6.4 `needNullModel`

An idea owes null-model work when it claims uniqueness, necessity, superiority, or special explanatory status.

A null model asks:

```text
Can a simpler or rival model explain the same thing?
```

Minimal retirement condition, option A:

```text
Supply at least one null model.
```

Minimal retirement condition, option B:

```text
Explicitly disclaim uniqueness, necessity, or superiority.
```

This matters. If an idea does not claim uniqueness, it should not be forced into unnecessary null-model labor. It may retire `needNullModel` by saying:

```text
I do not claim this bridge is necessary or unique. I only claim compatibility worth exploring.
```

Lean shape:

```lean
structure NullModel where
  Carrier : Type
  explains : Prop
```

A null model does not automatically refute an idea. It blocks inflated claims.

### 6.5 `needObstruction`

An idea owes obstruction work when known blockers, no-go results, or contradiction risks apply.

Examples:

```text
chirality obstruction
anomaly obstruction
Lorentz recovery problem
Born rule problem
continuum-limit problem
faithfulness problem
parameter padding
normalization artifact
```

Minimal retirement condition:

```text
State one obstruction, explain whether it applies, and record the consequence.
```

Possible consequences:

```text
obstruction does not apply
idea must be modified
idea fails as stated
idea downgraded to analogy only
idea remains open with known blocker
```

Lean shape:

```lean
structure Obstruction where
  assumptions : Prop
  forbidden : Prop
  noGo : assumptions -> Not forbidden
```

Retiring obstruction debt does not always mean victory. Sometimes it means honest downgrade.

### 6.6 `needFaithfulnessReview`

Faithfulness review asks one question:

```text
Does the formalization match the intended claim?
```

Allowed answers:

```text
yes
no
contested
```

Minimal contested rule:

```text
If contested, the idea remains unpromoted.
The contesting party must state one sentence explaining the mismatch.
The owner may repair, revise, or accept the block.
```

No silent vetoes.

No committee required at the minimum layer.

Faithfulness review matters because Lean can prove the wrong theorem if the definitions are too weak, distorted, or smuggle in the conclusion.

---

## 7. Final-Truth Claim Handling

The field:

```lean
containsFinalTruthClaim : Bool
```

is not meant to be a punishment mechanism.

It is a promotion blocker.

### 7.1 Owner-Declared, Reviewer-Flaggable, Repairable

The owner should be asked:

```text
Does this claim assert final truth about nature?
```

If yes, promotion is blocked until rephrased.

A tool or reviewer may flag possible final-truth language, but the fix is revision, not punishment.

### 7.2 Examples

Blocked:

```text
This proves the final theory.
This is the true substrate of nature.
This solves quantum gravity.
```

Repairable as:

```text
This is a candidate bridge.
This is a proposed substrate hypothesis.
This is a toy-supported formal obligation.
This is a promoted artifact under current debt conditions.
```

### 7.3 Minimal Rule

```text
Final-truth language does not kill the idea.
It blocks promotion until repaired.
```

---

## 8. Dormant Versus Active

The protocol may optionally distinguish attention state.

```lean
inductive Attention
| active
| dormant
deriving DecidableEq, Repr
```

But this must never become a hidden death sentence.

```text
Dormant means “not currently being worked on,” not “less valid,” “rejected,” or “dead.”
```

Operational rules:

```text
Alive does not mean active.
Unpromoted does not mean prioritized.
Dormant does not mean dead.
Only debt-moving ideas should consume active attention.
```

This prevents an infinite idea graveyard from consuming focus while preserving the right of unfinished ideas to remain in the room.

---

## 9. The Living Ledger

The ledger is living because knowledge is living.

An idea can move like this:

```text
alive with debt -> promoted -> new obstruction filed -> alive with debt again -> repaired -> promoted again
```

There is no shame in returning to debt.

The purpose is not to protect prestige. The purpose is to preserve contact with constraint.

### 9.1 The Only Temporal Rule

```text
New evidence creates new debt.
```

No scheduled reviews.  
No automatic expiration.  
No death penalty for unfinished ideas.

### 9.2 Reinstating Debt

A new obstruction:

```lean
def fileObstruction (i : Idea) : Idea :=
  addDebt DebtItem.needObstruction i
```

A new faithfulness concern:

```lean
def fileFaithfulnessConcern (i : Idea) : Idea :=
  addDebt DebtItem.needFaithfulnessReview i
```

A new null-model challenge:

```lean
def fileNullModelChallenge (i : Idea) : Idea :=
  addDebt DebtItem.needNullModel i
```

These operations do not delete the idea. They reopen work.

---

## 10. Seamless Accounting

The accounting process should be mostly invisible.

The theorist should write naturally. The system should infer debt movements when possible.

Example:

```text
The invariant is boundary distinguishability capacity I(B).
```

System inference:

```text
needInvariant retired or partially retired.
```

Example:

```text
Map area A(B) to capacity I(B) by monotone scaling.
```

System inference:

```text
needMap partially retired.
needToyCheck still unpaid.
```

Example:

```text
A tensor-network-only explanation may reproduce the same capacity behavior.
```

System inference:

```text
needNullModel retired or partially retired.
```

The user should not need to think like a clerk.

The protocol should behave like a quiet assistant.

---

## 11. The Three-Lane Workflow

The minimal user-facing workflow has three lanes:

```text
Inbox -> Workbench -> Promoted
```

### 11.1 Inbox

Raw ideas.

Required:

```text
owner
claim
```

### 11.2 Workbench

Ideas with active debt-paying work.

Each card shows only:

```text
claim
owner
debt checklist
next smallest useful move
notes/history
```

### 11.3 Promoted

Ideas with current debt retired and no final-truth language.

Promoted means:

```text
serious candidate artifact
```

not:

```text
truth
```

---

## 12. The Next Smallest Useful Move

Every idea should always have one visible next move.

Examples:

```text
Define the invariant in one sentence.
Name the domain and codomain.
Write the simplest map.
State that uniqueness is not claimed.
Find one null model.
State one obstruction.
Run one toy check.
Ask whether the formalization matches the claim.
Remove final-truth language.
```

The next move should be small enough to do now.

If a task cannot be reduced to a next smallest useful move, it is too large.

---

## 13. What to Remove Ruthlessly

EBP v2.1 removes anything that does not prevent a failure worse than its overhead.

Kill at entry:

```text
mandatory profiles
mandatory theory taxonomy
mandatory bridge type
mandatory Lean formalization
priority scoring
cost estimates
maturity-stage bureaucracy
multi-reviewer approval
full S+ records
large reports
dashboards before artifacts
formalization-cost estimates
incubation tracks
```

Keep:

```text
Idea
Debt
Retire debt
Reintroduce debt
Promotion requires debt paid
Final-truth claims block promotion
```

The ruthless test for every process:

```text
What failure mode does this prevent?
Is that failure worse than the overhead this process creates?
```

If not, kill the process.

---

## 14. Minimal Session Rule

Every working session should produce at least one of these:

```text
new idea captured
one debt retired
one obstruction filed
one null model filed
one toy check run
one faithfulness issue found
one final-truth phrasing repaired
one idea promoted or unpromoted
```

If a session produces none of these, it was probably entropy.

---

## 15. Minimum Implementation Surface

The first tool should be almost embarrassingly small.

Required commands:

```text
ebp capture
ebp status
ebp retire
ebp add-debt
ebp promote
```

Optional, later:

```text
ebp note
ebp flag-final-truth
ebp clear-final-truth
ebp dormant
ebp active
ebp history
```

Do not start with dashboards, profiles, theorem-prover integration, scoring, reviewer hierarchy, or workflow automation.

### 15.1 Capture Example

```text
ebp capture "Alice" "Boundary capacity may unify area and entropy"
```

Output:

```text
Idea captured.
Status: alive, unpromoted.
Debt:
- needMap
- needInvariant
- needToyCheck
- needNullModel
- needObstruction
- needFaithfulnessReview

Next move:
Define the candidate invariant in one sentence.
```

### 15.2 Retire Example

```text
ebp retire Alice-0001 needInvariant "Invariant: distinguishability capacity I(B)."
```

Output:

```text
Debt retired: needInvariant.
Remaining debt: 5.
Next move: define map.
```

### 15.3 Status Example

```text
ebp status Alice-0001
```

Output:

```text
Alive: yes.
Promoted: no.
Dormant: no.
Final-truth claim: no.
Remaining debt:
- needMap
- needToyCheck
- needNullModel
- needObstruction
- needFaithfulnessReview
```

This is enough for the first implementation.

---

## 16. Relationship to Lean

Lean is not the entry gate. Lean is the strongest promotion instrument.

### 16.1 What Lean Is For

Lean helps retire debt by turning claims into:

```text
definitions
structures
inductive types
theorem stubs
proofs
counterexamples
no-go results
executable finite checks
```

### 16.2 What Lean Is Not For

Lean does not prove nature.

Lean does not decide physical faithfulness.

Lean does not replace empirical tests.

Lean does not replace human judgment.

Lean proves only that a formal result follows from formal assumptions.

### 16.3 Lean-First Means Promotion-First, Not Entry-First

Lean-first does not mean every idea must begin in Lean.

It means promoted claims should increasingly have Lean-shaped obligations and, where possible, Lean-checked cores.

### 16.4 The Faithfulness Gate

A Lean theorem can be correct and still irrelevant if it formalizes the wrong claim.

Therefore, every Lean-based debt retirement that supports promotion must face the faithfulness question:

```text
Does the formalization match the intended claim?
```

If contested, promotion waits.

---

## 17. Relationship to Tools

EBP v2.1 is tool-agnostic.

It may be implemented in:

```text
plain Markdown
Lean files
Git repositories
local databases
Pithom Theory Workbench
Coq or Isabelle
Jupyter-style notebooks
paper sidecars
formal artifact registries
```

But none of these is required for the protocol to exist.

The protocol is the operational law:

```text
Ideas enter free. Promotion costs debt.
```

---

## 18. Relationship to PTW

Pithom Theory Workbench should be overhauled later as an implementation of EBP v2.1, but EBP v2.1 is not based on PTW.

PTW should eventually provide:

```text
one-line idea capture
automatic debt attachment
seamless debt inference
three-lane workflow
Lean debt-payment modules
living ledger
promotion/unpromotion history
artifact export
```

But PTW must not reintroduce bureaucracy.

The PTW implementation rule should be:

```text
Make the accounting invisible.
Make the promotion criteria explicit.
```

---

## 19. Example: Causal Boundary Coin

### 19.1 Capture

```text
Owner: Researcher
Claim: Boundary information capacity may be the shared invariant behind area, entropy, spin labels, and CFT state counting.
Source: optional
```

The idea enters alive.

Debt:

```text
needMap
needInvariant
needToyCheck
needNullModel
needObstruction
needFaithfulnessReview
```

### 19.2 Clarify Invariant

Researcher adds:

```text
The invariant is boundary distinguishability capacity I(B).
```

Retire or partially retire:

```text
needInvariant
```

### 19.3 Clarify Map

Researcher adds:

```text
Area, state count, spin-boundary labels, and code-cut capacity each map into I(B) by capacity-preserving or capacity-scaling maps.
```

Retire or partially retire:

```text
needMap
```

### 19.4 Toy Check

A finite model tests exact preservation and scaling preservation.

Possible result:

```text
Exact preservation succeeds for one toy face.
Scaling preservation succeeds for another.
Exact equality is not claimed globally.
```

Retire:

```text
needToyCheck
```

or leave debt if the toy model fails.

### 19.5 Null Model

Researcher adds:

```text
A tensor-network-only explanation may reproduce the same capacity behavior.
```

Retire:

```text
needNullModel
```

but downgrade uniqueness claims.

Alternatively, the researcher says:

```text
I do not claim uniqueness. I only claim compatibility worth exploring.
```

That may also retire `needNullModel` for the early-stage version.

### 19.6 Obstruction

Researcher states:

```text
The bridge may fail if the maps preserve only chosen normalization rather than invariant structure.
```

Retire or modify:

```text
needObstruction
```

### 19.7 Faithfulness

Reviewer asks:

```text
Does the Lean toy invariant actually represent the intended boundary-capacity claim?
```

If yes:

```text
needFaithfulnessReview retired
```

If contested:

```text
Idea remains unpromoted.
Contesting party states one sentence explaining mismatch.
Owner repairs, revises, or accepts block.
```

### 19.8 Promotion

If all debts are retired and no final-truth language exists, the idea is promoted to a candidate bridge artifact.

It is not declared true.

It is simply mature enough to carry forward.

---

## 20. Example: E8 Gauge Symmetry Idea

### 20.1 Capture

```text
Claim: E8 structure may control Standard Model gauge symmetries.
```

It enters alive.

No rejection.

### 20.2 Debt

It owes:

```text
map
invariant
toy check
null model
obstruction
faithfulness review
```

### 20.3 Obstruction Appears

Someone files:

```text
Direct E8 unification faces chirality and representation-content problems.
```

This does not kill the idea.

It creates or preserves obstruction debt.

### 20.4 Possible Outcomes

The idea may become:

```text
promoted after serious repair
modified into E8-structural rather than E8-literal
left dormant
converted into a null model
rejected as a promoted claim but preserved as historical idea
```

Debt, not deletion, carries the epistemic state.

---

## 21. Promotion Is Not Truth

This must be repeated because it prevents overclaiming.

```text
Promoted means: current operational debt is paid.
Promoted does not mean: nature is proven.
```

A promoted idea can still be:

```text
wrong
incomplete
physically unfaithful
empirically false later
superseded by a better null model
reopened by new obstruction
```

Promotion is a status of research hygiene, not metaphysical victory.

---

## 22. The Anti-Bureaucracy Contract

EBP v2.1 must never become a paperwork machine.

Therefore:

```text
No form before imagination.
No mandatory schema at entry.
No mandatory Lean at entry.
No mandatory review at entry.
No mandatory classification at entry.
No large report unless it retires debt.
No dashboard before artifact.
No accounting that becomes the work.
```

Accounting exists only to prevent accidental promotion.

If accounting becomes the work, the protocol has failed.

---

## 23. The Anti-Fragile Epistemology

An anti-fragile research protocol improves when attacked.

EBP v2.1 welcomes:

```text
objections
counterexamples
null models
faithfulness failures
formalization failures
new no-go results
better rival explanations
```

because each creates clearer debt.

The purpose is not to protect ideas from criticism.

The purpose is to protect the search from unearned certainty.

---

## 24. The Minimum Viable Implementation

The first implementation should only do five things:

```text
capture
status
retire
add-debt
promote
```

If the first tool does only this, it is enough.

A feature-rich dashboard that does not help retire debt is worse than a tiny CLI that keeps ideas honest.

The first implementation should feel like:

```text
A notebook with a debt conscience.
```

not:

```text
A research-management platform.
```

---

## 25. Final Doctrine

```text
Ideas enter free.
Promotion costs debt.
Debt does not kill.
Debt is forever payable.
New evidence creates new debt.
No final-truth claim may be promoted.
Dormant means inactive, not dead.
Contested faithfulness blocks promotion but requires a stated reason.
Null-model debt can be paid by a null model or by disclaiming uniqueness.
Lean is not required for entry.
Lean is one of the strongest ways to retire debt.
The protocol is tool-agnostic.
The accounting must be seamless.
Creativity lives at the door.
Rigor guards the throne.
Accounting must never become the work.
```

Or shorter:

> **EBP v2.1 is a notebook with a conscience: wide open to imagination, ruthless against unearned promotion.**

