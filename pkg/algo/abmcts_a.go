package algo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	tree "github.com/PithomLabs/treequest-go/pkg/tree"
)

// newUUID generates a random 16-byte hex UUID for NodeID and TrialID
func newUUID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

// ABMCTSAAlgoState holds the mutable state for the AB-MCTS-A algorithm run.
// Protected by sync.RWMutex.
type ABMCTSAAlgoState[S any] struct {
	mu         sync.RWMutex
	Tree       *tree.SearchTree[S]
	TrialStore *TrialStore[S]
	AllRewards map[tree.ActionLabel][]float64
}

// GenerateFn represents the generator function for generic MCTS states S
type GenerateFn[S any] func(ctx context.Context, parentState *S) (S, tree.Score, error)

// GenerateFns maps actions to their corresponding generator closures
type GenerateFns[S any] map[tree.ActionLabel]GenerateFn[S]

// ABMCTSA implements the generic Algorithm interface for the AB-MCTS-A strategy
type ABMCTSA[S any] struct {
	sampler Sampler
	codec   tree.StateCodec[S]
}

// NewABMCTSA creates a new AB-MCTS-A algorithm instance
func NewABMCTSA[S any](sampler Sampler, codec tree.StateCodec[S]) *ABMCTSA[S] {
	return &ABMCTSA[S]{
		sampler: sampler,
		codec:   codec,
	}
}

// InitTree initializes the algorithm state with the zero value of S
func (a *ABMCTSA[S]) InitTree(ctx context.Context) (*ABMCTSAAlgoState[S], error) {
	var zero S
	return a.InitTreeWithState(ctx, zero, nil)
}

// InitTreeWithState initializes the algorithm state with a specific root state and action set
func (a *ABMCTSA[S]) InitTreeWithState(ctx context.Context, rootState S, actions []tree.ActionLabel) (*ABMCTSAAlgoState[S], error) {
	t := tree.NewSearchTree(rootState, actions)
	return &ABMCTSAAlgoState[S]{
		Tree:       t,
		TrialStore: NewTrialStore[S](),
		AllRewards: make(map[tree.ActionLabel][]float64),
	}, nil
}

// AskBatch determines the next nodes and actions to expand, returning pending Trials.
// Adheres to Lock Order: ABMCTSAAlgoState.mu -> TrialStore.mu -> SearchTree.mu / Node Locks.
func (a *ABMCTSA[S]) AskBatch(
	ctx context.Context,
	state *ABMCTSAAlgoState[S],
	batchSize int,
	actions []tree.ActionLabel,
) (*ABMCTSAAlgoState[S], []Trial[S], error) {
	state.mu.Lock()
	defer state.mu.Unlock()

	// Initialize AllRewards keys if they do not exist
	for _, act := range actions {
		if _, ok := state.AllRewards[act]; !ok {
			state.AllRewards[act] = []float64{}
		}
	}

	trials := make([]Trial[S], 0, batchSize)
	for i := 0; i < batchSize; i++ {
		// selectNodeAndAction is called under state.mu lock
		node, selectedAction, path, err := a.selectNodeAndAction(state, actions)
		if err != nil {
			return nil, nil, err
		}

		trialID := tree.TrialID(newUUID())
		trial := Trial[S]{
			TrialID:      trialID,
			NodeToExpand: node.ID,
			Action:       selectedAction,
			Path:         path,
			ParentState:  &node.State,
			Status:       TrialPending,
		}
		// Write to TrialStore (nested lock TrialStore.mu)
		state.TrialStore.SetPending(trialID, trial)
		trials = append(trials, trial)
	}

	return state, trials, nil
}

// Ask is a convenience wrapper around AskBatch for retrieving a single next Trial
func (a *ABMCTSA[S]) Ask(
	ctx context.Context,
	state *ABMCTSAAlgoState[S],
	actions []tree.ActionLabel,
) (*ABMCTSAAlgoState[S], Trial[S], error) {
	state, trials, err := a.AskBatch(ctx, state, 1, actions)
	if err != nil {
		return nil, Trial[S]{}, err
	}
	return state, trials[0], nil
}

