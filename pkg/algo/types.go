package algo

import (
	"context"
	"sync"
	tree "github.com/PithomLabs/treequest-go/pkg/tree"
)

// TrialStatus represents the current lifecycle stage of an MCTS action trial
type TrialStatus string

const (
	TrialPending   TrialStatus = "pending"
	TrialCompleted TrialStatus = "completed"
	TrialFailed    TrialStatus = "failed"
	TrialCanceled  TrialStatus = "canceled"
)

// Trial stores information necessary to track, resume, and audit search branches
type Trial[S any] struct {
	TrialID      tree.TrialID     `json:"trial_id"`
	NodeToExpand tree.NodeID      `json:"node_to_expand"`
	Action       tree.ActionLabel `json:"action"`
	Path         []tree.NodeID    `json:"path"`
	Result       *StateScore[S]   `json:"result,omitempty"`
	ParentState  *S               `json:"parent_state,omitempty"`
	Status       TrialStatus      `json:"status"`
}

// TrialStore is an index tracking running and completed trials
type TrialStore[S any] struct {
	mu       sync.RWMutex
	Pending  map[tree.TrialID]Trial[S]  `json:"pending"`
	Finished map[tree.TrialID]Trial[S] `json:"finished"`
}

// NewTrialStore creates a new trial store instance
func NewTrialStore[S any]() *TrialStore[S] {
	return &TrialStore[S]{
		Pending:  make(map[tree.TrialID]Trial[S]),
		Finished: make(map[tree.TrialID]Trial[S]),
	}
}

// GetPending retrieves a pending trial by ID under RLock
func (ts *TrialStore[S]) GetPending(id tree.TrialID) (Trial[S], bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	t, ok := ts.Pending[id]
	return t, ok
}

// SetPending stores a pending trial under Lock
func (ts *TrialStore[S]) SetPending(id tree.TrialID, t Trial[S]) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.Pending[id] = t
}

// DeletePending deletes a pending trial under Lock
func (ts *TrialStore[S]) DeletePending(id tree.TrialID) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.Pending, id)
}

// GetFinished retrieves a finished trial by ID under RLock
func (ts *TrialStore[S]) GetFinished(id tree.TrialID) (Trial[S], bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	t, ok := ts.Finished[id]
	return t, ok
}

// SetFinished stores a finished trial under Lock
func (ts *TrialStore[S]) SetFinished(id tree.TrialID, t Trial[S]) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.Finished[id] = t
}

// Clone copies Pending and Finished under RLock into a new TrialStore instance
func (ts *TrialStore[S]) Clone() *TrialStore[S] {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	pending := make(map[tree.TrialID]Trial[S], len(ts.Pending))
	for k, v := range ts.Pending {
		pending[k] = v
	}

	finished := make(map[tree.TrialID]Trial[S], len(ts.Finished))
	for k, v := range ts.Finished {
		finished[k] = v
	}

	return &TrialStore[S]{
		Pending:  pending,
		Finished: finished,
	}
}

// Reward is an alias to tree.Reward for algorithm package compatibility
type Reward = tree.Reward

// StateScore is an alias to tree.StateScore[S] for algorithm package compatibility
type StateScore[S any] = tree.StateScore[S]

// Algorithm is the core stateless interface that generic tree search strategies implement
type Algorithm[S any, AS any] interface {
	// InitTree initializes the algorithm state
	InitTree(ctx context.Context) (AS, error)

	// AskBatch returns next target nodes and action labels to expand in a batch
	AskBatch(
		ctx context.Context,
		state AS,
		batchSize int,
		actions []tree.ActionLabel,
	) (AS, []Trial[S], error)

	// Ask returns the next target node and action label to expand
	Ask(
		ctx context.Context,
		state AS,
		actions []tree.ActionLabel,
	) (AS, Trial[S], error)

	// Tell registers a resulting state-score pair for a given trial
	Tell(
		ctx context.Context,
		state AS,
		trialID tree.TrialID,
		result StateScore[S],
	) (AS, error)

	// StateScorePairs retrieves all state-score pairs from the search tree
	StateScorePairs(state AS) []StateScore[S]
}
