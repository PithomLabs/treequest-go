package artifact

import (
	"ebp-paper-evaluator/pkg/budget"
	"ebp-paper-evaluator/pkg/document"
	"ebp-paper-evaluator/pkg/eval"
	"ebp-paper-evaluator/pkg/metadata"
	"ebp-paper-evaluator/pkg/policy"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type ClaimArtifact struct {
	Assessment   eval.AssessmentState
	TreeSnapshot any
}
type Provenance struct {
	Timestamp        string            `json:"timestamp"`
	DocumentHash     string            `json:"document_hash"`
	PolicySourceHash string            `json:"policy_source_hash"`
	PolicyIRHash     string            `json:"policy_ir_hash"`
	ProfileID        string            `json:"profile_id"`
	Models           map[string]string `json:"models"`
	StopReasons      map[string]string `json:"stop_reasons"`
	PromptHashes     []string          `json:"prompt_hashes"`
}

func Save(dir string, d document.DocumentBundle, p policy.PolicyBundle, prof policy.EvaluationProfile, pp metadata.PaperProfile, claims map[string]ClaimArtifact, b budget.UsageSnapshot, prov Provenance) error {
	dirs := []string{"source", "policy", "document", "claims", "tree", "report", "run"}
	for _, x := range dirs {
		if e := os.MkdirAll(filepath.Join(dir, x), 0755); e != nil {
			return e
		}
	}
	writes := map[string]any{"document/document_bundle.json": d, "document/evidence_ledger.json": evidence(pp), "policy/policy_ir.json": p.IR, "claims/claim_ledger.json": pp, "run/budget_usage.json": b}
	writes["run/prompt_ledger.json"] = b.PromptRecords
	prov.Timestamp = time.Now().UTC().Format(time.RFC3339)
	writes["run/provenance.json"] = prov
	for path, v := range writes {
		if e := writeJSON(filepath.Join(dir, path), v); e != nil {
			return e
		}
	}
	if e := os.WriteFile(filepath.Join(dir, "policy/policy_snapshot.md"), []byte(p.Markdown), 0644); e != nil {
		return e
	}
	os.WriteFile(filepath.Join(dir, "policy/policy_hash.txt"), []byte(p.SourceHash+"\n"+p.IRHash+"\n"), 0644)
	os.WriteFile(filepath.Join(dir, "source/source_hash.txt"), []byte(d.Hash+"\n"), 0644)
	writeJSON(filepath.Join(dir, "source/source_ref.json"), d.Source)
	for id, c := range claims {
		if e := writeJSON(filepath.Join(dir, "claims", "claim_"+id+"_assessment.json"), c.Assessment); e != nil {
			return e
		}
		if e := writeJSON(filepath.Join(dir, "tree", "claim_"+id+"_tree_snapshot.json"), c.TreeSnapshot); e != nil {
			return e
		}
	}
	report := render(p, prof, pp, claims)
	if secret(report) {
		return fmt.Errorf("report contains credential-like material")
	}
	for _, pattern := range p.IR.ReportLanguage.ForbiddenPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid report language pattern: %w", err)
		}
		if re.MatchString(report) {
			return fmt.Errorf("report contains policy-forbidden affirmative language")
		}
	}
	return os.WriteFile(filepath.Join(dir, "report", "assessment_report.md"), []byte(report), 0644)
}
func CopyLocalSource(sourcePath, dir string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(sourcePath))
	if ext != ".txt" && ext != ".md" {
		return fmt.Errorf("cannot copy unsupported source extension %q", ext)
	}
	return os.WriteFile(filepath.Join(dir, "source", "original"+ext), data, 0600)
}
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	if secret(string(b)) {
		return fmt.Errorf("artifact %s contains credential-like material", path)
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
func secret(s string) bool {
	return regexp.MustCompile(`(?i)(OPENROUTER_API_KEY|authorization:\s*bearer|sk-[A-Za-z0-9_-]{16,})`).MatchString(s)
}
func evidence(p metadata.PaperProfile) []document.EvidenceSpan {
	var x []document.EvidenceSpan
	for _, c := range p.Claims {
		x = append(x, c.EvidenceSpans...)
	}
	return x
}
func render(p policy.PolicyBundle, prof policy.EvaluationProfile, pp metadata.PaperProfile, cs map[string]ClaimArtifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Automated Candidate Assessment\n\nStatus: `%s`\n\n", prof.StatusLabel)
	for _, n := range prof.RequiredNotes {
		fmt.Fprintf(&b, "- %s\n", n)
	}
	for _, c := range pp.Claims {
		a := cs[c.ID].Assessment
		fmt.Fprintf(&b, "\n## %s\n\n%s\n\nDerived reward: %.3f\n\nAutomated review ready: %v\n", c.ID, c.Text, a.Reward.DerivedReward, a.AutomatedReviewReady)
	}
	return b.String()
}