// selectNodeAndAction performs the tree selection traversal and action selection.
// Assumes state.mu is locked by the caller.
func (a *ABMCTSA[S]) selectNodeAndAction(state *ABMCTSAAlgoState[S], actions []tree.ActionLabel) (*tree.Node[S], tree.ActionLabel, []tree.NodeID, error) {
	path := []tree.NodeID{}
	curr := state.Tree.Root

	for {
		path = append(path, curr.ID)

		curr.Lock()
		hasChildren := len(curr.Children) > 0
		curr.Unlock()

		if !hasChildren {
			act := a.selectAction(curr, state, actions)
			return curr, act, path, nil
		}

		curr.Lock()
		widerBP := curr.WiderBandit
		deeperBP := curr.DeeperBandit
		curr.Unlock()

		sWider := a.sampler.SampleBeta(widerBP.Alpha, widerBP.Beta)
		sDeeper := a.sampler.SampleBeta(deeperBP.Alpha, deeperBP.Beta)

		if sWider > sDeeper {
			act := a.selectAction(curr, state, actions)
			return curr, act, path, nil
		}

		// Go deeper: pick child with highest SelfBandit sample
		curr.Lock()
		var bestChild *tree.Node[S]
		bestVal := -1.0
		for _, c := range curr.Children {
			c.Lock()
			selfBP := c.SelfBandit
			c.Unlock()

			val := a.sampler.SampleBeta(selfBP.Alpha, selfBP.Beta)
			if val > bestVal {
				bestVal = val
				bestChild = c
			}
		}
		curr.Unlock()

		if bestChild == nil {
			return nil, "", nil, fmt.Errorf("AB-MCTS-A error: deeper selected but no children found")
		}
		curr = bestChild
	}
}

// selectAction runs a multi-armed bandit over global rewards to choose the action.
// Assumes state.mu is locked by the caller (no nested state.mu locks).
func (a *ABMCTSA[S]) selectAction(node *tree.Node[S], state *ABMCTSAAlgoState[S], actions []tree.ActionLabel) tree.ActionLabel {
	if len(actions) == 1 {
		return actions[0]
	}

	bestAction := actions[0]
	bestVal := -1.0
	for _, act := range actions {
		node.Lock()
		prior, exists := node.ActionBandits[act]
		if !exists {
			prior = tree.NewBetaParams()
		}
		node.Unlock()

		alpha := prior.Alpha
		beta := prior.Beta
		rewards := state.AllRewards[act]
		for _, r := range rewards {
			alpha += r
			beta += (1.0 - r)
		}

		val := a.sampler.SampleBeta(alpha, beta)
		if val > bestVal {
			bestVal = val
			bestAction = act
		}
	}
	return bestAction
}

