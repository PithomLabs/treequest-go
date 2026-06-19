package policy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestPlainMarkdownUsesBuiltinEBP21(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.md")
	if err := os.WriteFile(path, []byte("# Plain policy\n\nIdeas enter freely.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := LoadBundle(path, LoadOptions{TrustedRoots: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if b.IRSource != "builtin_ebp_v2_1" {
		t.Fatalf("got IR source %q", b.IRSource)
	}
	got, _ := canonicalIR(b.IR)
	want, _ := canonicalIR(BuiltinEBP21IR())
	if !reflect.DeepEqual(got, want) {
		t.Fatal("plain Markdown did not receive built-in EBP 2.1 IR")
	}
}

func TestBuiltinEBP21MatchesCheckedInPolicy(t *testing.T) {
	root := filepath.Join("..", "..", "policies")
	b, err := LoadBundle(filepath.Join(root, "ebp_v2_1.md"), LoadOptions{TrustedRoots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	if b.IRSource != "inline_policy_json" {
		t.Fatalf("got IR source %q", b.IRSource)
	}
	got, _ := canonicalIR(b.IR)
	want, _ := canonicalIR(BuiltinEBP21IR())
	if !reflect.DeepEqual(got, want) {
		t.Fatal("built-in EBP 2.1 IR drifted from checked-in policy")
	}
}

func TestCompiledSidecarTakesPrecedenceAndMismatchFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.md")
	sidecar := filepath.Join(dir, "policy.policy.json")
	if err := os.WriteFile(path, []byte("# Plain policy\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Compile(path, sidecar); err != nil {
		t.Fatal(err)
	}
	b, err := LoadBundle(path, LoadOptions{TrustedRoots: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if b.IRSource != "compiled_sidecar" {
		t.Fatalf("got IR source %q", b.IRSource)
	}
	if err := os.WriteFile(path, []byte("# Changed policy\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadBundle(path, LoadOptions{TrustedRoots: []string{dir}}); err == nil || !strings.Contains(err.Error(), "source hash mismatch") {
		t.Fatalf("expected hash mismatch, got %v", err)
	}
}

func TestMalformedExistingSidecarFailsClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.md")
	if err := os.WriteFile(path, []byte("# Plain policy\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.policy.json"), []byte("{bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBundle(path, LoadOptions{TrustedRoots: []string{dir}}); err == nil {
		t.Fatal("malformed sidecar silently fell back")
	}
}

func TestCompilePlainMarkdownUsesBuiltinEBP21(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.md")
	out := filepath.Join(dir, "compiled.json")
	if err := os.WriteFile(path, []byte("# Plain policy\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Compile(path, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var compiled compiledPolicy
	if err = json.Unmarshal(b, &compiled); err != nil {
		t.Fatal(err)
	}
	got, _ := canonicalIR(compiled.IR)
	want, _ := canonicalIR(BuiltinEBP21IR())
	if !reflect.DeepEqual(got, want) {
		t.Fatal("compiled plain policy used wrong IR")
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

func TestDefaultProfileNeedsNoFileAndExcludesHumanDebts(t *testing.T) {
	b := PolicyBundle{IR: BuiltinEBP21IR()}
	p := DefaultProfile(b)
	o, ok := p.DebtOverrides["needFaithfulnessReview"]
	if !ok || o.DefaultStatus != DebtNotAssessed || o.IncludedInAutomatedReadiness {
		t.Fatalf("bad generated profile: %#v", p)
	}
}
