package algo

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	tree "github.com/PithomLabs/treequest-go/pkg/tree"
)

// StringCodec is a simple codec for testing S = string
type StringCodec struct{}

func (c StringCodec) MarshalState(state string) ([]byte, error) {
	return []byte(state), nil
}

func (c StringCodec) UnmarshalState(data []byte) (string, error) {
	return string(data), nil
}

// FixedMockSampler returns a deterministic value for SampleBeta
type FixedMockSampler struct {
	val float64
}

func (s *FixedMockSampler) SampleBeta(alpha, beta float64) float64 {
	return s.val
}

func TestScoreValidation(t *testing.T) {
	tests := []struct {
		score   tree.Score
		wantErr bool
	}{
		{score: 0.0, wantErr: false},
		{score: 0.5, wantErr: false},
		{score: 1.0, wantErr: false},
		{score: -0.1, wantErr: true},
		{score: 1.01, wantErr: true},
		{score: tree.Score(math.NaN()), wantErr: true},
		{score: tree.Score(math.Inf(1)), wantErr: true},
		{score: tree.Score(math.Inf(-1)), wantErr: true},
	}

	for _, tt := range tests {
		err := validateScore(tt.score)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateScore(%f) error = %v, wantErr %v", float64(tt.score), err, tt.wantErr)
		}
	}
}

func TestABMCTSA_Lifecycle(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(42)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)

	actions := []tree.ActionLabel{"action-a", "action-b"}

	// 1. Initialize Tree
	state, err := algo.InitTreeWithState(ctx, "root-state", actions)
	if err != nil {
		t.Fatalf("InitTreeWithState failed: %v", err)
	}

	if state.Tree.Root.State != "root-state" {
		t.Errorf("expected root state to be %q, got %q", "root-state", state.Tree.Root.State)
	}

	// 2. Ask (Get next expansion target)
	state, trial, err := algo.Ask(ctx, state, actions)
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}

	if trial.Status != TrialPending {
		t.Errorf("expected trial status to be pending, got %q", trial.Status)
	}
	if trial.NodeToExpand != "root" {
		t.Errorf("expected node to expand to be root, got %q", trial.NodeToExpand)
	}
	if len(trial.Path) != 1 || trial.Path[0] != "root" {
		t.Errorf("expected path to contain only root, got %v", trial.Path)
	}

	// 3. Tell (Submit result)
	result := StateScore[string]{
		State: "child-state-1",
		Reward: Reward{
			Score: 0.8,
		},
	}
	state, err = algo.Tell(ctx, state, trial.TrialID, result)
	if err != nil {
		t.Fatalf("Tell failed: %v", err)
	}

	// Verify finished trial
	finishedTrial, exists := state.TrialStore.Finished[trial.TrialID]
	if !exists {
		t.Fatalf("expected trial %q to be in finished store", trial.TrialID)
	}
	if finishedTrial.Status != TrialCompleted {
		t.Errorf("expected finished trial status to be completed, got %q", finishedTrial.Status)
	}
	if finishedTrial.Result.Reward.Score != 0.8 {
		t.Errorf("expected score 0.8, got %f", finishedTrial.Result.Reward.Score)
	}

	// Verify tree size and root WiderBandit update
	if state.Tree.Size() != 2 {
		t.Errorf("expected tree size to be 2, got %d", state.Tree.Size())
	}
	if state.Tree.Root.WiderBandit.Alpha != 1.3 { // 0.5 + 0.8
		t.Errorf("expected root WiderBandit Alpha to be 1.3, got %f", state.Tree.Root.WiderBandit.Alpha)
	}

	// 4. Idempotency Check
	// Resubmit identical Tell
	state, err = algo.Tell(ctx, state, trial.TrialID, result)
	if err != nil {
		t.Errorf("expected idempotent Tell to succeed, got error: %v", err)
	}

	// Resubmit mismatching Tell
	badResult := result
	badResult.Reward.Score = 0.9
	_, err = algo.Tell(ctx, state, trial.TrialID, badResult)
	if err == nil {
		t.Errorf("expected mismatching Tell to fail, but it succeeded")
	}
}

