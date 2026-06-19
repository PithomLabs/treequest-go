package tree

import (
	"fmt"
)

// StateCodec defines how a generic state S is marshaled and unmarshaled for snapshots
type StateCodec[S any] interface {
	MarshalState(state S) ([]byte, error)
	UnmarshalState(data []byte) (S, error)
}

// NodeRecord is the flat, JSON-serializable representation of a search node
type NodeRecord struct {
	ID                NodeID                  `json:"id"`
	ParentID          *NodeID                 `json:"parent_id,omitempty"`
	ChildrenIDs       []NodeID                `json:"children_ids,omitempty"`
	EncodedState      []byte                  `json:"encoded_state"`
	Score             Score                   `json:"score"`
	Depth             int                     `json:"depth"`
	ExpandIdx         int                     `json:"expand_idx"`
	WiderBandit       BetaParams              `json:"wider_bandit"`
	DeeperBandit      BetaParams              `json:"deeper_bandit"`
	SelfBandit        BetaParams              `json:"self_bandit"`
	ActionBandits     map[ActionLabel]BetaParams `json:"action_bandits"`
	GeneratedByAction ActionLabel             `json:"generated_by_action"`
}

// TreeSnapshot contains the versioned flat list of node records representing a SearchTree
type TreeSnapshot struct {
	Version int          `json:"version"`
	Nodes   []NodeRecord `json:"nodes"`
}

// SaveSnapshot serializes the tree into a TreeSnapshot using the provided StateCodec.
func (t *SearchTree[S]) SaveSnapshot(codec StateCodec[S]) (*TreeSnapshot, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	records := make([]NodeRecord, 0, len(t.AllNodes))
	for _, n := range t.AllNodes {
		n.Lock()
		encoded, err := codec.MarshalState(n.State)
		if err != nil {
			n.Unlock()
			return nil, fmt.Errorf("failed to marshal state for node %q: %w", n.ID, err)
		}

		rec := NodeRecord{
			ID:                n.ID,
			ParentID:          n.ParentID,
			ChildrenIDs:       n.ChildrenIDs,
			EncodedState:      encoded,
			Score:             n.Score,
			Depth:             n.Depth,
			ExpandIdx:         n.ExpandIdx,
			WiderBandit:       n.WiderBandit,
			DeeperBandit:      n.DeeperBandit,
			SelfBandit:        n.SelfBandit,
			ActionBandits:     n.ActionBandits,
			GeneratedByAction: n.GeneratedByAction,
		}
		n.Unlock()
		records = append(records, rec)
	}

	return &TreeSnapshot{
		Version: 1,
		Nodes:   records,
	}, nil
}

// RestoreTree deserializes a TreeSnapshot and reconstructs the parent-child pointer graph.
func RestoreTree[S any](snapshot *TreeSnapshot, codec StateCodec[S]) (*SearchTree[S], error) {
	if snapshot == nil {
		return nil, fmt.Errorf("snapshot is nil")
	}

	allNodes := make(map[NodeID]*Node[S])

	// 1. Reconstruct all Node structs (flat pass)
	for _, rec := range snapshot.Nodes {
		state, err := codec.UnmarshalState(rec.EncodedState)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal state for node %q: %w", rec.ID, err)
		}

		n := &Node[S]{
			ID:                rec.ID,
			ParentID:          rec.ParentID,
			ChildrenIDs:       rec.ChildrenIDs,
			State:             state,
			Score:             rec.Score,
			Depth:             rec.Depth,
			ExpandIdx:         rec.ExpandIdx,
			WiderBandit:       rec.WiderBandit,
			DeeperBandit:      rec.DeeperBandit,
			SelfBandit:        rec.SelfBandit,
			ActionBandits:     rec.ActionBandits,
			GeneratedByAction: rec.GeneratedByAction,
			Children:          []*Node[S]{},
		}
		allNodes[n.ID] = n
	}

	// 2. Wire up parent and children pointers
	var root *Node[S]
	for _, n := range allNodes {
		if n.ParentID == nil {
			if root != nil {
				return nil, fmt.Errorf("multiple root nodes detected in snapshot")
			}
			root = n
			continue
		}

		parent, exists := allNodes[*n.ParentID]
		if !exists {
			return nil, fmt.Errorf("parent node %q not found for node %q", *n.ParentID, n.ID)
		}
		n.Parent = parent
		parent.Children = append(parent.Children, n)
	}

	if root == nil {
		return nil, fmt.Errorf("no root node found in snapshot")
	}

	return &SearchTree[S]{
		Root:     root,
		AllNodes: allNodes,
	}, nil
}
