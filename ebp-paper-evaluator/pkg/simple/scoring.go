package simple

import (
	"math"
	"regexp"
	"strings"
)

func scoreReviewer(c Config, r ReviewerResult, clusters []ClaimCluster, all []ReviewerResult) ReviewerScore {
	if r.Error != nil {
		return zeroScore(r.ReviewerID, "reviewer_call_failed", "not_applicable_no_content", "run_completeness_not_review_quality")
	}
	if r.Parsed == nil {
		return zeroScore(r.ReviewerID, "response_returned", "review_parse_failed", "review_quality_unscorable")
	}
	o := r.Parsed
	s := Scores{ClaimCoverageMeaning: "relative claim coverage among combined parseable reviewer claims"}
	covered := 0
	for _, cl := range clusters {
		if _, ok := cl.Claims[r.ReviewerID]; ok {
			covered++
		}
	}
	if len(clusters) > 0 {
		s.ClaimCoverage = float64(covered) / float64(len(clusters))
	}
	grounded := 0
	for _, cl := range o.MainClaims {
		if claimGrounded(c.Document, cl) {
			grounded++
		} else {
			for _, q := range cl.EvidenceQuotes {
				if strings.TrimSpace(q.SectionHint) != "" {
					s.SourceGroundingNotes = append(s.SourceGroundingNotes, cl.ClaimID+": section_hint_only")
					break
				}
			}
		}
	}
	if len(o.MainClaims) > 0 {
		s.SourceGrounding = float64(grounded) / float64(len(o.MainClaims))
	}
	required := map[string]bool{}
	for _, d := range c.Policy.IR.DebtItems {
		if d.Required && d.Automated {
			required[d.ID] = true
		}
	}
	found := map[string]bool{}
	for _, cl := range o.MainClaims {
		for _, d := range cl.EBPDebts {
			if required[d] {
				found[d] = true
			}
		}
	}
	if len(required) > 0 {
		s.EBPDebtCoverage = float64(len(found)) / float64(len(required))
	}
	s.MapInvariantQuality = (present(o.MapsIdentified) + present(o.InvariantsIdentified)) / 2
	s.ToyNullObstructionAwareness = (present(o.ToyChecksIdentified) + present(o.NullModelsIdentified) + present(o.ObstructionsIdentified)) / 3
	s.FaithfulnessHumility = faithfulnessScore(o)
	s.NoOverclaimDiscipline = overclaimScore(c, o)
	sets := map[string]map[string]bool{}
	for _, x := range all {
		if x.Parsed == nil {
			continue
		}
		sets[x.ReviewerID] = map[string]bool{}
		for i, cl := range clusters {
			if _, ok := cl.Claims[x.ReviewerID]; ok {
				sets[x.ReviewerID][string(rune(i))] = true
			}
		}
	}
	if len(sets) > 1 {
		own := sets[r.ReviewerID]
		sum, n := 0.0, 0
		for id, other := range sets {
			if id == r.ReviewerID {
				continue
			}
			inter, union := 0, len(own)
			for x := range other {
				if own[x] {
					inter++
				} else {
					union++
				}
			}
			if union == 0 {
				sum += 1
			} else {
				sum += float64(inter) / float64(union)
			}
			n++
		}
		s.CrossModelAgreement = sum / float64(n)
	}
	den := len(o.MainClaims)
	if den < 1 {
		den = 1
	}
	useful := nonblank(o.RecommendedNextSteps)
	if useful > den {
		useful = den
	}
	s.NextStepUsefulness = float64(useful) / float64(den)
	vals := []float64{s.ClaimCoverage, s.SourceGrounding, s.EBPDebtCoverage, s.MapInvariantQuality, s.ToyNullObstructionAwareness, s.FaithfulnessHumility, s.NoOverclaimDiscipline, s.CrossModelAgreement, s.NextStepUsefulness}
	total := 0.0
	for _, v := range vals {
		total += clamp(v)
	}
	final := clamp(total / float64(len(vals)))
	return ReviewerScore{ReviewerID: r.ReviewerID, CallStatus: "response_returned", ParseStatus: "parsed", Scores: s, ReviewerScore: final, FinalScore: final, Status: FinalStatus}
}

func zeroScore(id, call, parse, affects string) ReviewerScore {
	return ReviewerScore{ReviewerID: id, CallStatus: call, ParseStatus: parse, Scores: Scores{ClaimCoverageMeaning: "relative claim coverage among combined parseable reviewer claims"}, ReviewerScore: 0, FinalScore: 0, Status: FinalStatus, FailureAffects: affects}
}
func present(x []string) float64 {
	if nonblank(x) > 0 {
		return 1
	}
	return 0
}
func nonblank(x []string) int {
	n := 0
	for _, s := range x {
		if strings.TrimSpace(s) != "" {
			n++
		}
	}
	return n
}
func faithfulnessScore(o *ReviewerOutput) float64 {
	limit := present(o.FaithfulnessLimits)
	joined := strings.ToLower(strings.Join(o.Limitations, " "))
	explicit := 0.0
	if strings.Contains(joined, "human faithfulness review was not performed") {
		explicit = 1
	}
	return (limit + explicit) / 2
}
func overclaimScore(c Config, o *ReviewerOutput) float64 {
	b := strings.Join([]string{o.PaperSummary, o.OverallAssessment, strings.Join(o.RecommendedNextSteps, " ")}, " ")
	for _, h := range c.Policy.IR.HardFailures {
		for _, p := range h.Patterns {
			if re, e := regexp.Compile(p); e == nil && re.MatchString(b) {
				return 0
			}
		}
	}
	for _, p := range c.Policy.IR.ReportLanguage.ForbiddenPatterns {
		if re, e := regexp.Compile(p); e == nil && re.MatchString(b) {
			return 0
		}
	}
	return 1
}
func clamp(x float64) float64 {
	if math.IsNaN(x) || x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
