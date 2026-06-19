package eval

import (
	"ebp-paper-evaluator/pkg/policy"
	"fmt"
	"math"
)

func BuildReward(ir policy.PolicyIR, j EvaluatorJudgment, v ValidationResult) (RewardBreakdown, error) {
	r := RewardBreakdown{Dimensions: j.DimensionScores, AppliedCaps: v.RewardCaps}
	for _, d := range ir.RubricDimensions {
		x, ok := j.DimensionScores[d.ID]
		if !ok {
			return r, fmt.Errorf("missing rubric dimension %q", d.ID)
		}
		if x < 0 || x > 1 || math.IsNaN(x) || math.IsInf(x, 0) {
			return r, fmt.Errorf("invalid score for %q", d.ID)
		}
		r.WeightedBase += x * d.Weight
	}
	r.DerivedReward = r.WeightedBase
	for _, c := range v.RewardCaps {
		if c.Cap < r.DerivedReward {
			r.DerivedReward = c.Cap
		}
	}
	return r, nil
}