func TestABMCTSA_StepConvenience(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(100)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)

	actions := []tree.ActionLabel{"constructive", "adversarial"}
	state, err := algo.InitTreeWithState(ctx, "start-state", actions)
	if err != nil {
		t.Fatalf("InitTreeWithState failed: %v", err)
	}

	generateFns := GenerateFns[string]{
		"constructive": func(ctx context.Context, parent *string) (string, tree.Score, error) {
			if parent == nil {
				return "init", 0.7, nil
			}
			return *parent + "-A", 0.8, nil
		},
		"adversarial": func(ctx context.Context, parent *string) (string, tree.Score, error) {
			if parent == nil {
				return "init", 0.3, nil
			}
			return *parent + "-B", 0.4, nil
		},
	}

	// Run step 1
	node1, state, err := algo.Step(ctx, state, actions, generateFns)
	if err != nil {
		t.Fatalf("Step 1 failed: %v", err)
	}

	if node1 == nil {
		t.Fatalf("expected node1 to be non-nil")
	}
	if node1.ExpandIdx != 0 {
		t.Errorf("expected expand_idx 0, got %d", node1.ExpandIdx)
	}

	// Run step 2
	node2, state, err := algo.Step(ctx, state, actions, generateFns)
	if err != nil {
		t.Fatalf("Step 2 failed: %v", err)
	}

	if node2 == nil {
		t.Fatalf("expected node2 to be non-nil")
	}
	if node2.ExpandIdx != 1 {
		t.Errorf("expected expand_idx 1, got %d", node2.ExpandIdx)
	}
}

func TestSnapshotRestore(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(1234)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)

	actions := []tree.ActionLabel{"act-1", "act-2"}
	state, err := algo.InitTreeWithState(ctx, "root-state", actions)
	if err != nil {
		t.Fatalf("InitTree failed: %v", err)
	}

	generateFns := GenerateFns[string]{
		"act-1": func(ctx context.Context, parent *string) (string, tree.Score, error) {
			return "state-1", 0.6, nil
		},
		"act-2": func(ctx context.Context, parent *string) (string, tree.Score, error) {
			return "state-2", 0.4, nil
		},
	}

	// Add a few nodes
	for i := 0; i < 3; i++ {
		_, state, err = algo.Step(ctx, state, actions, generateFns)
		if err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}

	// Create snapshot
	snapshot, err := state.Tree.SaveSnapshot(codec)
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	if len(snapshot.Nodes) != 4 {
		t.Errorf("expected 4 nodes in snapshot, got %d", len(snapshot.Nodes))
	}

	// Restore tree
	restoredTree, err := tree.RestoreTree(snapshot, codec)
	if err != nil {
		t.Fatalf("RestoreTree failed: %v", err)
	}

	if restoredTree.Size() != 4 {
		t.Errorf("expected restored tree size to be 4, got %d", restoredTree.Size())
	}

	// Verify root state in restored tree
	if restoredTree.Root.State != "root-state" {
		t.Errorf("expected restored root state %q, got %q", "root-state", restoredTree.Root.State)
	}

	// Verify child count and state reconstruction
	restoredNodes := restoredTree.GetNodes()
	if len(restoredNodes) != 4 {
		t.Fatalf("expected 4 restored nodes, got %d", len(restoredNodes))
	}

	// Check parent-child linkage
	for _, n := range restoredNodes {
		if n.ID == restoredTree.Root.ID {
			if n.Parent != nil {
				t.Errorf("root node parent should be nil")
			}
		} else {
			if n.Parent == nil {
				t.Errorf("non-root node parent should not be nil")
			}
			foundInParentChildren := false
			for _, child := range n.Parent.Children {
				if child.ID == n.ID {
					foundInParentChildren = true
					break
				}
			}
			if !foundInParentChildren {
				t.Errorf("node %q not found in parent's Children slice", n.ID)
			}
		}
	}
}

