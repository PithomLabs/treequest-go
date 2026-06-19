package eval

import (
	"context"
	"ebp-paper-evaluator/pkg/document"
	"ebp-paper-evaluator/pkg/metadata"
	"ebp-paper-evaluator/pkg/policy"
	"fmt"
	"regexp"
	"strings"
)

type ValidationInput struct {
	Policy    policy.PolicyBundle
	Profile   policy.EvaluationProfile
	Document  document.DocumentBundle
	Claim     metadata.ClaimRecord
	Candidate AssessmentState
}
type Validator interface {
	ID() string
	Validate(context.Context, ValidationInput) ValidationResult
}
type GenericValidator struct{}

func (GenericValidator) ID() string { return "generic_policy_validator" }
func (GenericValidator) Validate(_ context.Context, in ValidationInput) ValidationResult {
	r := ValidationResult{Passed: true}
	add := func(id, msg string) {
		r.Passed = false
		r.Flags = append(r.Flags, ValidationFlag{ID: id, Message: msg})
		for _, h := range in.Policy.IR.HardFailures {
			if h.Flag == id {
				r.RewardCaps = append(r.RewardCaps, RewardCap{FailureID: h.ID, Cap: h.RewardCap})
				if h.BlocksReadiness {
					r.ReadinessBlocked = true
				}
			}
		}
	}
	ids := map[string]policy.DebtItem{}
	for _, d := range in.Policy.IR.DebtItems {
		ids[d.ID] = d
	}
	if len(in.Candidate.Debts) != len(ids) {
		add("invalid_debt_partition", "debt set does not match policy")
	}
	for id, dec := range in.Candidate.Debts {
		if _, ok := ids[id]; !ok {
			add("invalid_debt_partition", "unknown debt "+id)
		}
		if dec.ItemID != id {
			add("invalid_debt_partition", "debt key mismatch")
		}
	}
	for id, o := range in.Profile.DebtOverrides {
		if dec, ok := in.Candidate.Debts[id]; ok && o.DefaultStatus == policy.DebtNotAssessed && dec.Status != policy.DebtNotAssessed {
			add("profile_status_violation", fmt.Sprintf("%s must remain not_assessed", id))
		}
	}
	if len(in.Claim.EvidenceSpans) == 0 {
		add("missing_evidence_span", "claim has no evidence")
	}
	for _, s := range in.Claim.EvidenceSpans {
		if e := document.VerifySpan(in.Document, s); e != nil {
			add("invented_source_quote", e.Error())
		}
	}
	texts := strings.Join([]string{in.Candidate.ConstructiveAnalysis.Analysis, in.Candidate.AdversarialAnalysis.Analysis, in.Candidate.EvaluatorJudgment.Rationale}, "\n")
	for _, h := range in.Policy.IR.HardFailures {
		for _, p := range h.Patterns {
			if regexp.MustCompile(p).MatchString(texts) {
				add(h.Flag, "configured forbidden language detected")
			}
		}
	}
	return r
}
func AutomatedReady(ir policy.PolicyIR, p policy.EvaluationProfile, s AssessmentState) bool {
	if s.Validation.ReadinessBlocked {
		return false
	}
	for _, d := range ir.DebtItems {
		include := d.Required
		if o, ok := p.DebtOverrides[d.ID]; ok {
			include = o.IncludedInAutomatedReadiness
		}
		if include {
			st := s.Debts[d.ID].Status
			if st != policy.DebtRetired && st != policy.DebtNotApplicable {
				return false
			}
		}
	}
	return true
}
