package artifact

import (
	"ebp-paper-evaluator/pkg/budget"
	"ebp-paper-evaluator/pkg/document"
	"ebp-paper-evaluator/pkg/metadata"
	"ebp-paper-evaluator/pkg/policy"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactBundle_NoPromotionLanguage(t *testing.T) {
	dir := t.TempDir()
	p := policy.PolicyBundle{IR: policy.PolicyIR{ReportLanguage: policy.ReportLanguagePolicy{StatusLabel: "CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED", RequiredStatements: []string{"Automated candidate assessment."}, ForbiddenPatterns: []string{"(?i)validated physics"}}}}
	prof := policy.EvaluationProfile{StatusLabel: "CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED", RequiredNotes: []string{"Automated candidate assessment."}}
	d := document.DocumentBundle{Hash: "h", Source: document.SourceMetadata{InputPath: "paper.txt"}}
	pp := metadata.PaperProfile{}
	if e := Save(dir, d, p, prof, pp, map[string]ClaimArtifact{}, budget.New().Snapshot(), Provenance{}); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(dir, "report", "assessment_report.md"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(strings.ToLower(string(b)), "validated physics") {
		t.Fatal("unsafe language")
	}
}
func TestArtifactBundle_NoSecrets(t *testing.T) {
	if !secret("authorization: bearer token") || !secret("sk-abcdefghijklmnop") {
		t.Fatal("secret detector regression")
	}
}
func TestCopyLocalSource(t *testing.T) {
	src := filepath.Join(t.TempDir(), "paper.txt")
	if e := os.WriteFile(src, []byte("original"), 0600); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	if e := os.Mkdir(filepath.Join(dir, "source"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := CopyLocalSource(src, dir); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "source", "original.txt"))
	if string(b) != "original" {
		t.Fatal("source changed")
	}
}