type QueueSampler struct {
	vals []float64
	idx  int
}

func (s *QueueSampler) SampleBeta(alpha, beta float64) float64 {
	if s.idx >= len(s.vals) {
		panic("QueueSampler exhausted: not enough deterministic mock samples provided in trace")
	}
	v := s.vals[s.idx]
	s.idx++
	return v
}
func closeTo(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestParityDeterministicTrace(t *testing.T) {
	ctx := context.Background()
	codec := StringCodec{}

	// Setup our deterministic queue of samples
	sampler := &QueueSampler{
		vals: []float64{
			// Step 1:
			0.7, // act-1
			0.3, // act-2
			// Step 2:
			0.6, // Wider
			0.4, // Deeper
			0.7, // act-1
			0.3, // act-2
			// Step 3:
			0.3, // Wider
			0.8, // Deeper
			0.9, // child1 SelfBandit
			0.5, // child2 SelfBandit
			0.7, // act-1
			0.3, // act-2
		},
	}

	algo := NewABMCTSA[string](sampler, codec)
	actions := []tree.ActionLabel{"act-1", "act-2"}

	// Init
	state, err := algo.InitTreeWithState(ctx, "root", actions)
	if err != nil {
		t.Fatalf("InitTreeWithState failed: %v", err)
	}

	// Step 1: expectation: child1 added with score 0.8
	// Wider: Root WiderBandit = (1.3, 0.7)
	// action: Root ActionBandits["act-1"] = (1.3, 0.7)
	state, trial1, err := algo.Ask(ctx, state, actions)
	if err != nil {
		t.Fatalf("Ask 1 failed: %v", err)
	}
	state, err = algo.Tell(ctx, state, trial1.TrialID, StateScore[string]{
		State: "state-1",
		Reward: Reward{Score: 0.8},
	})
	if err != nil {
		t.Fatalf("Tell 1 failed: %v", err)
	}

	// Step 2: expectation: child2 added under root with score 0.6
	// Wider: Root WiderBandit = (1.9, 1.1)
	// action: Root ActionBandits["act-1"] = (1.9, 1.1)
	state, trial2, err := algo.Ask(ctx, state, actions)
	if err != nil {
		t.Fatalf("Ask 2 failed: %v", err)
	}
	state, err = algo.Tell(ctx, state, trial2.TrialID, StateScore[string]{
		State: "state-2",
		Reward: Reward{Score: 0.6},
	})
	if err != nil {
		t.Fatalf("Tell 2 failed: %v", err)
	}

	// Step 3: expectation: child3 added under child1 with score 0.9
	// Intermediate parent child1:
	// - child1.WiderBandit = (1.4, 0.6)
	// - child1.ActionBandits["act-1"] = (1.4, 0.6)
	// Ancestor root:
	// - root.DeeperBandit = (1.4, 0.6)
	// - root.ActionBandits["act-1"] = (2.8, 1.2)
	// - child1.SelfBandit = (2.2, 0.8)
	state, trial3, err := algo.Ask(ctx, state, actions)
	if err != nil {
		t.Fatalf("Ask 3 failed: %v", err)
	}
	state, err = algo.Tell(ctx, state, trial3.TrialID, StateScore[string]{
		State: "state-3",
		Reward: Reward{Score: 0.9},
	})
	if err != nil {
		t.Fatalf("Tell 3 failed: %v", err)
	}

	// Get all nodes and check their values
	nodes := state.Tree.GetNodes() // sorted by ExpandIdx: root, child1, child2, child3
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}

	rootNode := nodes[0]
	child1 := nodes[1]
	child2 := nodes[2]
	child3 := nodes[3]

	// Verify Root node
	if !closeTo(rootNode.WiderBandit.Alpha, 1.9) || !closeTo(rootNode.WiderBandit.Beta, 1.1) {
		t.Errorf("root WiderBandit got (%f, %f), expected (1.9, 1.1)", rootNode.WiderBandit.Alpha, rootNode.WiderBandit.Beta)
	}
	if !closeTo(rootNode.DeeperBandit.Alpha, 1.4) || !closeTo(rootNode.DeeperBandit.Beta, 0.6) {
		t.Errorf("root DeeperBandit got (%f, %f), expected (1.4, 0.6)", rootNode.DeeperBandit.Alpha, rootNode.DeeperBandit.Beta)
	}
	if !closeTo(rootNode.ActionBandits["act-1"].Alpha, 2.8) || !closeTo(rootNode.ActionBandits["act-1"].Beta, 1.2) {
		t.Errorf("root ActionBandits[\"act-1\"] got (%f, %f), expected (2.8, 1.2)", rootNode.ActionBandits["act-1"].Alpha, rootNode.ActionBandits["act-1"].Beta)
	}

	// Verify child1 node
	if !closeTo(child1.WiderBandit.Alpha, 1.4) || !closeTo(child1.WiderBandit.Beta, 0.6) {
		t.Errorf("child1 WiderBandit got (%f, %f), expected (1.4, 0.6)", child1.WiderBandit.Alpha, child1.WiderBandit.Beta)
	}
	if !closeTo(child1.SelfBandit.Alpha, 2.2) || !closeTo(child1.SelfBandit.Beta, 0.8) {
		t.Errorf("child1 SelfBandit got (%f, %f), expected (2.2, 0.8)", child1.SelfBandit.Alpha, child1.SelfBandit.Beta)
	}
	if !closeTo(child1.ActionBandits["act-1"].Alpha, 1.4) || !closeTo(child1.ActionBandits["act-1"].Beta, 0.6) {
		t.Errorf("child1 ActionBandits[\"act-1\"] got (%f, %f), expected (1.4, 0.6)", child1.ActionBandits["act-1"].Alpha, child1.ActionBandits["act-1"].Beta)
	}

	// Verify child2 node
	if !closeTo(child2.SelfBandit.Alpha, 1.1) || !closeTo(child2.SelfBandit.Beta, 0.9) {
		t.Errorf("child2 SelfBandit got (%f, %f), expected (1.1, 0.9)", child2.SelfBandit.Alpha, child2.SelfBandit.Beta)
	}

	// Verify child3 node
	if !closeTo(child3.SelfBandit.Alpha, 1.4) || !closeTo(child3.SelfBandit.Beta, 0.6) {
		t.Errorf("child3 SelfBandit got (%f, %f), expected (1.4, 0.6)", child3.SelfBandit.Alpha, child3.SelfBandit.Beta)
	}
}

