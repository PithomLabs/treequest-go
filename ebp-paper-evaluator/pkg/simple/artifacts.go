package simple

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var secretRE = regexp.MustCompile(`(?i)(OPENROUTER_API_KEY|authorization:\s*bearer|sk-[A-Za-z0-9_-]{16,})`)

func saveArtifacts(c Config, r Result, clusters []ClaimCluster) error {
	for _, d := range []string{"source", "policy", "reviews", "consensus", "report", "run"} {
		if err := os.MkdirAll(filepath.Join(c.Out, d), 0755); err != nil {
			return err
		}
	}
	if err := writeJSON(c.Out, "source/source_ref.json", c.Document.Source); err != nil {
		return err
	}
	if err := writeText(c.Out, "source/source_hash.txt", c.Document.Hash+"\n", 0644); err != nil {
		return err
	}
	if err := writeText(c.Out, "policy/policy_snapshot.md", c.Policy.Markdown, 0644); err != nil {
		return err
	}
	if err := writeJSON(c.Out, "policy/policy_ir.json", c.Policy.IR); err != nil {
		return err
	}
	if err := writeText(c.Out, "policy/policy_hash.txt", c.Policy.SourceHash+"\n"+c.Policy.IRHash+"\n", 0644); err != nil {
		return err
	}
	for _, x := range r.Reviewers {
		prefix := "reviews/" + x.ReviewerID
		if x.Raw != "" {
			if err := writeText(c.Out, prefix+"_raw.txt", x.Raw, 0644); err != nil {
				return err
			}
		}
		if x.Parsed != nil {
			if err := writeJSON(c.Out, prefix+"_parsed.json", x.Parsed); err != nil {
				return err
			}
		}
		if x.Error != nil {
			if err := writeJSON(c.Out, prefix+"_error.json", x.Error); err != nil {
				return err
			}
		}
		if err := writeJSON(c.Out, prefix+"_score.json", x.Score); err != nil {
			return err
		}
	}
	if err := writeJSON(c.Out, "consensus/agreement_ledger.json", r.Agreement); err != nil {
		return err
	}
	disagreement := map[string]any{"unique_claims_by_reviewer": r.Agreement.UniqueClaimsByReviewer, "partially_shared_claims": r.Agreement.PartiallySharedClaims, "conflicting_claims": r.Agreement.ConflictingClaims, "unique_debts_by_reviewer": r.Agreement.UniqueDebtsByReviewer, "possible_hallucinations": r.Agreement.PossibleHallucinations}
	if err := writeJSON(c.Out, "consensus/disagreement_ledger.json", disagreement); err != nil {
		return err
	}
	if err := writeJSON(c.Out, "consensus/combined_claims.json", clusters); err != nil {
		return err
	}
	if err := writeJSON(c.Out, "consensus/scoring_summary.json", r.Summary); err != nil {
		return err
	}
	usage := sortedPromptRecords(c.Tracker.Snapshot())
	if err := writeJSON(c.Out, "run/budget_usage.json", usage); err != nil {
		return err
	}
	if err := writeJSON(c.Out, "run/prompt_ledger.json", usage.PromptRecords); err != nil {
		return err
	}
	if err := writeJSON(c.Out, "run/provenance.json", r.Provenance); err != nil {
		return err
	}
	report := renderReport(c, r)
	if secretRE.MatchString(report) {
		return errors.New("report contains credential-like material")
	}
	for _, p := range c.Policy.IR.ReportLanguage.ForbiddenPatterns {
		re, e := regexp.Compile(p)
		if e != nil {
			return e
		}
		if re.MatchString(report) {
			return errors.New("report contains policy-forbidden affirmative language")
		}
	}
	return writeText(c.Out, "report/triple_review_report.md", report, 0644)
}

