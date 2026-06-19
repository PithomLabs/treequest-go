package simple

import (
	"ebp-paper-evaluator/pkg/document"
	"regexp"
	"sort"
	"strings"
)

var wordRE = regexp.MustCompile(`[a-z0-9]+`)

func normalize(s string) string {
	return strings.Join(wordRE.FindAllString(strings.ToLower(s), -1), " ")
}

func tokens(s string) map[string]bool {
	m := map[string]bool{}
	for _, x := range strings.Fields(normalize(s)) {
		m[x] = true
	}
	return m
}

func similarity(a, b string) float64 {
	na, nb := normalize(a), normalize(b)
	if na == "" || nb == "" {
		return 0
	}
	if na == nb {
		return 1
	}
	ta, tb := tokens(a), tokens(b)
	inter, union := 0, len(ta)
	for x := range tb {
		if ta[x] {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func buildAgreement(doc document.DocumentBundle, rs []ReviewerResult) (AgreementLedger, []ClaimCluster) {
	clusters := []ClaimCluster{}
	for _, r := range rs {
		if r.Parsed == nil {
			continue
		}
		for _, claim := range r.Parsed.MainClaims {
			best, bestScore := -1, 0.0
			for i := range clusters {
				x := similarity(claim.ClaimText, clusters[i].CanonicalText)
				if x > bestScore {
					best, bestScore = i, x
				}
			}
			if best < 0 || bestScore < .60 {
				clusters = append(clusters, ClaimCluster{CanonicalText: claim.ClaimText, Claims: map[string][]string{r.ReviewerID: {claim.ClaimText}}})
			} else {
				clusters[best].Claims[r.ReviewerID] = append(clusters[best].Claims[r.ReviewerID], claim.ClaimText)
			}
		}
	}
	for i := range clusters {
		for id := range clusters[i].Claims {
			clusters[i].Reviewers = append(clusters[i].Reviewers, id)
		}
		sort.Strings(clusters[i].Reviewers)
	}
	sort.Slice(clusters, func(i, j int) bool {
		return normalize(clusters[i].CanonicalText) < normalize(clusters[j].CanonicalText)
	})
	ledger := AgreementLedger{UniqueClaimsByReviewer: map[string][]string{}, UniqueDebtsByReviewer: map[string][]string{}, PossibleHallucinations: map[string][]string{}}
	ledger.SharedClaims = []ClaimCluster{}
	ledger.PartiallySharedClaims = []ClaimCluster{}
	ledger.ConflictingClaims = []string{}
	ledger.SharedDebts = []string{}
	ledger.SharedObstructions = []string{}
	for _, r := range rs {
		ledger.UniqueClaimsByReviewer[r.ReviewerID] = []string{}
		ledger.UniqueDebtsByReviewer[r.ReviewerID] = []string{}
		ledger.PossibleHallucinations[r.ReviewerID] = []string{}
	}
	for _, c := range clusters {
		if len(c.Reviewers) == 3 {
			ledger.SharedClaims = append(ledger.SharedClaims, c)
		}
		if len(c.Reviewers) == 2 {
			ledger.PartiallySharedClaims = append(ledger.PartiallySharedClaims, c)
		}
		if len(c.Reviewers) == 1 {
			id := c.Reviewers[0]
			ledger.UniqueClaimsByReviewer[id] = append(ledger.UniqueClaimsByReviewer[id], c.CanonicalText)
		}
		if polarityConflict(c) {
			ledger.ConflictingClaims = append(ledger.ConflictingClaims, c.CanonicalText)
		}
	}
	parseable := 0
	sets := map[string]map[string]bool{}
	for _, r := range rs {
		if r.Parsed == nil {
			continue
		}
		parseable++
		sets[r.ReviewerID] = map[string]bool{}
		for i, c := range clusters {
			if _, ok := c.Claims[r.ReviewerID]; ok {
				sets[r.ReviewerID][string(rune(i))] = true
			}
		}
		for _, c := range r.Parsed.MainClaims {
			if !claimGrounded(doc, c) {
				ledger.PossibleHallucinations[r.ReviewerID] = append(ledger.PossibleHallucinations[r.ReviewerID], c.ClaimText)
			}
		}
	}
	ledger.AgreementScore = meanPairwise(sets)
	switch parseable {
	case 3:
		ledger.AgreementStatus = "full"
	case 2:
		ledger.AgreementStatus = "partial"
	case 1:
		ledger.AgreementStatus = "insufficient_parseable_reviews"
	default:
		ledger.AgreementStatus = "unavailable"
	}
	ledger.SharedDebts, ledger.UniqueDebtsByReviewer = compareLists(rs, func(o *ReviewerOutput) []string {
		var x []string
		for _, c := range o.MainClaims {
			x = append(x, c.EBPDebts...)
		}
		return x
	})
	ledger.SharedObstructions, _ = compareLists(rs, func(o *ReviewerOutput) []string { return o.ObstructionsIdentified })
	return ledger, clusters
}

func compareLists(rs []ReviewerResult, get func(*ReviewerOutput) []string) ([]string, map[string][]string) {
	counts, originals := map[string]int{}, map[string]string{}
	per := map[string]map[string]bool{}
	parseable := 0
	for _, r := range rs {
		per[r.ReviewerID] = map[string]bool{}
		if r.Parsed == nil {
			continue
		}
		parseable++
		for _, v := range get(r.Parsed) {
			n := normalize(v)
			if n != "" && !per[r.ReviewerID][n] {
				per[r.ReviewerID][n] = true
				counts[n]++
				originals[n] = v
			}
		}
	}
	shared := []string{}
	unique := map[string][]string{}
	for n, count := range counts {
		if count == parseable && parseable > 1 {
			shared = append(shared, originals[n])
		}
	}
	for _, r := range rs {
		unique[r.ReviewerID] = []string{}
		for n := range per[r.ReviewerID] {
			if counts[n] == 1 {
				unique[r.ReviewerID] = append(unique[r.ReviewerID], originals[n])
			}
		}
		sort.Strings(unique[r.ReviewerID])
	}
	sort.Strings(shared)
	return shared, unique
}

func meanPairwise(sets map[string]map[string]bool) float64 {
	ids := []string{}
	for id := range sets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) < 2 {
		return 0
	}
	sum, n := 0.0, 0
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			a, b := sets[ids[i]], sets[ids[j]]
			inter, union := 0, len(a)
			for x := range b {
				if a[x] {
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
	}
	return clamp(sum / float64(n))
}

func polarityConflict(c ClaimCluster) bool {
	pos, neg := false, false
	for _, cs := range c.Claims {
		for _, s := range cs {
			n := " " + normalize(s) + " "
			isNeg := strings.Contains(n, " not ") || strings.Contains(n, " no ") || strings.Contains(n, " cannot ")
			neg = neg || isNeg
			pos = pos || !isNeg
		}
	}
	return pos && neg
}

func paperText(d document.DocumentBundle) string {
	var b strings.Builder
	for _, s := range d.Sections {
		b.WriteString(s.Text)
		b.WriteByte('\n')
	}
	return normalize(b.String())
}
func claimGrounded(d document.DocumentBundle, c ReviewerClaim) bool {
	p := paperText(d)
	for _, q := range c.EvidenceQuotes {
		if normalize(q.Quote) != "" && strings.Contains(p, normalize(q.Quote)) {
			return true
		}
	}
	return false
}