func TestTrialStore_ConcurrentTell(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(42)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)
	actions := []tree.ActionLabel{"act-1"}

	state, err := algo.InitTreeWithState(ctx, "root", actions)
	if err != nil {
		t.Fatalf("InitTree failed: %v", err)
	}

	state, trials, err := algo.AskBatch(ctx, state, 10, actions)
	if err != nil {
		t.Fatalf("AskBatch failed: %v", err)
	}

	var wg sync.WaitGroup
	for _, tr := range trials {
		wg.Add(1)
		go func(trial Trial[string]) {
			defer wg.Done()
			result := StateScore[string]{
				State: "child-state",
				Reward: tree.Reward{Score: 0.5},
			}
			_, tellErr := algo.Tell(ctx, state, trial.TrialID, result)
			if tellErr != nil {
				t.Errorf("Tell failed concurrently: %v", tellErr)
			}
		}(tr)
	}
	wg.Wait()

	state.mu.RLock()
	finishedCount := len(state.TrialStore.Finished)
	pendingCount := len(state.TrialStore.Pending)
	state.mu.RUnlock()

	if finishedCount != 10 {
		t.Errorf("Expected 10 finished trials, got %d", finishedCount)
	}
	if pendingCount != 0 {
		t.Errorf("Expected 0 pending trials, got %d", pendingCount)
	}
}