func renderReport(c Config, r Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Automated Candidate EBP Triple Review\n\nStatus: `%s`\n\n## Paper metadata\n\n- Source: `%s`\n- Source hash: `%s`\n- Policy: `%s`\n- Profile: `%s`\n- Mode: `simple_triple_review`\n- TreeQuest used: `false`\n\n## Reviewers and scores\n\n| Reviewer | Model | Call | Parse | Reviewer score |\n|---|---|---|---|---:|\n", FinalStatus, c.Document.Source.InputPath, c.Document.Hash, c.Policy.ID, c.Profile.ID)
	for _, x := range r.Reviewers {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %.3f |\n", x.ReviewerID, x.ModelID, x.Score.CallStatus, x.Score.ParseStatus, x.Score.ReviewerScore)
	}
	fmt.Fprintf(&b, "\nRun status: `%s`; run completeness: `%.2f`; parseable-only mean reviewer score: `%.3f`; failure-inclusive mean: `%.3f`.\n", r.Summary.RunStatus, r.Summary.RunCompleteness, r.Summary.MeanReviewerScoreParseableOnly, r.Summary.MeanReviewerScoreWithFailures)
	if r.Summary.RunCompleteness < 1 {
		fmt.Fprintf(&b, "\nThis was a partial triple-review run. Agreement metrics are degraded.\n")
	}
	fmt.Fprintf(&b, "\n## Shared claims\n\n")
	if len(r.Agreement.SharedClaims) == 0 {
		b.WriteString("None detected across all three parseable reviewers.\n")
	}
	for _, x := range r.Agreement.SharedClaims {
		fmt.Fprintf(&b, "- %s\n", x.CanonicalText)
	}
	fmt.Fprintf(&b, "\n## Unique and partially shared claims\n\n")
	if len(r.Agreement.PartiallySharedClaims) == 0 {
		b.WriteString("No claims were found by exactly two reviewers.\n\n")
	}
	for _, x := range r.Agreement.PartiallySharedClaims {
		fmt.Fprintf(&b, "- %s (reviewers: %s)\n", x.CanonicalText, strings.Join(x.Reviewers, ", "))
	}
	ids := []string{}
	for id := range r.Agreement.UniqueClaimsByReviewer {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(&b, "### %s\n\n", id)
		if len(r.Agreement.UniqueClaimsByReviewer[id]) == 0 {
			b.WriteString("- None\n")
		}
		for _, x := range r.Agreement.UniqueClaimsByReviewer[id] {
			fmt.Fprintf(&b, "- %s\n", x)
		}
	}
	b.WriteString("\n## Debts by reviewer\n\n")
	for _, x := range r.Reviewers {
		fmt.Fprintf(&b, "### %s\n\n", x.ReviewerID)
		if x.Parsed == nil {
			b.WriteString("- Not available\n")
			continue
		}
		seen := map[string]bool{}
		for _, cl := range x.Parsed.MainClaims {
			for _, d := range cl.EBPDebts {
				if !seen[d] {
					fmt.Fprintf(&b, "- %s\n", d)
					seen[d] = true
				}
			}
		}
	}
	fmt.Fprintf(&b, "\n## Agreement and disagreement\n\nAgreement score: `%.3f` (`%s`).\n\nConflicts detected: %d. Shared obstructions: %d.\n", r.Agreement.AgreementScore, r.Agreement.AgreementStatus, len(r.Agreement.ConflictingClaims), len(r.Agreement.SharedObstructions))
	b.WriteString("\n## Possible unsupported or weakly grounded claims\n\n")
	for _, id := range ids {
		for _, x := range r.Agreement.PossibleHallucinations[id] {
			fmt.Fprintf(&b, "- %s: %s\n", id, x)
		}
	}
	b.WriteString("\n## Recommended human review steps\n\n- Verify quoted evidence against the local paper.\n- Review unique and conflicting claims manually.\n- Assess formal-to-physical faithfulness.\n- Treat relative claim coverage as reviewer-relative, not paper-truth coverage.\n\n## Limitations\n\n")
	for _, x := range RequiredLimitations {
		fmt.Fprintf(&b, "%s\n\n", x)
	}
	return b.String()
}

func writeJSON(root, path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return writeText(root, path, string(append(b, '\n')), 0644)
}
func writeText(root, path, s string, mode os.FileMode) error {
	if secretRE.MatchString(s) {
		return fmt.Errorf("artifact %s contains credential-like material", path)
	}
	return os.WriteFile(filepath.Join(root, path), []byte(s), mode)
}
