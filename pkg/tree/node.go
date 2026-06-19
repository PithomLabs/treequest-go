package tree

import (
	"sync"
)

// Neutral core type set
type ActionLabel string
type NodeID string
type TrialID string
type Score float64

// Reward holds the normalized score and any application-specific metadata
type Reward struct {
	Score         Score              `json:"score"`
	Components    map[string]float64 `json:"components,omitempty"`
	Justification string             `json:"justification,omitempty"`
	Meta          map[string]any     `json:"meta,omitempty"`
}

// StateScore holds a search state, its associated reward, and its validation hash
type StateScore[S any] struct {
	State      S      `json:"state"`
	Reward     Reward `json:"reward"`
	ResultHash string `json:"result_hash"`
}

// BetaParams holds parameters for Beta Thompson Sampling
type BetaParams struct {
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
}

// NewBetaParams returns standard flat Jeffrey's prior parameters (Alpha=0.5, Beta=0.5)
func NewBetaParams() BetaParams {
	return BetaParams{Alpha: 0.5, Beta: 0.5}
}

// Node represents a generic node in the MCTS search tree.
// S is the user-defined generic state type.
type Node[S any] struct {
	ID                NodeID                      `json:"id"`
	ParentID          *NodeID                     `json:"parent_id,omitempty"`
	ChildrenIDs       []NodeID                    `json:"children_ids,omitempty"`
	State             S                           `json:"state"`
	Score             Score                       `json:"score"`
	Depth             int                         `json:"depth"`
	ExpandIdx         int                         `json:"expand_idx"`
	WiderBandit       BetaParams                  `json:"wider_bandit"`
	DeeperBandit      BetaParams                  `json:"deeper_bandit"`
	SelfBandit        BetaParams                  `json:"self_bandit"`
	ActionBandits     map[ActionLabel]BetaParams `json:"action_bandits"`
	GeneratedByAction ActionLabel                 `json:"generated_by_action"`

	// Non-serialized memory pointers and lock
	Parent   *Node[S]   `json:"-"`
	Children []*Node[S] `json:"-"`
	mu       sync.Mutex `json:"-"`
}

// NewRootNode creates a properly initialized root node for a search tree.
func NewRootNode[S any](state S, actions []ActionLabel) *Node[S] {
	actionBandits := make(map[ActionLabel]BetaParams, len(actions))
	for _, act := range actions {
		actionBandits[act] = NewBetaParams()
	}
	return &Node[S]{
		ID:            NodeID("root"),
		ParentID:      nil,
		ChildrenIDs:   []NodeID{},
		State:         state,
		Score:         -1.0, // Placeholder score for root node
		Depth:         0,
		ExpandIdx:     -1, // Root always starts with -1 expand index
		WiderBandit:   NewBetaParams(),
		DeeperBandit:  NewBetaParams(),
		SelfBandit:    NewBetaParams(),
		ActionBandits: actionBandits,
		Children:      []*Node[S]{},
	}
}

// NewChildNode creates and attaches a new child node to the parent.
func NewChildNode[S any](parent *Node[S], nodeID NodeID, state S, score Score, action ActionLabel, expandIdx int) *Node[S] {
	actionBandits := make(map[ActionLabel]BetaParams, len(parent.ActionBandits))
	for act := range parent.ActionBandits {
		actionBandits[act] = NewBetaParams()
	}
	child := &Node[S]{
		ID:                nodeID,
		ParentID:          &parent.ID,
		ChildrenIDs:       []NodeID{},
		State:             state,
		Score:             score,
		Depth:             parent.Depth + 1,
		ExpandIdx:         expandIdx,
		WiderBandit:       NewBetaParams(),
		DeeperBandit:      NewBetaParams(),
		SelfBandit:        BetaParams{Alpha: 0.5 + float64(score), Beta: 0.5 + (1.0 - float64(score))},
		ActionBandits:     actionBandits,
		GeneratedByAction: action,
		Parent:            parent,
		Children:          []*Node[S]{},
	}
	return child
}

// Lock locks the node's mutex for thread safety.
func (n *Node[S]) Lock() {
	n.mu.Lock()
}

// Unlock unlocks the node's mutex.
func (n *Node[S]) Unlock() {
	n.mu.Unlock()
}