func TestTell_UnknownTrialID(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(42)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)
	actions := []tree.ActionLabel{"act-1"}

	state, err := algo.InitTreeWithState(ctx, "root", actions)
	if err != nil {
		t.Fatalf("InitTree failed: %v", err)
	}

	result := StateScore[string]{
		State: "state-1",
		Reward: tree.Reward{Score: 0.5},
	}

	_, err = algo.Tell(ctx, state, tree.TrialID("unknown-id"), result)
	if err == nil || !strings.Contains(err.Error(), "unknown trial") {
		t.Errorf("Expected ErrUnknownTrial, got %v", err)
	}
}

func TestTell_OutOfOrder(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(42)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)
	actions := []tree.ActionLabel{"act-1"}

	state, err := algo.InitTreeWithState(ctx, "root", actions)
	if err != nil {
		t.Fatalf("InitTree failed: %v", err)
	}

	state, trials, err := algo.AskBatch(ctx, state, 2, actions)
	if err != nil {
		t.Fatalf("AskBatch failed: %v", err)
	}

	result2 := StateScore[string]{
		State: "state-2",
		Reward: tree.Reward{Score: 0.6},
	}
	state, err = algo.Tell(ctx, state, trials[1].TrialID, result2)
	if err != nil {
		t.Fatalf("Tell on second trial failed: %v", err)
	}

	result1 := StateScore[string]{
		State: "state-1",
		Reward: tree.Reward{Score: 0.7},
	}
	state, err = algo.Tell(ctx, state, trials[0].TrialID, result1)
	if err != nil {
		t.Fatalf("Tell on first trial failed: %v", err)
	}

	if state.Tree.Size() != 3 {
		t.Errorf("Expected tree size 3, got %d", state.Tree.Size())
	}
}

func TestTell_DuplicateDifferentHash(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(42)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)
	actions := []tree.ActionLabel{"act-1"}

	state, err := algo.InitTreeWithState(ctx, "root", actions)
	if err != nil {
		t.Fatalf("InitTree failed: %v", err)
	}

	state, trial, err := algo.Ask(ctx, state, actions)
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}

	result := StateScore[string]{
		State: "state-1",
		Reward: tree.Reward{Score: 0.5},
	}
	state, err = algo.Tell(ctx, state, trial.TrialID, result)
	if err != nil {
		t.Fatalf("First Tell failed: %v", err)
	}

	resultDiff := StateScore[string]{
		State: "state-1",
		Reward: tree.Reward{Score: 0.8},
	}
	_, err = algo.Tell(ctx, state, trial.TrialID, resultDiff)
	if err == nil || !strings.Contains(err.Error(), "result hash mismatch") {
		t.Errorf("Expected result hash mismatch error, got %v", err)
	}
}

