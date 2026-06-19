package eval

import (
	"context"
	"ebp-paper-evaluator/pkg/document"
	"ebp-paper-evaluator/pkg/metadata"
	"ebp-paper-evaluator/pkg/policy"
	"path/filepath"
	"testing"
)

func TestPolicyRewardCapAndProfile(t *testing.T) {
	root := filepath.Join("..", "..", "policies")
	p, e := policy.LoadBundle(filepath.Join(root, "ebp_v2_1.md"), policy.LoadOptions{TrustedRoots: []string{root}})
	if e != nil {
		t.Fatal(e)
	}
	prof, e := policy.LoadProfile(filepath.Join(root, "profiles", "automated-no-faithfulness.json"), p)
	if e != nil {
		t.Fatal(e)
	}
	d := document.DocumentBundle{ID: "d", Hash: "h", Sections: []document.Section{{ID: "s", Text: "quoted evidence"}}}
	sp := document.EvidenceSpan{ID: "x", Section: "s", Start: 0, End: 15, Quote: "quoted evidence", SourceHash: "h"}
	cl := metadata.ClaimRecord{ID: "c", Text: "claim", EvidenceSpans: []document.EvidenceSpan{sp}, SourceHash: "h"}
	s := InitialState(d, p, prof, cl)
	foundNotAssessed := false
	for id, o := range prof.DebtOverrides {
		if o.DefaultStatus == policy.DebtNotAssessed && s.Debts[id].Status == policy.DebtNotAssessed {
			foundNotAssessed = true
		}
	}
	if !foundNotAssessed {
		t.Fatal("profile not_assessed status")
	}
	s.ConstructiveAnalysis.Analysis = "This solved the problem"
	for _, x := range p.IR.DebtItems {
		q := s.Debts[x.ID]
		if q.Status != policy.DebtNotAssessed {
			q.Status = policy.DebtRetired
		}
		s.Debts[x.ID] = q
	}
	v := GenericValidator{}.Validate(context.Background(), ValidationInput{Policy: p, Profile: prof, Document: d, Claim: cl, Candidate: s})
	scores := map[string]float64{}
	for _, x := range p.IR.RubricDimensions {
		scores[x.ID] = 1
	}
	r, e := BuildReward(p.IR, EvaluatorJudgment{DimensionScores: scores}, v)
	if e != nil {
		t.Fatal(e)
	}
	if r.DerivedReward > .2 {
		t.Fatalf("cap not applied: %v", r.DerivedReward)
	}
	s.Validation = v
	if AutomatedReady(p.IR, prof, s) {
		t.Fatal("forbidden language should block readiness")
	}
}
