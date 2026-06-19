package main

import (
	"ebp-paper-evaluator/pkg/document"
	"ebp-paper-evaluator/pkg/policy"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_NoURLPaperKind(t *testing.T) {
	b, e := os.ReadFile("main.go")
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	if strings.Contains(s, "paper-kind") || strings.Contains(s, "URLIngestor") || strings.Contains(s, "document.Ingest(") {
		t.Fatal("remote ingestion CLI surface present")
	}
}

func TestTripleReview_RequiresThreeModelsWhenNotMock(t *testing.T) {
	if err := validateModels([]string{"a", "b"}); err == nil {
		t.Fatal("accepted fewer than three models")
	}
	if err := validateModels([]string{"a", "b", "a"}); err == nil {
		t.Fatal("accepted duplicate models")
	}
}

func TestTripleReview_RejectsRemotePaperInput(t *testing.T) {
	err := tripleReview([]string{"--paper", "https://example.com/paper.txt", "--mock", "--out", t.TempDir()})
	if !errors.Is(err, document.ErrRemotePaperUnsupported) {
		t.Fatalf("got %v", err)
	}
}

func TestTripleReview_MockRunSucceeds(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bundle")
	err := tripleReview([]string{"--paper", filepath.Join("..", "..", "testdata", "papers", "local_physics_paper.txt"), "--policy", filepath.Join("..", "..", "policies", "ebp_v2_1.md"), "--profile", filepath.Join("..", "..", "policies", "profiles", "automated-no-faithfulness.json"), "--allow-untrusted-policy", "--mock", "--out", out})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(out, "report", "triple_review_report.md")); err != nil {
		t.Fatal(err)
	}
}

func TestTripleReview_PlainPolicyNeedsNoSidecarOrTrustFlag(t *testing.T) {
	out := filepath.Join(t.TempDir(), "plain-policy")
	err := tripleReview([]string{"--paper", filepath.Join("..", "..", "testdata", "papers", "local_physics_paper.txt"), "--policy", filepath.Join("..", "..", "ebp_v2.1.md"), "--mock", "--out", out})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "run", "provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"policy_ir_source": "builtin_ebp_v2_1"`) {
		t.Fatalf("fallback not recorded: %s", b)
	}
}

func TestResolveProfile_OptionalAndBuiltinIDNeedNoFile(t *testing.T) {
	bundle := policy.PolicyBundle{IR: policy.BuiltinEBP21IR()}
	for _, ref := range []string{"", "automated-no-faithfulness", "automated-no-faithfulness.json"} {
		p, err := resolveProfile(ref, bundle)
		if err != nil {
			t.Fatalf("ref %q: %v", ref, err)
		}
		o, ok := p.DebtOverrides["needFaithfulnessReview"]
		if !ok || o.DefaultStatus != policy.DebtNotAssessed || o.IncludedInAutomatedReadiness {
			t.Fatalf("ref %q: %#v", ref, p)
		}
	}
}

func TestTripleReview_TreeQuestNotUsed(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "pkg", "simple", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if strings.Contains(s, "treequest-go") || strings.Contains(s, "pkg/tree") || strings.Contains(s, "pkg/algo") {
			t.Fatalf("TreeQuest reference in %s", file)
		}
	}
}

func TestNonEBPPolicy_StillWorks(t *testing.T) {
	out := filepath.Join(t.TempDir(), "non-ebp")
	err := tripleReview([]string{"--paper", filepath.Join("..", "..", "testdata", "papers", "local_physics_paper.txt"), "--policy", filepath.Join("..", "..", "testdata", "policies", "simple_review_policy.md"), "--allow-untrusted-policy", "--mock", "--out", out})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "run", "provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"treequest_used": false`) {
		t.Fatal("missing TreeQuest boundary")
	}
}

func TestNoFaithfulnessProfile_RemainsNotAssessed(t *testing.T) {
	root := filepath.Join("..", "..", "policies")
	bundle, err := policy.LoadBundle(filepath.Join(root, "ebp_v2_1.md"), policy.LoadOptions{TrustedRoots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := policy.LoadProfile(filepath.Join(root, "profiles", "automated-no-faithfulness.json"), bundle)
	if err != nil {
		t.Fatal(err)
	}
	override, ok := profile.DebtOverrides["needFaithfulnessReview"]
	if !ok || override.DefaultStatus != policy.DebtNotAssessed || override.IncludedInAutomatedReadiness {
		t.Fatalf("faithfulness profile changed: %#v", override)
	}
}
func TestRun_DoesNotInstantiateURLIngestor(t *testing.T) {
	b, e := os.ReadFile("main.go")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), "document.LocalTextIngestor") {
		t.Fatal("run does not use local text ingestor")
	}
}
