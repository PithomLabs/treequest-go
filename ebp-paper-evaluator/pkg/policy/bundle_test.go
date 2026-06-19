package policy

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestPolicyTrustSchemaAndNonEBP(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "policies")
	path := filepath.Join(root, "simple_review_policy.md")
	if _, e := LoadBundle(path, LoadOptions{}); !errors.Is(e, ErrUntrustedPolicy) {
		t.Fatalf("want untrusted error, got %v", e)
	}
	b, e := LoadBundle(path, LoadOptions{TrustedRoots: []string{root}})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, d := range b.IR.DebtItems {
		found = found || d.ID == "sourceSupport"
	}
	if len(b.IR.DebtItems) != 3 || !found {
		t.Fatalf("non-EBP policy not loaded: %#v", b.IR.DebtItems)
	}
}
func TestAutomatedProfileLeavesFaithfulnessNotAssessed(t *testing.T) {
	root := filepath.Join("..", "..", "policies")
	b, e := LoadBundle(filepath.Join(root, "ebp_v2_1.md"), LoadOptions{TrustedRoots: []string{root}})
	if e != nil {
		t.Fatal(e)
	}
	p, e := LoadProfile(filepath.Join(root, "profiles", "automated-no-faithfulness.json"), b)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for id, o := range p.DebtOverrides {
		for _, d := range b.IR.DebtItems {
			if d.ID == id && !d.Automated {
				found = true
				if o.DefaultStatus != DebtNotAssessed || o.IncludedInAutomatedReadiness {
					t.Fatalf("bad override: %#v", o)
				}
			}
		}
	}
	if !found {
		t.Fatal("profile did not override a non-automated debt")
	}
}