func TestSnapshot_RoundTripWithPendingTrials(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(42)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)
	actions := []tree.ActionLabel{"act-1"}

	state, err := algo.InitTreeWithState(ctx, "root", actions)
	if err != nil {
		t.Fatalf("InitTree failed: %v", err)
	}

	state, trial1, err := algo.Ask(ctx, state, actions)
	if err != nil {
		t.Fatalf("Ask 1 failed: %v", err)
	}
	state, err = algo.Tell(ctx, state, trial1.TrialID, StateScore[string]{
		State: "child-1",
		Reward: tree.Reward{Score: 0.5},
	})
	if err != nil {
		t.Fatalf("Tell 1 failed: %v", err)
	}

	state, trial2, err := algo.Ask(ctx, state, actions)
	if err != nil {
		t.Fatalf("Ask 2 failed: %v", err)
	}

	snap, err := algo.SaveAlgoSnapshot(state)
	if err != nil {
		t.Fatalf("SaveAlgoSnapshot failed: %v", err)
	}

	if len(snap.PendingTrials) != 1 || snap.PendingTrials[0].TrialID != trial2.TrialID {
		t.Errorf("Expected 1 pending trial in snapshot matching trial2, got %d", len(snap.PendingTrials))
	}
	if len(snap.FinishedTrials) != 1 || snap.FinishedTrials[0].TrialID != trial1.TrialID {
		t.Errorf("Expected 1 finished trial in snapshot matching trial1, got %d", len(snap.FinishedTrials))
	}

	restoredState, err := algo.RestoreAlgoSnapshot(snap)
	if err != nil {
		t.Fatalf("RestoreAlgoSnapshot failed: %v", err)
	}

	if restoredState.Tree.Size() != 2 {
		t.Errorf("Expected restored tree size 2, got %d", restoredState.Tree.Size())
	}

	pendingTrial, ok := restoredState.TrialStore.GetPending(trial2.TrialID)
	if !ok {
		t.Fatalf("Expected restored pending trial %q to exist", trial2.TrialID)
	}
	if pendingTrial.NodeToExpand != trial2.NodeToExpand {
		t.Errorf("Expected node to expand %q, got %q", trial2.NodeToExpand, pendingTrial.NodeToExpand)
	}

	restoredState, err = algo.Tell(ctx, restoredState, trial2.TrialID, StateScore[string]{
		State: "child-2",
		Reward: tree.Reward{Score: 0.8},
	})
	if err != nil {
		t.Fatalf("Tell on restored state failed: %v", err)
	}

	if restoredState.Tree.Size() != 3 {
		t.Errorf("Expected restored tree size 3 after Tell, got %d", restoredState.Tree.Size())
	}
}

func TestSelectAction_DeterministicTiebreak(t *testing.T) {
	sampler := &FixedMockSampler{val: 0.5}
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)

	state, err := algo.InitTreeWithState(context.Background(), "root", []tree.ActionLabel{"act-z", "act-a", "act-b"})
	if err != nil {
		t.Fatalf("InitTree failed: %v", err)
	}

	act := algo.selectAction(state.Tree.Root, state, []tree.ActionLabel{"act-z", "act-a", "act-b"})
	if act != "act-z" {
		t.Errorf("Expected first action 'act-z' to win tie-break, got %q", act)
	}

	act = algo.selectAction(state.Tree.Root, state, []tree.ActionLabel{"act-b", "act-z"})
	if act != "act-b" {
		t.Errorf("Expected first action 'act-b' to win tie-break, got %q", act)
	}
}

func TestTopK(t *testing.T) {
	ctx := context.Background()
	sampler := NewBetaSampler(42)
	codec := StringCodec{}
	algo := NewABMCTSA[string](sampler, codec)
	actions := []tree.ActionLabel{"act-1"}

	state, err := algo.InitTreeWithState(ctx, "root", actions)
	if err != nil {
		t.Fatalf("InitTree failed: %v", err)
	}

	scores := []float64{0.2, 0.9, 0.5, 0.9}
	for i, s := range scores {
		state, trial, err := algo.Ask(ctx, state, actions)
		if err != nil {
			t.Fatalf("Ask failed: %v", err)
		}
		state, err = algo.Tell(ctx, state, trial.TrialID, StateScore[string]{
			State:  fmt.Sprintf("state-%d", i),
			Reward: tree.Reward{Score: tree.Score(s)},
		})
		if err != nil {
			t.Fatalf("Tell failed: %v", err)
		}
	}

	top := state.Tree.TopK(2)
	if len(top) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(top))
	}

	if top[0].Reward.Score != 0.9 || top[0].State != "state-1" {
		t.Errorf("Expected top[0] to be state-1 with score 0.9, got %s (score %f)", top[0].State, top[0].Reward.Score)
	}
	if top[1].Reward.Score != 0.9 || top[1].State != "state-3" {
		t.Errorf("Expected top[1] to be state-3 with score 0.9, got %s (score %f)", top[1].State, top[1].Reward.Score)
	}
}

