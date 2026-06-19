package main

import (
	"context"
	"ebp-paper-evaluator/pkg/artifact"
	"ebp-paper-evaluator/pkg/budget"
	"ebp-paper-evaluator/pkg/document"
	"ebp-paper-evaluator/pkg/eval"
	"ebp-paper-evaluator/pkg/llm"
	"ebp-paper-evaluator/pkg/metadata"
	"ebp-paper-evaluator/pkg/policy"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/PithomLabs/treequest-go/pkg/algo"
	tree "github.com/PithomLabs/treequest-go/pkg/tree"
	"log"
	"os"
	"path/filepath"
)

type codec struct{}

func (codec) MarshalState(s eval.AssessmentState) ([]byte, error) { return json.Marshal(s) }
func (codec) UnmarshalState(b []byte) (eval.AssessmentState, error) {
	var s eval.AssessmentState
	e := json.Unmarshal(b, &s)
	return s, e
}
func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: ebp-paper-evaluator <run|compile-policy>")
	}
	switch os.Args[1] {
	case "compile-policy":
		compile(os.Args[2:])
	case "run":
		run(os.Args[2:])
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}
func compile(args []string) {
	f := flag.NewFlagSet("compile-policy", flag.ExitOnError)
	in := f.String("policy", "", "policy markdown")
	out := f.String("out", "", "compiled output")
	f.Parse(args)
	if *in == "" || *out == "" {
		log.Fatal("--policy and --out are required")
	}
	if e := policy.Compile(*in, *out); e != nil {
		log.Fatal(e)
	}
}
func run(args []string) {
	f := flag.NewFlagSet("run", flag.ExitOnError)
	paper := f.String("paper", "", "local .txt or .md paper")
	inputRoot := f.String("input-root", "", "optional paper containment root")
	maxPaperBytes := f.Int64("max-paper-bytes", document.DefaultMaxPaperBytes, "maximum local paper size")
	includeLocalPaths := f.Bool("include-local-paths", false, "include resolved absolute source path in artifacts")
	copySource := f.Bool("copy-source", false, "copy immutable original source into the artifact bundle")
	pol := f.String("policy", "policies/ebp_v2_1.md", "policy markdown")
	profPath := f.String("profile", "policies/profiles/automated-no-faithfulness.json", "profile path or ID")
	allow := f.Bool("allow-untrusted-policy", false, "execute a policy outside trusted roots")
	claims := f.String("claim-file", "", "debug claim override")
	provider := f.String("provider", "openrouter", "provider")
	ma := f.String("worker-a-model", "worker-a", "model")
	mb := f.String("worker-b-model", "worker-b", "model")
	me := f.String("evaluator-model", "evaluator", "model")
	out := f.String("out", "out/audit", "output")
	iters := f.Int("iterations-per-claim", 5, "search iterations per claim")
	mock := f.Bool("mock", false, "deterministic mock run")
	f.Parse(args)
	if *paper == "" {
		log.Fatal("--paper is required")
	}
	ctx := context.Background()
	doc, e := (document.LocalTextIngestor{}).Ingest(ctx, document.PaperInput{Path: *paper, InputRoot: *inputRoot, MaxBytes: *maxPaperBytes, IncludeResolvedPath: *includeLocalPaths})
	if e != nil {
		log.Fatal(e)
	}
	trusted, _ := filepath.Abs("policies")
	trustedFixtures, _ := filepath.Abs(filepath.Join("testdata", "policies"))
	bundle, e := policy.LoadBundle(*pol, policy.LoadOptions{AllowUntrusted: *allow, TrustedRoots: []string{trusted, trustedFixtures}})
	if e != nil {
		log.Fatal(e)
	}
	if filepath.Base(*profPath) == *profPath {
		*profPath = filepath.Join("policies", "profiles", *profPath+".json")
	}
	profile, e := policy.LoadProfile(*profPath, bundle)
	profileExplicit := false
	f.Visit(func(x *flag.Flag) {
		if x.Name == "profile" {
			profileExplicit = true
		}
	})
	if e != nil && profileExplicit {
		log.Fatal(e)
	}
	if e != nil {
		profile = policy.EvaluationProfile{ID: "policy-default", Name: "Policy default profile", StatusLabel: bundle.IR.ReportLanguage.StatusLabel, RequiredNotes: bundle.IR.ReportLanguage.RequiredStatements}
		e = nil
	}
	var base llm.LLMClient
	if *mock {
		base = mockClient(doc, bundle)
	} else {
		if *provider != "openrouter" {
			log.Fatalf("unsupported provider %q", *provider)
		}
		base, e = llm.NewOpenRouterClient("OPENROUTER_API_KEY")
		if e != nil {
			log.Fatal(e)
		}
	}
	bt := budget.New()
	aClient := budget.Client{Inner: base, Tracker: bt, Role: "worker_a"}
	bClient := budget.Client{Inner: base, Tracker: bt, Role: "worker_b"}
	eClient := budget.Client{Inner: base, Tracker: bt, Role: "evaluator"}
	roles := metadata.RoleClients{WorkerA: aClient, WorkerB: bClient, Evaluator: eClient}
	var pp metadata.PaperProfile
	if *claims != "" {
		b, e := os.ReadFile(*claims)
		if e != nil {
			log.Fatal(e)
		}
		if e = json.Unmarshal(b, &pp); e != nil {
			log.Fatal(e)
		}
		if e = metadata.VerifyProfile(doc, pp); e != nil {
			log.Fatal(e)
		}
	} else {
		pp, e = (metadata.ConsensusV1{Models: map[string]string{"metadata_a": *ma, "metadata_b": *mb, "metadata_e": *me}}).BuildPaperProfile(ctx, doc, bundle, roles)
		if e != nil {
			log.Fatal(e)
		}
	}
	arts := map[string]artifact.ClaimArtifact{}
	stops := map[string]string{}
	for _, cl := range pp.Claims {
		root := eval.InitialState(doc, bundle, profile, cl)
		search := algo.NewABMCTSA[eval.AssessmentState](algo.NewBetaSampler(42), codec{})
		state, e := search.InitTreeWithState(ctx, root, []tree.ActionLabel{"constructive", "adversarial"})
		if e != nil {
			log.Fatal(e)
		}
		gens := eval.NewGenerators(eval.GeneratorConfig{Document: doc, Policy: bundle, Profile: profile, Clients: eval.RoleClients{WorkerA: aClient, WorkerB: bClient, Evaluator: eClient}, Models: eval.RoleModels{WorkerA: *ma, WorkerB: *mb, Evaluator: *me}, MaxTokens: 3000})
		stop := "iterations_exhausted"
		for i := 0; i < *iters; i++ {
			node, next, er := search.Step(ctx, state, []tree.ActionLabel{"constructive", "adversarial"}, gens)
			if er != nil {
				log.Fatal(er)
			}
			state = next
			if node.State.AutomatedReviewReady {
				stop = "automated_review_ready"
				break
			}
		}
		best := state.Tree.BestNode()
		assessment := root
		if best != nil {
			assessment = best.State
		}
		snap, e := state.Tree.SaveSnapshot(codec{})
		if e != nil {
			log.Fatal(e)
		}
		arts[cl.ID] = artifact.ClaimArtifact{Assessment: assessment, TreeSnapshot: snap}
		stops[cl.ID] = stop
	}
	usage := bt.Snapshot()
	hashes := make([]string, 0, len(usage.PromptRecords))
	for _, r := range usage.PromptRecords {
		hashes = append(hashes, r.PromptHash)
	}
	prov := artifact.Provenance{DocumentHash: doc.Hash, PolicySourceHash: bundle.SourceHash, PolicyIRHash: bundle.IRHash, ProfileID: profile.ID, Models: map[string]string{"worker_a": *ma, "worker_b": *mb, "evaluator": *me}, StopReasons: stops, PromptHashes: hashes}
	if e = artifact.Save(*out, doc, bundle, profile, pp, arts, usage, prov); e != nil {
		log.Fatal(e)
	}
	if *copySource {
		copyPath := *paper
		if *inputRoot != "" && !filepath.IsAbs(copyPath) {
			copyPath = filepath.Join(*inputRoot, copyPath)
		}
		if e = artifact.CopyLocalSource(copyPath, *out); e != nil {
			log.Fatal(e)
		}
	}
	fmt.Println(*out)
}
func mockClient(d document.DocumentBundle, p policy.PolicyBundle) llm.LLMClient {
	return &llm.MockLLMClient{CustomMockFn: func(r llm.GenerateRequest) (llm.GenerateResponse, error) {
		content := "Candidate analysis with explicit uncertainty and source references."
		if len(d.Sections) == 0 {
			return llm.GenerateResponse{}, fmt.Errorf("empty document")
		}
		sec := d.Sections[0]
		end := len(sec.Text)
		if end > 160 {
			end = 160
		}
		span := document.EvidenceSpan{ID: "span-1", Section: sec.ID, Start: 0, End: end, Quote: sec.Text[:end], SourceHash: d.Hash}
		switch r.Role + "/" + r.Task {
		case "metadata_a/metadata_extract", "metadata_b/metadata_critique", "metadata_e/metadata_resolve":
			env := map[string]any{"claims": []metadata.ClaimRecord{{Text: span.Quote, ClaimType: firstEnum(p.IR.ClaimTypes), FunctionClass: firstEnum(p.IR.FunctionClasses), MaturityStage: firstEnum(p.IR.MaturityStages), EvidenceSpans: []document.EvidenceSpan{span}, Confidence: .8}}}
			b, _ := json.Marshal(env)
			content = string(b)
		case "evaluator/evaluator_judgment":
			dims := map[string]float64{}
			for _, x := range p.IR.RubricDimensions {
				dims[x.ID] = .7
			}
			debts := map[string]eval.DebtDecision{}
			for _, x := range p.IR.DebtItems {
				status := policy.DebtRetired
				if !x.Automated {
					status = policy.DebtNotAssessed
				}
				debts[x.ID] = eval.DebtDecision{ItemID: x.ID, Status: status, Rationale: "mock", EvidenceSpanIDs: []string{span.ID}, Confidence: .7}
			}
			b, _ := json.Marshal(eval.EvaluatorJudgment{DimensionScores: dims, DebtDecisions: debts, Rationale: "Automated mock judgment."})
			content = string(b)
		case "worker_a/worker_constructive", "worker_b/worker_adversarial":
		default:
			return llm.GenerateResponse{}, fmt.Errorf("unsupported mock role/task %s/%s", r.Role, r.Task)
		}
		return llm.GenerateResponse{Content: content, ModelID: r.Model, Usage: llm.TokenUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20}, FinishReason: "stop", PromptHash: "mock-" + r.Role + "-" + r.Task}, nil
	}}
}
func firstEnum(x []policy.PolicyEnum) string {
	if len(x) == 0 {
		return ""
	}
	return x[0].ID
}
