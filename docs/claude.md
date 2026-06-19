All three goals are right instincts. Let me go through each.

## 1 & 2 — Splitting into two repos

This is the correct move, and it falls directly out of taking "general-purpose" seriously. The dashed box in the diagram — `GenerateFn[S]` — is the *entire* surface area treequest-go should know about. Once you commit to that, EBP literally cannot live inside treequest-go: there's nowhere to put it. The library has no concept of "LLM," "claim," "debt," or "evaluator" — it only knows `func(ctx, *S) (S, score, error)`. That's the contract, and the contract is intentionally boring.

So the split looks like:

- **`treequest-go`** — `Node[S]`, `SearchTree[S]`, `BetaSampler`, `ABMCTSA[S]`, checkpointing. Zero imports of anything LLM-related. Could be tested entirely with `S = string` and `rand.Float64()` as the generator, the way Sakana's own README example does it.
- **`ebp-paper-eval`** (or whatever you name it) — imports `treequest-go` as a Go module dependency, defines `ClaimEvalState`, defines the `LLMProvider` interface (this lives in the *consumer*, not the library — treequest-go shouldn't even know LLMs exist), and wraps Worker A, Worker B, and Evaluator E as closures that satisfy `GenerateFn[ClaimEvalState]`.

The win isn't just tidiness — it's that treequest-go becomes reusable for anything tree-searchable: code generation, theorem search, even non-LLM domains where `generate` is a simulator. EBP is just one consumer among many you could write later.

## 3 — Turn budget per worker: reasonable, but it's one layer of a hierarchy

Yes, it's a sound mechanism — but the framing of a single flag undersells what's actually a **three-layer budget stack**, and it's worth naming all three so you don't conflate them later:

1. **Search-level budget** — total `Step()` calls on the tree. This already exists in AB-MCTS by design; Sakana's own ARC-AGI-2 experiments capped this at 250. This is your outermost dial.
2. **Worker-turn budget** — what you're describing. Within *one* `GenerateFn` invocation, does the worker get to have an internal back-and-forth with its backing LLM (read paper section → draft → self-critique → revise) before finalizing the `(state, score)` it hands back to the tree? Bounding this prevents a single node expansion from silently consuming unbounded calls.
3. **Token/cost budget per turn** — a turn count alone is a weak cost proxy, since one "turn" can be 200 tokens or 20,000. If actual $ control matters, pair `MaxTurns` with a `MaxTokensPerTurn` or running cost ceiling.

The good news: because of the repo split in part 1, layer 2 doesn't need to be a treequest-go concept at all. It's entirely internal to how you write the Worker A / Worker B closures in `ebp-paper-eval` — the tree algorithm never sees it, never needs to. Something like:

```go
type WorkerConfig struct {
    MaxTurns        int
    MaxTokensPerTurn int  // optional, layer 3
}

func GenerateA(provider llm.LLMProvider, cfg WorkerConfig) algo.GenerateFn[ClaimEvalState] {
    return func(ctx context.Context, parent *ClaimEvalState) (ClaimEvalState, float64, error) {
        var transcript []llm.Message
        for turn := 0; turn < cfg.MaxTurns; turn++ {
            resp, err := provider.Complete(ctx, transcript)
            if isFinal(resp) || turn == cfg.MaxTurns-1 {
                return finalize(resp, turn, cfg.MaxTurns), score, nil
            }
            transcript = append(transcript, ...) // continue the agentic loop
        }
    }
}
```

One refinement worth building in from day one, because it's literally your own doctrine: **if a worker hits `MaxTurns` without converging, that's an honest-incompleteness event, not a silent truncation.** Workbench2 says it directly — *"The Workbench rewards honest incompleteness. It penalizes hidden incompleteness."* So the finalize step on budget exhaustion should set an explicit flag (e.g. `BudgetExhausted: true` on the state) that Evaluator E sees and factors into scoring — rather than the worker quietly handing back a confident-sounding answer that ran out of room to finish its own reasoning. That keeps the budget mechanism aligned with EBP rather than fighting it.

Also worth deciding deliberately, given your phrasing "respective LLM" — should A and B get the *same* `MaxTurns`, or asymmetric budgets? There's a real argument for B getting more: the Workbench2 adversarial pass (claim type, assumptions, bridge violations, inverse constraints, rival differential — seven checks) is naturally a longer deliberation than A's single constructive pass. Symmetric budgets are simpler to reason about for cost, but asymmetric ones may better reflect the actual cognitive load of each role.

On the "output is a research paper" point — that's correctly a downstream concern, not a search concern. It's the `Reporter` stage in the diagram: it walks the best path (or full tree) after the search loop ends and renders it against the Workbench2 Core Report Template, with the turn-budget/incompleteness flags surfaced per claim rather than buried. That stays in `ebp-paper-eval` too, fully decoupled from how the tree was searched.