// Tell registers a generated state score and updates the bandit distributions.
// Adheres to Lock Order: ABMCTSAAlgoState.mu -> TrialStore.mu -> SearchTree.mu / Node Locks.
func (a *ABMCTSA[S]) Tell(
	ctx context.Context,
	state *ABMCTSAAlgoState[S],
	trialID tree.TrialID,
	result StateScore[S],
) (*ABMCTSAAlgoState[S], error) {
	if err := validateScore(result.Reward.Score); err != nil {
		return nil, err
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	// 1. Look up trial (nested TrialStore.mu Lock)
	trial, exists := state.TrialStore.GetPending(trialID)
	if !exists {
		// Idempotency check
		finishedTrial, inFinished := state.TrialStore.GetFinished(trialID)
		if inFinished {
			if result.ResultHash == "" {
				derivedHash, err := a.deriveHash(result)
				if err != nil {
					return nil, err
				}
				result.ResultHash = derivedHash
			}
			if finishedTrial.Result.ResultHash == result.ResultHash {
				return state, nil
			}
			return nil, fmt.Errorf("%w: result mismatch for completed trial %q", ErrResultMismatch, trialID)
		}
		return nil, fmt.Errorf("%w: unknown trial ID %q", ErrUnknownTrial, trialID)
	}

	// 2. Derive hash if empty
	if result.ResultHash == "" {
		derivedHash, err := a.deriveHash(result)
		if err != nil {
			return nil, err
		}
		result.ResultHash = derivedHash
	}

	// 3. Complete trial (nested TrialStore.mu locks)
	state.TrialStore.DeletePending(trialID)
	trial.Status = TrialCompleted
	trial.Result = &result
	state.TrialStore.SetFinished(trialID, trial)

	// 4. Add node to SearchTree (nested SearchTree.mu Lock)
	childID := tree.NodeID(newUUID())
	child, err := state.Tree.AddNode(trial.NodeToExpand, childID, result.State, result.Reward.Score, trial.Action)
	if err != nil {
		return nil, err
	}

	// 5. Update global rewards
	state.AllRewards[trial.Action] = append(state.AllRewards[trial.Action], float64(result.Reward.Score))

	// 6. Backpropagation (nested Node Locks)
	a.backpropagate(state.Tree, trial.Path, result.Reward.Score, trial.Action, child.ID)

	return state, nil
}

// deriveHash computes the ResultHash using the configured StateCodec and score
func (a *ABMCTSA[S]) deriveHash(result StateScore[S]) (string, error) {
	encoded, err := a.codec.MarshalState(result.State)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state for hash: %w", err)
	}
	hasher := sha256.New()
	hasher.Write(encoded)
	hasher.Write([]byte(fmt.Sprintf(":%f", float64(result.Reward.Score))))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// backpropagate updates the bandit parameters up the selection path
func (a *ABMCTSA[S]) backpropagate(t *tree.SearchTree[S], path []tree.NodeID, score tree.Score, action tree.ActionLabel, newChildID tree.NodeID) {
	if len(path) == 0 {
		return
	}

	// Immediate parent is the last element in the path
	parentID := path[len(path)-1]
	parent, exists := t.GetNode(parentID)
	if exists {
		parent.Lock()
		parent.WiderBandit = UpdateBandit(parent.WiderBandit, score)
		parent.ActionBandits[action] = UpdateBandit(parent.ActionBandits[action], score)
		parent.Unlock()
	}

	// Ancestors updates: loop through path steps up to intermediate parents
	for i := 0; i < len(path)-1; i++ {
		ancID := path[i]
		childID := path[i+1]

		anc, ancExists := t.GetNode(ancID)
		child, childExists := t.GetNode(childID)

		if ancExists {
			anc.Lock()
			anc.DeeperBandit = UpdateBandit(anc.DeeperBandit, score)
			anc.ActionBandits[action] = UpdateBandit(anc.ActionBandits[action], score)
			anc.Unlock()
		}
		if childExists {
			child.Lock()
			child.SelfBandit = UpdateBandit(child.SelfBandit, score)
			child.Unlock()
		}
	}
}

// StateScorePairs retrieves all non-root state-score pairs from the tree
func (a *ABMCTSA[S]) StateScorePairs(state *ABMCTSAAlgoState[S]) []StateScore[S] {
	state.mu.RLock()
	defer state.mu.RUnlock()

	nodes := state.Tree.GetNodes()
	pairs := make([]StateScore[S], 0, len(nodes))
	for _, n := range nodes {
		if n.ID == state.Tree.Root.ID {
			continue
		}
		pairs = append(pairs, StateScore[S]{
			State: n.State,
			Reward: tree.Reward{
				Score: n.Score,
			},
		})
	}
	return pairs
}

// Step runs a single synchronous selection-expansion-backpropagation iteration
func (a *ABMCTSA[S]) Step(ctx context.Context, state *ABMCTSAAlgoState[S], actions []tree.ActionLabel, fns GenerateFns[S]) (*tree.Node[S], *ABMCTSAAlgoState[S], error) {
	state, trial, err := a.Ask(ctx, state, actions)
	if err != nil {
		return nil, nil, err
	}

	var parentStatePtr *S
	if trial.ParentState != nil {
		parentStatePtr = trial.ParentState
	}
	newState, score, err := fns[trial.Action](ctx, parentStatePtr)
	if err != nil {
		return nil, nil, err
	}

	state, err = a.Tell(ctx, state, trial.TrialID, StateScore[S]{
		State: newState,
		Reward: tree.Reward{
			Score: score,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	nodes := state.Tree.GetNodes()
	var lastNode *tree.Node[S]
	if len(nodes) > 1 {
		lastNode = nodes[len(nodes)-1]
	}

	return lastNode, state, nil
}

// TrialRecord is a plain data record for serializing Trial[S] (no mutexes)
type TrialRecord struct {
	TrialID      tree.TrialID      `json:"trial_id"`
	NodeToExpand tree.NodeID       `json:"node_to_expand"`
	Action       tree.ActionLabel  `json:"action"`
	Path         []tree.NodeID     `json:"path"`
	Result       *StateScoreRecord `json:"result,omitempty"`
	Status       TrialStatus       `json:"status"`
}

// StateScoreRecord is a plain data record for serializing StateScore[S] (no mutexes)
type StateScoreRecord struct {
	EncodedState []byte      `json:"encoded_state"`
	Reward       tree.Reward `json:"reward"`
	ResultHash   string      `json:"result_hash"`
}

// AlgoStateSnapshot represents a plain-data checkpoint of the entire ABMCTSAAlgoState[S]
type AlgoStateSnapshot struct {
	TreeSnapshot   *tree.TreeSnapshot             `json:"tree_snapshot"`
	PendingTrials  []TrialRecord                  `json:"pending_trials"`
	FinishedTrials []TrialRecord                  `json:"finished_trials"`
	AllRewards     map[tree.ActionLabel][]float64 `json:"all_rewards"`
}

// trialToRecord converts a Trial[S] into a plain TrialRecord
func (a *ABMCTSA[S]) trialToRecord(t Trial[S]) (TrialRecord, error) {
	var rec TrialRecord
	rec.TrialID = t.TrialID
	rec.NodeToExpand = t.NodeToExpand
	rec.Action = t.Action
	rec.Status = t.Status
	if t.Path != nil {
		rec.Path = make([]tree.NodeID, len(t.Path))
		copy(rec.Path, t.Path)
	}
	if t.Result != nil {
		encoded, err := a.codec.MarshalState(t.Result.State)
		if err != nil {
			return rec, err
		}
		rec.Result = &StateScoreRecord{
			EncodedState: encoded,
			Reward:       t.Result.Reward,
			ResultHash:   t.Result.ResultHash,
		}
	}
	return rec, nil
}

// recordToTrial converts a plain TrialRecord into a Trial[S]
func (a *ABMCTSA[S]) recordToTrial(rec TrialRecord) (Trial[S], error) {
	var t Trial[S]
	t.TrialID = rec.TrialID
	t.NodeToExpand = rec.NodeToExpand
	t.Action = rec.Action
	t.Status = rec.Status
	if rec.Path != nil {
		t.Path = make([]tree.NodeID, len(rec.Path))
		copy(t.Path, rec.Path)
	}
	if rec.Result != nil {
		state, err := a.codec.UnmarshalState(rec.Result.EncodedState)
		if err != nil {
			return t, err
		}
		t.Result = &StateScore[S]{
			State:      state,
			Reward:     rec.Result.Reward,
			ResultHash: rec.Result.ResultHash,
		}
	}
	return t, nil
}

// SaveAlgoSnapshot serializes the entire algorithm state to a plain AlgoStateSnapshot.
// Copies all maps and slices under lock to ensure concurrent safety.
func (a *ABMCTSA[S]) SaveAlgoSnapshot(state *ABMCTSAAlgoState[S]) (*AlgoStateSnapshot, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	treeSnap, err := state.Tree.SaveSnapshot(a.codec)
	if err != nil {
		return nil, err
	}

	state.TrialStore.mu.RLock()
	defer state.TrialStore.mu.RUnlock()

	pending := make([]TrialRecord, 0, len(state.TrialStore.Pending))
	for _, t := range state.TrialStore.Pending {
		rec, err := a.trialToRecord(t)
		if err != nil {
			return nil, err
		}
		pending = append(pending, rec)
	}

	finished := make([]TrialRecord, 0, len(state.TrialStore.Finished))
	for _, t := range state.TrialStore.Finished {
		rec, err := a.trialToRecord(t)
		if err != nil {
			return nil, err
		}
		finished = append(finished, rec)
	}

	rewardsCopy := make(map[tree.ActionLabel][]float64)
	for k, v := range state.AllRewards {
		vCopy := make([]float64, len(v))
		copy(vCopy, v)
		rewardsCopy[k] = vCopy
	}

	return &AlgoStateSnapshot{
		TreeSnapshot:   treeSnap,
		PendingTrials:  pending,
		FinishedTrials: finished,
		AllRewards:     rewardsCopy,
	}, nil
}

// RestoreAlgoSnapshot reconstructs an ABMCTSAAlgoState[S] from a plain AlgoStateSnapshot
func (a *ABMCTSA[S]) RestoreAlgoSnapshot(snap *AlgoStateSnapshot) (*ABMCTSAAlgoState[S], error) {
	t, err := tree.RestoreTree(snap.TreeSnapshot, a.codec)
	if err != nil {
		return nil, err
	}

	ts := NewTrialStore[S]()
	for _, rec := range snap.PendingTrials {
		trial, err := a.recordToTrial(rec)
		if err != nil {
			return nil, err
		}
		ts.Pending[trial.TrialID] = trial
	}

	for _, rec := range snap.FinishedTrials {
		trial, err := a.recordToTrial(rec)
		if err != nil {
			return nil, err
		}
		ts.Finished[trial.TrialID] = trial
	}

	rewardsCopy := make(map[tree.ActionLabel][]float64)
	for k, v := range snap.AllRewards {
		vCopy := make([]float64, len(v))
		copy(vCopy, v)
		rewardsCopy[k] = vCopy
	}

	return &ABMCTSAAlgoState[S]{
		Tree:       t,
		TrialStore: ts,
		AllRewards: rewardsCopy,
	}, nil
}

// Error definitions
var (
	ErrResultMismatch = fmt.Errorf("result hash mismatch")
	ErrUnknownTrial   = fmt.Errorf("unknown trial")
)

// validateScore checks score constraints
func validateScore(score tree.Score) error {
	f := float64(score)
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0.0 || f > 1.0 {
		return fmt.Errorf("invalid score value %f; must be a scalar in range [0, 1]", f)
	}
	return nil
}
