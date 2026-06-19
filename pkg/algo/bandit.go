package algo

import (
	"math/rand/v2"
	"sort"
	"sync"

	"gonum.org/v1/gonum/stat/distuv"
	tree "github.com/PithomLabs/treequest-go/pkg/tree"
)

// Sampler defines the interface for Beta distribution random sampling.
type Sampler interface {
	SampleBeta(alpha, beta float64) float64
}

// BetaSampler implements the Sampler interface using gonum's Beta distribution.
type BetaSampler struct {
	mu  sync.Mutex
	src rand.Source
}

// NewBetaSampler creates a new BetaSampler with a given seed.
func NewBetaSampler(seed int64) *BetaSampler {
	// A simple SplitMix64 mixer to derive a non-colliding second seed component
	s1 := uint64(seed)
	s2 := s1 + 0x9e3779b97f4a7c15
	s2 = (s2 ^ (s2 >> 30)) * 0xbf58476d1ce4e5b9
	s2 = (s2 ^ (s2 >> 27)) * 0x94d049bb133111eb
	s2 = s2 ^ (s2 >> 31)

	return &BetaSampler{
		src: rand.NewPCG(s1, s2),
	}
}

// SampleBeta draws a single sample from Beta(alpha, beta) using gonum's distuv.
func (s *BetaSampler) SampleBeta(alpha, beta float64) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := distuv.Beta{
		Alpha: alpha,
		Beta:  beta,
		Src:   s.src,
	}
	return d.Rand()
}

// ChooseWider samples from Wider and Deeper bandits and returns true if wider wins.
func ChooseWider(sampler Sampler, wider, deeper tree.BetaParams) bool {
	return sampler.SampleBeta(wider.Alpha, wider.Beta) > sampler.SampleBeta(deeper.Alpha, deeper.Beta)
}

// ChooseAction samples from each action's bandit and returns the action with the highest sample.
// Ties are broken deterministically by sorting action labels alphabetically.
func ChooseAction(sampler Sampler, bandits map[tree.ActionLabel]tree.BetaParams) tree.ActionLabel {
	keys := make([]tree.ActionLabel, 0, len(bandits))
	for k := range bandits {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	var bestAction tree.ActionLabel
	bestVal := -1.0
	for _, act := range keys {
		params := bandits[act]
		val := sampler.SampleBeta(params.Alpha, params.Beta)
		if val > bestVal {
			bestVal = val
			bestAction = act
		}
	}
	return bestAction
}

// UpdateBandit applies fractional Thompson Sampling updates.
func UpdateBandit(bp tree.BetaParams, score tree.Score) tree.BetaParams {
	return tree.BetaParams{
		Alpha: bp.Alpha + float64(score),
		Beta:  bp.Beta + (1.0 - float64(score)),
	}
}
