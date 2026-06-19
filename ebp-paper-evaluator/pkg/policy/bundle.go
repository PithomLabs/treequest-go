package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const SupportedSchemaVersion = "1"

var ErrUntrustedPolicy = errors.New("untrusted policy requires explicit approval")

type LoadOptions struct {
	CompiledPath   string
	AllowUntrusted bool
	TrustedRoots   []string
}
type compiledPolicy struct {
	SchemaVersion string   `json:"schema_version"`
	SourceHash    string   `json:"source_hash"`
	IRHash        string   `json:"ir_hash"`
	IR            PolicyIR `json:"ir"`
}

func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func isTrusted(path string, roots []string) bool {
	a, _ := filepath.Abs(path)
	for _, r := range roots {
		rr, _ := filepath.Abs(r)
		if rel, e := filepath.Rel(rr, a); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func LoadBundle(path string, o LoadOptions) (PolicyBundle, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PolicyBundle{}, err
	}
	trusted := isTrusted(path, o.TrustedRoots)
	if !trusted && !o.AllowUntrusted {
		return PolicyBundle{}, ErrUntrustedPolicy
	}
	sourceHash := hash(b)
	var ir PolicyIR
	irSource := ""
	block, found, err := extractPolicyJSON(string(b))
	if err != nil {
		return PolicyBundle{}, err
	}
	if found {
		if err = decodeStrict([]byte(block), &ir); err != nil {
			return PolicyBundle{}, fmt.Errorf("policy-json: %w", err)
		}
		irSource = "inline_policy_json"
	} else {
		cp := o.CompiledPath
		if cp == "" {
			cp = strings.TrimSuffix(path, filepath.Ext(path)) + ".policy.json"
		}
		cb, e := os.ReadFile(cp)
		if errors.Is(e, os.ErrNotExist) {
			ir = BuiltinEBP21IR()
			irSource = "builtin_ebp_v2_1"
		} else {
			if e != nil {
				return PolicyBundle{}, fmt.Errorf("load compiled policy sidecar: %w", e)
			}
			var c compiledPolicy
			if e = decodeStrict(cb, &c); e != nil {
				return PolicyBundle{}, e
			}
			if c.SourceHash != sourceHash {
				return PolicyBundle{}, errors.New("compiled policy source hash mismatch")
			}
			ir = c.IR
			irSource = "compiled_sidecar"
		}
	}
	if err = ValidateIR(ir); err != nil {
		return PolicyBundle{}, err
	}
	canon, _ := canonicalIR(ir)
	irHash := hash(canon)
	return PolicyBundle{ID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Version: ir.SchemaVersion, SourcePath: path, SourceHash: sourceHash, IRHash: irHash, IRSource: irSource, Markdown: string(b), IR: ir, Trusted: trusted}, nil
}

func Compile(path, out string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	block, ok, e := extractPolicyJSON(string(b))
	if e != nil {
		return e
	}
	var ir PolicyIR
	if ok {
		if e = decodeStrict([]byte(block), &ir); e != nil {
			return e
		}
	} else {
		ir = BuiltinEBP21IR()
	}
	if e = ValidateIR(ir); e != nil {
		return e
	}
	c, _ := canonicalIR(ir)
	env := compiledPolicy{SchemaVersion: SupportedSchemaVersion, SourceHash: hash(b), IRHash: hash(c), IR: ir}
	data, _ := json.MarshalIndent(env, "", "  ")
	return os.WriteFile(out, append(data, '\n'), 0644)
}
func decodeStrict(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.More() {
		return errors.New("trailing JSON")
	}
	return nil
}
func canonicalIR(ir PolicyIR) ([]byte, error) {
	sort.Slice(ir.DebtItems, func(i, j int) bool { return ir.DebtItems[i].ID < ir.DebtItems[j].ID })
	sort.Slice(ir.RubricDimensions, func(i, j int) bool { return ir.RubricDimensions[i].ID < ir.RubricDimensions[j].ID })
	return json.Marshal(ir)
}
func extractPolicyJSON(s string) (string, bool, error) {
	re := regexp.MustCompile("(?s)```policy-json\\s*(.*?)\\s*```")
	m := re.FindAllStringSubmatch(s, -1)
	if len(m) > 1 {
		return "", false, errors.New("multiple policy-json blocks")
	}
	if len(m) == 0 {
		return "", false, nil
	}
	return m[0][1], true, nil
}

func ValidateIR(ir PolicyIR) error {
	if ir.SchemaVersion != SupportedSchemaVersion {
		return fmt.Errorf("unsupported PolicyIR schema_version %q", ir.SchemaVersion)
	}
	seen := map[string]bool{}
	valid := map[DebtStatus]bool{DebtRemaining: true, DebtRetired: true, DebtNotApplicable: true, DebtNotAssessed: true}
	for _, d := range ir.DebtItems {
		if d.ID == "" || seen[d.ID] {
			return fmt.Errorf("duplicate or empty debt id %q", d.ID)
		}
		seen[d.ID] = true
		if !valid[d.DefaultStatus] {
			return fmt.Errorf("unknown debt status %q", d.DefaultStatus)
		}
	}
	w := 0.0
	for _, d := range ir.RubricDimensions {
		if d.ID == "" || d.Weight < 0 || math.IsNaN(d.Weight) || math.IsInf(d.Weight, 0) {
			return fmt.Errorf("invalid rubric dimension %q", d.ID)
		}
		w += d.Weight
	}
	if math.Abs(w-1) > 1e-9 {
		return fmt.Errorf("rubric weights sum to %g, want 1", w)
	}
	for _, h := range ir.HardFailures {
		if h.RewardCap < 0 || h.RewardCap > 1 || math.IsNaN(h.RewardCap) {
			return fmt.Errorf("invalid reward cap for %s", h.ID)
		}
		for _, p := range h.Patterns {
			if _, e := regexp.Compile(p); e != nil {
				return fmt.Errorf("hard failure %s pattern: %w", h.ID, e)
			}
		}
	}
	if len(ir.RequiredOutputs) == 0 {
		return errors.New("required outputs must be declared")
	}
	return nil
}

func LoadProfile(path string, b PolicyBundle) (EvaluationProfile, error) {
	data, e := os.ReadFile(path)
	if e != nil {
		return EvaluationProfile{}, e
	}
	var p EvaluationProfile
	if e = decodeStrict(data, &p); e != nil {
		return p, e
	}
	ids := map[string]bool{}
	for _, d := range b.IR.DebtItems {
		ids[d.ID] = true
	}
	for id, o := range p.DebtOverrides {
		if !ids[id] {
			return p, fmt.Errorf("profile references unknown debt %q", id)
		}
		if o.DefaultStatus != DebtRemaining && o.DefaultStatus != DebtRetired && o.DefaultStatus != DebtNotApplicable && o.DefaultStatus != DebtNotAssessed {
			return p, fmt.Errorf("invalid profile status %q", o.DefaultStatus)
		}
	}
	return p, nil
}

// DefaultProfile derives an in-memory automated profile from the active
// policy. Human-only debts remain not_assessed and do not block automated
// readiness, so no profile file is required for the standard CLI workflow.
func DefaultProfile(b PolicyBundle) EvaluationProfile {
	p := EvaluationProfile{
		ID:            "policy-default",
		Name:          "Policy default automated profile",
		DebtOverrides: map[string]DebtOverride{},
		StatusLabel:   b.IR.ReportLanguage.StatusLabel,
		RequiredNotes: append([]string(nil), b.IR.ReportLanguage.RequiredStatements...),
	}
	for _, d := range b.IR.DebtItems {
		if !d.Automated {
			p.ID = "automated-no-faithfulness"
			p.Name = "Automated profile excluding human-only review"
			p.DebtOverrides[d.ID] = DebtOverride{DefaultStatus: DebtNotAssessed, IncludedInAutomatedReadiness: false}
		}
	}
	return p
}
